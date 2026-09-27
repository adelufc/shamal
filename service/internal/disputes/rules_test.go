package disputes

import "testing"
import "time"

func TestDisputeRules(t *testing.T) {
	if Allowed("open", "closed") || Allowed("open", "resolved") || !Allowed("open", "classified") {
		t.Fatal("open transitions")
	}
	if !Allowed("proposed", "resolved") || Allowed("resolved", "proposed") || !Allowed("closed", "reopened") {
		t.Fatal("decision transitions")
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	arrival := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	departure := arrival.Add(48 * time.Hour)
	if !Critical("entry_failure", arrival, departure, now) {
		t.Fatal("current stay is critical")
	}
	if Critical("cleanliness", arrival, departure, now) || Critical("entry_failure", arrival, departure, arrival.Add(-time.Hour)) {
		t.Fatal("other cases stay normal")
	}
	resolved := now
	if !WithinObjection(resolved, now.Add(6*24*time.Hour)) || WithinObjection(resolved, now.Add(8*24*time.Hour)) {
		t.Fatal("objection window")
	}
}
