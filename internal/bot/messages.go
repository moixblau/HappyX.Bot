package bot

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
	"github.com/moixblau/HappyX.Bot/internal/i18n"
	"github.com/moixblau/HappyX.Bot/internal/report"
)

// moodEmojis are the 1-5 answers, worst first. Their labels are mood.1-mood.5 in the catalogs.
var moodEmojis = []string{"😞", "😕", "😐", "🙂", "🤩"}

func moodEmoji(m int) string {
	if m < 1 || m > len(moodEmojis) {
		return "❔"
	}
	return moodEmojis[m-1]
}

func surveyMessage(l *i18n.Locale, g api.Guild, day string) *discordgo.MessageSend {
	buttons := make([]discordgo.MessageComponent, 0, len(moodEmojis))
	for i, emoji := range moodEmojis {
		buttons = append(buttons, discordgo.Button{
			Label:    l.T(fmt.Sprintf("mood.%d", i+1)),
			Emoji:    &discordgo.ComponentEmoji{Name: emoji},
			Style:    discordgo.SecondaryButton,
			CustomID: surveyID{Action: actionMood, GuildID: g.DiscordID, Day: day, Mood: i + 1}.String(),
		})
	}
	privacy := l.T("survey.anonymous")
	if !g.Anonymous {
		privacy = l.T("survey.named")
	}
	return &discordgo.MessageSend{
		Content: l.T("survey.question", "guild", g.Name) + "\n" + privacy,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: buttons},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    l.T("survey.stop_button"),
					Emoji:    &discordgo.ComponentEmoji{Name: "🔕"},
					Style:    discordgo.SecondaryButton,
					CustomID: surveyID{Action: actionStop, GuildID: g.DiscordID, Day: day}.String(),
				},
			}},
		},
	}
}

// answeredMessage replaces the survey once answered, offering to add a comment.
func answeredMessage(l *i18n.Locale, id surveyID, comment string) *discordgo.InteractionResponseData {
	content := l.T("answer.thanks", "emoji", moodEmoji(id.Mood), "day", id.Day)
	components := []discordgo.MessageComponent{}
	if comment == "" {
		content += "\n" + l.T("answer.comment_prompt")
		note := id
		note.Action = actionNote
		components = []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: l.T("answer.comment_button"), Emoji: &discordgo.ComponentEmoji{Name: "✏️"}, Style: discordgo.PrimaryButton, CustomID: note.String()},
		}}}
	} else {
		content += "\n> " + strings.ReplaceAll(comment, "\n", "\n> ")
	}
	return &discordgo.InteractionResponseData{Content: content, Components: components}
}

func noteModal(l *i18n.Locale, id surveyID) *discordgo.InteractionResponseData {
	id.Action = actionNoteModal
	return &discordgo.InteractionResponseData{
		CustomID: id.String(),
		Title:    l.T("modal.title"),
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.TextInput{
				CustomID:    "comment",
				Label:       l.T("modal.label"),
				Style:       discordgo.TextInputParagraph,
				Required:    true,
				MaxLength:   500,
				Placeholder: l.T("modal.placeholder"),
			},
		}}},
	}
}

// reportMessage sends the report as an image, with the comments as embed fields (they can
// be long and are easier to read as text). If the image can't be rendered, the embed carries
// a text summary instead.
func reportMessage(l *i18n.Locale, g api.Guild, r api.Report) *discordgo.MessageSend {
	embed := &discordgo.MessageEmbed{
		Title:  fmt.Sprintf("HappyX · %s", g.Name),
		Footer: &discordgo.MessageEmbedFooter{Text: l.T("report.period", "from", r.From, "to", r.To)},
		Color:  0x2a78d6,
	}
	msg := &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}}
	if png, err := report.Render(l, g.Name, r); err == nil {
		msg.Files = []*discordgo.File{{Name: "report.png", ContentType: "image/png", Reader: bytes.NewReader(png)}}
		embed.Image = &discordgo.MessageEmbedImage{URL: "attachment://report.png"}
	} else {
		embed.Description = textSummary(l, r)
	}
	if len(r.Messages) > 0 {
		embed.Description += "**" + l.T("report.comments") + "**"
	}
	const maxMessages = 15
	for i, m := range r.Messages {
		if i == maxMessages {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name: "…", Value: l.T("report.more_comments", "n", len(r.Messages)-maxMessages),
			})
			break
		}
		name := fmt.Sprintf("%s %s", moodEmoji(m.Mood), m.Day)
		if m.Username != "" {
			name += " · " + m.Username
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: name, Value: truncate(m.Message, 1000)})
	}
	return msg
}

func textSummary(l *i18n.Locale, r api.Report) string {
	if r.Responses == 0 {
		return l.T("report.no_responses") + "\n\n"
	}
	var b strings.Builder
	b.WriteString(l.T("report.summary",
		"average", l.T("report.average"), "value", fmt.Sprintf("%.2f", r.Average), "emoji", moodEmoji(int(r.Average+0.5)),
		"responses", l.T("report.responses"), "count", r.Responses,
		"people", l.T("report.people"), "people_count", r.Respondents))
	b.WriteString("\n\n")
	for i := len(moodEmojis); i >= 1; i-- {
		n := r.Distribution[fmt.Sprint(i)]
		fmt.Fprintf(&b, "%s %s %d\n", moodEmoji(i), bar(n, r.Responses), n)
	}
	return b.String() + "\n"
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
