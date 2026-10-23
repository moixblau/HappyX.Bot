package bot

import (
	"strings"
	"testing"

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
	e := reportMessage(i18n.Get("es"), api.Guild{Name: strings.Repeat("x", 100)}, r).Embeds[0]
	total := len([]rune(e.Title)) + len([]rune(e.Description)) + len([]rune(e.Footer.Text))
	if len([]rune(e.Description)) > 4096 || total > 6000 {
		t.Fatalf("description %d, total %d characters", len([]rune(e.Description)), total)
	}
	if !strings.Contains(e.Description, "comentarios más") {
		t.Fatal("should say how many comments were left out")
	}
}
