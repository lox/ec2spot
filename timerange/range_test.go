package timerange_test

import (
	"testing"
	"time"

	"github.com/lox/ec2spot/timerange"
)

func TestTimeRangeContains(t *testing.T) {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	t2 := time.Date(2009, time.November, 17, 23, 0, 0, 0, time.UTC)
	r := timerange.Range{t1, t2}

	if r.Contains(time.Date(2009, time.November, 22, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("Shouldn't include dates outside of range")
	}

	if !r.Contains(time.Date(2009, time.November, 14, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("Should include dates inside of range")
	}

	if !r.Contains(t1) {
		t.Fatal("Should include dates equal to start")
	}

	if !r.Contains(t2) {
		t.Fatal("Should include dates equal to end")
	}
}

func TestTimeRangeDays(t *testing.T) {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	t2 := time.Date(2009, time.November, 17, 23, 0, 0, 0, time.UTC)

	r := timerange.Range{t1, t2}
	days := r.Days()

	if l := len(days); l != 7 {
		t.Fatalf("Expected 7 days, got %d", len(days))
	}
}

func TestTimeRangeSplit(t *testing.T) {
	t1 := time.Date(2009, time.November, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2009, time.November, 10, 20, 0, 0, 0, time.UTC)

	r := timerange.Range{t1, t2}
	split := r.Split(time.Hour)

	if l := len(split); l != 10 {
		t.Fatalf("Expected 10 hours, got %d", l)
	}
}
