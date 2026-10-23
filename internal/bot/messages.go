package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
)

var moods = []struct {
	Emoji string
	Label string
}{
	{"😞", "Muy mal"},
	{"😕", "Mal"},
	{"😐", "Normal"},
	{"🙂", "Bien"},
	{"🤩", "Genial"},
}

func moodEmoji(m int) string {
	if m < 1 || m > len(moods) {
		return "❔"
	}
	return moods[m-1].Emoji
}

func surveyMessage(g api.Guild, day string) *discordgo.MessageSend {
	buttons := make([]discordgo.MessageComponent, 0, len(moods))
	for i, m := range moods {
		buttons = append(buttons, discordgo.Button{
			Label:    m.Label,
			Emoji:    &discordgo.ComponentEmoji{Name: m.Emoji},
			Style:    discordgo.SecondaryButton,
			CustomID: surveyID{Action: actionMood, GuildID: g.DiscordID, Day: day, Mood: i + 1}.String(),
		})
	}
	privacy := "Tu respuesta es **anónima**: el administrador solo ve los resultados agregados."
	if !g.Anonymous {
		privacy = "En este servidor el administrador **puede ver quién responde**."
	}
	return &discordgo.MessageSend{
		Content: fmt.Sprintf("👋 ¿Cómo te encuentras hoy? (**%s**)\n%s", g.Name, privacy),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: buttons},
		},
	}
}

// answeredMessage replaces the survey once answered, offering to add a comment.
func answeredMessage(id surveyID, comment string) *discordgo.InteractionResponseData {
	content := fmt.Sprintf("Gracias, apuntado %s para el %s.", moodEmoji(id.Mood), id.Day)
	var components []discordgo.MessageComponent
	if comment == "" {
		content += "\n¿Quieres añadir un comentario? Es opcional."
		note := id
		note.Action = actionNote
		components = []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "Añadir comentario", Emoji: &discordgo.ComponentEmoji{Name: "✏️"}, Style: discordgo.PrimaryButton, CustomID: note.String()},
		}}}
	} else {
		content += "\n> " + strings.ReplaceAll(comment, "\n", "\n> ")
	}
	return &discordgo.InteractionResponseData{Content: content, Components: components}
}

func noteModal(id surveyID) *discordgo.InteractionResponseData {
	id.Action = actionNoteModal
	return &discordgo.InteractionResponseData{
		CustomID: id.String(),
		Title:    "Comentario (opcional)",
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.TextInput{
				CustomID:    "comment",
				Label:       "¿Algo que quieras contar?",
				Style:       discordgo.TextInputParagraph,
				Required:    true,
				MaxLength:   500,
				Placeholder: "Escribe aquí…",
			},
		}}},
	}
}

// reportMessage is a text version of the report. The image version replaces it later.
func reportMessage(g api.Guild, r api.Report) *discordgo.MessageSend {
	var b strings.Builder
	if r.Responses == 0 {
		b.WriteString("No hubo respuestas en este periodo.")
	} else {
		fmt.Fprintf(&b, "**Media:** %.2f / 5 %s\n", r.Average, moodEmoji(int(r.Average+0.5)))
		fmt.Fprintf(&b, "**Respuestas:** %d de %d personas\n\n", r.Responses, r.Respondents)
		for i := len(moods); i >= 1; i-- {
			n := r.Distribution[fmt.Sprint(i)]
			fmt.Fprintf(&b, "%s %s %d\n", moodEmoji(i), bar(n, r.Responses), n)
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("HappyX · %s", g.Name),
		Description: b.String(),
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Del %s al %s", r.From, r.To)},
		Color:       0x5865F2,
	}
	const maxMessages = 15
	for i, m := range r.Messages {
		if i == maxMessages {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name: "…", Value: fmt.Sprintf("y %d comentarios más", len(r.Messages)-maxMessages),
			})
			break
		}
		name := fmt.Sprintf("%s %s", moodEmoji(m.Mood), m.Day)
		if m.Username != "" {
			name += " · " + m.Username
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: name, Value: truncate(m.Message, 1000)})
	}
	return &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}}
}

func bar(n, total int) string {
	const width = 10
	filled := 0
	if total > 0 {
		filled = (n*width + total/2) / total
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
