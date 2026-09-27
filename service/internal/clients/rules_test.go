package clients

import "testing"

func TestRequestPathAndRejectedJumps(t *testing.T) {
	path := []string{StageNew, StageContacted, StageInterested, StageVisit, StageNegotiation, StageAgreement, StageConverted}
	for i := 0; i < len(path)-1; i++ {
		if !CanTransition(path[i], path[i+1]) {
			t.Fatalf("expected %s -> %s", path[i], path[i+1])
		}
	}
	if CanTransition(StageNew, StageConverted) {
		t.Fatal("a new request cannot jump to converted")
	}
	if CanTransition(StageConverted, StageNegotiation) {
		t.Fatal("converted is terminal")
	}
	if TransitionIssue(StageInterested, StageUnqualified, "") != "reason_required" {
		t.Fatal("unqualified requires a reason")
	}
	if TransitionIssue(StageInterested, StageUnqualified, "لا يطابق الاحتياج") != "" {
		t.Fatal("reason should allow closing as unqualified")
	}
}

func TestBudgetIsEnteredNotAssumed(t *testing.T) {
	if issues := RequestIssues("sale", "phone", "n", "SAR", nil); len(issues) != 0 {
		t.Fatalf("missing budget is allowed, got %v", issues)
	}
	amount := int64(0)
	issues := RequestIssues("sale", "phone", "n", "USD", &amount)
	if !has(issues, "invalid_price") || !has(issues, "currency_not_sar") {
		t.Fatalf("got %v", issues)
	}
}

func TestClientJourneyHidesInternalStages(t *testing.T) {
	if ClientJourney(StageContacted, "") != "follow_up" {
		t.Fatal("contacted is follow-up for the client")
	}
	if ClientJourney(StageAgreement, "closed_success") != "completed" {
		t.Fatal("successful close is completed")
	}
	if ClientJourney(StageNegotiation, "cancelled") != "closed_without_agreement" {
		t.Fatal("cancelled deal is closed without agreement")
	}
}

func has(items []string, code string) bool {
	for _, item := range items {
		if item == code {
			return true
		}
	}
	return false
}
