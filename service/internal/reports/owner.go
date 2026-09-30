package reports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"shamal/service/internal/platform"
)

type OwnerListing struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	TransactionType string `json:"transaction_type"`
	UnitNumber      string `json:"unit_number"`
	TitleAR         string `json:"title_ar"`
	TitleEN         string `json:"title_en"`
}

type OwnerView struct {
	GeneratedAt        time.Time      `json:"generated_at"`
	Timezone           string         `json:"timezone"`
	PeriodStart        string         `json:"period_start"`
	PeriodEnd          string         `json:"period_end"`
	DefinitionAR       string         `json:"definition_ar"`
	DefinitionEN       string         `json:"definition_en"`
	ExclusionsAR       string         `json:"exclusions_ar"`
	ExclusionsEN       string         `json:"exclusions_en"`
	CommissionIncluded bool           `json:"commission_included"`
	Measures           []Measure      `json:"measures"`
	Listings           []OwnerListing `json:"listings"`
}

func (s Service) Owner(ctx context.Context, userID, from, to string, now time.Time) (OwnerView, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return OwnerView{}, platform.E("validation", 422)
	}
	period, err := ParsePeriod(from, to, now)
	if err != nil {
		return OwnerView{}, platform.E("invalid_dates", 422)
	}
	var units, total, published int64
	err = s.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*)::bigint FROM catalog.units u JOIN catalog.properties p ON p.id = u.property_id WHERE p.owner_user_id = $1::uuid),
		  (SELECT count(*)::bigint FROM catalog.listings l JOIN catalog.properties p ON p.id = l.property_id WHERE p.owner_user_id = $1::uuid),
		  (SELECT count(*)::bigint FROM catalog.listings l JOIN catalog.properties p ON p.id = l.property_id WHERE p.owner_user_id = $1::uuid AND l.status = 'published')
	`, userID).Scan(&units, &total, &published)
	if err != nil {
		return OwnerView{}, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT l.id::text, l.status, l.transaction_type, u.unit_number,
		       COALESCE((SELECT title FROM catalog.listing_translations WHERE listing_id = l.id AND locale = 'ar'), ''),
		       COALESCE((SELECT title FROM catalog.listing_translations WHERE listing_id = l.id AND locale = 'en'), '')
		FROM catalog.listings l
		JOIN catalog.properties p ON p.id = l.property_id
		JOIN catalog.units u ON u.id = l.unit_id
		WHERE p.owner_user_id = $1::uuid
		ORDER BY l.created_at DESC, l.id
	`, userID)
	if err != nil {
		return OwnerView{}, err
	}
	defer rows.Close()
	listings := make([]OwnerListing, 0)
	for rows.Next() {
		var item OwnerListing
		if err := rows.Scan(&item.ID, &item.Status, &item.TransactionType, &item.UnitNumber, &item.TitleAR, &item.TitleEN); err != nil {
			return OwnerView{}, err
		}
		listings = append(listings, item)
	}
	if err := rows.Err(); err != nil {
		return OwnerView{}, err
	}
	stayRows, err := s.Pool.Query(ctx, `
		SELECT b.unit_id::text, b.arrival, b.departure
		FROM stays.bookings b
		JOIN catalog.listings l ON l.id = b.listing_id
		JOIN catalog.properties p ON p.id = l.property_id
		WHERE p.owner_user_id = $1::uuid
		  AND b.status = 'confirmed'
		  AND b.arrival < $2::date AND b.departure > $3::date
	`, userID, period.End, period.From)
	if err != nil {
		return OwnerView{}, err
	}
	defer stayRows.Close()
	nights := 0
	occupied := map[string]struct{}{}
	for stayRows.Next() {
		var unit string
		var arrival, departure time.Time
		if err := stayRows.Scan(&unit, &arrival, &departure); err != nil {
			return OwnerView{}, err
		}
		overlap := OverlapNights(arrival, departure, period.From, period.End)
		if overlap > 0 {
			nights += overlap
			occupied[unit] = struct{}{}
		}
	}
	if err := stayRows.Err(); err != nil {
		return OwnerView{}, err
	}
	view := OwnerView{
		GeneratedAt: now.UTC(), Timezone: Timezone,
		PeriodStart: period.FromText, PeriodEnd: period.ToText,
		DefinitionAR:       "عدادات وحدات المالك وإعلاناته، وليالي الإشغال المؤكدة على وحداته فقط.",
		DefinitionEN:       "Counts of this owner's units and listings, and confirmed occupied nights on those units only.",
		ExclusionsAR:       "وحدات الملاك الآخرين غير داخلة. التثبيت والإلغاء لا يُحسبان ليالي. نسبة الإشغال فارغة لأن مقام السعة غير معتمد. لا عمولة ولا ضريبة ولا تسوية ولا إدارة إيراد.",
		ExclusionsEN:       "Other owners' units are excluded. Holds and cancellations are not occupied nights. The occupancy rate stays empty because a capacity denominator is not adopted. Commission, tax, settlement, and revenue management are not included.",
		CommissionIncluded: false,
		Listings:           listings,
		Measures: []Measure{
			{Code: "units", Value: units, Unit: "count", DefinitionAR: "عدد الوحدات على عقارات هذا المالك.", DefinitionEN: "Units on this owner's properties."},
			{Code: "listings_total", Value: total, Unit: "count", DefinitionAR: "عدد الإعلانات بكل حالاتها على عقارات هذا المالك.", DefinitionEN: "Listings in every status on this owner's properties."},
			{Code: "listings_published", Value: published, Unit: "count", DefinitionAR: "الإعلانات المنشورة فقط.", DefinitionEN: "Published listings only."},
			{Code: "occupied_nights", Value: int64(nights), Unit: "nights", DefinitionAR: "ليالي الإقامة المؤكدة التي تتقاطع مع النافذة على وحدات هذا المالك.", DefinitionEN: "Confirmed stay nights that overlap the window on this owner's units."},
			{Code: "distinct_units", Value: int64(len(occupied)), Unit: "count", DefinitionAR: "الوحدات المختلفة التي لها ليلة مشغولة داخل النافذة.", DefinitionEN: "Distinct units with an occupied night inside the window."},
			{Code: "occupancy_rate", Value: nil, Unit: "ratio", DefinitionAR: "النسبة غير معتمدة. لا يُخترع مقام للسعة ولا يُحسب ADR أو RevPAR.", DefinitionEN: "The rate is not adopted. No capacity denominator is invented, and ADR or RevPAR are not calculated."},
		},
	}
	return view, nil
}
