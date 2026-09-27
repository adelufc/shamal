package deals

import (
	"testing"
	"time"
)

func TestDealPathAndRejectedMoves(t *testing.T) {
	path := []string{Draft, Qualified, Negotiation, AwaitingDocs, Preliminary, AwaitingExternal, ClosedSuccess}
	for i := 0; i < len(path)-1; i++ {
		if !CanTransition(path[i], path[i+1]) {
			t.Fatalf("expected %s -> %s", path[i], path[i+1])
		}
	}
	if CanTransition(Draft, ClosedSuccess) {
		t.Fatal("an accepted offer path cannot skip the close gate")
	}
	if CanTransition(ClosedSuccess, Negotiation) {
		t.Fatal("a closed deal cannot move again")
	}
	if MoveIssue(Negotiation, Qualified, "") != "reason_required" {
		t.Fatal("going back needs a reason")
	}
	if MoveIssue(Negotiation, Qualified, "السعر غير مكتمل") != "" {
		t.Fatal("a reason should keep the return")
	}
	if MoveIssue(Qualified, ClosedLost, "") != "reason_required" {
		t.Fatal("a lost deal needs a reason")
	}
}

func TestCloseGateDoesNotInventTerms(t *testing.T) {
	in := CloseInput{
		AgreementRef: "AGR-1", AmountMinor: 100, Currency: "SAR",
		ClientID: "c", OwnerID: "o", BrokerID: "b",
		CloseDate:   time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC),
		RequestedBy: "editor", ApproverID: "manager",
	}
	if issues := CloseIssues(in); len(issues) != 0 {
		t.Fatalf("complete close was rejected: %v", issues)
	}
	in.ApproverID = "editor"
	if !has(CloseIssues(in), "separation_of_duties") {
		t.Fatal("the person who requested the close cannot approve it")
	}
	in.ApproverID = "manager"
	in.HasCommission = true
	if !has(CloseIssues(in), "commercial_terms_unapproved") {
		t.Fatal("a commission was accepted without an adopted rule")
	}
	in.HasCommission = false
	in.HasCancelFee = true
	if !has(CloseIssues(in), "commercial_terms_unapproved") {
		t.Fatal("a cancellation fee was accepted")
	}
	in.HasCancelFee = false
	in.AgreementRef = ""
	in.AmountMinor = 0
	if issues := CloseIssues(in); !has(issues, "agreement_required") || !has(issues, "invalid_price") {
		t.Fatalf("close gate %v", issues)
	}
	if !AudienceExcluded(Cancelled, false) || !AudienceExcluded(ClosedSuccess, true) || AudienceExcluded(ClosedSuccess, false) {
		t.Fatal("audience exclusion mismatch")
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
