package geography

import (
	"strings"
	"unicode"
)

func Normalize(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch r {
		case 'أ', 'إ', 'آ', 'ٱ':
			b.WriteRune('ا')
		case 'ى':
			b.WriteRune('ي')
		case 'ة':
			b.WriteRune('ه')
		case 'ؤ':
			b.WriteRune('و')
		case 'ئ':
			b.WriteRune('ي')
		case 'ـ':
			continue
		default:
			if r >= 0x064B && r <= 0x0652 {
				continue
			}
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func NameLengthOK(name string) bool {
	n := len([]rune(strings.TrimSpace(name)))
	return n >= 2 && n <= 80
}

func CanTransition(from, to string) bool {
	switch from {
	case "draft":
		return to == "in_review"
	case "in_review":
		return to == "changes_requested" || to == "published"
	case "changes_requested":
		return to == "draft" || to == "in_review"
	case "published":
		return to == "suspended"
	case "suspended":
		return to == "published"
	default:
		return false
	}
}

type Draft struct {
	NameAR          string
	NameEN          string
	HasBoundary     bool
	BoundaryValid   bool
	InsideCity      bool
	Overlaps        bool
	OfficialClaim   bool
	CityOfficial    bool
	AllowApprox     bool
	Duplicate       bool
}

func Issues(d Draft) []string {
	issues := make([]string, 0)
	if !NameLengthOK(d.NameAR) || !NameLengthOK(d.NameEN) {
		issues = append(issues, "name_required")
	}
	if !d.HasBoundary || !d.BoundaryValid {
		issues = append(issues, "invalid_boundary")
	}
	if d.HasBoundary && d.BoundaryValid && !d.InsideCity {
		issues = append(issues, "outside_city")
	}
	if d.Overlaps {
		issues = append(issues, "boundary_overlap")
	}
	if d.Duplicate {
		issues = append(issues, "duplicate_neighborhood")
	}
	if !d.CityOfficial && !d.AllowApprox {
		issues = append(issues, "city_boundary_unapproved")
	}
	if d.OfficialClaim && !d.CityOfficial {
		issues = append(issues, "official_boundary_unapproved")
	}
	return issues
}
