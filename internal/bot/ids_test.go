package bot

import "testing"

func TestSurveyIDRoundTrip(t *testing.T) {
	id := surveyID{Action: actionMood, GuildID: "100000000000000001", Day: "2026-10-22", Mood: 4}
	got, ok := parseSurveyID(id.String())
	if !ok || got != id {
		t.Fatalf("round trip: got %+v %v", got, ok)
	}
	if len(id.String()) > 100 {
		t.Fatal("custom IDs must fit Discord's 100 character limit")
	}
	stop := surveyID{Action: actionStop, GuildID: "100000000000000001", Day: "2026-10-22"}
	if got, ok := parseSurveyID(stop.String()); !ok || got != stop {
		t.Fatalf("stop round trip: got %+v %v", got, ok)
	}
	for _, bad := range []string{"", "hx:mood:1:2026-10-22", "hx:mood:1:2026-10-22:9", "hx:mood:1:d:0", "hx:stop:1:d:3", "xx:mood:1:d:3", "hx:drop:1:d:3"} {
		if _, ok := parseSurveyID(bad); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}
