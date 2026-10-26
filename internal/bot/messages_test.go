package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/happyx-app/HappyX.Bot/internal/api"
	"github.com/happyx-app/HappyX.Bot/internal/i18n"
)

func TestReportMessageAttachesImage(t *testing.T) {
	g := api.Guild{DiscordID: "100000000000000001", Name: "Test"}
	r := api.Report{From: "2026-10-19", To: "2026-10-25", Responses: 1, Respondents: 1, Average: 4,
		Distribution: map[string]int{"4": 1},
		Days:         []api.ReportDay{{Day: "2026-10-19", Responses: 1, Average: 4}},
		Messages:     []api.ReportMsg{{Day: "2026-10-19", Mood: 4, Message: "bien"}}}
	msg := reportMessage(i18n.Get("es"), g, r)
	if len(msg.Files) != 1 || msg.Embeds[0].Image == nil || msg.Embeds[0].Image.URL != "attachment://"+msg.Files[0].Name {
		t.Fatalf("report should embed the attached PNG: %+v", msg.Embeds[0])
	}
	if !strings.Contains(msg.Embeds[0].Description, "> 🙂 bien") {
		t.Fatalf("comments should be quoted in the description: %q", msg.Embeds[0].Description)
	}
}

// A busy server's comments must still fit Discord's embed limits.
func TestReportMessageFitsDiscordLimits(t *testing.T) {
	r := api.Report{From: "2026-10-01", To: "2026-10-31", Responses: 500, Respondents: 120, Average: 3.4,
		Distribution: map[string]int{"3": 500}}
	for i := 0; i < 400; i++ {
		r.Messages = append(r.Messages, api.ReportMsg{Day: "2026-10-19", Mood: 1 + i%5,
			Message: strings.Repeat("palabra ", 80), Username: "someone_with_a_long_name"})
	}
	r.Kudos = api.KudosSummary{Count: 60, Givers: 30, Receivers: 25}
	for i := 0; i < 5; i++ {
		r.Kudos.Highlights = append(r.Kudos.Highlights, api.Kudos{Day: "2026-10-19", Message: strings.Repeat("gracias ", 40),
			FromUsername: "someone_with_a_long_name", ToUsername: "someone_else_with_a_long_name"})
	}
	e := reportMessage(i18n.Get("es"), api.Guild{Name: strings.Repeat("x", 100)}, r).Embeds[0]
	total := len([]rune(e.Title)) + len([]rune(e.Description)) + len([]rune(e.Footer.Text))
	if len([]rune(e.Description)) > 4096 || total > 6000 {
		t.Fatalf("description %d, total %d characters", len([]rune(e.Description)), total)
	}
	if !strings.Contains(e.Description, "comentarios más") {
		t.Fatal("should say how many comments were left out")
	}
}

func TestReportMessageKudos(t *testing.T) {
	r := api.Report{From: "2026-10-19", To: "2026-10-25", Anonymous: true,
		Kudos: api.KudosSummary{Count: 7, Givers: 3, Receivers: 2, Highlights: []api.Kudos{
			{Day: "2026-10-23", FromUsername: "ana", ToUsername: "bea", Message: "gracias por el cable\ncon el despliegue"},
		}}}
	d := reportMessage(i18n.Get("es"), api.Guild{Name: "Test"}, r).Embeds[0].Description
	for _, want := range []string{"### 🙌 Kudos", "**7** kudos · **2** personas", "> gracias por el cable\n> con el despliegue", "-# ana → bea · vi 23 oct", "y 6 kudos más"} {
		if !strings.Contains(strings.ToLower(d), strings.ToLower(want)) {
			t.Errorf("missing %q in:\n%s", want, d)
		}
	}
	if strings.Contains(reportMessage(i18n.Get("es"), api.Guild{}, api.Report{}).Embeds[0].Description, "Kudos") {
		t.Error("no kudos section without kudos")
	}
}

func TestKudosMessages(t *testing.T) {
	l := i18n.Get("es")
	ana := kudosPerson{ID: "200000000000000001", Name: "Ana", Avatar: "https://cdn/ana.png"}
	bea := kudosPerson{ID: "200000000000000002", Name: "Bea", Avatar: "https://cdn/bea.png"}
	card := kudosCard(l, ana, bea, "gracias @everyone")
	e := card.Embeds[0]
	if e.Title != "🎉 ¡Kudos para Bea!" || e.Description != "> gracias @everyone" || e.Thumbnail == nil || e.Thumbnail.URL != bea.Avatar {
		t.Fatalf("card: %+v", e)
	}
	if len(e.Fields) != 2 || e.Fields[0].Value != "<@200000000000000001>" || e.Fields[1].Value != "<@200000000000000002>" {
		t.Fatalf("card fields: %+v %+v", e.Fields[0], e.Fields[1])
	}
	if e.Footer == nil || e.Footer.Text != "🙌 Da los tuyos con /happyx kudos" {
		t.Fatalf("card footer: %+v", e.Footer)
	}
	if card.AllowedMentions == nil || len(card.AllowedMentions.Parse) != 0 {
		t.Fatal("the card must not ping anyone")
	}
	dm := kudosDM(l, api.Guild{Name: "Equipo"}, ana, "gracias", "https://discord.com/channels/1/2/3", "kudos")
	d := dm.Embeds[0]
	if d.Description != "> gracias" || d.Thumbnail.URL != ana.Avatar || d.Fields[1].Value != "Equipo" {
		t.Fatalf("dm: %+v", d)
	}
	btn := dm.Components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button)
	if btn.Style != discordgo.LinkButton || btn.URL != "https://discord.com/channels/1/2/3" || btn.Label != "Verlo en #kudos" {
		t.Fatalf("dm should link to the card: %+v", btn)
	}
	// Without the channel's name, and without avatars.
	dm = kudosDM(l, api.Guild{Name: "Equipo"}, kudosPerson{ID: "1", Name: "x"}, "gracias", "https://x", "")
	if btn := dm.Components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button); btn.Label != "Verlo en el servidor" || dm.Embeds[0].Thumbnail != nil {
		t.Fatalf("fallbacks: %+v %+v", btn, dm.Embeds[0].Thumbnail)
	}
}

func TestKudosCommand(t *testing.T) {
	var kudos, config *discordgo.ApplicationCommandOption
	for _, o := range commands()[0].Options {
		switch o.Name {
		case cmdKudos:
			kudos = o
		case cmdConfig:
			config = o
		}
	}
	if kudos == nil || len(kudos.Options) != 2 || !kudos.Options[0].Required || !kudos.Options[1].Required {
		t.Fatalf("/happyx kudos needs a required user and message: %+v", kudos)
	}
	if kudos.Options[0].Type != discordgo.ApplicationCommandOptionUser || kudos.Options[1].MaxLength != kudosMaxLength {
		t.Fatalf("kudos options: %+v %+v", kudos.Options[0], kudos.Options[1])
	}
	configOpts := map[string]*discordgo.ApplicationCommandOption{}
	for _, o := range config.Options {
		configOpts[o.Name] = o
	}
	if o := configOpts[optKudosChan]; o == nil || o.Type != discordgo.ApplicationCommandOptionChannel {
		t.Fatalf("/happyx config should take a kudos channel: %+v", o)
	}
	if o := configOpts[optWorkDays]; o == nil || len(o.Choices) != len(workingDayPresets) {
		t.Fatalf("/happyx config should offer the working day presets: %+v", o)
	}
	if o := configOpts[optGames]; o == nil || o.Type != discordgo.ApplicationCommandOptionBoolean {
		t.Fatalf("/happyx config should turn games on and off: %+v", o)
	}
}

func TestGameIDs(t *testing.T) {
	id := gameID{Action: gameAnswer, GuildID: "100000000000000001", Week: "2026-10-19"}
	if got, ok := parseGameID(id.String()); !ok || got != id {
		t.Fatalf("round trip: %+v %v", got, ok)
	}
	for _, bad := range []string{"hx:mood:1:2026-10-19:3", "hxg:other:1:2026-10-19", "hxg:answer:1:monday", "hxg:answer:1"} {
		if _, ok := parseGameID(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
	// A survey ID is never taken for a game one, and vice versa.
	if _, ok := parseSurveyID(id.String()); ok {
		t.Error("game ID parsed as a survey ID")
	}
}

func TestGameProposals(t *testing.T) {
	l := i18n.Get("es")
	g := api.Guild{DiscordID: "100000000000000001", Name: "Equipo"}
	r := api.GameRound{Week: "2026-10-19", Question: "¿Tu olor favorito?", Answers: 5}
	buttons := func(m *discordgo.MessageSend) []discordgo.Button {
		var out []discordgo.Button
		for _, c := range m.Components[0].(discordgo.ActionsRow).Components {
			out = append(out, c.(discordgo.Button))
		}
		return out
	}
	// Thursday: guess on the web, and propose next week's question from Discord.
	bs := buttons(gameGuessMessage(l, g, r, "https://web/#/game/1"))
	if len(bs) != 2 || bs[0].URL == "" {
		t.Fatalf("guess DM buttons: %+v", bs)
	}
	id, ok := parseGameID(bs[1].CustomID)
	if !ok || id.Action != gamePropose || id.GuildID != g.DiscordID {
		t.Fatalf("propose button: %q", bs[1].CustomID)
	}
	modal := gameModal(l, id)
	if mid, ok := parseGameID(modal.CustomID); !ok || mid.Action != gameProposeModal || modal.Title != l.T("game.propose_modal_title") {
		t.Fatalf("propose modal: %+v", modal)
	}
	// Without enough answers (or a dashboard) the DM only asks for a question.
	if m := gameGuessMessage(l, g, r, ""); len(buttons(m)) != 1 || strings.Contains(m.Content, r.Question) {
		t.Fatalf("proposal-only DM: %+v", m)
	}
	// A drawn proposal is announced as the team's, without saying whose.
	if m := gameQuestionMessage(l, g, api.GameRound{Week: r.Week, Question: r.Question, Proposed: true}, ""); !strings.Contains(m.Content, l.T("game.proposed")) {
		t.Fatalf("proposed question DM: %q", m.Content)
	}
	if m := gameQuestionMessage(l, g, r, ""); strings.Contains(m.Content, l.T("game.proposed")) {
		t.Fatalf("bot's question announced as proposed: %q", m.Content)
	}
}

func TestGameQuestionRotates(t *testing.T) {
	l := i18n.Get("es")
	seen := map[string]bool{}
	week, _ := time.Parse("2006-01-02", "2026-10-19")
	for i := 0; i < 20; i++ {
		q := gameQuestion(l, week.AddDate(0, 0, 7*i).Format("2006-01-02"))
		if strings.HasPrefix(q, "game.q.") || seen[q] {
			t.Fatalf("week %d: %q missing or repeated", i, q)
		}
		seen[q] = true
	}
}

// cmd/preview builds every scene in every language; none may be empty.
func TestPreviews(t *testing.T) {
	for _, code := range i18n.Codes() {
		for _, p := range Previews(code) {
			if p.Msg == nil || (p.Msg.Content == "" && len(p.Msg.Embeds) == 0) {
				t.Errorf("%s: %q is empty", code, p.Title)
			}
		}
	}
}

func TestAnsweredMessage(t *testing.T) {
	es := i18n.Get("es")
	g := api.Guild{Name: "Equipo"}
	id := surveyID{Action: actionMood, GuildID: "1", Day: "2026-10-23", Mood: 4}
	a := answeredMessage(es, g, id, "Ana", 6, "")
	e := a.Embeds[0]
	if a.Content != "" || e.Description != "### 🙂 ¡Gracias, Ana!\nApuntado: **Bien**.\n\n¿Quieres añadir un comentario? Es opcional." {
		t.Fatalf("answered: %q %q", a.Content, e.Description)
	}
	if len(e.Fields) != 2 || e.Fields[0].Value != "viernes, 23 de octubre" || e.Fields[1].Value != "6 días" || len(a.Components) != 1 {
		t.Fatalf("answered fields or button: %+v %+v", e.Fields, a.Components)
	}
	// With a comment there's no button, and an API that didn't count the streak hides it.
	a = answeredMessage(es, g, id, "Ana", 0, "buena semana\ny tanto")
	if e := a.Embeds[0]; !strings.HasSuffix(e.Description, "\n> buena semana\n> y tanto") || len(e.Fields) != 1 || len(a.Components) != 0 {
		t.Fatalf("commented: %q %+v %+v", e.Description, e.Fields, a.Components)
	}
}

func TestWorkingDaysText(t *testing.T) {
	en, es := i18n.Get("en"), i18n.Get("es")
	for _, c := range []struct {
		l    *i18n.Locale
		days []int
		want string
	}{
		{en, nil, "Mon–Fri"},
		{en, []int{1, 2, 3, 4, 5}, "Mon–Fri"},
		{es, []int{1, 2, 3, 4, 5, 6}, "lu–sá"},
		{en, []int{7, 1, 2, 3, 4}, "Sun–Thu"},
		{en, []int{1, 3, 5}, "Mon, Wed, Fri"},
		{en, []int{1, 2, 4, 5, 6}, "Mon, Tue, Thu–Sat"},
		{en, []int{6, 7}, "Sat, Sun"},
		{en, []int{1, 2, 3, 4, 5, 6, 7}, "every day"},
		{es, []int{1, 2, 3, 4, 5, 6, 7}, "cada día"},
	} {
		if got := workingDaysText(c.l, c.days); got != c.want {
			t.Errorf("%v: got %q, want %q", c.days, got, c.want)
		}
	}
}

func TestSurveyMessage(t *testing.T) {
	es := i18n.Get("es")
	g := api.Guild{DiscordID: "1", Name: "Equipo", Anonymous: true}
	ana := surveyRecipient{ID: "2", Name: "Ana", Streak: 5}

	// No custom message: a greeting and the question in the guild's language, on a card with
	// the server, the day, the streak and the privacy notice.
	def := surveyMessage(es, g, "2026-10-23", ana, "https://cdn/icon.png")
	e := def.Embeds[0]
	if e.Description != "### 👋 ¡Hola, Ana!\n¿Cómo te encuentras hoy?" || e.Author.Name != "Equipo · Encuesta diaria" {
		t.Fatalf("default: %q %q", e.Description, e.Author.Name)
	}
	if e.Thumbnail == nil || e.Thumbnail.URL != "https://cdn/icon.png" || e.Footer.Text != es.T("survey.anonymous") {
		t.Fatalf("icon or notice: %+v %+v", e.Thumbnail, e.Footer)
	}
	if len(e.Fields) != 2 || e.Fields[0].Value != "viernes, 23 de octubre" || e.Fields[1].Value != "5 días" {
		t.Fatalf("fields: %+v", e.Fields)
	}
	// No streak yet, or a streak of one, and no server icon.
	ana.Streak = 0
	if e := surveyMessage(es, g, "2026-10-23", ana, "").Embeds[0]; len(e.Fields) != 1 || e.Thumbnail != nil {
		t.Fatalf("no streak: %+v %+v", e.Fields, e.Thumbnail)
	}
	ana.Streak = 1
	if e := surveyMessage(es, g, "2026-10-23", ana, "").Embeds[0]; e.Fields[1].Value != "1 día" {
		t.Fatalf("one day: %+v", e.Fields[1])
	}
	ana.Streak = 5

	g.SurveyMessage = "Hola {{user.name}} ({{ user.mention }}), {{weekday}} — {{date}} en {{guild.name}}: {{user.streak}} días. {{user.nope}} @everyone"
	msg := surveyMessage(es, g, "2026-10-23", ana, "")
	want := "Hola Ana (<@2>), viernes — viernes, 23 de octubre en Equipo: 5 días. {{user.nope}} @everyone"
	if got := msg.Embeds[0].Description; got != want {
		t.Fatalf("custom:\n got %q\nwant %q", got, want)
	}
	// The service always adds the mood buttons and «Don't ask me again», and nothing pings.
	if len(msg.Components) != 2 || msg.AllowedMentions == nil || len(msg.AllowedMentions.Parse) != 0 {
		t.Fatalf("components or mentions: %+v %+v", msg.Components, msg.AllowedMentions)
	}
	g.Anonymous = false
	if msg := surveyMessage(es, g, "2026-10-23", ana, ""); msg.Embeds[0].Footer.Text != es.T("survey.named") {
		t.Fatalf("named notice: %q", msg.Embeds[0].Footer.Text)
	}

	for code, want := range map[string]string{
		"en": "Friday, October 23", "ca": "divendres, 23 d'octubre", "de": "Freitag, 23. Oktober",
		"fr": "vendredi 23 octobre", "pt-BR": "sexta-feira, 23 de outubro",
	} {
		if got := longDate(i18n.Get(code), time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)); got != want {
			t.Errorf("%s: got %q, want %q", code, got, want)
		}
	}
}
