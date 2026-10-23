package bot

import (
	"testing"

	"github.com/moixblau/HappyX.Bot/internal/api"
)

func TestReportMessageAttachesImage(t *testing.T) {
	g := api.Guild{DiscordID: "100000000000000001", Name: "Test"}
	r := api.Report{From: "2026-10-19", To: "2026-10-25", Responses: 1, Respondents: 1, Average: 4,
		Distribution: map[string]int{"4": 1},
		Days:         []api.ReportDay{{Day: "2026-10-19", Responses: 1, Average: 4}},
		Messages:     []api.ReportMsg{{Day: "2026-10-19", Mood: 4, Message: "bien"}}}
	msg := reportMessage(g, r)
	if len(msg.Files) != 1 || msg.Embeds[0].Image == nil || msg.Embeds[0].Image.URL != "attachment://"+msg.Files[0].Name {
		t.Fatalf("report should embed the attached PNG: %+v", msg.Embeds[0])
	}
	if len(msg.Embeds[0].Fields) != 1 {
		t.Fatalf("comments should be embed fields: %+v", msg.Embeds[0].Fields)
	}
}
