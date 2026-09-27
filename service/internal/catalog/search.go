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
