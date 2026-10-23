package schedule

import (
	"testing"
	"time"
)

func TestSurveyDue(t *testing.T) {
	s := Settings{Timezone: "Europe/Madrid", SurveyHour: 13}
	// 11:30 UTC is 13:30 in Madrid (CEST, UTC+2).
	day, due := SurveyDue(s, time.Date(2026, 10, 22, 11, 30, 0, 0, time.UTC))
	if !due || day != "2026-10-22" {
		t.Fatalf("got %s %v, want 2026-10-22 true", day, due)
	}
	if _, due := SurveyDue(s, time.Date(2026, 10, 22, 10, 59, 0, 0, time.UTC)); due {
		t.Fatal("should not be due before the survey hour")
	}
	// 22:30 UTC is already the next day in Madrid.
	if day, _ := SurveyDue(s, time.Date(2026, 10, 22, 22, 30, 0, 0, time.UTC)); day != "2026-10-23" {
		t.Fatalf("day should follow the guild timezone, got %s", day)
	}
}

func TestReportDue(t *testing.T) {
	weekly := Settings{Timezone: "UTC", SurveyHour: 9, ReportFrequency: "weekly"}
	from, to, due := ReportDue(weekly, time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)) // Monday
	if !due || from != "2026-10-19" || to != "2026-10-25" {
		t.Fatalf("weekly: got %s..%s %v", from, to, due)
	}
	if _, _, due := ReportDue(weekly, time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)); due {
		t.Fatal("weekly report only on Mondays")
	}
	if _, _, due := ReportDue(weekly, time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)); due {
		t.Fatal("not before the survey hour")
	}

	monthly := Settings{Timezone: "UTC", SurveyHour: 9, ReportFrequency: "monthly"}
	from, to, due = ReportDue(monthly, time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC))
	if !due || from != "2026-10-01" || to != "2026-10-31" {
		t.Fatalf("monthly: got %s..%s %v", from, to, due)
	}
}

func TestCurrentPeriod(t *testing.T) {
	now := time.Date(2026, 10, 22, 22, 30, 0, 0, time.UTC) // Friday 23rd in Madrid
	weekly := Settings{Timezone: "Europe/Madrid", ReportFrequency: "weekly"}
	if from, to := CurrentPeriod(weekly, now); from != "2026-10-19" || to != "2026-10-23" {
		t.Fatalf("weekly: got %s..%s", from, to)
	}
	monthly := Settings{Timezone: "Europe/Madrid", ReportFrequency: "monthly"}
	if from, to := CurrentPeriod(monthly, now); from != "2026-10-01" || to != "2026-10-23" {
		t.Fatalf("monthly: got %s..%s", from, to)
	}
	// On a Monday the week in progress is just today.
	if from, to := CurrentPeriod(weekly, time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)); from != "2026-10-26" || to != "2026-10-26" {
		t.Fatalf("monday: got %s..%s", from, to)
	}
}
