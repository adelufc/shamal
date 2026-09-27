package catalog

import "testing"

func ready() ListingDraft {
	rooms := 3
	return ListingDraft{
		Transaction: "annual", TitleAR: "شقة", TitleEN: "Apartment",
		DescriptionAR: "وصف", DescriptionEN: "Description",
		PriceMinor: 12000000, Currency: "SAR", LicenseRef: "LIC-1",
		BrokerID: "broker", OwnerID: "owner", RequiresRooms: true, Rooms: &rooms,
		AreaSqm: 140, NeighborhoodPublished: true, ServiceEnabled: true, PointInside: true,
	}
}

func TestPublishableListingHasNoIssues(t *testing.T) {
	if issues := ListingIssues(ready()); len(issues) != 0 {
		t.Fatal(issues)
	}
}

func TestLandDoesNotRequireRooms(t *testing.T) {
	d := ready()
	d.Transaction = "sale"
	d.RequiresRooms = false
	d.Rooms = nil
	if issues := ListingIssues(d); len(issues) != 0 {
		t.Fatal(issues)
	}
}

func TestServiceAndBoundaryAndPrice(t *testing.T) {
	d := ready()
	d.ServiceEnabled = false
	d.PointInside = false
	d.PriceMinor = 0
	d.Currency = "USD"
	issues := ListingIssues(d)
	want := map[string]bool{"invalid_price": true, "currency_not_sar": true, "service_disabled": true, "outside_neighborhood": true}
	for _, issue := range issues {
		delete(want, issue)
	}
	if len(want) != 0 {
		t.Fatalf("missing %v in %v", want, issues)
	}
}

func TestLocationWaiverClearsOutside(t *testing.T) {
	d := ready()
	d.PointInside = false
	d.HasLocationWaiver = true
	if issues := ListingIssues(d); len(issues) != 0 {
		t.Fatal(issues)
	}
}

func TestListingTransitions(t *testing.T) {
	if !CanTransition("draft", "in_review") || !CanTransition("in_review", "approved") || !CanTransition("approved", "published") {
		t.Fatal("happy path")
	}
	if CanTransition("draft", "published") || CanTransition("published", "approved") {
		t.Fatal("skipped transition accepted")
	}
	if !SensitiveChange(true, false, false, false) || SensitiveChange(false, false, false, false) {
		t.Fatal("sensitive change detection")
	}
}
