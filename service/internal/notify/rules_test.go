package notify

import "testing"

func TestChannelAndReview(t *testing.T) {
	if err := AllowChannel("in_app"); err != nil {
		t.Fatal(err)
	}
	if err := AllowChannel("email"); err == nil || err.Error() == "" {
		t.Fatal("email channel was accepted")
	}
	if !ReviewStatus("pending_review") || !ReviewStatus("mismatch_review") || ReviewStatus("collected") {
		t.Fatal("review status gate changed")
	}
}

func TestDedupKeys(t *testing.T) {
	if DedupListing("u", "l", "s") != "u|listing_match|l|s" {
		t.Fatal(DedupListing("u", "l", "s"))
	}
	if DedupVisit("u", "v", "confirmed", 2) != "u|visit_change|v|confirmed|2" {
		t.Fatal(DedupVisit("u", "v", "confirmed", 2))
	}
	if DedupPayment("u", "a", "pending_review") != "u|payment_review|a|pending_review" {
		t.Fatal(DedupPayment("u", "a", "pending_review"))
	}
}
