package recurrence

import (
	"testing"
	"time"
)

func TestNextMonthlyPreservesAnchorDay(t *testing.T) {
	start := date(2025, 1, 31)
	feb := Next(start, start, "monthly", 1)
	mar := Next(feb, start, "monthly", 1)
	if feb.Day() != 28 || feb.Month() != time.February || mar.Day() != 31 || mar.Month() != time.March {
		t.Fatalf("got %v then %v", feb, mar)
	}
}
func TestNextWeekly(t *testing.T) {
	start := date(2026, 1, 1)
	if got := Next(start, start, "weekly", 1); !got.Equal(date(2026, 1, 8)) {
		t.Fatal(got)
	}
}
func TestIntervalGreaterThanOne(t *testing.T) {
	start := date(2026, 1, 15)
	if got := Next(start, start, "monthly", 3); !got.Equal(date(2026, 4, 15)) {
		t.Fatal(got)
	}
}
func TestEndDate(t *testing.T) {
	start := date(2026, 1, 1)
	end := date(2026, 1, 15)
	plan := BuildPlan(Rule{Frequency: "weekly", Interval: 1, StartDate: start, NextRunAt: start, EndDate: &end, Active: true, AccountActive: true}, date(2026, 2, 1), 10)
	if len(plan.Dates) != 3 {
		t.Fatalf("%+v", plan)
	}
}
func TestIdempotentDuplicatePrevention(t *testing.T) {
	dates := []time.Time{date(2026, 1, 1), date(2026, 1, 8)}
	seen := map[string]bool{dates[0].Format(time.RFC3339Nano): true}
	first := FilterUnseen(dates, seen)
	second := FilterUnseen(dates, seen)
	if len(first) != 1 || len(second) != 0 {
		t.Fatalf("first=%v second=%v", first, second)
	}
}
func TestInactiveAndArchivedAccount(t *testing.T) {
	base := Rule{Frequency: "monthly", Interval: 1, StartDate: date(2026, 1, 1), NextRunAt: date(2026, 1, 1), AccountActive: true}
	if got := BuildPlan(base, date(2026, 2, 1), 10); len(got.Dates) != 0 {
		t.Fatal(got)
	}
	base.Active = true
	base.AccountActive = false
	got := BuildPlan(base, date(2026, 2, 1), 10)
	if !got.Pause || len(got.Dates) != 0 {
		t.Fatal(got)
	}
}
func TestDailyRecurrencePreservesWallClockAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip(err)
	}
	start := time.Date(2026, 3, 28, 9, 0, 0, 0, location)
	next := Next(start, start, "daily", 1)
	after := Next(next, start, "daily", 1)
	if next.Hour() != 9 || after.Hour() != 9 {
		t.Fatalf("wall clock drifted: %v %v", next, after)
	}
	if next.Sub(start) == after.Sub(next) {
		t.Fatal("expected UTC duration to change across DST")
	}
}
func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
