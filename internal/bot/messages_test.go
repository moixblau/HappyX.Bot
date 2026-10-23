package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
	"github.com/moixblau/HappyX.Bot/internal/i18n"
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
	card := kudosCard(l, "200000000000000001", "200000000000000002", "gracias @everyone")
	e := card.Embeds[0]
	if !strings.Contains(e.Description, "<@200000000000000001> da kudos a <@200000000000000002>") || !strings.Contains(e.Description, "> gracias @everyone") {
		t.Fatalf("card: %q", e.Description)
	}
	if card.AllowedMentions == nil || len(card.AllowedMentions.Parse) != 0 {
		t.Fatal("the card must not ping anyone")
	}
	dm := kudosDM(l, api.Guild{Name: "Equipo"}, "200000000000000001", "gracias", "https://discord.com/channels/1/2/3")
	if !strings.Contains(dm.Content, "**Equipo**") || !strings.Contains(dm.Content, "> gracias") {
		t.Fatalf("dm: %q", dm.Content)
	}
	btn := dm.Components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button)
	if btn.Style != discordgo.LinkButton || btn.URL != "https://discord.com/channels/1/2/3" {
		t.Fatalf("dm should link to the card: %+v", btn)
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
