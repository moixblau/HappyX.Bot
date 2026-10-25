package api

import (
	"encoding/json"
	"testing"
)

func TestSendsReportDM(t *testing.T) {
	for body, want := range map[string]bool{
		`{}`:                  true, // an API without the setting keeps sending the report
		`{"report_dm":true}`:  true,
		`{"report_dm":false}`: false,
	} {
		var g Guild
		if err := json.Unmarshal([]byte(body), &g); err != nil {
			t.Fatal(err)
		}
		if got := g.SendsReportDM(); got != want {
			t.Errorf("%s: got %v, want %v", body, got, want)
		}
	}
}
