package recurrence

import "time"

type Rule struct {
	Frequency     string
	Interval      int
	StartDate     time.Time
	NextRunAt     time.Time
	EndDate       *time.Time
	Active        bool
	AccountActive bool
}

type Plan struct {
	Dates []time.Time
	Pause bool
}

func Next(current, start time.Time, frequency string, interval int) time.Time {
	if interval < 1 {
		interval = 1
	}
	switch frequency {
	case "daily":
		return current.AddDate(0, 0, interval)
	case "weekly":
		return current.AddDate(0, 0, 7*interval)
	case "yearly":
		year := current.Year() + interval
		month := start.Month()
		day := min(start.Day(), daysInMonth(year, month))
		return time.Date(year, month, day, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
	default:
		base := time.Date(current.Year(), current.Month(), 1, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location()).AddDate(0, interval, 0)
		day := min(start.Day(), daysInMonth(base.Year(), base.Month()))
		return time.Date(base.Year(), base.Month(), day, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
	}
}

func BuildPlan(rule Rule, now time.Time, max int) Plan {
	result := Plan{Dates: make([]time.Time, 0)}
	if !rule.Active {
		return result
	}
	if !rule.AccountActive {
		result.Pause = true
		return result
	}
	if max < 1 {
		max = 100
	}
	current := rule.NextRunAt
	for len(result.Dates) < max && !current.After(now) {
		if rule.EndDate != nil && dateAfter(current, *rule.EndDate) {
			break
		}
		result.Dates = append(result.Dates, current)
		current = Next(current, rule.StartDate, rule.Frequency, rule.Interval)
	}
	return result
}

func FilterUnseen(dates []time.Time, seen map[string]bool) []time.Time {
	result := make([]time.Time, 0, len(dates))
	for _, date := range dates {
		key := date.UTC().Format(time.RFC3339Nano)
		if !seen[key] {
			result = append(result, date)
			seen[key] = true
		}
	}
	return result
}
func dateAfter(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	if ay != by {
		return ay > by
	}
	if am != bm {
		return am > bm
	}
	return ad > bd
}
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
