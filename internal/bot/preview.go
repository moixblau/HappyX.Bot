package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
	"github.com/moixblau/HappyX.Bot/internal/i18n"
)

// Preview is one message the bot sends, built from sample data with the real builders, so
// cmd/preview can show what users see without a Discord server.
type Preview struct {
	Title     string // what the scene shows
	Channel   string // where it appears, e.g. "DM" or "#general"
	Command   string // the slash command that triggered it, if any
	Ephemeral bool   // only the user who ran the command sees it
	Msg       *discordgo.MessageSend
}

// Sample IDs and names used by the previews. cmd/preview shows these IDs as names.
const (
	PreviewGuildID   = "100000000000000001"
	PreviewAdminID   = "300000000000000001"
	PreviewAnaID     = "200000000000000001"
	PreviewBeaID     = "200000000000000002"
	PreviewChannelID = "400000000000000001"
)

// Previews builds every scene in language code.
func Previews(code string) []Preview {
	l := i18n.Get(code)
	g := api.Guild{DiscordID: PreviewGuildID, Name: "Equipo Demo", AdminDiscordID: PreviewAdminID, Anonymous: true,
		Timezone: "Europe/Madrid", SurveyHour: 13, ReportFrequency: "weekly", Language: code, KudosChannelID: PreviewChannelID}
	b := &Bot{}
	public, no := g, false
	public.Anonymous = false
	day := "2026-10-23"
	kudosMsg := "Gracias por echarme un cable con el despliegue del viernes, sin ti no llegábamos 🚀"
	link := "https://discord.com/channels/" + PreviewGuildID + "/" + PreviewChannelID + "/1"

	answered := answeredMessage(l, surveyID{Action: actionMood, GuildID: g.DiscordID, Day: day, Mood: 4}, "")
	commented := answeredMessage(l, surveyID{Action: actionMood, GuildID: g.DiscordID, Day: day, Mood: 4}, "Semana intensa pero buen ambiente")

	text := func(s string) *discordgo.MessageSend { return &discordgo.MessageSend{Content: s} }
	data := func(d *discordgo.InteractionResponseData) *discordgo.MessageSend {
		return &discordgo.MessageSend{Content: d.Content, Embeds: d.Embeds, Components: d.Components}
	}

	return []Preview{
		{Title: "Daily survey", Channel: "DM", Msg: surveyMessage(l, g, day)},
		{Title: "After answering", Channel: "DM", Msg: data(answered)},
		{Title: "After adding a comment", Channel: "DM", Msg: data(commented)},
		{Title: "Kudos card", Channel: "#kudos", Command: "/happyx kudos", Msg: kudosCard(l, PreviewAnaID, PreviewBeaID, kudosMsg)},
		{Title: "Kudos confirmation", Channel: "#general", Command: "/happyx kudos", Ephemeral: true,
			Msg: text(l.T("kudos.sent", "to", "<@"+PreviewBeaID+">", "channel", "<#"+PreviewChannelID+">"))},
		{Title: "Kudos DM to the receiver", Channel: "DM", Msg: kudosDM(l, g, PreviewAnaID, kudosMsg, link)},
		{Title: "Kudos to yourself", Channel: "#general", Command: "/happyx kudos", Ephemeral: true, Msg: text(l.T("kudos.no_self"))},
		{Title: "Kudos to a bot", Channel: "#general", Command: "/happyx kudos", Ephemeral: true, Msg: text(l.T("kudos.no_bots"))},
		{Title: "Kudos daily limit", Channel: "#general", Command: "/happyx kudos", Ephemeral: true, Msg: text(l.T("kudos.too_many"))},
		{Title: "Status", Channel: "#general", Command: "/happyx status", Ephemeral: true, Msg: text(l.T("status.text",
			"hour", "13", "timezone", g.Timezone, "frequency", l.T("freq.weekly"), "admin", "<@"+PreviewAdminID+">",
			"anonymous", yesNo(l, true), "language", l.Name, "participants", 12, "you", l.T("status.receiving")) +
			"\n" + kudosChannelLine(l, g))},
		{Title: "Turning anonymity off", Channel: "#general", Command: "/happyx config anonymous:False", Ephemeral: true,
			Msg: text(configSaved(l, public, api.GuildPatch{Anonymous: &no}))},
		{Title: "Pause", Channel: "#general", Command: "/happyx pause", Ephemeral: true, Msg: text(l.T("pause.done", "resume_cmd", b.resumeMention()))},
		{Title: "Weekly report", Channel: "DM", Msg: reportMessage(l, g, previewReport())},
	}
}

func previewReport() api.Report {
	r := api.Report{From: "2026-10-19", To: "2026-10-25", Anonymous: true, Responses: 41, Respondents: 9, Average: 3.7,
		Distribution: map[string]int{"1": 1, "2": 4, "3": 10, "4": 17, "5": 9},
		Messages: []api.ReportMsg{
			{Day: "2026-10-20", Mood: 2, Message: "Muchas reuniones seguidas, me cuesta concentrarme"},
			{Day: "2026-10-22", Mood: 5, Message: "¡Por fin salió la release!"},
			{Day: "2026-10-23", Mood: 4, Message: "Semana intensa pero buen ambiente"},
		},
		Kudos: api.KudosSummary{Count: 6, Givers: 5, Receivers: 4, Highlights: []api.Kudos{
			{Day: "2026-10-23", FromUsername: "ana", ToUsername: "bea", Message: "Gracias por echarme un cable con el despliegue del viernes, sin ti no llegábamos 🚀"},
			{Day: "2026-10-22", FromUsername: "carlos", ToUsername: "dani", Message: "Por la revisión tan detallada del PR"},
			{Day: "2026-10-21", FromUsername: "bea", ToUsername: "ana", Message: "Por organizar la retro y traer café ☕"},
		}},
	}
	for i, avg := range []float64{3.2, 2.9, 3.8, 4.3, 4.1, 0, 0} {
		d := api.ReportDay{Day: fmt.Sprintf("2026-10-%d", 19+i), Average: avg}
		if avg > 0 {
			d.Responses = 8
		}
		r.Days = append(r.Days, d)
	}
	return r
}
