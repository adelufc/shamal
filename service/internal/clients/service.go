package clients

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"shamal/service/internal/audit"
	"shamal/service/internal/platform"
)

type Profile struct {
	ID          string    `json:"id"`
	CompanyID   string    `json:"company_id"`
	UserID      string    `json:"user_id,omitempty"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}

type Request struct {
	ID              string    `json:"id"`
	CompanyID       string    `json:"company_id"`
	BranchID        string    `json:"branch_id"`
	ProfileID       string    `json:"profile_id"`
	NeighborhoodID  string    `json:"neighborhood_id"`
	TransactionType string    `json:"transaction_type"`
	BudgetMinor     *int64    `json:"budget_minor,omitempty"`
	Currency        string    `json:"currency"`
	TimingNote      string    `json:"timing_note"`
	SpecsAR         string    `json:"specs_ar"`
	SpecsEN         string    `json:"specs_en"`
	ContactMethod   string    `json:"contact_method"`
	Stage           string    `json:"stage"`
	ClientStage     string    `json:"client_stage"`
	AssigneeID      string    `json:"assignee_id"`
	CloseReason     string    `json:"close_reason,omitempty"`
	DealID          string    `json:"deal_id,omitempty"`
	DisplayName     string    `json:"display_name"`
	Email           string    `json:"email"`
	Version         int       `json:"version"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

type Input struct {
	DisplayName    string
	Email          string
	Phone          string
	Transaction    string
	BudgetMinor    *int64
	Currency       string
	NeighborhoodID string
	TimingNote     string
	SpecsAR        string
	SpecsEN        string
	ContactMethod  string
	BranchID       string
	CompanyID      string
	UserID         string
}

type Service struct {
	DB platform.DB
}

func (s Service) Use(db platform.DB) Service {
	return Service{DB: db}
}

func (s Service) ProfileByUser(ctx context.Context, userID string) (Profile, error) {
	var item Profile
	err := s.DB.QueryRow(ctx, `
		SELECT id::text, company_id::text, COALESCE(user_id::text,''), display_name, email, phone, source, status, version, created_at
		FROM clients.profiles WHERE user_id = $1
	`, userID).Scan(&item.ID, &item.CompanyID, &item.UserID, &item.DisplayName, &item.Email, &item.Phone, &item.Source, &item.Status, &item.Version, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, platform.E("not_found", 404)
	}
	return item, err
}

func (s Service) FindProfileByEmail(ctx context.Context, email string) (Profile, error) {
	var item Profile
	var userID *string
	err := s.DB.QueryRow(ctx, `
		SELECT id::text, company_id::text, user_id::text, display_name, email, phone, source, status, version, created_at
		FROM clients.profiles WHERE lower(email) = lower($1)
	`, strings.TrimSpace(email)).Scan(&item.ID, &item.CompanyID, &userID, &item.DisplayName, &item.Email, &item.Phone, &item.Source, &item.Status, &item.Version, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, pgx.ErrNoRows
	}
	if userID != nil {
		item.UserID = *userID
	}
	return item, err
}

func (s Service) EnsureProfile(ctx context.Context, actor, requestID string, in Input) (Profile, error) {
	if existing, err := s.FindProfileByEmail(ctx, in.Email); err == nil {
		if in.UserID != "" && existing.UserID == "" {
			_, _ = s.DB.Exec(ctx, `UPDATE clients.profiles SET user_id = $2, status = 'active', updated_at = now(), version = version + 1 WHERE id = $1`, existing.ID, in.UserID)
			existing.UserID = in.UserID
			existing.Status = "active"
		}
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, err
	}
	status := "potential"
	if in.UserID != "" {
		status = "active"
	}
	var item Profile
	err := s.DB.QueryRow(ctx, `
		INSERT INTO clients.profiles (id, company_id, user_id, display_name, email, phone, source, status, created_by)
		VALUES (gen_random_uuid(), $1, NULLIF($2,'')::uuid, $3, $4, $5, 'request', $6, NULLIF($7,'')::uuid)
		RETURNING id::text, company_id::text, COALESCE(user_id::text,''), display_name, email, phone, source, status, version, created_at
	`, in.CompanyID, in.UserID, strings.TrimSpace(in.DisplayName), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone), status, actor).Scan(
		&item.ID, &item.CompanyID, &item.UserID, &item.DisplayName, &item.Email, &item.Phone, &item.Source, &item.Status, &item.Version, &item.CreatedAt)
	if err != nil {
		return Profile{}, mapUnique(err)
	}
	_ = audit.Record(ctx, s.DB, audit.Event{ActorID: actor, Action: "client.create", EntityType: "client", EntityID: item.ID, CompanyID: in.CompanyID, RequestID: requestID, Result: "ok"})
	return item, nil
}

func (s Service) CreateRequest(ctx context.Context, actor, requestID string, in Input) (Request, error) {
	if strings.TrimSpace(in.DisplayName) == "" || strings.TrimSpace(in.Email) == "" {
		return Request{}, platform.E("name_required", 422)
	}
	if in.Currency == "" {
		in.Currency = "SAR"
	}
	issues := RequestIssues(in.Transaction, in.ContactMethod, in.NeighborhoodID, in.Currency, in.BudgetMinor)
	if len(issues) > 0 {
		return Request{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueMap(issues)}
	}
	profile, err := s.EnsureProfile(ctx, actor, requestID, in)
	if err != nil {
		return Request{}, err
	}
	var id string
	err = s.DB.QueryRow(ctx, `
		INSERT INTO clients.requests (
			id, company_id, branch_id, profile_id, neighborhood_id, transaction_type, budget_minor, currency,
			timing_note, specs_ar, specs_en, contact_method, stage, created_by
		) VALUES (
			gen_random_uuid(), $1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'new', NULLIF($12,'')::uuid
		) RETURNING id::text
	`, in.CompanyID, in.BranchID, profile.ID, in.NeighborhoodID, in.Transaction, in.BudgetMinor, in.Currency, in.TimingNote, in.SpecsAR, in.SpecsEN, in.ContactMethod, actor).Scan(&id)
	if err != nil {
		return Request{}, err
	}
	_ = audit.Record(ctx, s.DB, audit.Event{ActorID: actor, Action: "request.create", EntityType: "client_request", EntityID: id, CompanyID: in.CompanyID, RequestID: requestID, Result: "ok"})
	return s.Get(ctx, id)
}

func (s Service) Get(ctx context.Context, id string) (Request, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT r.id::text, r.company_id::text, COALESCE(r.branch_id::text,''), r.profile_id::text, COALESCE(r.neighborhood_id::text,''),
		       r.transaction_type, r.budget_minor, r.currency, r.timing_note, r.specs_ar, r.specs_en, r.contact_method, r.stage,
		       COALESCE(r.assignee_id::text,''), r.close_reason, COALESCE(r.deal_id::text,''), p.display_name, p.email,
		       r.version, COALESCE(r.created_by::text,''), r.created_at, COALESCE(d.status,'')
		FROM clients.requests r
		JOIN clients.profiles p ON p.id = r.profile_id
		LEFT JOIN deals.deals d ON d.id = r.deal_id
		WHERE r.id = $1
	`, id)
	return scanRequest(row)
}

func (s Service) List(ctx context.Context, companyID, stage, query string, unassigned bool, page, size int) ([]Request, int, error) {
	query = strings.TrimSpace(query)
	var total int
	if err := s.DB.QueryRow(ctx, `
		SELECT count(*)
		FROM clients.requests r
		JOIN clients.profiles p ON p.id = r.profile_id
		WHERE r.company_id = $1
		  AND ($2 = '' OR r.stage = $2)
		  AND ($3 = '' OR p.email ILIKE '%' || $3 || '%' OR p.display_name ILIKE '%' || $3 || '%' OR r.id::text ILIKE '%' || $3 || '%')
		  AND ($4 = false OR r.assignee_id IS NULL)
	`, companyID, stage, query, unassigned).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.Query(ctx, `
		SELECT r.id::text
		FROM clients.requests r
		JOIN clients.profiles p ON p.id = r.profile_id
		WHERE r.company_id = $1
		  AND ($2 = '' OR r.stage = $2)
		  AND ($3 = '' OR p.email ILIKE '%' || $3 || '%' OR p.display_name ILIKE '%' || $3 || '%' OR r.id::text ILIKE '%' || $3 || '%')
		  AND ($4 = false OR r.assignee_id IS NULL)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $5 OFFSET $6
	`, companyID, stage, query, unassigned, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return s.collect(ctx, rows, total)
}

func (s Service) Mine(ctx context.Context, userID string) ([]Request, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT r.id::text FROM clients.requests r
		JOIN clients.profiles p ON p.id = r.profile_id
		WHERE p.user_id = $1
		ORDER BY r.created_at DESC, r.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, _, err := s.collect(ctx, rows, 0)
	return items, err
}

func (s Service) Move(ctx context.Context, actor, requestID, id, to, reason string, version int) (Request, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Request{}, err
	}
	if current.Version != version {
		return Request{}, platform.E("version_conflict", 409)
	}
	if code := TransitionIssue(current.Stage, to, reason); code != "" {
		status := 409
		if code == "reason_required" {
			status = 422
		}
		return Request{}, platform.E(code, status)
	}
	tag, err := s.DB.Exec(ctx, `
		UPDATE clients.requests
		SET stage = $3, close_reason = CASE WHEN $4 = '' THEN close_reason ELSE $4 END,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, to, strings.TrimSpace(reason))
	if err != nil {
		return Request{}, err
	}
	if tag.RowsAffected() == 0 {
		return Request{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.DB, audit.Event{ActorID: actor, Action: "request." + to, EntityType: "client_request", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason, Before: current.Stage, After: to})
	return s.Get(ctx, id)
}

func (s Service) Assign(ctx context.Context, actor, requestID, id, assignee string, version int) (Request, error) {
	tag, err := s.DB.Exec(ctx, `
		UPDATE clients.requests SET assignee_id = NULLIF($3,'')::uuid, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, assignee)
	if err != nil {
		return Request{}, err
	}
	if tag.RowsAffected() == 0 {
		return Request{}, platform.E("version_conflict", 409)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Request{}, err
	}
	_ = audit.Record(ctx, s.DB, audit.Event{ActorID: actor, Action: "request.assign", EntityType: "client_request", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", After: assignee})
	return current, nil
}

func (s Service) AttachDeal(ctx context.Context, requestID, dealID string, version int) error {
	tag, err := s.DB.Exec(ctx, `
		UPDATE clients.requests SET stage = 'converted', deal_id = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2 AND stage = 'agreement'
	`, requestID, version, dealID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return platform.E("version_conflict", 409)
	}
	return nil
}

func (s Service) collect(ctx context.Context, rows pgx.Rows, total int) ([]Request, int, error) {
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	out := make([]Request, 0, len(ids))
	for _, id := range ids {
		item, err := s.Get(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRequest(row scanner) (Request, error) {
	var item Request
	var budget *int64
	var dealStatus string
	err := row.Scan(&item.ID, &item.CompanyID, &item.BranchID, &item.ProfileID, &item.NeighborhoodID, &item.TransactionType, &budget, &item.Currency, &item.TimingNote, &item.SpecsAR, &item.SpecsEN, &item.ContactMethod, &item.Stage, &item.AssigneeID, &item.CloseReason, &item.DealID, &item.DisplayName, &item.Email, &item.Version, &item.CreatedBy, &item.CreatedAt, &dealStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Request{}, err
	}
	item.BudgetMinor = budget
	item.ClientStage = ClientJourney(item.Stage, dealStatus)
	return item, nil
}

func issueMap(issues []string) map[string]string {
	out := map[string]string{}
	for _, issue := range issues {
		out[issue] = issue
	}
	return out
}

func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return platform.E("email_taken", 409)
	}
	return err
}
