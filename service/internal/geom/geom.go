package geom

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const MaxPoints = 10000

type Polygon struct {
	Rings [][][2]float64
}

type MultiPolygon struct {
	Parts []Polygon
}

func (m MultiPolygon) PointCount() int {
	n := 0
	for _, p := range m.Parts {
		for _, r := range p.Rings {
			n += len(r)
		}
	}
	return n
}

func Parse(raw []byte) (MultiPolygon, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return MultiPolygon{}, fmt.Errorf("invalid_geojson")
	}
	switch head.Type {
	case "Polygon":
		var p struct {
			Coordinates [][][2]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal(raw, &p); err != nil || len(p.Coordinates) == 0 {
			return MultiPolygon{}, fmt.Errorf("invalid_geojson")
		}
		return MultiPolygon{Parts: []Polygon{{Rings: p.Coordinates}}}, nil
	case "MultiPolygon":
		var p struct {
			Coordinates [][][][2]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal(raw, &p); err != nil || len(p.Coordinates) == 0 {
			return MultiPolygon{}, fmt.Errorf("invalid_geojson")
		}
		parts := make([]Polygon, 0, len(p.Coordinates))
		for _, rings := range p.Coordinates {
			if len(rings) == 0 {
				return MultiPolygon{}, fmt.Errorf("invalid_geojson")
			}
			parts = append(parts, Polygon{Rings: rings})
		}
		return MultiPolygon{Parts: parts}, nil
	default:
		return MultiPolygon{}, fmt.Errorf("invalid_geojson")
	}
}

func Validate(m MultiPolygon) error {
	if len(m.Parts) == 0 {
		return errors.New("invalid_boundary")
	}
	if m.PointCount() > MaxPoints {
		return errors.New("boundary_too_large")
	}
	for _, part := range m.Parts {
		if err := validateRing(part.Rings[0]); err != nil {
			return err
		}
	}
	return nil
}

func CoveredBy(inner, outer MultiPolygon) bool {
	for _, part := range inner.Parts {
		inside := false
		for _, city := range outer.Parts {
			if ringCoveredBy(part.Rings[0], city.Rings[0]) {
				inside = true
				break
			}
		}
		if !inside {
			return false
		}
	}
	return true
}

func PositiveOverlap(a, b MultiPolygon) bool {
	for _, pa := range a.Parts {
		for _, pb := range b.Parts {
			if ringsOverlap(pa.Rings[0], pb.Rings[0]) {
				return true
			}
		}
	}
	return false
}

func ContainsPoint(m MultiPolygon, lon, lat float64) bool {
	p := [2]float64{lon, lat}
	for _, part := range m.Parts {
		if pointStrictlyInside(p, part.Rings[0]) || pointOnRing(p, part.Rings[0]) {
			return true
		}
	}
	return false
}

func validateRing(ring [][2]float64) error {
	if len(ring) < 4 {
		return errors.New("invalid_boundary")
	}
	if !same(ring[0], ring[len(ring)-1]) {
		return errors.New("ring_not_closed")
	}
	if selfIntersects(ring) {
		return errors.New("self_intersection")
	}
	if math.Abs(signedArea(ring)) < 1e-12 {
		return errors.New("non_positive_area")
	}
	return nil
}

func ringCoveredBy(inner, outer [][2]float64) bool {
	for _, p := range inner {
		if !pointStrictlyInside(p, outer) && !pointOnRing(p, outer) {
			return false
		}
	}
	return true
}

func ringsOverlap(a, b [][2]float64) bool {
	for _, p := range a[:len(a)-1] {
		if pointStrictlyInside(p, b) {
			return true
		}
	}
	for _, p := range b[:len(b)-1] {
		if pointStrictlyInside(p, a) {
			return true
		}
	}
	for i := 0; i < len(a)-1; i++ {
		for j := 0; j < len(b)-1; j++ {
			if properIntersect(a[i], a[i+1], b[j], b[j+1]) {
				return true
			}
		}
	}
	return false
}

func selfIntersects(ring [][2]float64) bool {
	n := len(ring) - 1
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if abs(i-j) <= 1 || (i == 0 && j == n-1) {
				continue
			}
			if properIntersect(ring[i], ring[i+1], ring[j], ring[j+1]) {
				return true
			}
		}
	}
	return false
}

func signedArea(ring [][2]float64) float64 {
	sum := 0.0
	for i := 0; i < len(ring)-1; i++ {
		sum += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return sum / 2
}

func pointStrictlyInside(p [2]float64, ring [][2]float64) bool {
	if pointOnRing(p, ring) {
		return false
	}
	inside := false
	for i, j := 0, len(ring)-2; i < len(ring)-1; j, i = i, i+1 {
		yi, yj := ring[i][1], ring[j][1]
		xi, xj := ring[i][0], ring[j][0]
		if (yi > p[1]) != (yj > p[1]) {
			x := (xj-xi)*(p[1]-yi)/(yj-yi) + xi
			if p[0] < x {
				inside = !inside
			}
		}
	}
	return inside
}

func pointOnRing(p [2]float64, ring [][2]float64) bool {
	for i := 0; i < len(ring)-1; i++ {
		if pointOnSegment(p, ring[i], ring[i+1]) {
			return true
		}
	}
	return false
}

func pointOnSegment(p, a, b [2]float64) bool {
	if !collinear(a, b, p) {
		return false
	}
	return p[0] >= math.Min(a[0], b[0])-1e-12 && p[0] <= math.Max(a[0], b[0])+1e-12 &&
		p[1] >= math.Min(a[1], b[1])-1e-12 && p[1] <= math.Max(a[1], b[1])+1e-12
}

func properIntersect(a, b, c, d [2]float64) bool {
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	if o1 == 0 || o2 == 0 || o3 == 0 || o4 == 0 {
		return false
	}
	return o1 != o2 && o3 != o4
}

func orient(a, b, c [2]float64) int {
	v := (b[1]-a[1])*(c[0]-b[0]) - (b[0]-a[0])*(c[1]-b[1])
	if math.Abs(v) < 1e-15 {
		return 0
	}
	if v > 0 {
		return 1
	}
	return -1
}

func collinear(a, b, c [2]float64) bool {
	return orient(a, b, c) == 0
}

func same(a, b [2]float64) bool {
	return math.Abs(a[0]-b[0]) < 1e-9 && math.Abs(a[1]-b[1]) < 1e-9
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
