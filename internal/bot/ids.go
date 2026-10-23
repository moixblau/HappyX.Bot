package bot

import (
	"fmt"
	"strconv"
	"strings"
)

// Survey components live in DMs, which carry no guild, so the guild and day travel in the
// component's custom ID. The user always comes from the interaction itself, and the API checks
// they are a member of that guild, so a forged ID can't write into someone else's data.

const (
	actionMood      = "mood"      // button: record the mood
	actionNote      = "note"      // button: open the comment modal
	actionNoteModal = "notemodal" // modal submit: record mood + comment
	idPrefix        = "hx"
)

type surveyID struct {
	Action  string
	GuildID string
	Day     string
	Mood    int
}

func (s surveyID) String() string {
	return fmt.Sprintf("%s:%s:%s:%s:%d", idPrefix, s.Action, s.GuildID, s.Day, s.Mood)
}

func parseSurveyID(v string) (surveyID, bool) {
	p := strings.Split(v, ":")
	if len(p) != 5 || p[0] != idPrefix {
		return surveyID{}, false
	}
	mood, err := strconv.Atoi(p[4])
	if err != nil || mood < 1 || mood > 5 {
		return surveyID{}, false
	}
	switch p[1] {
	case actionMood, actionNote, actionNoteModal:
	default:
		return surveyID{}, false
	}
	return surveyID{Action: p[1], GuildID: p[2], Day: p[3], Mood: mood}, true
}
