package timerange

import (
	"fmt"
	"time"
)

// Range is a timeframe between two time points
type Range [2]time.Time

// String returns a string representation of the time range
func (r Range) String() string {
	return fmt.Sprintf("%s - %s", r[0].String(), r[1].String())
}

// Contains returns true if t falls within the current Range
func (r Range) Contains(t time.Time) bool {
	return (t.Before(r[1]) && t.After(r[0])) || t.Equal(r[0]) || t.Equal(r[1])
}

// Split returns a slice of Ranges for chunks of t duration during the current Range
func (r Range) Split(d time.Duration) []Range {
	parts := []Range{}
	start := r[0]
	end := r[0].Add(d).Add(-time.Second)

	for r.Contains(end) {
		parts = append(parts, Range{start, end})
		start = start.Add(d)
		end = start.Add(d).Add(-time.Second)
	}

	// if r.Contains(start) {
	// 	parts = append(parts, Range{start, r[1]})
	// }

	return parts
}

// AddDate shifts the current range by the specified amount of time
func (r Range) AddDate(years int, months int, days int) Range {
	return Range{r[0].AddDate(years, months, days), r[1].AddDate(years, months, days)}
}

// Days returns a slice of Ranges for each day within the current Range
func (r Range) Days() []Range {
	days := []Range{}
	day := Day(r[0])

	for r.Contains(day[1]) {
		days = append(days, day)
		day = day.AddDate(0, 0, 1)
	}

	return days
}

// StartOfDay returns the first second of the day that t falls within
func StartOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// Day returns a range from the start to end of the day in t
func Day(t time.Time) Range {
	start := StartOfDay(t)
	return Range{start, start.AddDate(0, 0, 1).Add(-time.Second)}
}

// DaysAgo returns a range of days from N days ago to t
func DaysAgo(t time.Time, daysAgo int) Range {
	return Range{t.AddDate(0, 0, daysAgo*-1), t}
}
