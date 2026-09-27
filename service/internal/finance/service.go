package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/audit"
	"shamal/service/internal/platform"
)

type StayBill struct {
	ID          string
	ActorID     string
	CompanyID   string
	Transaction string
	Arrival     string
	Status      string
	Nights      int
	Months      int
	StayDue     int64
}

type Attempt struct {
	ID                 string     `json:"id"`
	StayID             string     `json:"stay_id"`
	Method             string     `json:"method"`
	Provider           string     `json:"provider"`
	GatewayAdopted     bool       `json:"gateway_adopted"`
	AmountMinor        int64      `json:"amount_minor"`
	Currency           string     `json:"currency"`
	Status             string     `json:"status"`
	Decisive           bool       `json:"decisive"`
	Success            bool       `json:"success"`
	ReceiptID          string     `json:"receipt_id,omitempty"`
	InvoiceID          string     `json:"invoice_id,omitempty"`
	LabelAR            string     `json:"label_ar"`
	LabelEN            string     `json:"label_en"`
	ReviewDeadline     *time.Time `json:"review_deadline,omitempty"`
	OperationalDefault bool       `json:"operational_default"`
	Contractual        bool       `json:"contractual"`
	BeneficiaryAdopted bool       `json:"beneficiary_adopted"`
	ProofConfirms      bool       `json:"proof_confirms"`
	ProviderQuery      string     `json:"provider_query"`
}

type Invoice struct {
	ID            string        `json:"id"`
	Serial        int64         `json:"serial"`
	StayID        string        `json:"stay_id"`
	AttemptID     string        `json:"attempt_id,omitempty"`
	Currency      string        `json:"currency"`
	TotalMinor    int64         `json:"total_minor"`
	SellerAdopted bool          `json:"seller_adopted"`
	SellerNoteAR  string        `json:"seller_note_ar"`
	SellerNoteEN  string        `json:"seller_note_en"`
	Lines         []InvoiceLine `json:"lines"`
	IssuedAt      time.Time     `json:"issued_at"`
}

type InvoiceLine struct {
	Code         string `json:"code"`
	Quantity     int    `json:"quantity"`
	UnitMinor    int64  `json:"unit_minor"`
	AmountMinor  int64  `json:"amount_minor"`
	CountsAsStay bool   `json:"counts_as_stay"`
}

type Refund struct {
	ID             string `json:"id"`
	StayID         string `json:"stay_id"`
	AmountMinor    int64  `json:"amount_minor"`
	Currency       string `json:"currency"`
	Reason         string `json:"reason"`
	Status         string `json:"status"`
	Success        bool   `json:"success"`
	Approvals      int    `json:"approvals"`
	RequesterID    string `json:"requester_id"`
	FirstApprover  string `json:"first_approver,omitempty"`
	SecondApprover string `json:"second_approver,omitempty"`
}

type ProviderEvent struct {
	ID        string
	AttemptID string
	Amount    int64
	Currency  string
	Outcome   string
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) OpenAttempt(ctx context.Context, actor, requestID, key string, bill StayBill, method string, offered *int64, fields map[string]string) (Attempt, error) {
	if key == "" || len(key) > 200 {
		return Attempt{}, platform.E("idempotency_required", 400)
	}
	if RefusesCard(fields) {
		return Attempt{}, platform.E("card_data_refused", 422)
	}
	if bill.Status != "confirmed" || bill.ActorID != actor {
		return Attempt{}, platform.E("stay_not_confirmed", 422)
	}
	if method != "mada" && method != "visa" && method != "mastercard" && method != "bank_transfer" {
		return Attempt{}, platform.E("invalid_method", 422)
	}
	if offered != nil && *offered != bill.StayDue {
		return Attempt{}, platform.E("amount_mismatch", 422)
	}
	arrival, ok := riyadhMidnight(bill.Arrival)
	if !ok {
		return Attempt{}, platform.E("invalid_dates", 422)
	}
	now := time.Now().UTC()
	var deadline *time.Time
	if method == "bank_transfer" {
		if !TransferOpen(now, arrival) {
			return Attempt{}, platform.E("transfer_too_soon", 422)
		}
		end := now.Add(TransferHold)
		deadline = &end
	}
	hash := requestHash("attempt", bill.ID, method, bill.StayDue)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Attempt{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockStay(ctx, tx, bill.ID); err != nil {
		return Attempt{}, err
	}
	if existing, found, err := readAttemptKey(ctx, tx, actor, key, hash); err != nil {
		return Attempt{}, err
	} else if found {
		return s.loadAttempt(ctx, existing.ID)
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO finance.attempts (
			id, stay_id, actor_id, company_id, method, provider, amount_minor, currency, status,
			gateway_adopted, review_deadline, idempotency_key, request_hash
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, 'test_adapter', $5, 'SAR', 'pending_review',
			false, $6, $7, $8
		) RETURNING id::text
	`, bill.ID, actor, bill.CompanyID, method, bill.StayDue, deadline, key, hash).Scan(&id)
	if err != nil {
		return Attempt{}, err
	}
	invoiceID, err := ensureInvoice(ctx, tx, bill, id)
	if err != nil {
		return Attempt{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "finance.attempt", EntityType: "payment_attempt", EntityID: id,
		CompanyID: bill.CompanyID, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Attempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Attempt{}, err
	}
	item, err := s.loadAttempt(ctx, id)
	if err != nil {
		return Attempt{}, err
	}
	item.InvoiceID = invoiceID
	return item, nil
}

func (s Service) GetAttempt(ctx context.Context, id, actor string) (Attempt, error) {
	item, err := s.loadAttempt(ctx, id)
	if err != nil {
		return Attempt{}, err
	}
	var owner string
	if err := s.Pool.QueryRow(ctx, `SELECT actor_id::text FROM finance.attempts WHERE id = $1`, id).Scan(&owner); err != nil {
		return Attempt{}, err
	}
	if owner != actor {
		return Attempt{}, platform.E("forbidden", 403)
	}
	return item, nil
}

func (s Service) RecordProof(ctx context.Context, actor, attemptID, note string) (Attempt, error) {
	if RefusesCard(map[string]string{"note": note}) {
		return Attempt{}, platform.E("card_data_refused", 422)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE finance.attempts SET proof_note = $3
		WHERE id = $1 AND actor_id = $2 AND proof_confirms = false
	`, attemptID, actor, note)
	if err != nil {
		return Attempt{}, err
	}
	if tag.RowsAffected() == 0 {
		return Attempt{}, platform.E("not_found", 404)
	}
	return s.GetAttempt(ctx, attemptID, actor)
}

func (s Service) Invoice(ctx context.Context, stayID, actor string) (Invoice, error) {
	return loadInvoice(ctx, s.Pool, stayID, actor)
}

func (s Service) CorrectInvoice(ctx context.Context, actor, invoiceID, reason string) error {
	if reason == "" {
		return platform.E("reason_required", 422)
	}
	var owner string
	var total int64
	err := s.Pool.QueryRow(ctx, `SELECT actor_id::text, total_minor FROM finance.invoices WHERE id = $1`, invoiceID).Scan(&owner, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return platform.E("not_found", 404)
	}
	if err != nil {
		return err
	}
	if owner != actor {
		return platform.E("forbidden", 403)
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO finance.corrections (id, invoice_id, actor_id, reason) VALUES (gen_random_uuid(), $1, $2, $3)
	`, invoiceID, actor, reason)
	return err
}

func (s Service) RequestRefund(ctx context.Context, actor, requestID, key string, bill StayBill, amount int64, reason string) (Refund, error) {
	if key == "" {
		return Refund{}, platform.E("idempotency_required", 400)
	}
	if reason == "" {
		return Refund{}, platform.E("reason_required", 422)
	}
	if bill.ActorID != actor || bill.Status != "confirmed" {
		return Refund{}, platform.E("stay_not_confirmed", 422)
	}
	hash := requestHash("refund", bill.ID, reason, amount)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Refund{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockStay(ctx, tx, bill.ID); err != nil {
		return Refund{}, err
	}
	if existing, found, err := readRefundKey(ctx, tx, actor, key, hash); err != nil {
		return Refund{}, err
	} else if found {
		return existing, nil
	}
	collected, err := collectedMinor(ctx, tx, bill.ID)
	if err != nil {
		return Refund{}, err
	}
	pending, err := pendingRefunds(ctx, tx, bill.ID)
	if err != nil {
		return Refund{}, err
	}
	if collected == 0 {
		return Refund{}, platform.E("nothing_collected", 422)
	}
	if !RefundRoom(collected, pending, amount) {
		return Refund{}, platform.E("refund_exceeds_collected", 422)
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO finance.refunds (
			id, stay_id, requester_id, amount_minor, currency, reason, status, idempotency_key, request_hash
		) VALUES (
			gen_random_uuid(), $1, $2, $3, 'SAR', $4, 'under_review', $5, $6
		) RETURNING id::text
	`, bill.ID, actor, amount, reason, key, hash).Scan(&id)
	if err != nil {
		return Refund{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "finance.refund_request", EntityType: "refund", EntityID: id,
		CompanyID: bill.CompanyID, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Refund{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Refund{}, err
	}
	return s.loadRefund(ctx, id)
}

func (s Service) ApproveRefund(ctx context.Context, approver, requestID, companyID, refundID string) (Refund, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Refund{}, err
	}
	defer tx.Rollback(ctx)
	item, err := lockRefund(ctx, tx, refundID)
	if err != nil {
		return Refund{}, err
	}
	if item.RequesterID == approver || item.FirstApprover == approver {
		return Refund{}, platform.E("separation_of_duties", 403)
	}
	if item.Status == "approved" || item.Status == "successful" {
		return item, nil
	}
	if item.Status != "under_review" {
		return Refund{}, platform.E("invalid_transition", 409)
	}
	if item.FirstApprover == "" {
		_, err = tx.Exec(ctx, `UPDATE finance.refunds SET first_approver = $2 WHERE id = $1 AND status = 'under_review'`, refundID, approver)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE finance.refunds SET second_approver = $2, status = 'approved'
			WHERE id = $1 AND status = 'under_review' AND second_approver IS NULL
		`, refundID, approver)
	}
	if err != nil {
		return Refund{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: approver, Action: "finance.refund_approve", EntityType: "refund", EntityID: refundID,
		CompanyID: companyID, RequestID: requestID, Result: "ok",
	}); err != nil {
		return Refund{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Refund{}, err
	}
	return s.loadRefund(ctx, refundID)
}

func (s Service) CancelRefund(ctx context.Context, actor, refundID string) (Refund, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE finance.refunds SET status = 'cancelled'
		WHERE id = $1 AND requester_id = $2 AND status = 'under_review'
	`, refundID, actor)
	if err != nil {
		return Refund{}, err
	}
	if tag.RowsAffected() == 0 {
		current, loadErr := s.loadRefund(ctx, refundID)
		if loadErr != nil {
			return Refund{}, loadErr
		}
		if current.Status == "cancelled" {
			return current, nil
		}
		return Refund{}, platform.E("invalid_transition", 409)
	}
	item, err := s.loadRefund(ctx, refundID)
	if err != nil {
		return Refund{}, err
	}
	if item.Success {
		return Refund{}, platform.E("invalid_transition", 409)
	}
	return item, nil
}

func (s Service) ListRefunds(ctx context.Context) ([]Refund, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, stay_id::text, amount_minor, currency, reason, status, requester_id::text,
		       COALESCE(first_approver::text,''), COALESCE(second_approver::text,'')
		FROM finance.refunds ORDER BY created_at DESC LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Refund, 0)
	for rows.Next() {
		item, err := scanRefund(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) ListStayRefunds(ctx context.Context, stayID, actor string) ([]Refund, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, stay_id::text, amount_minor, currency, reason, status, requester_id::text,
		       COALESCE(first_approver::text,''), COALESCE(second_approver::text,'')
		FROM finance.refunds WHERE stay_id = $1 AND requester_id = $2 ORDER BY created_at DESC LIMIT 50
	`, stayID, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Refund, 0)
	for rows.Next() {
		item, err := scanRefund(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) ApplyProviderEvent(ctx context.Context, event ProviderEvent) (Attempt, error) {
	if event.ID == "" || event.AttemptID == "" {
		return Attempt{}, platform.E("validation", 422)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Attempt{}, err
	}
	defer tx.Rollback(ctx)
	var stayID string
	var amount int64
	err = tx.QueryRow(ctx, `
		SELECT stay_id::text, amount_minor FROM finance.attempts WHERE id = $1 FOR UPDATE
	`, event.AttemptID).Scan(&stayID, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Attempt{}, err
	}
	var existing string
	err = tx.QueryRow(ctx, `SELECT attempt_id::text FROM finance.provider_events WHERE id = $1`, event.ID).Scan(&existing)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return Attempt{}, err
		}
		return s.loadAttempt(ctx, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO finance.provider_events (id, attempt_id, amount_minor, currency, outcome)
		VALUES ($1, $2, $3, $4, $5)
	`, event.ID, event.AttemptID, event.Amount, event.Currency, event.Outcome); err != nil {
		return Attempt{}, err
	}
	if event.Outcome == "captured" && event.Amount == amount && event.Currency == "SAR" {
		if _, err := tx.Exec(ctx, `UPDATE finance.attempts SET status = 'collected' WHERE id = $1`, event.AttemptID); err != nil {
			return Attempt{}, err
		}
		var invoiceID string
		_ = tx.QueryRow(ctx, `SELECT id::text FROM finance.invoices WHERE stay_id = $1`, stayID).Scan(&invoiceID)
		if err := postJournal(ctx, tx, "collection", stayID, event.AttemptID, invoiceID, event.ID, []ledgerLine{
			{Account: "provider_clearing", Direction: "debit", Amount: amount},
			{Account: "receivable", Direction: "credit", Amount: amount},
		}); err != nil {
			return Attempt{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO finance.receipts (id, attempt_id, invoice_id, amount_minor, currency, event_id)
			VALUES (gen_random_uuid(), $1, NULLIF($2,'')::uuid, $3, 'SAR', $4)
		`, event.AttemptID, invoiceID, amount, event.ID); err != nil {
			return Attempt{}, err
		}
	} else if event.Outcome == "captured" {
		if _, err := tx.Exec(ctx, `UPDATE finance.attempts SET status = 'mismatch_review' WHERE id = $1`, event.AttemptID); err != nil {
			return Attempt{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Attempt{}, err
	}
	return s.loadAttempt(ctx, event.AttemptID)
}

func (s Service) JournalCount(ctx context.Context, stayID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM finance.journals WHERE stay_id = $1`, stayID).Scan(&n)
	return n, err
}

func (s Service) LedgerBalanced(ctx context.Context, stayID string) (bool, error) {
	var debit, credit int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_minor) FILTER (WHERE direction = 'debit'), 0),
		       COALESCE(SUM(amount_minor) FILTER (WHERE direction = 'credit'), 0)
		FROM finance.ledger_lines l
		JOIN finance.journals j ON j.id = l.journal_id
		WHERE j.stay_id = $1
	`, stayID).Scan(&debit, &credit)
	return JournalBalanced(debit, credit), err
}

type ledgerLine struct {
	Account   string
	Direction string
	Amount    int64
}

func postJournal(ctx context.Context, tx pgx.Tx, kind, stayID, attemptID, invoiceID, eventID string, lines []ledgerLine) error {
	var debit, credit int64
	for _, line := range lines {
		if line.Direction == "debit" {
			debit += line.Amount
		} else {
			credit += line.Amount
		}
	}
	if !JournalBalanced(debit, credit) {
		return platform.E("unbalanced", 500)
	}
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO finance.journals (id, kind, stay_id, attempt_id, invoice_id, event_id)
		VALUES (gen_random_uuid(), $1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,''))
		RETURNING id::text
	`, kind, stayID, attemptID, invoiceID, eventID).Scan(&id)
	if err != nil {
		return err
	}
	for _, line := range lines {
		if line.Amount <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO finance.ledger_lines (id, journal_id, account, direction, amount_minor, currency)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, 'SAR')
		`, id, line.Account, line.Direction, line.Amount); err != nil {
			return err
		}
	}
	return nil
}

func ensureInvoice(ctx context.Context, tx pgx.Tx, bill StayBill, attemptID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM finance.invoices WHERE stay_id = $1`, bill.ID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO finance.invoices (id, serial, stay_id, attempt_id, actor_id, company_id, currency, total_minor, seller_adopted)
		VALUES (gen_random_uuid(), nextval('finance.invoice_serial'), $1, $2, $3, $4, 'SAR', $5, false)
		RETURNING id::text
	`, bill.ID, attemptID, bill.ActorID, bill.CompanyID, bill.StayDue).Scan(&id)
	if err != nil {
		return "", err
	}
	qty, unit := billQuantity(bill)
	lines := []InvoiceLine{
		{Code: "base", Quantity: qty, UnitMinor: unit, AmountMinor: bill.StayDue, CountsAsStay: true},
		{Code: "discount", Quantity: 1, UnitMinor: 0, AmountMinor: 0, CountsAsStay: true},
		{Code: "service", Quantity: 1, UnitMinor: 0, AmountMinor: 0, CountsAsStay: true},
		{Code: "tax", Quantity: 1, UnitMinor: 0, AmountMinor: 0, CountsAsStay: true},
		{Code: "deposit", Quantity: 1, UnitMinor: 0, AmountMinor: 0, CountsAsStay: false},
	}
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO finance.invoice_lines (id, invoice_id, code, quantity, unit_minor, amount_minor, counts_as_stay)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		`, id, line.Code, line.Quantity, line.UnitMinor, line.AmountMinor, line.CountsAsStay); err != nil {
			return "", err
		}
	}
	if err := postJournal(ctx, tx, "invoice", bill.ID, attemptID, id, "", []ledgerLine{
		{Account: "receivable", Direction: "debit", Amount: bill.StayDue},
		{Account: "stay_revenue", Direction: "credit", Amount: bill.StayDue},
	}); err != nil {
		return "", err
	}
	return id, nil
}

func billQuantity(bill StayBill) (int, int64) {
	qty := bill.Nights
	if bill.Transaction == "monthly" {
		qty = bill.Months
	}
	if qty > 0 && bill.StayDue%int64(qty) == 0 {
		return qty, bill.StayDue / int64(qty)
	}
	return 1, bill.StayDue
}

func lockStay(ctx context.Context, tx pgx.Tx, stayID string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO finance.stay_locks (stay_id) VALUES ($1) ON CONFLICT DO NOTHING`, stayID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT stay_id FROM finance.stay_locks WHERE stay_id = $1 FOR UPDATE`, stayID)
	return err
}

func collectedMinor(ctx context.Context, tx pgx.Tx, stayID string) (int64, error) {
	var amount int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.amount_minor), 0)
		FROM finance.ledger_lines l
		JOIN finance.journals j ON j.id = l.journal_id
		WHERE j.stay_id = $1 AND j.kind = 'collection' AND l.account = 'provider_clearing' AND l.direction = 'debit'
	`, stayID).Scan(&amount)
	return amount, err
}

func pendingRefunds(ctx context.Context, tx pgx.Tx, stayID string) (int64, error) {
	var amount int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_minor), 0) FROM finance.refunds
		WHERE stay_id = $1 AND status IN ('under_review','approved','sent_to_provider','successful')
	`, stayID).Scan(&amount)
	return amount, err
}

func (s Service) loadAttempt(ctx context.Context, id string) (Attempt, error) {
	var item Attempt
	var deadline *time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT a.id::text, a.stay_id::text, a.method, a.provider, a.gateway_adopted, a.amount_minor, a.currency, a.status,
		       a.review_deadline, a.proof_confirms, COALESCE(i.id::text,''), COALESCE(r.id::text,'')
		FROM finance.attempts a
		LEFT JOIN finance.invoices i ON i.stay_id = a.stay_id
		LEFT JOIN finance.receipts r ON r.attempt_id = a.id
		WHERE a.id = $1
	`, id).Scan(&item.ID, &item.StayID, &item.Method, &item.Provider, &item.GatewayAdopted, &item.AmountMinor, &item.Currency, &item.Status, &deadline, &item.ProofConfirms, &item.InvoiceID, &item.ReceiptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Attempt{}, err
	}
	item.ReviewDeadline = deadline
	return presentAttempt(item), nil
}

func presentAttempt(item Attempt) Attempt {
	item.GatewayAdopted = false
	item.Contractual = false
	item.OperationalDefault = item.Method == "bank_transfer"
	item.BeneficiaryAdopted = false
	item.ProofConfirms = false
	item.ProviderQuery = "not_run"
	item.Success = false
	item.Decisive = false
	switch item.Status {
	case StatusCollected:
		item.Decisive = item.ReceiptID != ""
		item.Success = item.Decisive
		item.LabelAR = "تم التحصيل وفق إشعار المزود"
		item.LabelEN = "Collected from a provider notice"
	case StatusMismatch:
		item.LabelAR = "قيد مراجعة فرق المبلغ"
		item.LabelEN = "Amount difference is in review"
	default:
		item.Status = StatusReview
		item.LabelAR = "قيد التحقق"
		item.LabelEN = "Pending verification"
	}
	return item
}

func loadInvoice(ctx context.Context, db platform.DB, stayID, actor string) (Invoice, error) {
	var item Invoice
	err := db.QueryRow(ctx, `
		SELECT id::text, serial, stay_id::text, COALESCE(attempt_id::text,''), currency, total_minor, seller_adopted, issued_at
		FROM finance.invoices WHERE stay_id = $1 AND actor_id = $2
	`, stayID, actor).Scan(&item.ID, &item.Serial, &item.StayID, &item.AttemptID, &item.Currency, &item.TotalMinor, &item.SellerAdopted, &item.IssuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Invoice{}, err
	}
	item.SellerAdopted = false
	item.SellerNoteAR = "جهة إصدار الفاتورة لم تُعتمد بعد. هذه ليست إيصال تحصيل."
	item.SellerNoteEN = "The invoice issuer has not been adopted. This is not a collection receipt."
	rows, err := db.Query(ctx, `
		SELECT code, quantity, unit_minor, amount_minor, counts_as_stay
		FROM finance.invoice_lines WHERE invoice_id = $1 ORDER BY code
	`, item.ID)
	if err != nil {
		return Invoice{}, err
	}
	defer rows.Close()
	item.Lines = make([]InvoiceLine, 0)
	for rows.Next() {
		var line InvoiceLine
		if err := rows.Scan(&line.Code, &line.Quantity, &line.UnitMinor, &line.AmountMinor, &line.CountsAsStay); err != nil {
			return Invoice{}, err
		}
		item.Lines = append(item.Lines, line)
	}
	return item, rows.Err()
}

func (s Service) loadRefund(ctx context.Context, id string) (Refund, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, stay_id::text, amount_minor, currency, reason, status, requester_id::text,
		       COALESCE(first_approver::text,''), COALESCE(second_approver::text,'')
		FROM finance.refunds WHERE id = $1
	`, id)
	item, err := scanRefund(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, platform.E("not_found", 404)
	}
	return item, err
}

func lockRefund(ctx context.Context, tx pgx.Tx, id string) (Refund, error) {
	row := tx.QueryRow(ctx, `
		SELECT id::text, stay_id::text, amount_minor, currency, reason, status, requester_id::text,
		       COALESCE(first_approver::text,''), COALESCE(second_approver::text,'')
		FROM finance.refunds WHERE id = $1 FOR UPDATE
	`, id)
	item, err := scanRefund(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, platform.E("not_found", 404)
	}
	return item, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRefund(row scanner) (Refund, error) {
	var item Refund
	err := row.Scan(&item.ID, &item.StayID, &item.AmountMinor, &item.Currency, &item.Reason, &item.Status, &item.RequesterID, &item.FirstApprover, &item.SecondApprover)
	if err != nil {
		return Refund{}, err
	}
	if item.FirstApprover != "" {
		item.Approvals = 1
	}
	if item.SecondApprover != "" {
		item.Approvals = 2
	}
	item.Success = false
	return item, nil
}

func readAttemptKey(ctx context.Context, tx pgx.Tx, actor, key, hash string) (Attempt, bool, error) {
	var id, stored string
	err := tx.QueryRow(ctx, `
		SELECT id::text, request_hash FROM finance.attempts WHERE actor_id = $1 AND idempotency_key = $2
	`, actor, key).Scan(&id, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, false, nil
	}
	if err != nil {
		return Attempt{}, false, err
	}
	if stored != hash {
		return Attempt{}, false, platform.E("idempotency_mismatch", 409)
	}
	var item Attempt
	err = tx.QueryRow(ctx, `
		SELECT id::text, stay_id::text, method, provider, gateway_adopted, amount_minor, currency, status, review_deadline, proof_confirms
		FROM finance.attempts WHERE id = $1
	`, id).Scan(&item.ID, &item.StayID, &item.Method, &item.Provider, &item.GatewayAdopted, &item.AmountMinor, &item.Currency, &item.Status, &item.ReviewDeadline, &item.ProofConfirms)
	if err != nil {
		return Attempt{}, false, err
	}
	return presentAttempt(item), true, nil
}

func readRefundKey(ctx context.Context, tx pgx.Tx, actor, key, hash string) (Refund, bool, error) {
	var id, stored string
	err := tx.QueryRow(ctx, `
		SELECT id::text, request_hash FROM finance.refunds WHERE requester_id = $1 AND idempotency_key = $2
	`, actor, key).Scan(&id, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, false, nil
	}
	if err != nil {
		return Refund{}, false, err
	}
	if stored != hash {
		return Refund{}, false, platform.E("idempotency_mismatch", 409)
	}
	row := tx.QueryRow(ctx, `
		SELECT id::text, stay_id::text, amount_minor, currency, reason, status, requester_id::text,
		       COALESCE(first_approver::text,''), COALESCE(second_approver::text,'')
		FROM finance.refunds WHERE id = $1
	`, id)
	item, err := scanRefund(row)
	return item, err == nil, err
}

func requestHash(kind, stay, extra string, amount int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", kind, stay, extra, amount)))
	return hex.EncodeToString(sum[:])
}

func riyadhMidnight(raw string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}
	loc, err := time.LoadLocation("Asia/Riyadh")
	if err != nil {
		loc = time.FixedZone("AST", 3*60*60)
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc), true
}
