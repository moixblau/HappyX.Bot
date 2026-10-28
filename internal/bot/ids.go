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
	actionStop      = "stop"      // button: stop getting this guild's survey (Mood is 0)
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
	if err != nil {
		return surveyID{}, false
	}
	switch p[1] {
	case actionMood, actionNote, actionNoteModal:
		if mood < 1 || mood > 5 {
			return surveyID{}, false
		}
	case actionStop:
		if mood != 0 {
			return surveyID{}, false
		}
	default:
		return surveyID{}, false
	}
	return surveyID{Action: p[1], GuildID: p[2], Day: p[3], Mood: mood}, true
}

// kudosID is the custom ID of the modal opened by "Give kudos" in the right-click menu. It
// carries the receiver and, when given from a message, where that message is. The giver and
// the guild come from the interaction itself.
type kudosID struct {
	To        string
	ChannelID string // empty unless given from a message
	MessageID string
}

const kudosPrefix = "hxk"

func (k kudosID) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", kudosPrefix, k.To, k.ChannelID, k.MessageID)
}

func parseKudosID(v string) (kudosID, bool) {
	p := strings.Split(v, ":")
	if len(p) != 4 || p[0] != kudosPrefix || !snowflake(p[1]) {
		return kudosID{}, false
	}
	if (p[2] == "") != (p[3] == "") || (p[2] != "" && (!snowflake(p[2]) || !snowflake(p[3]))) {
		return kudosID{}, false
	}
	return kudosID{To: p[1], ChannelID: p[2], MessageID: p[3]}, true
}

func snowflake(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}
