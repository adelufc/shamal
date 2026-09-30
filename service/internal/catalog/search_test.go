package catalog

import "testing"

func TestSearchRejectsMixedPricesAndBadBoxes(t *testing.T) {
	min := int64(100)
	max := int64(50)
	west, south, east, north := 1.0, 1.0, 0.0, 2.0
	issues := SearchIssues("", "newest", &min, nil, false, nil, nil, nil, nil)
	if !has(issues, "price_needs_transaction") {
		t.Fatalf("price without a transaction was accepted: %v", issues)
	}
	issues = SearchIssues("sale", "newest", &min, &max, false, nil, nil, nil, nil)
	if !has(issues, "invalid_price") {
		t.Fatalf("inverted price range was accepted: %v", issues)
	}
	issues = SearchIssues("daily", "sponsored", nil, nil, false, nil, nil, nil, nil)
	if !has(issues, "invalid_sort") {
		t.Fatalf("unknown sort was accepted: %v", issues)
	}
	issues = SearchIssues("sale", "price_asc", nil, nil, false, &west, &south, &east, &north)
	if len(issues) != 0 {
		t.Fatalf("a map box changed the search before it was requested: %v", issues)
	}
	issues = SearchIssues("sale", "price_asc", nil, nil, true, &west, &south, &east, &north)
	if !has(issues, "invalid_boundary") {
		t.Fatalf("bad area was accepted: %v", issues)
	}
	issues = SearchIssues("annual", "area_desc", nil, nil, false, nil, nil, nil, nil)
	if len(issues) != 0 {
		t.Fatalf("a valid search was rejected: %v", issues)
	}
}

func TestClustersStartAfterTheMarkerCap(t *testing.T) {
	if UseClusters(MarkerCap) || !UseClusters(MarkerCap+1) {
		t.Fatal("marker cap")
	}
	if OrderBy("price_asc") == OrderBy("newest") || OrderBy("nope") != OrderBy("newest") {
		t.Fatal("sort sql")
	}
}

func TestBedroomsAndAmenities(t *testing.T) {
	if _, issues := ParseBedrooms("0"); !has(issues, "invalid_rooms") {
		t.Fatal(issues)
	}
	rooms, issues := ParseBedrooms("3")
	if len(issues) != 0 || rooms == nil || *rooms != 3 {
		t.Fatalf("%v %v", rooms, issues)
	}
	_, _, _, _, _, _, issues = AmenityFlags("pool")
	if !has(issues, "invalid_amenity") {
		t.Fatal(issues)
	}
	furnished, cooling, _, _, _, _, amenityIssues := AmenityFlags("furnished,air_conditioning")
	issues = amenityIssues
	if len(issues) != 0 || !furnished || !cooling {
		t.Fatalf("%v %v %v", furnished, cooling, issues)
	}
}

func has(issues []string, code string) bool {
	for _, item := range issues {
		if item == code {
			return true
		}
	}
	return false
}
