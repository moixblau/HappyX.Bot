// Package schedule decides when a guild is due a survey or a report. It is pure so it can be
// tested without Discord or the API; sending exactly once is guaranteed by the API's dispatch
// claims, so being "due" for a whole day is fine.
package schedule

import "time"

const DateLayout = "2006-01-02"

// Settings is the part of a guild's configuration the schedule needs.
type Settings struct {
	Timezone        string
	SurveyHour      int
	ReportFrequency string // "weekly" or "monthly"
}

func (s Settings) location() *time.Location {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// SurveyDue reports whether today's survey should go out: it is due from the survey hour until
// the end of the guild's local day. day is the guild's local date.
func SurveyDue(s Settings, now time.Time) (day string, due bool) {
	local := now.In(s.location())
	return local.Format(DateLayout), local.Hour() >= s.SurveyHour
}

// ReportDue reports whether the report for the last complete period should go out. It is due
// on the first day of the new period (Monday, or the 1st of the month) from the survey hour on.
// from and to are the inclusive dates of the period to report.
func ReportDue(s Settings, now time.Time) (from, to string, due bool) {
	local := now.In(s.location())
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	var start time.Time
	if s.ReportFrequency == "monthly" {
		if today.Day() != 1 {
			return "", "", false
		}
		start = today.AddDate(0, -1, 0)
	} else {
		if today.Weekday() != time.Monday {
			return "", "", false
		}
		start = today.AddDate(0, 0, -7)
	}
	if local.Hour() < s.SurveyHour {
		return "", "", false
	}
	return start.Format(DateLayout), today.AddDate(0, 0, -1).Format(DateLayout), true
}

// CurrentPeriod is the period in progress, from its first day (Monday, or the 1st of the
// month) to today, in the guild's timezone.
func CurrentPeriod(s Settings, now time.Time) (from, to string) {
	local := now.In(s.location())
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	if s.ReportFrequency == "monthly" {
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return start.Format(DateLayout), today.Format(DateLayout)
}

// GameQuestionDue reports whether the week's «Who's who?» question should go out: from the
// survey hour, Monday to Wednesday, so a game turned on mid-week still starts. week is the
// round's Monday.
func GameQuestionDue(s Settings, now time.Time) (week string, due bool) {
	return gameDue(s, now, 0, 2)
}

// GameGuessDue reports whether members should be told guessing is open: from the survey hour,
// Thursday to Sunday. week is the round's Monday.
func GameGuessDue(s Settings, now time.Time) (week string, due bool) {
	return gameDue(s, now, 3, 6)
}

// gameDue is due on days first..last of the week (0 is Monday) from the survey hour.
func gameDue(s Settings, now time.Time, first, last int) (string, bool) {
	local := now.In(s.location())
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(today.Weekday()) + 6) % 7
	if offset < first || offset > last || local.Hour() < s.SurveyHour {
		return "", false
	}
	return today.AddDate(0, 0, -offset).Format(DateLayout), true
}
