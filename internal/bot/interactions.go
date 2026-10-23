package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
	"github.com/moixblau/HappyX.Bot/internal/i18n"
	"github.com/moixblau/HappyX.Bot/internal/schedule"
)

// Command and option names are the English ones; Discord shows the localized names from the
// catalogs but always sends these back.
const (
	cmdStatus = "status"
	cmdPause  = "pause"
	cmdResume = "resume"
	cmdReport = "report"
	cmdConfig = "config"
	cmdKudos  = "kudos"

	optAnonymous = "anonymous"
	optHour      = "hour"
	optTimezone  = "timezone"
	optFrequency = "frequency"
	optAdmin     = "admin"
	optLanguage  = "language"
	optPeriod    = "period"
	optUser      = "user"
	optMessage   = "message"
	optKudosChan = "kudos_channel"
	optWorkDays  = "working_days"

	periodLast    = "last"
	periodCurrent = "current"
)

func commands() []*discordgo.ApplicationCommand {
	no := false
	minHour := 0.0
	one := 1
	en := i18n.Get(i18n.Default)

	sub := func(name string, opts ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:                     discordgo.ApplicationCommandOptionSubCommand,
			Name:                     name,
			NameLocalizations:        *localized("cmd." + name + ".name"),
			Description:              en.T("cmd." + name + ".description"),
			DescriptionLocalizations: *localized("cmd." + name + ".description"),
			Options:                  opts,
		}
	}
	opt := func(t discordgo.ApplicationCommandOptionType, name string) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:                     t,
			Name:                     name,
			NameLocalizations:        *localized("opt." + name + ".name"),
			Description:              en.T("opt." + name + ".description"),
			DescriptionLocalizations: *localized("opt." + name + ".description"),
		}
	}

	hour := opt(discordgo.ApplicationCommandOptionInteger, optHour)
	hour.MinValue, hour.MaxValue = &minHour, 23
	frequency := opt(discordgo.ApplicationCommandOptionString, optFrequency)
	for _, f := range []string{"weekly", "monthly"} {
		frequency.Choices = append(frequency.Choices, &discordgo.ApplicationCommandOptionChoice{
			Name: en.T("freq." + f), NameLocalizations: *localized("freq." + f), Value: f,
		})
	}
	period := opt(discordgo.ApplicationCommandOptionString, optPeriod)
	for _, p := range []string{periodLast, periodCurrent} {
		period.Choices = append(period.Choices, &discordgo.ApplicationCommandOptionChoice{
			Name: en.T("period." + p), NameLocalizations: *localized("period." + p), Value: p,
		})
	}
	kudosUser := opt(discordgo.ApplicationCommandOptionUser, optUser)
	kudosUser.Required = true
	kudosMessage := opt(discordgo.ApplicationCommandOptionString, optMessage)
	kudosMessage.Required, kudosMessage.MinLength, kudosMessage.MaxLength = true, &one, kudosMaxLength
	kudosChannel := opt(discordgo.ApplicationCommandOptionChannel, optKudosChan)
	kudosChannel.ChannelTypes = []discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews}
	workingDays := opt(discordgo.ApplicationCommandOptionString, optWorkDays)
	for _, p := range workingDayPresets {
		workingDays.Choices = append(workingDays.Choices, &discordgo.ApplicationCommandOptionChoice{
			Name: en.T("days." + p.key), NameLocalizations: *localized("days." + p.key), Value: p.key,
		})
	}
	language := opt(discordgo.ApplicationCommandOptionString, optLanguage)
	for _, code := range i18n.Codes() {
		language.Choices = append(language.Choices, &discordgo.ApplicationCommandOptionChoice{
			Name: i18n.Get(code).Name, Value: code,
		})
	}

	return []*discordgo.ApplicationCommand{{
		Name:                     "happyx",
		Description:              en.T("cmd.description"),
		DescriptionLocalizations: localized("cmd.description"),
		DMPermission:             &no,
		Options: []*discordgo.ApplicationCommandOption{
			sub(cmdStatus),
			sub(cmdPause),
			sub(cmdResume),
			sub(cmdReport, period),
			sub(cmdKudos, kudosUser, kudosMessage),
			sub(cmdConfig,
				opt(discordgo.ApplicationCommandOptionBoolean, optAnonymous),
				hour,
				opt(discordgo.ApplicationCommandOptionString, optTimezone),
				frequency,
				language,
				opt(discordgo.ApplicationCommandOptionUser, optAdmin),
				kudosChannel,
				workingDays,
			),
		},
	}}
}

// localized builds Discord's per-locale map for key from every catalog that claims a Discord
// locale and differs from English.
func localized(key string) *map[discordgo.Locale]string {
	en := i18n.Get(i18n.Default).T(key)
	m := map[discordgo.Locale]string{}
	for _, code := range i18n.Codes() {
		l := i18n.Get(code)
		if !l.Has(key) || l.T(key) == en {
			continue
		}
		for _, d := range l.Discord {
			m[discordgo.Locale(d)] = l.T(key)
		}
	}
	return &m
}

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(ctx, i)
	case discordgo.InteractionMessageComponent:
		b.handleSurveyButton(ctx, i)
	case discordgo.InteractionModalSubmit:
		b.handleNoteModal(ctx, i)
	}
}

func interactionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User
	}
	return i.User
}

// guildLocale is the language configured for the guild, English if it can't be read.
func (b *Bot) guildLocale(ctx context.Context, guildID string) *i18n.Locale {
	g, err := b.api.Guild(ctx, guildID)
	if err != nil {
		return i18n.Get(i18n.Default)
	}
	return i18n.Get(g.Language)
}

// userLocale answers slash commands in the user's Discord language when the bot has it, and
// in the guild's language otherwise.
func (b *Bot) userLocale(ctx context.Context, i *discordgo.InteractionCreate) *i18n.Locale {
	if l, ok := i18n.FromDiscord(string(i.Locale)); ok {
		return l
	}
	return b.guildLocale(ctx, i.GuildID)
}

// --- Survey answers (in DMs, in the guild's language like the survey itself) ---

func (b *Bot) handleSurveyButton(ctx context.Context, i *discordgo.InteractionCreate) {
	id, ok := parseSurveyID(i.MessageComponentData().CustomID)
	if !ok {
		return
	}
	l := b.guildLocale(ctx, id.GuildID)
	user := interactionUser(i)
	switch id.Action {
	case actionMood:
		if err := b.api.RecordMood(ctx, id.GuildID, user.ID, id.Day, id.Mood, ""); err != nil {
			b.replyError(i, l, err)
			return
		}
		b.respond(i, discordgo.InteractionResponseUpdateMessage, answeredMessage(l, id, ""))
	case actionNote:
		b.respond(i, discordgo.InteractionResponseModal, noteModal(l, id))
	case actionStop:
		err := b.api.RemoveMember(ctx, id.GuildID, user.ID)
		if err != nil && !errors.Is(err, api.ErrNotFound) {
			b.replyError(i, l, err)
			return
		}
		b.respond(i, discordgo.InteractionResponseUpdateMessage, &discordgo.InteractionResponseData{
			Content:    l.T("stop.done", "resume_cmd", b.resumeMention()),
			Components: []discordgo.MessageComponent{},
		})
	}
}

func (b *Bot) handleNoteModal(ctx context.Context, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	id, ok := parseSurveyID(data.CustomID)
	if !ok || id.Action != actionNoteModal {
		return
	}
	l := b.guildLocale(ctx, id.GuildID)
	comment := modalValue(data, "comment")
	if err := b.api.RecordMood(ctx, id.GuildID, interactionUser(i).ID, id.Day, id.Mood, comment); err != nil {
		b.replyError(i, l, err)
		return
	}
	b.respond(i, discordgo.InteractionResponseUpdateMessage, answeredMessage(l, id, comment))
}

func modalValue(data discordgo.ModalSubmitInteractionData, field string) string {
	for _, c := range data.Components {
		row, ok := c.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, rc := range row.Components {
			if in, ok := rc.(*discordgo.TextInput); ok && in.CustomID == field {
				return in.Value
			}
		}
	}
	return ""
}

// --- Slash commands (in a server) ---

func (b *Bot) handleCommand(ctx context.Context, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "happyx" || len(data.Options) == 0 || i.GuildID == "" {
		return
	}
	sub := data.Options[0]
	user := interactionUser(i)
	l := b.userLocale(ctx, i)
	switch sub.Name {
	case cmdStatus:
		b.cmdStatus(ctx, i, l, user)
	case cmdPause:
		err := b.api.RemoveMember(ctx, i.GuildID, user.ID)
		if err != nil && !errors.Is(err, api.ErrNotFound) {
			b.replyError(i, l, err)
			return
		}
		b.ephemeral(i, l.T("pause.done", "resume_cmd", b.resumeMention()))
	case cmdResume:
		yes := true
		if err := b.api.UpsertMember(ctx, i.GuildID, user.ID, user.Username, &yes); err != nil {
			b.replyError(i, l, err)
			return
		}
		b.ephemeral(i, l.T("resume.done"))
	case cmdConfig:
		b.cmdConfig(ctx, i, l, user, sub.Options)
	case cmdReport:
		b.cmdReport(ctx, i, l, user, sub.Options)
	case cmdKudos:
		b.cmdKudos(ctx, i, l, user, sub.Options)
	}
}

func (b *Bot) cmdStatus(ctx context.Context, i *discordgo.InteractionCreate, l *i18n.Locale, user *discordgo.User) {
	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, l, err)
		return
	}
	members, err := b.api.Members(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, l, err)
		return
	}
	you := l.T("status.paused")
	for _, m := range members {
		if m.DiscordID == user.ID {
			you = l.T("status.receiving")
		}
	}
	b.ephemeral(i, l.T("status.text",
		"days", workingDaysText(l, g.WorkingDays), "hour", fmt.Sprintf("%02d", g.SurveyHour), "timezone", g.Timezone,
		"frequency", l.T("freq."+g.ReportFrequency), "admin", "<@"+g.AdminDiscordID+">",
		"anonymous", yesNo(l, g.Anonymous), "language", i18n.Get(g.Language).Name,
		"participants", len(members), "you", you)+"\n"+kudosChannelLine(l, g))
}

func (b *Bot) cmdConfig(ctx context.Context, i *discordgo.InteractionCreate, l *i18n.Locale, user *discordgo.User, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if !b.isAdmin(ctx, i, l, user) {
		return
	}
	var p api.GuildPatch
	for _, o := range opts {
		switch o.Name {
		case optAnonymous:
			v := o.BoolValue()
			p.Anonymous = &v
		case optHour:
			v := int(o.IntValue())
			p.SurveyHour = &v
		case optTimezone:
			v := o.StringValue()
			p.Timezone = &v
		case optFrequency:
			v := o.StringValue()
			p.ReportFrequency = &v
		case optLanguage:
			v := o.StringValue()
			p.Language = &v
		case optAdmin:
			v := o.UserValue(nil).ID
			p.AdminDiscordID = &v
		case optKudosChan:
			v := o.ChannelValue(nil).ID
			p.KudosChannelID = &v
		case optWorkDays:
			p.WorkingDays = presetDays(o.StringValue())
		}
	}
	g, err := b.api.UpdateGuild(ctx, i.GuildID, p)
	if err != nil {
		b.replyError(i, l, err)
		return
	}
	if p.Language != nil {
		l = i18n.Get(g.Language) // confirm in the language just chosen
	}
	b.ephemeral(i, configSaved(l, g, p))
}

// configSaved confirms the new settings. When the change turned anonymity off, it adds that
// earlier answers stay anonymous.
func configSaved(l *i18n.Locale, g api.Guild, p api.GuildPatch) string {
	s := l.T("config.saved",
		"days", workingDaysText(l, g.WorkingDays), "hour", fmt.Sprintf("%02d", g.SurveyHour), "timezone", g.Timezone,
		"frequency", l.T("freq."+g.ReportFrequency), "admin", "<@"+g.AdminDiscordID+">",
		"anonymous", yesNo(l, g.Anonymous), "language", i18n.Get(g.Language).Name) + "\n" + kudosChannelLine(l, g)
	if p.Anonymous != nil && !*p.Anonymous {
		s += "\n" + l.T("config.now_public")
	}
	return s
}

func (b *Bot) cmdReport(ctx context.Context, i *discordgo.InteractionCreate, l *i18n.Locale, user *discordgo.User, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if !b.isAdmin(ctx, i, l, user) {
		return
	}
	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, l, err)
		return
	}
	from, to := "", "" // the API defaults to the last completed period
	for _, o := range opts {
		if o.Name == optPeriod && o.StringValue() == periodCurrent {
			from, to = schedule.CurrentPeriod(schedule.Settings{Timezone: g.Timezone, ReportFrequency: g.ReportFrequency}, b.now())
		}
	}
	r, err := b.api.Report(ctx, i.GuildID, from, to)
	if err != nil {
		b.replyError(i, l, err)
		return
	}
	msg := reportMessage(l, g, r)
	b.respond(i, discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: msg.Embeds, Files: msg.Files, Flags: discordgo.MessageFlagsEphemeral,
	})
}

// isAdmin checks the caller is the guild's HappyX admin, and answers them if not.
func (b *Bot) isAdmin(ctx context.Context, i *discordgo.InteractionCreate, l *i18n.Locale, user *discordgo.User) bool {
	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, l, err)
		return false
	}
	if g.AdminDiscordID != user.ID {
		b.ephemeral(i, l.T("admin.only", "admin", "<@"+g.AdminDiscordID+">"))
		return false
	}
	return true
}

// resumeMention is a clickable /happyx resume, which Discord shows in each user's language.
func (b *Bot) resumeMention() string {
	if id := b.commandID.Load(); id != nil {
		return fmt.Sprintf("</happyx %s:%s>", cmdResume, *id)
	}
	return "`/happyx " + cmdResume + "`"
}

// --- Responses ---

func (b *Bot) respond(i *discordgo.InteractionCreate, t discordgo.InteractionResponseType, data *discordgo.InteractionResponseData) {
	if err := b.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: t, Data: data}); err != nil {
		b.log.Error("interaction respond", "err", err)
	}
}

func (b *Bot) ephemeral(i *discordgo.InteractionCreate, content string) {
	b.respond(i, discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Content: content, Flags: discordgo.MessageFlagsEphemeral,
		AllowedMentions: &discordgo.MessageAllowedMentions{}, // show mentions without pinging
	})
}

func (b *Bot) replyError(i *discordgo.InteractionCreate, l *i18n.Locale, err error) {
	var apiErr *api.Error
	msg := l.T("error.generic")
	switch {
	case errors.Is(err, api.ErrForbidden):
		msg = l.T("error.not_member", "resume_cmd", b.resumeMention())
	case errors.Is(err, api.ErrConflict):
		msg = l.T("error.inactive")
	case errors.As(err, &apiErr) && apiErr.Status == 400:
		msg = l.T("error.invalid", "detail", apiErr.Message)
	default:
		b.log.Error("interaction failed", "err", err)
	}
	b.ephemeral(i, msg)
}

func yesNo(l *i18n.Locale, v bool) string {
	if v {
		return l.T("yes")
	}
	return l.T("no")
}
