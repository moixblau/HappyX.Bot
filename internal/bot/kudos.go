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
	kudosMaxLength = 300 // the API's limit
	kudosColor     = 0xf5a524
)

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
	if to == nil || message == "" {
		return
	}
	// Discord only resolves members of this guild.
	if refusal := kudosRefusal(l, giver, to, resolved.Members[to.ID] != nil); refusal != "" {
		b.ephemeral(i, refusal)
		return
	}
	b.giveKudos(ctx, i, l, giver, to, message, nil)
}

// cmdKudosMenu answers "Give kudos" in the right-click Apps menu of a user or a message (the
// kudos goes to its author) with a modal for the kudos' text.
func (b *Bot) cmdKudosMenu(i *discordgo.InteractionCreate, l *i18n.Locale, giver *discordgo.User) {
	data := i.ApplicationCommandData()
	if data.Resolved == nil {
		return
	}
	id := kudosID{To: data.TargetID}
	var to *discordgo.User
	inServer := false
	switch data.CommandType {
	case discordgo.UserApplicationCommand:
		to = data.Resolved.Users[data.TargetID]
		inServer = data.Resolved.Members[data.TargetID] != nil
	case discordgo.MessageApplicationCommand:
		m := data.Resolved.Messages[data.TargetID]
		if m == nil || m.Author == nil {
			return
		}
		to, id = m.Author, kudosID{To: m.Author.ID, ChannelID: i.ChannelID, MessageID: m.ID}
		if m.WebhookID != "" {
			to = &discordgo.User{ID: m.Author.ID, Bot: true}
		}
		if !to.Bot { // messages don't come with their author's membership
			_, err := b.dg.GuildMember(i.GuildID, to.ID)
			inServer = err == nil
		}
	}
	if to == nil {
		return
	}
	if refusal := kudosRefusal(l, giver, to, inServer); refusal != "" {
		b.ephemeral(i, refusal)
		return
	}
	b.respond(i, discordgo.InteractionResponseModal, kudosModal(l, id, to))
}

// handleKudosModal gives the kudos written in the modal opened by cmdKudosMenu.
func (b *Bot) handleKudosModal(ctx context.Context, i *discordgo.InteractionCreate, id kudosID, data discordgo.ModalSubmitInteractionData) {
	l := b.userLocale(ctx, i)
	giver := interactionUser(i)
	message := strings.TrimSpace(modalValue(data, "message"))
	if message == "" || i.GuildID == "" {
		return
	}
	// The receiver may have left while the modal was open.
	m, err := b.dg.GuildMember(i.GuildID, id.To)
	if err != nil || m.User == nil {
		b.ephemeral(i, l.T("kudos.not_in_server", "user", "<@"+id.To+">"))
		return
	}
	if refusal := kudosRefusal(l, giver, m.User, true); refusal != "" {
		b.ephemeral(i, refusal)
		return
	}
	var origin *kudosOrigin
	if id.MessageID != "" {
		origin = &kudosOrigin{ChannelID: id.ChannelID, Link: messageLink(i.GuildID, id.ChannelID, id.MessageID)}
	}
	b.giveKudos(ctx, i, l, giver, m.User, message, origin)
}

// kudosRefusal is why giver can't give kudos to to, or "" if they can.
func kudosRefusal(l *i18n.Locale, giver, to *discordgo.User, inServer bool) string {
	switch {
	case to.Bot:
		return l.T("kudos.no_bots")
	case to.ID == giver.ID:
		return l.T("kudos.no_self")
	case !inServer:
		return l.T("kudos.not_in_server", "user", "<@"+to.ID+">")
	}
	return ""
}

// giveKudos records the kudos, posts its card and DMs the receiver. origin is the message it
// was given from, if any.
func (b *Bot) giveKudos(ctx context.Context, i *discordgo.InteractionCreate, l *i18n.Locale, giver, to *discordgo.User, message string, origin *kudosOrigin) {
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
	card := kudosCard(gl, giver.ID, to.ID, message, origin)
	posted := b.postKudos(i, l, g, card, to.ID)
	if posted == nil {
		return
	}
	link := messageLink(i.GuildID, posted.ChannelID, posted.ID)
	if err := b.dm(to.ID, kudosDM(gl, g, giver.ID, message, link)); err != nil {
		b.log.Warn("kudos DM failed", "guild", i.GuildID, "user", to.ID, "err", err)
	}
}

// kudosOrigin is the message a kudos was given from with the right-click menu.
type kudosOrigin struct {
	ChannelID string
	Link      string
}

func messageLink(guildID, channelID, messageID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

// kudosModal asks for the kudos' text, required and up to the API's limit like the slash
// command's option.
func kudosModal(l *i18n.Locale, id kudosID, to *discordgo.User) *discordgo.InteractionResponseData {
	name := to.GlobalName
	if name == "" {
		name = to.Username
	}
	title := []rune(l.T("kudos.modal_title", "user", name))
	if len(title) > 45 { // Discord's limit
		title = append(title[:44], '…')
	}
	return &discordgo.InteractionResponseData{
		CustomID: id.String(),
		Title:    string(title),
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
			CustomID:    "message",
			Label:       l.T("kudos.modal_label"),
			Style:       discordgo.TextInputParagraph,
			Required:    true,
			MinLength:   1,
			MaxLength:   kudosMaxLength,
			Placeholder: l.T("kudos.modal_placeholder"),
		}}}},
	}
}

// postKudos sends the card to the kudos channel and confirms privately to the giver, or answers
// the command with the card when there is no kudos channel or the bot can't post there. It
// returns the card's message, or nil if it couldn't be posted.
func (b *Bot) postKudos(i *discordgo.InteractionCreate, l *i18n.Locale, g api.Guild, card *discordgo.MessageSend, toID string) *discordgo.Message {
	if g.KudosChannelID != "" && g.KudosChannelID != i.ChannelID {
		m, err := b.dg.ChannelMessageSendComplex(g.KudosChannelID, card)
		if err == nil {
			b.ephemeral(i, l.T("kudos.sent", "to", "<@"+toID+">", "channel", "<#"+g.KudosChannelID+">"))
			return m
		}
		// Deleted channel or missing permissions: better here than lost.
		b.log.Warn("kudos channel post failed", "guild", g.DiscordID, "channel", g.KudosChannelID, "err", err)
	}
	b.respond(i, discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: card.Embeds, Components: card.Components, AllowedMentions: card.AllowedMentions,
	})
	m, err := b.dg.InteractionResponse(i.Interaction)
	if err != nil {
		b.log.Warn("read kudos response", "guild", g.DiscordID, "err", err)
		return nil
	}
	return m
}

// kudosCard is the public card. A kudos given from a message also says where it was and links
// to it.
func kudosCard(l *i18n.Locale, fromID, toID, message string, origin *kudosOrigin) *discordgo.MessageSend {
	desc := l.T("kudos.card", "from", "<@"+fromID+">", "to", "<@"+toID+">") + "\n\n" + quote(message)
	card := &discordgo.MessageSend{AllowedMentions: &discordgo.MessageAllowedMentions{}} // the receiver gets a DM instead
	if origin != nil {
		desc += "\n-# " + l.T("kudos.from_message", "channel", "<#"+origin.ChannelID+">")
		card.Components = []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: l.T("kudos.see_message"), Style: discordgo.LinkButton, URL: origin.Link},
		}}}
	}
	card.Embeds = []*discordgo.MessageEmbed{{Title: l.T("kudos.title"), Description: desc, Color: kudosColor}}
	return card
}

func kudosDM(l *i18n.Locale, g api.Guild, fromID, message, link string) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Content: l.T("kudos.dm", "from", "<@"+fromID+">", "guild", g.Name) + "\n" + quote(message),
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: l.T("kudos.see"), Style: discordgo.LinkButton, URL: link},
		}}},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
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
