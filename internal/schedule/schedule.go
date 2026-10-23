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
	// WorkingDays are the ISO days (1 is Monday, 7 Sunday) the survey goes out. Empty means
	// Monday to Friday, for an API that doesn't send them yet.
	WorkingDays []int
}

// Works reports whether day is one of the guild's working days.
func (s Settings) Works(day time.Weekday) bool {
	if len(s.WorkingDays) == 0 {
		return day >= time.Monday && day <= time.Friday
	}
	for _, n := range s.WorkingDays {
		if time.Weekday(n%7) == day {
			return true
		}
	}
	return false
}

func (s Settings) location() *time.Location {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// SurveyDue reports whether today's survey should go out: on working days it is due from the
// survey hour until the end of the guild's local day. day is the guild's local date.
func SurveyDue(s Settings, now time.Time) (day string, due bool) {
	local := now.In(s.location())
	return local.Format(DateLayout), s.Works(local.Weekday()) && local.Hour() >= s.SurveyHour
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
