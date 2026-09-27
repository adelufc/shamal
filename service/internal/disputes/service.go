package disputes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/audit"
	"shamal/service/internal/files"
	"shamal/service/internal/finance"
	"shamal/service/internal/messages"
	"shamal/service/internal/platform"
)

type Evidence struct {
	ID          string    `json:"id"`
	SHA256      string    `json:"sha256"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

type Note struct {
	ID         string    `json:"id"`
	Visibility string    `json:"visibility"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type Event struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	From         string    `json:"from_status,omitempty"`
	To           string    `json:"to_status,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	NoticeStatus string    `json:"notice_status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Case struct {
	ID                string     `json:"id"`
	Reference         string     `json:"reference"`
	Status            string     `json:"status"`
	Type              string     `json:"type"`
	Description       string     `json:"description"`
	OccurredAt        time.Time  `json:"occurred_at"`
	RequestedAmount   *int64     `json:"requested_amount_minor,omitempty"`
	SubjectKind       string     `json:"subject_kind"`
	SubjectID         string     `json:"subject_id"`
	StayID            string     `json:"stay_id,omitempty"`
	Priority          string     `json:"priority"`
	SLAAdopted        bool       `json:"sla_adopted"`
	Contractual       bool       `json:"contractual"`
	ResponseDue       time.Time  `json:"response_due_at"`
	AssigneeID        string     `json:"assignee_id,omitempty"`
	FinancialComplete bool       `json:"financial_complete"`
	MoneyMoved        bool       `json:"money_moved"`
	ExecutionStatus   string     `json:"execution_status"`
	RefundID          string     `json:"refund_id,omitempty"`
	RefundSuccess     bool       `json:"refund_success"`
	PolicyAdopted     bool       `json:"policy_adopted"`
	PolicyVersion     string     `json:"policy_version"`
	ObjectionDays     int        `json:"objection_days"`
	ExternalDelivery  string     `json:"external_delivery"`
	LandlordNotified  bool       `json:"landlord_notified"`
	HoldAmount        *int64     `json:"hold_amount_minor,omitempty"`
	OtherFundsFrozen  bool       `json:"other_funds_frozen"`
	Evidence          []Evidence `json:"evidence"`
	Notes             []Note     `json:"notes"`
	Events            []Event    `json:"events"`
}

type OpenInput struct {
	ActorID         string
	PartyID         string
	CompanyID       string
	RequestID       string
	SubjectKind     string
	SubjectID       string
	Type            string
	Description     string
	OccurredAt      time.Time
	RequestedAmount *int64
}

type MoveInput struct {
	To       string `json:"to"`
	Reason   string `json:"reason"`
	Type     string `json:"type"`
	Assignee string `json:"assignee_id"`
	Kind     string `json:"kind"`
	Amount   *int64 `json:"amount_minor"`
	Actions  string `json:"actions"`
}

type Service struct {
	Pool    *pgxpool.Pool
	Files   files.Local
	Finance finance.Service
}

func (s Service) Open(ctx context.Context, in OpenInput) (Case, error) {
	if !KnownSubject(in.SubjectKind) || !KnownType(in.Type) || strings.TrimSpace(in.Description) == "" || in.OccurredAt.IsZero() || in.SubjectID == "" {
		return Case{}, platform.E("validation", 422)
	}
	if in.RequestedAmount != nil && *in.RequestedAmount < 0 {
		return Case{}, platform.E("validation", 422)
	}
	stayID, _, arrival, departure, err := s.subject(ctx, in.PartyID, in.SubjectKind, in.SubjectID)
	if err != nil {
		return Case{}, err
	}
	if (in.Type == "compensation" || in.Type == "refund") && stayID == "" {
		return Case{}, platform.E("compensation_needs_booking", 422)
	}
	priority := "normal"
	wait := NormalWait
	if Critical(in.Type, arrival, departure, time.Now().UTC()) {
		priority = "critical"
		wait = CriticalWait
	}
	due := time.Now().UTC().Add(wait)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	var serial int64
	err = tx.QueryRow(ctx, `
		INSERT INTO disputes.cases (
			id, company_id, party_user_id, opened_by, subject_kind, subject_id, stay_id, type,
			description, occurred_at, requested_amount_minor, status, priority, response_due_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, NULLIF($6,'')::uuid, $7,
			$8, $9, $10, 'open', $11, $12
		) RETURNING id::text, serial
	`, in.CompanyID, in.PartyID, in.ActorID, in.SubjectKind, in.SubjectID, stayID, in.Type,
		strings.TrimSpace(in.Description), in.OccurredAt, in.RequestedAmount, priority, due).Scan(&id, &serial)
	if err != nil {
		return Case{}, err
	}
	if err := s.event(ctx, tx, id, in.ActorID, "opened", "", "open", ""); err != nil {
		return Case{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: in.ActorID, Action: "disputes.open", EntityType: "dispute", EntityID: id,
		CompanyID: in.CompanyID, RequestID: in.RequestID, Result: "ok", After: map[string]any{"reference": fmt.Sprintf("N-%d", serial)},
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return s.Get(ctx, id, in.PartyID, false)
}

func (s Service) List(ctx context.Context, actor string, staff bool) ([]Case, error) {
	query := `SELECT id::text FROM disputes.cases WHERE party_user_id = $1 ORDER BY created_at DESC LIMIT 50`
	args := []any{actor}
	if staff {
		query = `SELECT id::text FROM disputes.cases ORDER BY created_at DESC LIMIT 50`
		args = nil
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Case, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.Get(ctx, id, actor, staff)
		if err != nil {
			return nil, err
		}
		item.Notes = nil
		item.Events = nil
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) Get(ctx context.Context, id, actor string, staff bool) (Case, error) {
	item, party, err := s.load(ctx, id)
	if err != nil {
		return Case{}, err
	}
	if !staff && party != actor {
		return Case{}, platform.E("not_found", 404)
	}
	if err := s.fill(ctx, &item, staff); err != nil {
		return Case{}, err
	}
	return item, nil
}

func (s Service) Move(ctx context.Context, actor, requestID, company, id string, in MoveInput) (Case, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return Case{}, platform.E("reason_required", 422)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	row, err := lock(ctx, tx, id)
	if err != nil {
		return Case{}, err
	}
	if !Allowed(row.Status, in.To) {
		return Case{}, platform.E("invalid_transition", 409)
	}
	switch in.To {
	case "classified":
		if in.Type != "" {
			if !KnownType(in.Type) {
				return Case{}, platform.E("validation", 422)
			}
			row.Type = in.Type
		}
	case "proposed":
		if in.Kind != "compensation" && in.Kind != "reject" && in.Kind != "other" {
			return Case{}, platform.E("validation", 422)
		}
		if in.Kind == "compensation" && (in.Amount == nil || *in.Amount <= 0) {
			return Case{}, platform.E("validation", 422)
		}
		row.ProposalKind = in.Kind
		row.ProposalAmount = in.Amount
		row.ProposalReason = in.Reason
		row.ProposalActions = in.Actions
		row.ProposedBy = actor
	case "resolved":
		if row.ProposedBy == "" || row.ProposedBy == actor {
			return Case{}, platform.E("separation_of_duties", 403)
		}
		if err := s.decide(ctx, tx, actor, requestID, &row); err != nil {
			return Case{}, err
		}
		now := time.Now().UTC()
		row.ResolvedAt = &now
	case "closed":
		if row.PartyNotice == nil {
			return Case{}, platform.E("notice_required", 422)
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE disputes.cases SET
			status = $2, type = $3, assignee_id = COALESCE(NULLIF($4,'')::uuid, assignee_id),
			proposal_kind = $5, proposal_amount_minor = $6, proposal_reason = $7, proposal_actions = $8,
			proposed_by = NULLIF($9,'')::uuid, resolved_at = $10,
			execution_status = $11, refund_id = NULLIF($12,'')::uuid
		WHERE id = $1
	`, id, in.To, row.Type, in.Assignee, row.ProposalKind, row.ProposalAmount, row.ProposalReason, row.ProposalActions, row.ProposedBy, row.ResolvedAt, row.Execution, row.RefundID)
	if err != nil {
		return Case{}, err
	}
	if err := s.event(ctx, tx, id, actor, "transition", row.Status, in.To, in.Reason); err != nil {
		return Case{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "disputes.move", EntityType: "dispute", EntityID: id,
		CompanyID: company, RequestID: requestID, Result: "ok", Reason: in.Reason,
		Before: map[string]string{"status": row.Status}, After: map[string]string{"status": in.To},
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return s.Get(ctx, id, actor, true)
}

func (s Service) Assign(ctx context.Context, actor, requestID, company, id, assignee string) (Case, error) {
	if assignee == "" {
		return Case{}, platform.E("validation", 422)
	}
	var due time.Time
	var evidence string
	if err := s.Pool.QueryRow(ctx, `SELECT response_due_at FROM disputes.cases WHERE id = $1`, id).Scan(&due); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Case{}, platform.E("not_found", 404)
		}
		return Case{}, err
	}
	_ = s.Pool.QueryRow(ctx, `SELECT COALESCE(string_agg(id::text, ',' ORDER BY id), '') FROM disputes.evidence WHERE case_id = $1`, id).Scan(&evidence)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE disputes.cases SET assignee_id = $2 WHERE id = $1`, id, assignee)
	if err != nil {
		return Case{}, err
	}
	if tag.RowsAffected() == 0 {
		return Case{}, platform.E("not_found", 404)
	}
	if err := s.event(ctx, tx, id, actor, "assigned", "", "", assignee); err != nil {
		return Case{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "disputes.assign", EntityType: "dispute", EntityID: id,
		CompanyID: company, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	item, err := s.Get(ctx, id, actor, true)
	if err != nil {
		return Case{}, err
	}
	if !item.ResponseDue.Equal(due) {
		return Case{}, platform.E("internal", 500)
	}
	var after string
	_ = s.Pool.QueryRow(ctx, `SELECT COALESCE(string_agg(id::text, ',' ORDER BY id), '') FROM disputes.evidence WHERE case_id = $1`, id).Scan(&after)
	if after != evidence {
		return Case{}, platform.E("internal", 500)
	}
	return item, nil
}

func (s Service) Note(ctx context.Context, actor, requestID, company, id, visibility, body string, staff bool) (Case, error) {
	if visibility == "" {
		visibility = "party"
	}
	if visibility == "internal" && !staff {
		return Case{}, platform.E("forbidden", 403)
	}
	if visibility != "internal" && visibility != "party" {
		return Case{}, platform.E("validation", 422)
	}
	if code := messages.BodyCode(body); code != "" {
		return Case{}, platform.E(code, 422)
	}
	if _, _, err := s.load(ctx, id); err != nil {
		return Case{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO disputes.notes (id, case_id, author_id, visibility, body)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, id, actor, visibility, strings.TrimSpace(body)); err != nil {
		return Case{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "disputes.note", EntityType: "dispute", EntityID: id,
		CompanyID: company, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return s.Get(ctx, id, actor, staff)
}

func (s Service) Notice(ctx context.Context, actor, requestID, company, id, body string) (Case, error) {
	if code := messages.BodyCode(body); code != "" {
		return Case{}, platform.E(code, 422)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := lock(ctx, tx, id); err != nil {
		return Case{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO disputes.notes (id, case_id, author_id, visibility, body)
		VALUES (gen_random_uuid(), $1, $2, 'party', $3)
	`, id, actor, strings.TrimSpace(body)); err != nil {
		return Case{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE disputes.cases SET party_notice_at = now() WHERE id = $1`, id); err != nil {
		return Case{}, err
	}
	if err := s.event(ctx, tx, id, actor, "party_notice", "", "", body); err != nil {
		return Case{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "disputes.notice", EntityType: "dispute", EntityID: id,
		CompanyID: company, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return s.Get(ctx, id, actor, true)
}

func (s Service) Review(ctx context.Context, actor, requestID, company, id, reason string) (Case, error) {
	if strings.TrimSpace(reason) == "" {
		return Case{}, platform.E("reason_required", 422)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	row, err := lock(ctx, tx, id)
	if err != nil {
		return Case{}, err
	}
	if row.PartyID != actor {
		return Case{}, platform.E("not_found", 404)
	}
	within := row.Status == "closed" && row.ResolvedAt != nil && WithinObjection(*row.ResolvedAt, time.Now().UTC())
	if _, err := tx.Exec(ctx, `
		INSERT INTO disputes.reviews (id, case_id, requester_id, reason, within_window)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, id, actor, strings.TrimSpace(reason), within); err != nil {
		return Case{}, err
	}
	if within {
		if _, err := tx.Exec(ctx, `UPDATE disputes.cases SET status = 'reopened' WHERE id = $1`, id); err != nil {
			return Case{}, err
		}
		if err := s.event(ctx, tx, id, actor, "transition", "closed", "reopened", reason); err != nil {
			return Case{}, err
		}
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "disputes.review", EntityType: "dispute", EntityID: id,
		CompanyID: company, RequestID: requestID, Result: "ok", Reason: reason,
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return s.Get(ctx, id, actor, false)
}

func (s Service) AddEvidence(ctx context.Context, actor, requestID, company, id, contentType, name string, body []byte, staff bool) (Evidence, error) {
	if _, party, err := s.load(ctx, id); err != nil {
		return Evidence{}, err
	} else if !staff && party != actor {
		return Evidence{}, platform.E("not_found", 404)
	}
	if code := messages.AttachmentCode(contentType, name, int64(len(body))); code != "" {
		return Evidence{}, platform.E(code, 422)
	}
	sum := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(sum[:])
	var existing string
	err := s.Pool.QueryRow(ctx, `SELECT id::text FROM disputes.evidence WHERE case_id = $1 AND sha256 = $2`, id, fingerprint).Scan(&existing)
	if err == nil {
		return Evidence{}, platform.E("evidence_exists", 409)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Evidence{}, err
	}
	key := strings.ReplaceAll(id, "-", "") + fingerprint
	if err := s.Files.Put("private", key, bytes.NewReader(body)); err != nil {
		return Evidence{}, err
	}
	var item Evidence
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO disputes.evidence (id, case_id, sha256, content_type, size_bytes, storage_key, uploaded_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		RETURNING id::text, sha256, content_type, size_bytes, uploaded_at
	`, id, fingerprint, contentType, len(body), key, actor).Scan(&item.ID, &item.SHA256, &item.ContentType, &item.SizeBytes, &item.UploadedAt)
	if err != nil {
		return Evidence{}, err
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{
		ActorID: actor, Action: "disputes.evidence", EntityType: "dispute", EntityID: id,
		CompanyID: company, RequestID: requestID, Result: "ok", After: map[string]string{"sha256": fingerprint},
	})
	return item, nil
}

func (s Service) decide(ctx context.Context, tx pgx.Tx, actor, requestID string, row *caseRow) error {
	status := "no_transfer"
	var refundID string
	if row.ProposalKind == "compensation" {
		if row.StayID == "" || row.ProposalAmount == nil {
			return platform.E("compensation_needs_booking", 422)
		}
		var stayStatus, stayActor, stayCompany string
		err := tx.QueryRow(ctx, `SELECT status, actor_id::text, company_id::text FROM stays.bookings WHERE id = $1`, row.StayID).Scan(&stayStatus, &stayActor, &stayCompany)
		if err != nil {
			return err
		}
		refund, err := s.Finance.RequestRefund(ctx, stayActor, requestID, "dispute-"+row.ID, finance.StayBill{
			ID: row.StayID, ActorID: stayActor, CompanyID: stayCompany, Status: stayStatus,
		}, *row.ProposalAmount, row.ProposalReason)
		if err != nil {
			return err
		}
		if refund.Success || refund.AmountMinor != *row.ProposalAmount {
			return platform.E("unbalanced", 422)
		}
		refundID = refund.ID
		status = "refund_requested"
		if _, err := tx.Exec(ctx, `
			INSERT INTO disputes.holds (case_id, amount_minor, refund_id) VALUES ($1, $2, $3)
		`, row.ID, *row.ProposalAmount, refundID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO disputes.decisions (
			id, case_id, kind, amount_minor, reason, actions, proposer_id, approver_id, refund_id, execution_status
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NULLIF($8,'')::uuid, $9
		)
	`, row.ID, row.ProposalKind, row.ProposalAmount, row.ProposalReason, row.ProposalActions, row.ProposedBy, actor, refundID, status); err != nil {
		return err
	}
	row.Execution = status
	row.RefundID = refundID
	return nil
}

type caseRow struct {
	ID              string
	Serial          int64
	PartyID         string
	Status          string
	Type            string
	Description     string
	OccurredAt      time.Time
	Requested       *int64
	SubjectKind     string
	SubjectID       string
	StayID          string
	Priority        string
	ResponseDue     time.Time
	AssigneeID      string
	ProposalKind    string
	ProposalAmount  *int64
	ProposalReason  string
	ProposalActions string
	ProposedBy      string
	PartyNotice     *time.Time
	ResolvedAt      *time.Time
	PolicyVersion   string
	Execution       string
	RefundID        string
}

func (s Service) subject(ctx context.Context, party, kind, subject string) (string, string, time.Time, time.Time, error) {
	switch kind {
	case "booking":
		var id, actor, status string
		var arrival, departure time.Time
		err := s.Pool.QueryRow(ctx, `
			SELECT id::text, actor_id::text, status, arrival, departure FROM stays.bookings WHERE id = $1
		`, subject).Scan(&id, &actor, &status, &arrival, &departure)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && actor != party) {
			return "", "", time.Time{}, time.Time{}, platform.E("not_found", 404)
		}
		return id, status, arrival, departure, err
	case "payment":
		var stayID, actor string
		err := s.Pool.QueryRow(ctx, `SELECT stay_id::text, actor_id::text FROM finance.attempts WHERE id = $1`, subject).Scan(&stayID, &actor)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && actor != party) {
			return "", "", time.Time{}, time.Time{}, platform.E("not_found", 404)
		}
		if err != nil {
			return "", "", time.Time{}, time.Time{}, err
		}
		var status string
		var arrival, departure time.Time
		err = s.Pool.QueryRow(ctx, `SELECT status, arrival, departure FROM stays.bookings WHERE id = $1`, stayID).Scan(&status, &arrival, &departure)
		return stayID, status, arrival, departure, err
	case "service":
		var ok bool
		err := s.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM visits.appointments v JOIN clients.profiles p ON p.id = v.profile_id
				WHERE v.id = $1 AND p.user_id = $2
			)
		`, subject, party).Scan(&ok)
		if err != nil {
			return "", "", time.Time{}, time.Time{}, err
		}
		if !ok {
			return "", "", time.Time{}, time.Time{}, platform.E("not_found", 404)
		}
		return "", "", time.Time{}, time.Time{}, nil
	default:
		return "", "", time.Time{}, time.Time{}, platform.E("validation", 422)
	}
}

func lock(ctx context.Context, tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (caseRow, error) {
	var row caseRow
	err := tx.QueryRow(ctx, `
		SELECT id::text, serial, party_user_id::text, status, type, description, occurred_at, requested_amount_minor,
		       subject_kind, subject_id::text, COALESCE(stay_id::text,''), priority, response_due_at,
		       COALESCE(assignee_id::text,''), proposal_kind, proposal_amount_minor, proposal_reason, proposal_actions,
		       COALESCE(proposed_by::text,''), party_notice_at, resolved_at, policy_version, execution_status,
		       COALESCE(refund_id::text,'')
		FROM disputes.cases WHERE id = $1 FOR UPDATE
	`, id).Scan(
		&row.ID, &row.Serial, &row.PartyID, &row.Status, &row.Type, &row.Description, &row.OccurredAt, &row.Requested,
		&row.SubjectKind, &row.SubjectID, &row.StayID, &row.Priority, &row.ResponseDue, &row.AssigneeID,
		&row.ProposalKind, &row.ProposalAmount, &row.ProposalReason, &row.ProposalActions, &row.ProposedBy,
		&row.PartyNotice, &row.ResolvedAt, &row.PolicyVersion, &row.Execution, &row.RefundID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return caseRow{}, platform.E("not_found", 404)
	}
	return row, err
}

func (s Service) load(ctx context.Context, id string) (Case, string, error) {
	row, err := lock(ctx, s.Pool, id)
	if err != nil {
		return Case{}, "", err
	}
	return present(row), row.PartyID, nil
}

func present(row caseRow) Case {
	item := Case{
		ID: row.ID, Reference: fmt.Sprintf("N-%d", row.Serial), Status: row.Status, Type: row.Type,
		Description: row.Description, OccurredAt: row.OccurredAt, RequestedAmount: row.Requested,
		SubjectKind: row.SubjectKind, SubjectID: row.SubjectID, StayID: row.StayID, Priority: row.Priority,
		ResponseDue: row.ResponseDue, AssigneeID: row.AssigneeID, ExecutionStatus: row.Execution,
		RefundID: row.RefundID, PolicyVersion: row.PolicyVersion, ObjectionDays: ObjectionDays,
		ExternalDelivery: "not_sent",
	}
	return item
}

func (s Service) fill(ctx context.Context, item *Case, staff bool) error {
	if item.RefundID != "" {
		var status string
		if err := s.Pool.QueryRow(ctx, `SELECT status FROM finance.refunds WHERE id = $1`, item.RefundID).Scan(&status); err == nil {
			item.RefundSuccess = status == "successful"
		}
	}
	var amount int64
	var other bool
	err := s.Pool.QueryRow(ctx, `SELECT amount_minor, other_funds_frozen FROM disputes.holds WHERE case_id = $1`, item.ID).Scan(&amount, &other)
	if err == nil {
		item.HoldAmount = &amount
		item.OtherFundsFrozen = other
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, sha256, content_type, size_bytes, uploaded_at FROM disputes.evidence WHERE case_id = $1 ORDER BY uploaded_at
	`, item.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	item.Evidence = make([]Evidence, 0)
	for rows.Next() {
		var ev Evidence
		if err := rows.Scan(&ev.ID, &ev.SHA256, &ev.ContentType, &ev.SizeBytes, &ev.UploadedAt); err != nil {
			return err
		}
		item.Evidence = append(item.Evidence, ev)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	noteQuery := `SELECT id::text, visibility, body, created_at FROM disputes.notes WHERE case_id = $1`
	if !staff {
		noteQuery += ` AND visibility = 'party'`
	}
	noteQuery += ` ORDER BY created_at`
	notes, err := s.Pool.Query(ctx, noteQuery, item.ID)
	if err != nil {
		return err
	}
	defer notes.Close()
	item.Notes = make([]Note, 0)
	for notes.Next() {
		var note Note
		if err := notes.Scan(&note.ID, &note.Visibility, &note.Body, &note.CreatedAt); err != nil {
			return err
		}
		item.Notes = append(item.Notes, note)
	}
	events, err := s.Pool.Query(ctx, `
		SELECT id::text, kind, from_status, to_status, reason, notice_status, created_at
		FROM disputes.events WHERE case_id = $1 ORDER BY created_at
	`, item.ID)
	if err != nil {
		return err
	}
	defer events.Close()
	item.Events = make([]Event, 0)
	for events.Next() {
		var event Event
		if err := events.Scan(&event.ID, &event.Kind, &event.From, &event.To, &event.Reason, &event.NoticeStatus, &event.CreatedAt); err != nil {
			return err
		}
		item.Events = append(item.Events, event)
	}
	return events.Err()
}

func (s Service) event(ctx context.Context, tx pgx.Tx, id, actor, kind, from, to, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO disputes.events (id, case_id, actor_id, kind, from_status, to_status, reason)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
	`, id, actor, kind, from, to, reason)
	return err
}
