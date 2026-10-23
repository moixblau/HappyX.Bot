package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
)

func commands() []*discordgo.ApplicationCommand {
	no := false
	minHour := 0.0
	return []*discordgo.ApplicationCommand{{
		Name:         "happyx",
		Description:  "Encuesta diaria de ánimo",
		DMPermission: &no,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "estado", Description: "Ver la configuración y si recibes la encuesta"},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "pausar", Description: "Dejar de recibir la encuesta diaria"},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "activar", Description: "Volver a recibir la encuesta diaria"},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "report", Description: "(Admin) Ver ahora el report del último periodo"},
			{
				Type: discordgo.ApplicationCommandOptionSubCommand, Name: "config", Description: "(Admin) Cambiar la configuración",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionBoolean, Name: "anonimo", Description: "Ocultar quién responde en los reports"},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "hora", Description: "Hora de la encuesta (0-23)", MinValue: &minHour, MaxValue: 23},
					{Type: discordgo.ApplicationCommandOptionString, Name: "zona", Description: "Zona horaria, p. ej. Europe/Madrid"},
					{
						Type: discordgo.ApplicationCommandOptionString, Name: "frecuencia", Description: "Cada cuánto recibir el report",
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "Semanal", Value: "weekly"},
							{Name: "Mensual", Value: "monthly"},
						},
					},
					{Type: discordgo.ApplicationCommandOptionUser, Name: "admin", Description: "Quién recibe los reports"},
				},
			},
		},
	}}
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

// --- Survey answers (in DMs) ---

func (b *Bot) handleSurveyButton(ctx context.Context, i *discordgo.InteractionCreate) {
	id, ok := parseSurveyID(i.MessageComponentData().CustomID)
	if !ok {
		return
	}
	user := interactionUser(i)
	switch id.Action {
	case actionMood:
		if err := b.api.RecordMood(ctx, id.GuildID, user.ID, id.Day, id.Mood, ""); err != nil {
			b.replyError(i, err)
			return
		}
		b.respond(i, discordgo.InteractionResponseUpdateMessage, answeredMessage(id, ""))
	case actionNote:
		b.respond(i, discordgo.InteractionResponseModal, noteModal(id))
	}
}

func (b *Bot) handleNoteModal(ctx context.Context, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	id, ok := parseSurveyID(data.CustomID)
	if !ok || id.Action != actionNoteModal {
		return
	}
	comment := modalValue(data, "comment")
	if err := b.api.RecordMood(ctx, id.GuildID, interactionUser(i).ID, id.Day, id.Mood, comment); err != nil {
		b.replyError(i, err)
		return
	}
	b.respond(i, discordgo.InteractionResponseUpdateMessage, answeredMessage(id, comment))
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
	switch sub.Name {
	case "estado":
		b.cmdStatus(ctx, i, user)
	case "pausar":
		err := b.api.RemoveMember(ctx, i.GuildID, user.ID)
		if err != nil && !errors.Is(err, api.ErrNotFound) {
			b.replyError(i, err)
			return
		}
		b.ephemeral(i, "Hecho, ya no recibirás la encuesta de este servidor. Usa `/happyx activar` para volver.")
	case "activar":
		yes := true
		if err := b.api.UpsertMember(ctx, i.GuildID, user.ID, user.Username, &yes); err != nil {
			b.replyError(i, err)
			return
		}
		b.ephemeral(i, "¡Perfecto! Volverás a recibir la encuesta diaria por mensaje privado.")
	case "config":
		b.cmdConfig(ctx, i, user, sub.Options)
	case "report":
		b.cmdReport(ctx, i, user)
	}
}

func (b *Bot) cmdStatus(ctx context.Context, i *discordgo.InteractionCreate, user *discordgo.User) {
	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, err)
		return
	}
	members, err := b.api.Members(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, err)
		return
	}
	receiving := false
	for _, m := range members {
		if m.DiscordID == user.ID {
			receiving = true
		}
	}
	b.ephemeral(i, fmt.Sprintf(
		"**Encuesta:** cada día a las %02d:00 (%s)\n**Report:** %s, para <@%s>\n**Anónimo:** %s\n**Participantes:** %d\n**Tú:** %s",
		g.SurveyHour, g.Timezone, frequencyLabel(g.ReportFrequency), g.AdminDiscordID,
		yesNo(g.Anonymous), len(members), map[bool]string{true: "recibes la encuesta", false: "en pausa"}[receiving]))
}

func (b *Bot) cmdConfig(ctx context.Context, i *discordgo.InteractionCreate, user *discordgo.User, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if !b.isAdmin(ctx, i, user) {
		return
	}
	var p api.GuildPatch
	for _, o := range opts {
		switch o.Name {
		case "anonimo":
			v := o.BoolValue()
			p.Anonymous = &v
		case "hora":
			v := int(o.IntValue())
			p.SurveyHour = &v
		case "zona":
			v := o.StringValue()
			p.Timezone = &v
		case "frecuencia":
			v := o.StringValue()
			p.ReportFrequency = &v
		case "admin":
			v := o.UserValue(nil).ID
			p.AdminDiscordID = &v
		}
	}
	g, err := b.api.UpdateGuild(ctx, i.GuildID, p)
	if err != nil {
		b.replyError(i, err)
		return
	}
	b.ephemeral(i, fmt.Sprintf("Configuración guardada: encuesta a las %02d:00 (%s), report %s para <@%s>, anónimo: %s.",
		g.SurveyHour, g.Timezone, frequencyLabel(g.ReportFrequency), g.AdminDiscordID, yesNo(g.Anonymous)))
}

func (b *Bot) cmdReport(ctx context.Context, i *discordgo.InteractionCreate, user *discordgo.User) {
	if !b.isAdmin(ctx, i, user) {
		return
	}
	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, err)
		return
	}
	r, err := b.api.Report(ctx, i.GuildID, "", "")
	if err != nil {
		b.replyError(i, err)
		return
	}
	msg := reportMessage(g, r)
	b.respond(i, discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: msg.Embeds, Flags: discordgo.MessageFlagsEphemeral,
	})
}

// isAdmin checks the caller is the guild's HappyX admin, and answers them if not.
func (b *Bot) isAdmin(ctx context.Context, i *discordgo.InteractionCreate, user *discordgo.User) bool {
	g, err := b.api.Guild(ctx, i.GuildID)
	if err != nil {
		b.replyError(i, err)
		return false
	}
	if g.AdminDiscordID != user.ID {
		b.ephemeral(i, fmt.Sprintf("Solo el administrador de HappyX (<@%s>) puede hacer esto.", g.AdminDiscordID))
		return false
	}
	return true
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

func (b *Bot) replyError(i *discordgo.InteractionCreate, err error) {
	var apiErr *api.Error
	msg := "Algo ha fallado, inténtalo más tarde. 😖"
	switch {
	case errors.Is(err, api.ErrForbidden):
		msg = "No participas en la encuesta de este servidor. Usa `/happyx activar` allí para apuntarte."
	case errors.Is(err, api.ErrConflict):
		msg = "HappyX no está activo en este servidor."
	case errors.As(err, &apiErr) && apiErr.Status == 400:
		msg = "Dato no válido: " + apiErr.Message
	default:
		b.log.Error("interaction failed", "err", err)
	}
	b.ephemeral(i, msg)
}

func frequencyLabel(f string) string {
	if f == "monthly" {
		return "mensual"
	}
	return "semanal"
}

func yesNo(v bool) string {
	if v {
		return "sí"
	}
	return "no"
}
