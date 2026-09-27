package geography

import "testing"

func TestNormalizeIgnoresSpellingVariants(t *testing.T) {
	if Normalize("الْنَّرْجِس") != Normalize("النرجس") {
		t.Fatalf("%q", Normalize("الْنَّرْجِس"))
	}
	if Normalize("الياسمين") == Normalize("النرجس") {
		t.Fatal("different names collapsed")
	}
}

func TestPublishIssues(t *testing.T) {
	ok := Draft{NameAR: "النرجس", NameEN: "Al Narjis", HasBoundary: true, BoundaryValid: true, InsideCity: true, AllowApprox: true}
	if issues := Issues(ok); len(issues) != 0 {
		t.Fatal(issues)
	}
	blocked := ok
	blocked.AllowApprox = false
	blocked.Overlaps = true
	blocked.Duplicate = true
	issues := Issues(blocked)
	got := map[string]bool{}
	for _, issue := range issues {
		got[issue] = true
	}
	for _, code := range []string{"boundary_overlap", "duplicate_neighborhood", "city_boundary_unapproved"} {
		if !got[code] {
			t.Fatalf("missing %s in %v", code, issues)
		}
	}
}

func TestNeighborhoodTransitions(t *testing.T) {
	if !CanTransition("draft", "in_review") || !CanTransition("in_review", "published") || !CanTransition("published", "suspended") {
		t.Fatal("expected transitions")
	}
	if CanTransition("draft", "published") || CanTransition("archived", "published") {
		t.Fatal("illegal transition")
	}
}

func TestNameBounds(t *testing.T) {
	if NameLengthOK("ن") || !NameLengthOK("النرجس") {
		t.Fatal("name length")
	}
}
