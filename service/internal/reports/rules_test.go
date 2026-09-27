package reports

import (
	"testing"
	"time"
)

func TestRiyadhWindow(t *testing.T) {
	period, err := ParsePeriod("2032-11-17", "2032-11-17", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if period.StartAt.UTC().Format(time.RFC3339) != "2032-11-16T21:00:00Z" {
		t.Fatalf("start %s", period.StartAt.UTC().Format(time.RFC3339))
	}
	if period.EndAt.UTC().Format(time.RFC3339) != "2032-11-17T21:00:00Z" {
		t.Fatalf("end %s", period.EndAt.UTC().Format(time.RFC3339))
	}
	if _, err := ParsePeriod("2032-11-18", "2032-11-17", time.Now()); err == nil {
		t.Fatal("reversed period was accepted")
	}
}

func TestOverlapNights(t *testing.T) {
	arrival := date(2032, 11, 17)
	departure := date(2032, 11, 19)
	if got := OverlapNights(arrival, departure, date(2032, 11, 17), date(2032, 11, 18)); got != 1 {
		t.Fatalf("one night overlap = %d", got)
	}
	if got := OverlapNights(arrival, departure, date(2032, 11, 17), date(2032, 11, 19)); got != 2 {
		t.Fatalf("two night overlap = %d", got)
	}
	if got := OverlapNights(arrival, departure, date(2032, 11, 19), date(2032, 11, 20)); got != 0 {
		t.Fatalf("no overlap = %d", got)
	}
}

func TestRatio(t *testing.T) {
	if Ratio(0, 0) != nil {
		t.Fatal("zero denominator returned a ratio")
	}
	if Ratio(1, 2) != 0.5 {
		t.Fatalf("ratio %v", Ratio(1, 2))
	}
}

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}
