package deals

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/audit"
	"shamal/service/internal/platform"
)

type Deal struct {
	ID               string       `json:"id"`
	CompanyID        string       `json:"company_id"`
	BranchID         string       `json:"branch_id"`
	RequestID        string       `json:"request_id"`
	ProfileID        string       `json:"profile_id"`
	ListingID        string       `json:"listing_id"`
	PropertyID       string       `json:"property_id"`
	NeighborhoodID   string       `json:"neighborhood_id"`
	TransactionType  string       `json:"transaction_type"`
	BrokerUserID     string       `json:"broker_user_id"`
	OwnerUserID      string       `json:"owner_user_id"`
	Status           string       `json:"status"`
	AmountMinor      *int64       `json:"amount_minor,omitempty"`
	Currency         string       `json:"currency"`
	AgreementRef     string       `json:"agreement_ref"`
	CloseDate        *time.Time   `json:"close_date,omitempty"`
	CloseRequestedBy string       `json:"close_requested_by"`
	LossReason       string       `json:"loss_reason,omitempty"`
	Test             bool         `json:"is_test"`
	Version          int          `json:"version"`
	CreatedBy        string       `json:"created_by"`
	CreatedAt        time.Time    `json:"created_at"`
	DisplayName      string       `json:"display_name"`
	History          int          `json:"history_count"`
	Events           []StageEvent `json:"stage_events"`
}

type StageEvent struct {
	ID         string    `json:"id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Reason     string    `json:"reason"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Open(ctx context.Context, actor, requestID, clientRequest, profile, company, branch, transaction, neighborhood string, test bool) (Deal, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO deals.deals (
			id, company_id, branch_id, request_id, profile_id, neighborhood_id, transaction_type, status, is_test, created_by
		) VALUES (
			gen_random_uuid(), $1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4, NULLIF($5,'')::uuid, $6, 'draft', $7, $8
		) RETURNING id::text
	`, company, branch, clientRequest, profile, neighborhood, transaction, test, actor).Scan(&id)
	if err != nil {
		return Deal{}, err
	}
	_ = s.event(ctx, id, "", Draft, "", actor)
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "deal.create", EntityType: "deal", EntityID: id, CompanyID: company, RequestID: requestID, Result: "ok"})
	return s.Get(ctx, id)
}

func (s Service) Get(ctx context.Context, id string) (Deal, error) {
	var item Deal
	var amount *int64
	var closeDate *time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT d.id::text, d.company_id::text, COALESCE(d.branch_id::text,''), COALESCE(d.request_id::text,''), d.profile_id::text,
		       COALESCE(d.listing_id::text,''), COALESCE(d.property_id::text,''), COALESCE(d.neighborhood_id::text,''), d.transaction_type,
		       COALESCE(d.broker_user_id::text,''), COALESCE(d.owner_user_id::text,''), d.status, d.amount_minor, d.currency, d.agreement_ref,
		       d.close_date, COALESCE(d.close_requested_by::text,''), d.loss_reason, d.is_test, d.version, d.created_by::text, d.created_at,
		       p.display_name, (SELECT count(*) FROM deals.stage_events e WHERE e.deal_id = d.id)
		FROM deals.deals d
		JOIN clients.profiles p ON p.id = d.profile_id
		WHERE d.id = $1
	`, id).Scan(&item.ID, &item.CompanyID, &item.BranchID, &item.RequestID, &item.ProfileID, &item.ListingID, &item.PropertyID, &item.NeighborhoodID, &item.TransactionType, &item.BrokerUserID, &item.OwnerUserID, &item.Status, &amount, &item.Currency, &item.AgreementRef, &closeDate, &item.CloseRequestedBy, &item.LossReason, &item.Test, &item.Version, &item.CreatedBy, &item.CreatedAt, &item.DisplayName, &item.History)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deal{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Deal{}, err
	}
	item.AmountMinor = amount
	item.CloseDate = closeDate
	item.Events, err = s.stageEvents(ctx, item.ID)
	if err != nil {
		return Deal{}, err
	}
	return item, nil
}

func (s Service) stageEvents(ctx context.Context, id string) ([]StageEvent, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, from_status, to_status, reason, COALESCE(actor_id::text, ''), created_at
		FROM deals.stage_events WHERE deal_id = $1 ORDER BY created_at, id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StageEvent, 0)
	for rows.Next() {
		var item StageEvent
		if err := rows.Scan(&item.ID, &item.FromStatus, &item.ToStatus, &item.Reason, &item.ActorID, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) List(ctx context.Context, companyID, status string, page, size int) ([]Deal, int, error) {
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM deals.deals WHERE company_id = $1 AND ($2 = '' OR status = $2)`, companyID, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text FROM deals.deals WHERE company_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY updated_at DESC, id DESC LIMIT $3 OFFSET $4
	`, companyID, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Deal, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		item, err := s.Get(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s Service) Mine(ctx context.Context, userID string) ([]Deal, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT d.id::text FROM deals.deals d
		JOIN clients.profiles p ON p.id = d.profile_id
		WHERE p.user_id = $1 ORDER BY d.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Deal, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		item.LossReason = ""
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) Move(ctx context.Context, actor, requestID, id, to, reason string, version int, listing, property, owner, broker, currency string, amount *int64) (Deal, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Deal{}, err
	}
	if current.Version != version {
		return Deal{}, platform.E("version_conflict", 409)
	}
	if code := MoveIssue(current.Status, to, reason); code != "" {
		httpStatus := 409
		if code == "reason_required" {
			httpStatus = 422
		}
		return Deal{}, platform.E(code, httpStatus)
	}
	if to == Qualified && current.ListingID == "" && listing == "" && current.PropertyID == "" && property == "" {
		return Deal{}, platform.E("validation", 422)
	}
	if current.Status != Draft && current.Status != Qualified {
		if (listing != "" && current.ListingID != "" && listing != current.ListingID) || (property != "" && current.PropertyID != "" && property != current.PropertyID) {
			return Deal{}, platform.E("property_change_unapproved", 422)
		}
	}
	if to == Preliminary {
		value := int64(0)
		if amount != nil {
			value = *amount
		} else if current.AmountMinor != nil {
			value = *current.AmountMinor
		}
		money := current.Currency
		if currency != "" {
			money = currency
		}
		issues := CloseIssues(CloseInput{
			AgreementRef: "pending", AmountMinor: value, Currency: money,
			ClientID: current.ProfileID, OwnerID: first(owner, current.OwnerUserID), BrokerID: first(broker, current.BrokerUserID),
			CloseDate: time.Now().UTC(), RequestedBy: "other", ApproverID: "approver",
		})
		kept := make([]string, 0)
		for _, issue := range issues {
			if issue == "parties_required" || issue == "invalid_price" || issue == "currency_not_sar" {
				kept = append(kept, issue)
			}
		}
		if len(kept) > 0 {
			return Deal{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueMap(kept)}
		}
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE deals.deals SET
			status = $3,
			listing_id = COALESCE(NULLIF($4,'')::uuid, listing_id),
			property_id = COALESCE(NULLIF($5,'')::uuid, property_id),
			owner_user_id = COALESCE(NULLIF($6,'')::uuid, owner_user_id),
			broker_user_id = COALESCE(NULLIF($7,'')::uuid, broker_user_id),
			loss_reason = CASE WHEN $8 = '' THEN loss_reason ELSE $8 END,
			amount_minor = COALESCE($9, amount_minor),
			currency = CASE WHEN $10 = '' THEN currency ELSE $10 END,
			version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, to, listing, property, owner, broker, strings.TrimSpace(reason), amount, currency)
	if err != nil {
		return Deal{}, err
	}
	if tag.RowsAffected() == 0 {
		return Deal{}, platform.E("version_conflict", 409)
	}
	_ = s.event(ctx, id, current.Status, to, reason, actor)
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "deal." + to, EntityType: "deal", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason, Before: current.Status, After: to})
	return s.Get(ctx, id)
}

type CloseRequest struct {
	AgreementRef  string
	AmountMinor   int64
	Currency      string
	CloseDate     time.Time
	HasCommission bool
	HasTax        bool
	HasCancelFee  bool
}

func (s Service) RequestClose(ctx context.Context, actor, requestID, id string, version int, in CloseRequest) (Deal, error) {
	if in.HasCommission || in.HasTax || in.HasCancelFee {
		return Deal{}, platform.E("commercial_terms_unapproved", 422)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Deal{}, err
	}
	if current.Version != version {
		return Deal{}, platform.E("version_conflict", 409)
	}
	if current.Status != AwaitingExternal {
		return Deal{}, platform.E("invalid_transition", 409)
	}
	if in.Currency == "" {
		in.Currency = "SAR"
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE deals.deals SET
			agreement_ref = $3, amount_minor = $4, currency = $5, close_date = $6, close_requested_by = $7,
			version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2 AND status = 'awaiting_external'
	`, id, version, strings.TrimSpace(in.AgreementRef), in.AmountMinor, in.Currency, in.CloseDate, actor)
	if err != nil {
		return Deal{}, err
	}
	if tag.RowsAffected() == 0 {
		return Deal{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "deal.close_requested", EntityType: "deal", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok"})
	return s.Get(ctx, id)
}

func (s Service) ApproveClose(ctx context.Context, actor, requestID, id string, version int) (Deal, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Deal{}, err
	}
	if current.Version != version {
		return Deal{}, platform.E("version_conflict", 409)
	}
	if !CanTransition(current.Status, ClosedSuccess) {
		return Deal{}, platform.E("invalid_transition", 409)
	}
	amount := int64(0)
	if current.AmountMinor != nil {
		amount = *current.AmountMinor
	}
	closeDate := time.Time{}
	if current.CloseDate != nil {
		closeDate = *current.CloseDate
	}
	issues := CloseIssues(CloseInput{
		AgreementRef: current.AgreementRef, AmountMinor: amount, Currency: current.Currency,
		ClientID: current.ProfileID, OwnerID: current.OwnerUserID, BrokerID: current.BrokerUserID,
		CloseDate: closeDate, RequestedBy: current.CloseRequestedBy, ApproverID: actor,
	})
	if len(issues) > 0 {
		return Deal{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueMap(issues)}
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE deals.deals SET status = 'closed_success', version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2 AND status = 'awaiting_external'
	`, id, version)
	if err != nil {
		return Deal{}, err
	}
	if tag.RowsAffected() == 0 {
		return Deal{}, platform.E("version_conflict", 409)
	}
	_ = s.event(ctx, id, current.Status, ClosedSuccess, "", actor)
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "deal.close_approved", EntityType: "deal", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Before: current.Status, After: ClosedSuccess})
	return s.Get(ctx, id)
}

func (s Service) event(ctx context.Context, id, from, to, reason, actor string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO deals.stage_events (id, deal_id, from_status, to_status, reason, actor_id)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, NULLIF($5,'')::uuid)
	`, id, from, to, reason, actor)
	return err
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func issueMap(issues []string) map[string]string {
	out := map[string]string{}
	for _, issue := range issues {
		out[issue] = issue
	}
	return out
}
