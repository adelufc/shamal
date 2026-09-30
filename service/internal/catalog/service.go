package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/audit"
	"shamal/service/internal/platform"
)

type Type struct {
	Code          string `json:"code"`
	NameAR        string `json:"name_ar"`
	NameEN        string `json:"name_en"`
	RequiresRooms bool   `json:"requires_rooms"`
}

type Property struct {
	ID             string    `json:"id"`
	CompanyID      string    `json:"company_id"`
	BranchID       string    `json:"branch_id"`
	CityID         string    `json:"city_id"`
	NeighborhoodID string    `json:"neighborhood_id"`
	OwnerUserID    string    `json:"owner_user_id"`
	DelegationRef  string    `json:"delegation_ref"`
	AddressAR      string    `json:"address_ar"`
	AddressEN      string    `json:"address_en"`
	TypeCode       string    `json:"type_code"`
	AreaSqm        float64   `json:"area_sqm"`
	PreciseLon     *float64  `json:"precise_lon"`
	PreciseLat     *float64  `json:"precise_lat"`
	Version        int       `json:"version"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type Unit struct {
	ID         string `json:"id"`
	PropertyID string `json:"property_id"`
	Number     string `json:"unit_number"`
	Floor      *int   `json:"floor"`
	Rooms      *int   `json:"rooms"`
	Baths      *int   `json:"baths"`
	Status     string `json:"status"`
	Version    int    `json:"version"`
}

type Translation struct {
	Locale      string `json:"locale"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Listing struct {
	ID                  string        `json:"id"`
	PropertyID          string        `json:"property_id"`
	UnitID              string        `json:"unit_id"`
	NeighborhoodID      string        `json:"neighborhood_id"`
	CompanyID           string        `json:"company_id"`
	BranchID            string        `json:"branch_id"`
	TransactionType     string        `json:"transaction_type"`
	PriceMinor          int64         `json:"price_minor"`
	PriceUnit           string        `json:"price_unit"`
	Currency            string        `json:"currency"`
	LicenseRef          string        `json:"license_ref"`
	LicenseExceptionRef string        `json:"license_exception_ref"`
	BrokerUserID        string        `json:"broker_user_id"`
	Status              string        `json:"status"`
	Version             int           `json:"version"`
	Issues              []string      `json:"issues"`
	Pending             any           `json:"pending_changes,omitempty"`
	CreatedBy           string        `json:"created_by"`
	CreatedAt           time.Time     `json:"created_at"`
	Texts               []Translation `json:"texts"`
	AreaSqm             float64       `json:"area_sqm"`
	OwnerUserID         string        `json:"owner_user_id"`
	TypeCode            string        `json:"type_code"`
	Rooms               *int          `json:"rooms"`
	PreciseLon          *float64      `json:"precise_lon"`
	PreciseLat          *float64      `json:"precise_lat"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Types(ctx context.Context) ([]Type, error) {
	rows, err := s.Pool.Query(ctx, `SELECT code, name_ar, name_en, requires_rooms FROM catalog.property_types ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Type, 0)
	for rows.Next() {
		var item Type
		if err := rows.Scan(&item.Code, &item.NameAR, &item.NameEN, &item.RequiresRooms); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) CreateProperty(ctx context.Context, actor, requestID string, item Property) (Property, error) {
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO catalog.properties (
			id, company_id, branch_id, city_id, neighborhood_id, owner_user_id, delegation_ref,
			precise_point, address_ar, address_en, type_code, area_sqm, created_by, updated_by
		) VALUES (
			gen_random_uuid(), $1, NULLIF($2,'')::uuid, $3, $4, NULLIF($5,'')::uuid, $6,
			CASE WHEN $7::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($7, $8), 4326) END,
			$9, $10, $11, $12, $13, $13
		) RETURNING id::text, version, created_at
	`, item.CompanyID, item.BranchID, item.CityID, item.NeighborhoodID, item.OwnerUserID, item.DelegationRef, item.PreciseLon, item.PreciseLat, item.AddressAR, item.AddressEN, item.TypeCode, item.AreaSqm, actor).Scan(&item.ID, &item.Version, &item.CreatedAt)
	if err != nil {
		return Property{}, err
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "property.create", EntityType: "property", EntityID: item.ID, CompanyID: item.CompanyID, RequestID: requestID, Result: "ok"})
	return item, nil
}

func (s Service) ListProperties(ctx context.Context, companyID string, page, size int) ([]Property, int, error) {
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM catalog.properties WHERE company_id = $1`, companyID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, company_id::text, COALESCE(branch_id::text,''), city_id::text, neighborhood_id::text,
		       COALESCE(owner_user_id::text,''), delegation_ref, address_ar, address_en, type_code, COALESCE(area_sqm, 0),
		       ST_X(precise_point), ST_Y(precise_point), version, created_by::text, created_at
		FROM catalog.properties WHERE company_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, companyID, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Property, 0)
	for rows.Next() {
		var item Property
		if err := rows.Scan(&item.ID, &item.CompanyID, &item.BranchID, &item.CityID, &item.NeighborhoodID, &item.OwnerUserID, &item.DelegationRef, &item.AddressAR, &item.AddressEN, &item.TypeCode, &item.AreaSqm, &item.PreciseLon, &item.PreciseLat, &item.Version, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s Service) CreateUnit(ctx context.Context, actor, requestID, companyID string, item Unit) (Unit, error) {
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO catalog.units (id, property_id, unit_number, floor, rooms, baths, status)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 'preparing')
		RETURNING id::text, version
	`, item.PropertyID, item.Number, item.Floor, item.Rooms, item.Baths).Scan(&item.ID, &item.Version)
	if err != nil {
		return Unit{}, err
	}
	item.Status = "preparing"
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "unit.create", EntityType: "unit", EntityID: item.ID, CompanyID: companyID, RequestID: requestID, Result: "ok"})
	return item, nil
}

func (s Service) CreateListing(ctx context.Context, actor, requestID string, item Listing) (Listing, error) {
	issues := []string{}
	raw, _ := json.Marshal(issues)
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO catalog.listings (
			id, property_id, unit_id, neighborhood_id, company_id, branch_id, transaction_type, price_minor, price_unit, currency,
			license_ref, license_exception_ref, broker_user_id, status, validation_issues, created_by, updated_by
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, NULLIF($5,'')::uuid, $6, $7, $8, $9, $10, $11, NULLIF($12,'')::uuid, 'draft', $13, $14, $14
		) RETURNING id::text, version, created_at
	`, item.PropertyID, item.UnitID, item.NeighborhoodID, item.CompanyID, item.BranchID, item.TransactionType, item.PriceMinor, item.PriceUnit, item.Currency, item.LicenseRef, item.LicenseExceptionRef, item.BrokerUserID, raw, actor).Scan(&item.ID, &item.Version, &item.CreatedAt)
	if err != nil {
		return Listing{}, err
	}
	for _, text := range item.Texts {
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO catalog.listing_translations (listing_id, locale, title, description) VALUES ($1, $2, $3, $4)
		`, item.ID, text.Locale, text.Title, text.Description); err != nil {
			return Listing{}, err
		}
	}
	item.Status = "draft"
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "listing.create", EntityType: "listing", EntityID: item.ID, CompanyID: item.CompanyID, RequestID: requestID, Result: "ok"})
	return s.GetListing(ctx, item.ID)
}

func (s Service) GetListing(ctx context.Context, id string) (Listing, error) {
	var item Listing
	var issues []byte
	var pending []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT l.id::text, l.property_id::text, l.unit_id::text, l.neighborhood_id::text, l.company_id::text, COALESCE(l.branch_id::text,''),
		       l.transaction_type, COALESCE(l.price_minor, 0), l.price_unit, l.currency, l.license_ref, l.license_exception_ref,
		       COALESCE(l.broker_user_id::text,''), l.status, l.version, l.validation_issues, l.pending_changes, l.created_by::text, l.created_at,
		       COALESCE(p.area_sqm, 0), COALESCE(p.owner_user_id::text,''), p.type_code, u.rooms, ST_X(p.precise_point), ST_Y(p.precise_point)
		FROM catalog.listings l
		JOIN catalog.properties p ON p.id = l.property_id
		JOIN catalog.units u ON u.id = l.unit_id
		WHERE l.id = $1
	`, id).Scan(&item.ID, &item.PropertyID, &item.UnitID, &item.NeighborhoodID, &item.CompanyID, &item.BranchID, &item.TransactionType, &item.PriceMinor, &item.PriceUnit, &item.Currency, &item.LicenseRef, &item.LicenseExceptionRef, &item.BrokerUserID, &item.Status, &item.Version, &issues, &pending, &item.CreatedBy, &item.CreatedAt, &item.AreaSqm, &item.OwnerUserID, &item.TypeCode, &item.Rooms, &item.PreciseLon, &item.PreciseLat)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Listing{}, err
	}
	_ = json.Unmarshal(issues, &item.Issues)
	if len(pending) > 0 {
		_ = json.Unmarshal(pending, &item.Pending)
	}
	rows, err := s.Pool.Query(ctx, `SELECT locale, title, description FROM catalog.listing_translations WHERE listing_id = $1 ORDER BY locale`, id)
	if err != nil {
		return Listing{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var text Translation
		if err := rows.Scan(&text.Locale, &text.Title, &text.Description); err != nil {
			return Listing{}, err
		}
		item.Texts = append(item.Texts, text)
	}
	if item.Issues == nil {
		item.Issues = []string{}
	}
	if item.Texts == nil {
		item.Texts = []Translation{}
	}
	return item, rows.Err()
}

func (s Service) SaveIssues(ctx context.Context, id string, issues []string) error {
	raw, _ := json.Marshal(issues)
	_, err := s.Pool.Exec(ctx, `UPDATE catalog.listings SET validation_issues = $2, updated_at = now() WHERE id = $1`, id, raw)
	return err
}

func (s Service) Transition(ctx context.Context, actor, requestID, id string, version int, to string, issues []string) (Listing, error) {
	current, err := s.GetListing(ctx, id)
	if err != nil {
		return Listing{}, err
	}
	if current.Version != version {
		return Listing{}, platform.E("version_conflict", 409)
	}
	if !CanTransition(current.Status, to) {
		return Listing{}, platform.E("invalid_transition", 409)
	}
	if (to == "in_review" || to == "approved" || to == "published") && len(issues) > 0 {
		_ = s.SaveIssues(ctx, id, issues)
		return Listing{}, &platform.Error{Code: "validation", HTTP: 422, Fields: mapIssue(issues)}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Listing{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE catalog.listings
		SET status = $4, validation_issues = '[]'::jsonb, version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, actor, to)
	if err != nil {
		return Listing{}, err
	}
	if tag.RowsAffected() == 0 {
		return Listing{}, platform.E("version_conflict", 409)
	}
	if to == "published" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO catalog.price_history (id, listing_id, price_minor, price_unit, currency)
			VALUES (gen_random_uuid(), $1, $2, $3, $4)
		`, id, current.PriceMinor, current.PriceUnit, current.Currency); err != nil {
			return Listing{}, err
		}
	}
	if err := audit.Record(ctx, tx, audit.Event{ActorID: actor, Action: "listing." + to, EntityType: "listing", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok"}); err != nil {
		return Listing{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Listing{}, err
	}
	return s.GetListing(ctx, id)
}

func (s Service) StageSensitiveChange(ctx context.Context, actor, requestID string, current Listing, price int64, neighborhood, license, owner string) (Listing, error) {
	pending := map[string]any{"price_minor": price, "neighborhood_id": neighborhood, "license_ref": license, "owner_user_id": owner}
	raw, _ := json.Marshal(pending)
	tag, err := s.Pool.Exec(ctx, `
		UPDATE catalog.listings SET pending_changes = $3, version = version + 1, updated_by = $2, updated_at = now()
		WHERE id = $1 AND version = $4 AND status = 'published'
	`, current.ID, actor, raw, current.Version)
	if err != nil {
		return Listing{}, err
	}
	if tag.RowsAffected() == 0 {
		return Listing{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "listing.pending_change", EntityType: "listing", EntityID: current.ID, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", After: pending})
	return s.GetListing(ctx, current.ID)
}

func (s Service) PublicList(ctx context.Context, locale, neighborhoodID, transaction string) ([]Listing, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT l.id::text
		FROM catalog.listings l
		JOIN catalog.listing_translations t ON t.listing_id = l.id AND t.locale = $1
		WHERE l.status = 'published' AND ($2::uuid IS NULL OR l.neighborhood_id = $2::uuid) AND ($3 = '' OR l.transaction_type = $3)
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT 100
	`, locale, nullable(neighborhoodID), transaction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Listing, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.GetListing(ctx, id)
		if err != nil {
			return nil, err
		}
		kept := make([]Translation, 0, 1)
		for _, text := range item.Texts {
			if text.Locale == locale {
				kept = append(kept, text)
			}
		}
		item.Texts = kept
		item.PreciseLon, item.PreciseLat = nil, nil
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) CountPublished(ctx context.Context) (int, map[string]int, error) {
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM catalog.listings WHERE status = 'published'`).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT neighborhood_id::text, count(*) FROM catalog.listings WHERE status = 'published' GROUP BY neighborhood_id`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	by := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return 0, nil, err
		}
		by[id] = n
	}
	return total, by, rows.Err()
}

type StaySource struct {
	ID             string
	UnitID         string
	CompanyID      string
	NeighborhoodID string
	Transaction    string
	Currency       string
	PriceMinor     int64
	Status         string
	AddressAR      string
	AddressEN      string
	TitleAR        string
	TitleEN        string
	Capacity       *int
}

func (s Service) StaySource(ctx context.Context, id string) (StaySource, error) {
	var item StaySource
	err := s.Pool.QueryRow(ctx, `
		SELECT l.id::text, l.unit_id::text, l.company_id::text, l.neighborhood_id::text,
		       l.transaction_type, l.currency, COALESCE(l.price_minor, 0), l.status,
		       p.address_ar, p.address_en, u.capacity,
		       COALESCE((SELECT title FROM catalog.listing_translations WHERE listing_id = l.id AND locale = 'ar'), ''),
		       COALESCE((SELECT title FROM catalog.listing_translations WHERE listing_id = l.id AND locale = 'en'), '')
		FROM catalog.listings l
		JOIN catalog.properties p ON p.id = l.property_id
		JOIN catalog.units u ON u.id = l.unit_id
		WHERE l.id = $1
	`, id).Scan(&item.ID, &item.UnitID, &item.CompanyID, &item.NeighborhoodID, &item.Transaction, &item.Currency, &item.PriceMinor, &item.Status, &item.AddressAR, &item.AddressEN, &item.Capacity, &item.TitleAR, &item.TitleEN)
	if errors.Is(err, pgx.ErrNoRows) {
		return StaySource{}, platform.E("not_found", 404)
	}
	return item, err
}

type ListingCard struct {
	ID             string          `json:"id"`
	Status         string          `json:"status"`
	Version        int             `json:"version"`
	Transaction    string          `json:"transaction_type"`
	PriceMinor     int64           `json:"price_minor"`
	NeighborhoodID string          `json:"neighborhood_id"`
	LicenseRef     string          `json:"license_ref"`
	OwnerUserID    string          `json:"owner_user_id"`
	TitleAR        string          `json:"title_ar"`
	TitleEN        string          `json:"title_en"`
	Pending        json.RawMessage `json:"pending_changes"`
}

func (s Service) ListCards(ctx context.Context, companyID, status string) ([]ListingCard, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT l.id::text, l.status, l.version, l.transaction_type, COALESCE(l.price_minor, 0), l.neighborhood_id::text,
		       l.license_ref, COALESCE(p.owner_user_id::text, ''),
		       COALESCE((SELECT title FROM catalog.listing_translations WHERE listing_id = l.id AND locale = 'ar'), ''),
		       COALESCE((SELECT title FROM catalog.listing_translations WHERE listing_id = l.id AND locale = 'en'), ''),
		       COALESCE(l.pending_changes, 'null'::jsonb)
		FROM catalog.listings l
		JOIN catalog.properties p ON p.id = l.property_id
		WHERE l.company_id = $1 AND ($2 = '' OR l.status = $2)
		ORDER BY l.updated_at DESC
		LIMIT 100
	`, companyID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ListingCard, 0)
	for rows.Next() {
		var item ListingCard
		if err := rows.Scan(&item.ID, &item.Status, &item.Version, &item.Transaction, &item.PriceMinor, &item.NeighborhoodID, &item.LicenseRef, &item.OwnerUserID, &item.TitleAR, &item.TitleEN, &item.Pending); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullable(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func mapIssue(issues []string) map[string]string {
	out := map[string]string{}
	for _, issue := range issues {
		out[issue] = issue
	}
	return out
}
