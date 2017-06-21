package timerange

import (
	"fmt"
	"time"
)

type Range [2]time.Time

func (r Range) String() string {
	return fmt.Sprintf("%s - %s", r[0].String(), r[1].String())
}

func (r Range) Contains(t time.Time) bool {
	return (t.Before(r[1]) && t.After(r[0])) || t.Equal(r[0]) || t.Equal(r[1])
}

func (r Range) Split(d time.Duration) []Range {
	parts := []Range{}
	start := r[0]
	end := r[0].Add(d).Add(-time.Second)

	for r.Contains(end) {
		parts = append(parts, Range{start, end})
		start = start.Add(d)
		end = start.Add(d).Add(-time.Second)
	}

	return parts
}

func (r Range) Days() []Range {
	days := []Range{}
	day := Day(r[0])

	for r.Contains(day[1]) {
		days = append(days, day)
		day = Tomorrow(day[0])
	}

	return days
}

func StartOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func Day(t time.Time) Range {
	start := StartOfDay(t)
	return Range{start, start.AddDate(0, 0, 1).Add(-time.Second)}
}

func Tomorrow(t time.Time) Range {
	start := StartOfDay(t).AddDate(0, 0, 1)
	return Range{start, start.AddDate(0, 0, 1).Add(-time.Second)}
}
