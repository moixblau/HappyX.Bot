package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/happyx-app/HappyX.Bot/internal/api"
	"github.com/happyx-app/HappyX.Bot/internal/i18n"
)

const (
	kudosMaxLength = 300      // the API's limit
	kudosColor     = 0xfbbf24 // HappyX's yellow
)

// kudosPerson is someone on a kudos card: their mention, name and avatar.
type kudosPerson struct {
	ID, Name, Avatar string
}

// personOf takes the name the server shows (nickname, else display name, else username).
func personOf(u *discordgo.User, m *discordgo.Member) kudosPerson {
	name := u.GlobalName
	if name == "" {
		name = u.Username
	}
	if m != nil && m.Nick != "" {
		name = m.Nick
	}
	return kudosPerson{ID: u.ID, Name: name, Avatar: u.AvatarURL("256")}
}

// cmdKudos posts a kudos card in the guild's kudos channel (or here, when there is none) and
// tells the receiver by DM. The card and the DM are in the guild's language; the replies to the
// giver are in theirs.
func (b *Bot) cmdKudos(ctx context.Context, i *discordgo.InteractionCreate, l *i18n.Locale, giver *discordgo.User, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	var toID, message string
	for _, o := range opts {
		switch o.Name {
		case optUser:
			toID = o.UserValue(nil).ID
		case optMessage:
			message = strings.TrimSpace(o.StringValue())
		}
	}
	resolved := i.ApplicationCommandData().Resolved
	var to *discordgo.User
	if resolved != nil {
		to = resolved.Users[toID]
	}
	switch {
	case to == nil || message == "":
		return
	case to.Bot:
		b.ephemeral(i, l.T("kudos.no_bots"))
		return
	case to.ID == giver.ID:
		b.ephemeral(i, l.T("kudos.no_self"))
		return
	case resolved.Members[to.ID] == nil: // Discord only resolves members of this guild
		b.ephemeral(i, l.T("kudos.not_in_server", "user", "<@"+to.ID+">"))
		return
	}

	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, l, err)
		return
	}
	// Either may be missing if they joined while the bot was offline. nil keeps their survey
	// opt-in as it is.
	for _, u := range []*discordgo.User{giver, to} {
		if err := b.api.UpsertMember(ctx, i.GuildID, u.ID, u.Username, nil); err != nil {
			b.replyError(i, l, err)
			return
		}
	}
	if _, err := b.api.GiveKudos(ctx, i.GuildID, giver.ID, to.ID, message); err != nil {
		if errors.Is(err, api.ErrTooMany) {
			b.ephemeral(i, l.T("kudos.too_many"))
			return
		}
		b.replyError(i, l, err)
		return
	}

	gl := i18n.Get(g.Language)
	from := personOf(giver, i.Member)
	card := kudosCard(gl, from, personOf(to, resolved.Members[to.ID]), message)
	posted := b.postKudos(i, l, g, card, to.ID)
	if posted == nil {
		return
	}
	link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, posted.ChannelID, posted.ID)
	channel := ""
	if c, err := b.dg.State.Channel(posted.ChannelID); err == nil {
		channel = c.Name
	}
	if err := b.dm(to.ID, kudosDM(gl, g, from, message, link, channel)); err != nil {
		b.log.Warn("kudos DM failed", "guild", i.GuildID, "user", to.ID, "err", err)
	}
}

// postKudos sends the card to the kudos channel and confirms privately to the giver, or answers
// the command with the card when there is no kudos channel or the bot can't post there. It
// returns the card's message, or nil if it couldn't be posted.
func (b *Bot) postKudos(i *discordgo.InteractionCreate, l *i18n.Locale, g api.Guild, card *discordgo.MessageSend, toID string) *discordgo.Message {
	if g.KudosChannelID != "" && g.KudosChannelID != i.ChannelID {
		m, err := b.dg.ChannelMessageSendComplex(g.KudosChannelID, card)
		if err == nil {
			b.respond(i, discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
				Embeds:          []*discordgo.MessageEmbed{kudosSent(l, toID, g.KudosChannelID)},
				Flags:           discordgo.MessageFlagsEphemeral,
				AllowedMentions: &discordgo.MessageAllowedMentions{},
			})
			return m
		}
		// Deleted channel or missing permissions: better here than lost.
		b.log.Warn("kudos channel post failed", "guild", g.DiscordID, "channel", g.KudosChannelID, "err", err)
	}
	b.respond(i, discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: card.Embeds, AllowedMentions: card.AllowedMentions,
	})
	m, err := b.dg.InteractionResponse(i.Interaction)
	if err != nil {
		b.log.Warn("read kudos response", "guild", g.DiscordID, "err", err)
		return nil
	}
	return m
}

// kudosCard celebrates the receiver, with their avatar, the message, who gave it and a hint
// on how to give kudos.
func kudosCard(l *i18n.Locale, from, to kudosPerson, message string) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title:       l.T("kudos.title", "name", to.Name),
			Description: quote(message),
			Color:       kudosColor,
			Thumbnail:   thumbnail(to.Avatar),
			Fields: []*discordgo.MessageEmbedField{
				{Name: l.T("kudos.from"), Value: "<@" + from.ID + ">", Inline: true},
				{Name: l.T("kudos.to"), Value: "<@" + to.ID + ">", Inline: true},
			},
			Footer: &discordgo.MessageEmbedFooter{Text: l.T("kudos.hint", "cmd", "/happyx "+l.T("cmd.kudos.name"))},
		}},
		AllowedMentions: &discordgo.MessageAllowedMentions{}, // the receiver gets a DM instead
	}
}

// kudosDM tells the receiver, with the giver's avatar and a link to the card in channel (its
// name, or empty if unknown).
func kudosDM(l *i18n.Locale, g api.Guild, from kudosPerson, message, link, channel string) *discordgo.MessageSend {
	see := l.T("kudos.see")
	if channel != "" {
		see = l.T("kudos.see_in", "channel", channel)
	}
	return &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title:       l.T("kudos.dm_title"),
			Description: quote(message),
			Color:       kudosColor,
			Thumbnail:   thumbnail(from.Avatar),
			Fields: []*discordgo.MessageEmbedField{
				{Name: l.T("kudos.from"), Value: "<@" + from.ID + ">", Inline: true},
				{Name: l.T("kudos.server"), Value: g.Name, Inline: true},
			},
		}},
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: see, Style: discordgo.LinkButton, URL: link},
		}}},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

// kudosSent confirms to the giver where the card went.
func kudosSent(l *i18n.Locale, toID, channelID string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       l.T("kudos.sent_title"),
		Description: l.T("kudos.sent", "to", "<@"+toID+">", "channel", "<#"+channelID+">"),
		Color:       kudosColor,
	}
}

func thumbnail(url string) *discordgo.MessageEmbedThumbnail {
	if url == "" {
		return nil
	}
	return &discordgo.MessageEmbedThumbnail{URL: url}
}

// kudosChannelLine describes where kudos go, for /happyx status and config.
func kudosChannelLine(l *i18n.Locale, g api.Guild) string {
	if g.KudosChannelID == "" {
		return l.T("kudos.channel_here")
	}
	return l.T("kudos.channel", "channel", "<#"+g.KudosChannelID+">")
}

func quote(s string) string {
	return "> " + strings.ReplaceAll(s, "\n", "\n> ")
}
