package geom

import "testing"

func square(x, y, d float64) MultiPolygon {
	return MultiPolygon{Parts: []Polygon{{Rings: [][][2]float64{{
		{x, y}, {x + d, y}, {x + d, y + d}, {x, y + d}, {x, y},
	}}}}}
}

func TestValidateClosedSquare(t *testing.T) {
	if err := Validate(square(46.2, 24.3, 0.2)); err != nil {
		t.Fatal(err)
	}
}

func TestRejectOpenRing(t *testing.T) {
	m := MultiPolygon{Parts: []Polygon{{Rings: [][][2]float64{{
		{0, 0}, {1, 0}, {1, 1}, {0, 1},
	}}}}}
	if err := Validate(m); err == nil || err.Error() != "ring_not_closed" {
		t.Fatalf("got %v", err)
	}
}

func TestRejectSelfIntersection(t *testing.T) {
	m := MultiPolygon{Parts: []Polygon{{Rings: [][][2]float64{{
		{0, 0}, {1, 1}, {0, 1}, {1, 0}, {0, 0},
	}}}}}
	if err := Validate(m); err == nil || err.Error() != "self_intersection" {
		t.Fatalf("got %v", err)
	}
}

func TestCoveredByAndOverlap(t *testing.T) {
	city := square(46.2, 24.3, 1.2)
	inner := square(46.5, 24.6, 0.05)
	if !CoveredBy(inner, city) {
		t.Fatal("inner should be covered by city")
	}
	outside := square(40, 20, 0.1)
	if CoveredBy(outside, city) {
		t.Fatal("outside polygon was accepted")
	}
	other := square(46.6, 24.6, 0.05)
	if PositiveOverlap(inner, other) {
		t.Fatal("separated squares overlap")
	}
	cross := square(46.52, 24.62, 0.05)
	if !PositiveOverlap(inner, cross) {
		t.Fatal("expected positive overlap")
	}
	touch := square(46.55, 24.6, 0.05)
	if PositiveOverlap(inner, touch) {
		t.Fatal("shared edge should not count as area overlap")
	}
}

func TestParsePolygon(t *testing.T) {
	raw := []byte(`{"type":"Polygon","coordinates":[[[46.7,24.7],[46.72,24.7],[46.72,24.72],[46.7,24.72],[46.7,24.7]]]}`)
	m, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(m); err != nil {
		t.Fatal(err)
	}
	if !ContainsPoint(m, 46.71, 24.71) {
		t.Fatal("point should be inside")
	}
	if ContainsPoint(m, 46.9, 24.9) {
		t.Fatal("point should be outside")
	}
}
