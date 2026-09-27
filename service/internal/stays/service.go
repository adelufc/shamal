package stays

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/audit"
	"shamal/service/internal/platform"
)

type Listing struct {
	ID             string
	UnitID         string
	CompanyID      string
	NeighborhoodID string
	Transaction    string
	Currency       string
	PriceMinor     int64
	AddressAR      string
	AddressEN      string
	TitleAR        string
	TitleEN        string
	Capacity       *int
}

type Policy struct {
	ID             string `json:"id"`
	Version        int    `json:"version"`
	SummaryAR      string `json:"summary_ar"`
	SummaryEN      string `json:"summary_en"`
	Contractual    bool   `json:"contractual"`
	NonContractual bool   `json:"non_contractual"`
}

type Booking struct {
	ID                string     `json:"id"`
	ListingID         string     `json:"listing_id"`
	UnitID            string     `json:"unit_id"`
	Transaction       string     `json:"transaction_type"`
	Arrival           string     `json:"arrival"`
	Departure         string     `json:"departure"`
	Guests            int        `json:"guests"`
	Status            string     `json:"status"`
	HoldExpiresAt     *time.Time `json:"hold_expires_at,omitempty"`
	Currency          string     `json:"currency"`
	Nights            int        `json:"nights"`
	Months            int        `json:"months"`
	BaseMinor         int64      `json:"base_minor"`
	DiscountMinor     int64      `json:"discount_minor"`
	ServiceMinor      int64      `json:"service_minor"`
	TaxMinor          int64      `json:"tax_minor"`
	DepositMinor      int64      `json:"deposit_minor"`
	StayDueMinor      int64      `json:"stay_due_minor"`
	CollectMinor      int64      `json:"collect_minor"`
	RevenueMinor      int64      `json:"revenue_minor"`
	DepositIsRevenue  bool       `json:"deposit_is_revenue"`
	CommissionAdopted bool       `json:"commission_adopted"`
	PaymentStatus     string     `json:"payment_status"`
	NonContractual    bool       `json:"non_contractual"`
	PolicyID          string     `json:"policy_id"`
	PolicyVersion     int        `json:"policy_version"`
	PolicyContractual bool       `json:"policy_contractual"`
	PolicySummaryAR   string     `json:"policy_summary_ar"`
	PolicySummaryEN   string     `json:"policy_summary_en"`
	AddressAR         string     `json:"address_ar"`
	AddressEN         string     `json:"address_en"`
	TitleAR           string     `json:"title_ar"`
	TitleEN           string     `json:"title_en"`
	QuoteExpiresAt    time.Time  `json:"quote_expires_at"`
	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty"`
	Version           int        `json:"version"`
	HoldMinutes       int        `json:"hold_minutes"`
	QuoteMinutes      int        `json:"quote_minutes"`
}

type Quote struct {
	Breakdown
	Available         bool   `json:"available"`
	Currency          string `json:"currency"`
	Policy            Policy `json:"policy"`
	NonContractual    bool   `json:"non_contractual"`
	PaymentStatus     string `json:"payment_status"`
	CommissionAdopted bool   `json:"commission_adopted"`
	DepositIsRevenue  bool   `json:"deposit_is_revenue"`
	TitleAR           string `json:"title_ar"`
	TitleEN           string `json:"title_en"`
	AddressAR         string `json:"address_ar"`
	AddressEN         string `json:"address_en"`
	HoldMinutes       int    `json:"hold_minutes"`
	QuoteMinutes      int    `json:"quote_minutes"`
	ListingID         string `json:"listing_id"`
}

type HoldRequest struct {
	ActorID        string
	ProfileID      string
	RequestID      string
	IdempotencyKey string
	Listing        Listing
	Arrival        time.Time
	Departure      time.Time
	Guests         int
	QuotedDue      int64
	PolicyVersion  int
	AcceptPolicy   bool
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Policy(ctx context.Context, transaction string) (Policy, error) {
	var item Policy
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, version, summary_ar, summary_en, contractual
		FROM stays.policies
		WHERE transaction_type = $1 AND contractual = false
		ORDER BY version DESC
		LIMIT 1
	`, transaction).Scan(&item.ID, &item.Version, &item.SummaryAR, &item.SummaryEN, &item.Contractual)
	if errors.Is(err, pgx.ErrNoRows) {
		return Policy{}, platform.E("policy_unadopted", 422)
	}
	if err != nil {
		return Policy{}, err
	}
	item.NonContractual = !item.Contractual
	return item, nil
}

func (s Service) Available(ctx context.Context, unitID string, arrival, departure time.Time) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := expireUnit(ctx, tx, unitID, time.Now().UTC()); err != nil {
		return false, err
	}
	var taken bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM stays.occupancy
			WHERE unit_id = $1
			  AND daterange(start_on, end_on, '[)') && daterange($2::date, $3::date, '[)')
		)
	`, unitID, dateOnly(arrival), dateOnly(departure)).Scan(&taken)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return !taken, nil
}

func (s Service) Hold(ctx context.Context, in HoldRequest) (Booking, error) {
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 200 {
		return Booking{}, platform.E("idempotency_required", 400)
	}
	if !in.AcceptPolicy {
		return Booking{}, platform.E("policy_required", 422)
	}
	now := time.Now().UTC()
	priced, issues := Price(Input{
		Transaction: in.Listing.Transaction, Arrival: in.Arrival, Departure: in.Departure,
		Guests: in.Guests, Capacity: in.Listing.Capacity, UnitMinor: in.Listing.PriceMinor, Now: now,
	})
	if len(issues) > 0 {
		return Booking{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueMap(issues)}
	}
	if in.Listing.Currency != "" && in.Listing.Currency != "SAR" {
		return Booking{}, platform.E("currency_not_sar", 422)
	}
	if priced.DiscountMinor != 0 || priced.ServiceMinor != 0 || priced.TaxMinor != 0 || priced.DepositMinor != 0 {
		return Booking{}, platform.E("commercial_terms_unapproved", 422)
	}
	if in.QuotedDue != priced.StayDueMinor {
		return Booking{}, platform.E("quote_changed", 409)
	}
	policy, err := s.Policy(ctx, in.Listing.Transaction)
	if err != nil {
		return Booking{}, err
	}
	if in.PolicyVersion != policy.Version {
		return Booking{}, platform.E("policy_version", 422)
	}
	hash := HoldHash(in.Listing.ID, priced.Arrival.Format("2006-01-02"), priced.Departure.Format("2006-01-02"), in.Guests, policy.Version, priced.StayDueMinor)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	if existing, ok, err := readIdempotent(ctx, tx, in.ActorID, in.IdempotencyKey, hash); err != nil {
		return Booking{}, err
	} else if ok {
		return existing, nil
	}
	if err := expireUnit(ctx, tx, in.Listing.UnitID, now); err != nil {
		return Booking{}, err
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO stays.bookings (
			id, company_id, listing_id, unit_id, profile_id, actor_id, transaction_type, arrival, departure, guests,
			status, hold_expires_at, currency, nights, months, base_minor, discount_minor, service_minor, tax_minor,
			deposit_minor, stay_due_minor, collect_minor, commission_minor, policy_id, policy_version,
			policy_summary_ar, policy_summary_en, address_ar, address_en, title_ar, title_en,
			accepted_policy_at, quote_expires_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9,
			'held', $10, 'SAR', $11, $12, $13, 0, 0, 0,
			0, $13, $13, NULL, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23
		) RETURNING id::text
	`, in.Listing.CompanyID, in.Listing.ID, in.Listing.UnitID, in.ProfileID, in.ActorID, in.Listing.Transaction,
		priced.Arrival, priced.Departure, in.Guests, now.Add(HoldMinutes*time.Minute), priced.Nights, priced.Months, priced.StayDueMinor,
		policy.ID, policy.Version, policy.SummaryAR, policy.SummaryEN, in.Listing.AddressAR, in.Listing.AddressEN,
		in.Listing.TitleAR, in.Listing.TitleEN, now, priced.ExpiresAt).Scan(&id)
	if err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stays.idempotency (actor_id, idempotency_key, request_hash, booking_id) VALUES ($1, $2, $3, $4)
	`, in.ActorID, in.IdempotencyKey, hash, id); err != nil {
		if existing, ok, readErr := replayUnique(ctx, tx, err, in.ActorID, in.IdempotencyKey, hash); readErr != nil || ok {
			return existing, readErr
		}
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stays.occupancy (id, unit_id, booking_id, kind, start_on, end_on)
		VALUES (gen_random_uuid(), $1, $2, 'hold', $3, $4)
	`, in.Listing.UnitID, id, priced.Arrival, priced.Departure); err != nil {
		if conflict(err) {
			return Booking{}, platform.E("stay_conflict", 409)
		}
		return Booking{}, err
	}
	item, err := loadBooking(ctx, tx, id)
	if err != nil {
		return Booking{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: in.ActorID, Action: "stay.hold", EntityType: "stay", EntityID: id,
		CompanyID: in.Listing.CompanyID, RequestID: in.RequestID, Result: "ok",
	}); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return present(item), nil
}

func (s Service) Confirm(ctx context.Context, actor, requestID, bookingID, key string) (Booking, error) {
	if key == "" || len(key) > 200 {
		return Booking{}, platform.E("idempotency_required", 400)
	}
	hash := ConfirmHash(bookingID)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	if existing, ok, err := readIdempotent(ctx, tx, actor, key, hash); err != nil {
		return Booking{}, err
	} else if ok {
		return existing, nil
	}
	item, err := lockBooking(ctx, tx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if item.ActorID != actor {
		return Booking{}, platform.E("forbidden", 403)
	}
	if item.Status == "confirmed" {
		return present(item), nil
	}
	if item.Status != "held" {
		return Booking{}, platform.E("hold_expired", 409)
	}
	now := time.Now().UTC()
	if item.HoldExpiresAt == nil || !item.HoldExpiresAt.After(now) {
		if err := expireBooking(ctx, tx, item.ID, actor, requestID); err != nil {
			return Booking{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Booking{}, err
		}
		return Booking{}, platform.E("hold_expired", 409)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE stays.bookings
		SET status = 'confirmed', confirmed_at = $2, updated_at = $2, version = version + 1
		WHERE id = $1 AND status = 'held'
	`, bookingID, now); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE stays.occupancy SET kind = 'confirmed' WHERE booking_id = $1 AND kind = 'hold'`, bookingID); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stays.idempotency (actor_id, idempotency_key, request_hash, booking_id) VALUES ($1, $2, $3, $4)
	`, actor, key, hash, bookingID); err != nil {
		if existing, ok, readErr := replayUnique(ctx, tx, err, actor, key, hash); readErr != nil || ok {
			return existing, readErr
		}
		return Booking{}, err
	}
	saved, err := loadBooking(ctx, tx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "stay.confirm", EntityType: "stay", EntityID: bookingID,
		CompanyID: saved.CompanyID, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return present(saved), nil
}

func (s Service) Mine(ctx context.Context, actor string) ([]Booking, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+bookingColumns+`
		FROM stays.bookings WHERE actor_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Booking, 0)
	for rows.Next() {
		item, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, present(item))
	}
	return out, rows.Err()
}

func (s Service) Owned(ctx context.Context, id, actor string) (Booking, string, error) {
	item, err := loadBooking(ctx, s.Pool, id)
	if err != nil {
		return Booking{}, "", err
	}
	if item.ActorID != actor {
		return Booking{}, "", platform.E("forbidden", 403)
	}
	return present(item), item.CompanyID, nil
}

func HoldHash(listing, arrival, departure string, guests, policy int, due int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("hold|%s|%s|%s|%d|%d|%d", listing, arrival, departure, guests, policy, due)))
	return hex.EncodeToString(sum[:])
}

func ConfirmHash(bookingID string) string {
	sum := sha256.Sum256([]byte("confirm|" + bookingID))
	return hex.EncodeToString(sum[:])
}

type rowScanner interface {
	Scan(dest ...any) error
}

type stored struct {
	Booking
	ActorID   string
	CompanyID string
}

const bookingColumns = `
	id::text, listing_id::text, unit_id::text, actor_id::text, company_id::text, transaction_type,
	arrival, departure, guests, status, hold_expires_at, currency, nights, months,
	base_minor, discount_minor, service_minor, tax_minor, deposit_minor, stay_due_minor, collect_minor,
	policy_id::text, policy_version, policy_summary_ar, policy_summary_en,
	address_ar, address_en, title_ar, title_en, quote_expires_at, confirmed_at, version`

func loadBooking(ctx context.Context, db platform.DB, id string) (stored, error) {
	row := db.QueryRow(ctx, `SELECT `+bookingColumns+` FROM stays.bookings WHERE id = $1`, id)
	item, err := scanBooking(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return stored{}, platform.E("not_found", 404)
	}
	return item, err
}

func lockBooking(ctx context.Context, db platform.DB, id string) (stored, error) {
	row := db.QueryRow(ctx, `SELECT `+bookingColumns+` FROM stays.bookings WHERE id = $1 FOR UPDATE`, id)
	item, err := scanBooking(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return stored{}, platform.E("not_found", 404)
	}
	return item, err
}

func scanBooking(row rowScanner) (stored, error) {
	var item stored
	var arrival, departure time.Time
	err := row.Scan(
		&item.ID, &item.ListingID, &item.UnitID, &item.ActorID, &item.CompanyID, &item.Transaction,
		&arrival, &departure, &item.Guests, &item.Status, &item.HoldExpiresAt, &item.Currency, &item.Nights, &item.Months,
		&item.BaseMinor, &item.DiscountMinor, &item.ServiceMinor, &item.TaxMinor, &item.DepositMinor, &item.StayDueMinor, &item.CollectMinor,
		&item.PolicyID, &item.PolicyVersion, &item.PolicySummaryAR, &item.PolicySummaryEN,
		&item.AddressAR, &item.AddressEN, &item.TitleAR, &item.TitleEN, &item.QuoteExpiresAt, &item.ConfirmedAt, &item.Version,
	)
	if err != nil {
		return stored{}, err
	}
	item.Arrival = arrival.Format("2006-01-02")
	item.Departure = departure.Format("2006-01-02")
	return item, nil
}

func present(item stored) Booking {
	item.RevenueMinor = item.StayDueMinor
	item.DepositIsRevenue = false
	item.CommissionAdopted = false
	item.PaymentStatus = "not_collected"
	item.NonContractual = true
	item.PolicyContractual = false
	item.HoldMinutes = HoldMinutes
	item.QuoteMinutes = QuoteMinutes
	return item.Booking
}

func readIdempotent(ctx context.Context, db platform.DB, actor, key, hash string) (Booking, bool, error) {
	var bookingID, storedHash string
	err := db.QueryRow(ctx, `
		SELECT booking_id::text, request_hash FROM stays.idempotency WHERE actor_id = $1 AND idempotency_key = $2
	`, actor, key).Scan(&bookingID, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, nil
	}
	if err != nil {
		return Booking{}, false, err
	}
	if storedHash != hash {
		return Booking{}, false, platform.E("idempotency_mismatch", 409)
	}
	item, err := loadBooking(ctx, db, bookingID)
	if err != nil {
		return Booking{}, false, err
	}
	return present(item), true, nil
}

func replayUnique(ctx context.Context, db platform.DB, err error, actor, key, hash string) (Booking, bool, error) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return Booking{}, false, nil
	}
	return readIdempotent(ctx, db, actor, key, hash)
}

func expireUnit(ctx context.Context, db platform.DB, unitID string, now time.Time) error {
	rows, err := db.Query(ctx, `
		UPDATE stays.bookings
		SET status = 'expired', updated_at = $2, version = version + 1
		WHERE unit_id = $1 AND status = 'held' AND hold_expires_at <= $2
		RETURNING id::text
	`, unitID, now)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := expireBooking(ctx, db, id, "", ""); err != nil {
			return err
		}
	}
	return nil
}

func expireBooking(ctx context.Context, db platform.DB, id, actor, requestID string) error {
	if _, err := db.Exec(ctx, `
		UPDATE stays.bookings
		SET status = 'expired', updated_at = now(), version = version + 1
		WHERE id = $1 AND status = 'held'
	`, id); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `DELETE FROM stays.occupancy WHERE booking_id = $1 AND kind = 'hold'`, id); err != nil {
		return err
	}
	return audit.Record(ctx, db, audit.Event{
		ActorID: actor, Action: "stay.expire", EntityType: "stay", EntityID: id, RequestID: requestID, Result: "ok",
	})
}

func conflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

func issueMap(issues []string) map[string]string {
	out := map[string]string{}
	for _, issue := range issues {
		out[issue] = issue
	}
	return out
}
