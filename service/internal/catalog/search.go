package catalog

import "strings"

const MarkerCap = 500

type SearchQuery struct {
	Locale         string
	NeighborhoodID string
	Transaction    string
	PriceMin       *int64
	PriceMax       *int64
	Sort           string
	Text           string
	AllowedIDs     []string
	NameIDs        []string
	Page           int
	Size           int
	RoomsMin       *int
	Furnished      bool
	Cooling        bool
	Internet       bool
	Parking        bool
	Bedding        bool
	SelfEntry      bool
	Verified       bool
}

type SearchHit struct {
	ID              string  `json:"id"`
	NeighborhoodID  string  `json:"neighborhood_id"`
	TransactionType string  `json:"transaction_type"`
	PriceMinor      int64   `json:"price_minor"`
	PriceUnit       string  `json:"price_unit"`
	Currency        string  `json:"currency"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	AreaSqm         float64 `json:"area_sqm"`
	Rooms           *int    `json:"rooms"`
	TypeCode        string  `json:"type_code"`
	OwnerVerified   bool    `json:"owner_verified"`
}

type SearchMarker struct {
	ID              string  `json:"id"`
	NeighborhoodID  string  `json:"neighborhood_id"`
	TransactionType string  `json:"transaction_type"`
	PriceMinor      int64   `json:"price_minor"`
	PriceUnit       string  `json:"price_unit"`
	Currency        string  `json:"currency"`
	Title           string  `json:"title"`
	Lon             float64 `json:"lon"`
	Lat             float64 `json:"lat"`
}

type SearchCluster struct {
	NeighborhoodID string  `json:"neighborhood_id"`
	Count          int     `json:"count"`
	PriceMin       int64   `json:"price_min_minor"`
	PriceMax       int64   `json:"price_max_minor"`
	Lon            float64 `json:"lon,omitempty"`
	Lat            float64 `json:"lat,omitempty"`
}

func SearchIssues(transaction, sort string, priceMin, priceMax *int64, inArea bool, west, south, east, north *float64) []string {
	issues := make([]string, 0)
	switch transaction {
	case "", "sale", "annual", "monthly", "daily":
	default:
		issues = append(issues, "invalid_transaction")
	}
	switch sort {
	case "", "newest", "price_asc", "price_desc", "area_asc", "area_desc":
	default:
		issues = append(issues, "invalid_sort")
	}
	if priceMin != nil || priceMax != nil {
		if transaction == "" {
			issues = append(issues, "price_needs_transaction")
		}
		if priceMin != nil && *priceMin <= 0 {
			issues = append(issues, "invalid_price")
		}
		if priceMax != nil && *priceMax <= 0 {
			issues = append(issues, "invalid_price")
		}
		if priceMin != nil && priceMax != nil && *priceMin > *priceMax {
			issues = append(issues, "invalid_price")
		}
	}
	if inArea {
		if west == nil || south == nil || east == nil || north == nil || *west >= *east || *south >= *north {
			issues = append(issues, "invalid_boundary")
		}
	}
	return issues
}

func UseClusters(total int) bool {
	return total > MarkerCap
}

func OrderBy(sort string) string {
	switch sort {
	case "price_asc":
		return "l.price_minor ASC, l.id ASC"
	case "price_desc":
		return "l.price_minor DESC, l.id DESC"
	case "area_asc":
		return "p.area_sqm ASC NULLS LAST, l.id ASC"
	case "area_desc":
		return "p.area_sqm DESC NULLS LAST, l.id DESC"
	default:
		return "COALESCE((SELECT max(h.recorded_at) FROM catalog.price_history h WHERE h.listing_id = l.id), l.created_at) DESC, l.id DESC"
	}
}

func NormalizeSearchText(value string) string {
	return strings.TrimSpace(value)
}

func AmenityFlags(raw string) (furnished, cooling, internet, parking, bedding, entry bool, issues []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(part) {
		case "":
		case "furnished":
			furnished = true
		case "air_conditioning":
			cooling = true
		case "internet":
			internet = true
		case "parking":
			parking = true
		case "bedding":
			bedding = true
		case "self_entry":
			entry = true
		default:
			issues = append(issues, "invalid_amenity")
		}
	}
	return
}

func ParseBedrooms(raw string) (*int, []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return nil, []string{"invalid_rooms"}
		}
		value = value*10 + int(r-'0')
	}
	if value < 1 || value > 30 {
		return nil, []string{"invalid_rooms"}
	}
	return &value, nil
}
