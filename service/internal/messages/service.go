package messages

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

type Preview struct {
	SubjectKind    string `json:"subject_kind"`
	SubjectID      string `json:"subject_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Received       int    `json:"received"`
	Created        bool   `json:"created"`
}

type Item struct {
	ID              string     `json:"id"`
	SubjectKind     string     `json:"subject_kind"`
	SubjectID       string     `json:"subject_id"`
	ReviewStatus    string     `json:"review_status"`
	Unread          int        `json:"unread"`
	Received        int        `json:"received"`
	AssigneeID      string     `json:"assignee_id,omitempty"`
	RecipientLocale string     `json:"recipient_locale"`
	ClientName      string     `json:"client_name"`
	FirstHumanReply *time.Time `json:"first_human_reply_at,omitempty"`
	ProviderAdopted bool       `json:"provider_adopted"`
	DeliveryStatus  string     `json:"delivery_status"`
}

type Message struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	SenderID        string    `json:"sender_id"`
	Kind            string    `json:"kind"`
	Body            string    `json:"body"`
	Hidden          bool      `json:"hidden"`
	HideReason      string    `json:"hide_reason,omitempty"`
	SendStatus      string    `json:"send_status"`
	DeliveryStatus  string    `json:"delivery_status"`
	ProviderAdopted bool      `json:"provider_adopted"`
	CorrectsID      string    `json:"corrects_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Canned struct {
	ID     string `json:"id"`
	Locale string `json:"locale"`
	Body   string `json:"body"`
}

type PostInput struct {
	ActorID        string
	Staff          bool
	CompanyID      string
	RequestID      string
	Key            string
	SubjectKind    string
	SubjectID      string
	ConversationID string
	Body           string
	Kind           string
	CorrectsID     string
	CannedID       string
	Locale         string
	Fields         map[string]string
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Preview(ctx context.Context, actor, kind, subject string) (Preview, error) {
	if !SubjectKind(kind) || subject == "" {
		return Preview{}, platform.E("validation", 422)
	}
	if err := s.owned(ctx, actor, kind, subject); err != nil {
		return Preview{}, err
	}
	out := Preview{SubjectKind: kind, SubjectID: subject, Created: false}
	var id string
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text FROM messages.conversations
		WHERE subject_kind = $1 AND subject_id = $2 AND created_by = $3
	`, kind, subject, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Preview{}, err
	}
	out.ConversationID = id
	if err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM messages.messages
		WHERE conversation_id = $1 AND kind <> 'internal_note' AND hidden_at IS NULL
	`, id).Scan(&out.Received); err != nil {
		return Preview{}, err
	}
	return out, nil
}

func (s Service) Post(ctx context.Context, in PostInput) (Message, error) {
	if in.Key == "" {
		return Message{}, platform.E("idempotency_required", 400)
	}
	if RefusesCard(in.Fields) || RefusesCard(map[string]string{"body": in.Body}) {
		return Message{}, platform.E("card_data_refused", 422)
	}
	kind := in.Kind
	if kind == "" {
		kind = "message"
	}
	if in.CannedID != "" {
		kind = "canned"
	}
	if kind == "internal_note" && !in.Staff {
		return Message{}, platform.E("forbidden", 403)
	}
	if kind == "canned" && !in.Staff {
		return Message{}, platform.E("forbidden", 403)
	}
	if in.CorrectsID != "" {
		kind = "correction"
	}
	body := strings.TrimSpace(in.Body)
	cannedLocale := ""
	if kind == "canned" {
		loaded, locale, err := s.canned(ctx, in.CannedID)
		if err != nil {
			return Message{}, err
		}
		body = loaded
		cannedLocale = locale
	}
	if code := BodyCode(body); code != "" && kind != "canned" {
		return Message{}, platform.E(code, 422)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(ctx)
	var existing, existingBody string
	err = tx.QueryRow(ctx, `SELECT id::text, body FROM messages.messages WHERE sender_id = $1 AND idempotency_key = $2`, in.ActorID, in.Key).Scan(&existing, &existingBody)
	if err == nil {
		if existingBody != body {
			return Message{}, platform.E("idempotency_mismatch", 409)
		}
		return s.loadMessage(ctx, existing, in.Staff)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, err
	}
	conv, createdBy, locale, err := s.ensureConversation(ctx, tx, in)
	if err != nil {
		return Message{}, err
	}
	if kind == "canned" && locale != cannedLocale {
		return Message{}, platform.E("canned_locale", 422)
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO messages.messages (
			id, conversation_id, sender_id, kind, body, corrects_id, idempotency_key
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, NULLIF($5,'')::uuid, $6
		) RETURNING id::text
	`, conv, in.ActorID, kind, body, in.CorrectsID, in.Key).Scan(&id)
	if err != nil {
		return Message{}, err
	}
	if in.Staff && (kind == "message" || kind == "correction") {
		if _, err := tx.Exec(ctx, `
			UPDATE messages.conversations SET first_human_reply_at = COALESCE(first_human_reply_at, now())
			WHERE id = $1
		`, conv); err != nil {
			return Message{}, err
		}
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: in.ActorID, Action: "messages.post", EntityType: "conversation", EntityID: conv,
		CompanyID: in.CompanyID, RequestID: in.RequestID, Result: "ok",
	}); err != nil {
		return Message{}, err
	}
	_ = createdBy
	if err := tx.Commit(ctx); err != nil {
		return Message{}, err
	}
	return s.loadMessage(ctx, id, in.Staff)
}

func (s Service) Inbox(ctx context.Context, actor string, staff bool) ([]Item, error) {
	query := `
		SELECT c.id::text, c.subject_kind, c.subject_id::text, c.review_status,
		       COALESCE(c.assignee_id::text,''), c.recipient_locale,
		       COALESCE((
		         SELECT u.display_name FROM messages.participants p
		         JOIN identity.users u ON u.id = p.user_id
		         WHERE p.conversation_id = c.id AND p.role = 'client'
		         LIMIT 1
		       ), ''),
		       c.first_human_reply_at,
		       (SELECT count(*) FROM messages.messages m
		         WHERE m.conversation_id = c.id AND m.kind <> 'internal_note' AND m.hidden_at IS NULL),
		       (SELECT count(*) FROM messages.messages m
		         WHERE m.conversation_id = c.id AND m.sender_id <> $1
		           AND m.kind IN ('message','correction','canned') AND m.hidden_at IS NULL
		           AND (r.read_at IS NULL OR m.created_at > r.read_at))
		FROM messages.conversations c
		LEFT JOIN messages.reads r ON r.conversation_id = c.id AND r.user_id = $1
	`
	args := []any{actor}
	if staff {
		query += ` WHERE c.company_id = $2 ORDER BY c.created_at DESC LIMIT 50`
		args = append(args, companyOf(ctx, s, actor))
	} else {
		query += ` WHERE EXISTS (
			SELECT 1 FROM messages.participants p WHERE p.conversation_id = c.id AND p.user_id = $1 AND p.role = 'client'
		) AND EXISTS (
			SELECT 1 FROM messages.messages m WHERE m.conversation_id = c.id AND m.kind <> 'internal_note' AND m.hidden_at IS NULL
		) ORDER BY c.created_at DESC LIMIT 50`
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.SubjectKind, &item.SubjectID, &item.ReviewStatus, &item.AssigneeID, &item.RecipientLocale, &item.ClientName, &item.FirstHumanReply, &item.Received, &item.Unread); err != nil {
			return nil, err
		}
		item.ProviderAdopted = false
		item.DeliveryStatus = "not_sent"
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) Thread(ctx context.Context, actor, conversationID, after string, staff, reveal bool) ([]Message, error) {
	if err := s.canRead(ctx, actor, conversationID, staff); err != nil {
		return nil, err
	}
	query := `
		SELECT id::text, conversation_id::text, sender_id::text, kind,
		       CASE WHEN hidden_at IS NULL OR $2 THEN body ELSE '' END,
		       hidden_at IS NOT NULL, hide_reason, send_status, delivery_status, provider_adopted,
		       COALESCE(corrects_id::text,''), created_at
		FROM messages.messages
		WHERE conversation_id = $1
	`
	args := []any{conversationID, reveal}
	if !staff {
		query += ` AND kind <> 'internal_note'`
	}
	if after != "" {
		query += ` AND (created_at, id) > (SELECT created_at, id FROM messages.messages WHERE id = $3 AND conversation_id = $1)`
		args = append(args, after)
	}
	query += ` ORDER BY created_at, id`
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Message, 0)
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.SenderID, &item.Kind, &item.Body, &item.Hidden, &item.HideReason, &item.SendStatus, &item.DeliveryStatus, &item.ProviderAdopted, &item.CorrectsID, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) MarkRead(ctx context.Context, actor, conversationID string, staff bool) error {
	if err := s.canRead(ctx, actor, conversationID, staff); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO messages.reads (conversation_id, user_id, read_at)
		VALUES ($1, $2, now())
		ON CONFLICT (conversation_id, user_id) DO UPDATE SET read_at = now()
	`, conversationID, actor)
	return err
}

func (s Service) Report(ctx context.Context, actor, requestID, company, conversationID, kind, reason string) error {
	if reason == "" || (kind != "abuse" && kind != "block") {
		return platform.E("reason_required", 422)
	}
	if err := s.canRead(ctx, actor, conversationID, false); err != nil {
		return err
	}
	status := "abuse_review"
	if kind == "block" {
		status = "blocked_review"
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO messages.reports (id, conversation_id, reporter_id, reason, kind)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, conversationID, actor, reason, kind); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE messages.conversations SET review_status = $2 WHERE id = $1`, conversationID, status); err != nil {
		return err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "messages.report", EntityType: "conversation", EntityID: conversationID,
		CompanyID: company, RequestID: requestID, Result: "ok", Reason: reason,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Service) Hide(ctx context.Context, actor, requestID, company, messageID, reason string) (Message, error) {
	if strings.TrimSpace(reason) == "" {
		return Message{}, platform.E("reason_required", 422)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(ctx)
	var conv, body string
	err = tx.QueryRow(ctx, `SELECT conversation_id::text, body FROM messages.messages WHERE id = $1 FOR UPDATE`, messageID).Scan(&conv, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Message{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE messages.messages SET hidden_at = now(), hidden_by = $2, hide_reason = $3
		WHERE id = $1 AND hidden_at IS NULL
	`, messageID, actor, reason); err != nil {
		return Message{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "messages.hide", EntityType: "message", EntityID: messageID,
		CompanyID: company, RequestID: requestID, Result: "ok", Reason: reason, Before: map[string]string{"body": body},
	}); err != nil {
		return Message{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Message{}, err
	}
	return s.loadMessage(ctx, messageID, true)
}

func (s Service) Assign(ctx context.Context, actor, requestID, company, conversationID, assignee string) error {
	if assignee == "" {
		return platform.E("validation", 422)
	}
	var granted bool
	if err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM permissions.rules
			WHERE grantee_id = $1 AND domain = 'messages' AND action = 'view' AND effect = 'allow'
		)
	`, assignee).Scan(&granted); err != nil {
		return err
	}
	if !granted {
		return platform.E("forbidden", 403)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE messages.conversations SET assignee_id = $2 WHERE id = $1`, conversationID, assignee)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return platform.E("not_found", 404)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO messages.participants (conversation_id, user_id, role, added_by)
		VALUES ($1, $2, 'staff', $3)
		ON CONFLICT DO NOTHING
	`, conversationID, assignee, actor); err != nil {
		return err
	}
	if err := audit.Record(ctx, tx, audit.Event{
		ActorID: actor, Action: "messages.assign", EntityType: "conversation", EntityID: conversationID,
		CompanyID: company, RequestID: requestID, Result: "ok",
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Service) Canned(ctx context.Context, locale string) ([]Canned, error) {
	if locale != "en" {
		locale = "ar"
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text, locale, body FROM messages.canned WHERE active AND locale = $1`, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Canned, 0)
	for rows.Next() {
		var item Canned
		if err := rows.Scan(&item.ID, &item.Locale, &item.Body); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) ensureConversation(ctx context.Context, tx pgx.Tx, in PostInput) (string, string, string, error) {
	locale := "ar"
	if in.Locale == "en" {
		locale = "en"
	}
	if in.ConversationID != "" {
		var id, createdBy, stored string
		err := tx.QueryRow(ctx, `
			SELECT id::text, created_by::text, recipient_locale FROM messages.conversations WHERE id = $1
		`, in.ConversationID).Scan(&id, &createdBy, &stored)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", platform.E("not_found", 404)
		}
		if err != nil {
			return "", "", "", err
		}
		if err := s.canReadTx(ctx, tx, in.ActorID, id, in.Staff); err != nil {
			return "", "", "", err
		}
		role := "client"
		if in.Staff {
			role = "staff"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO messages.participants (conversation_id, user_id, role, added_by)
			VALUES ($1, $2, $3, $2) ON CONFLICT DO NOTHING
		`, id, in.ActorID, role); err != nil {
			return "", "", "", err
		}
		return id, createdBy, stored, nil
	}
	if !SubjectKind(in.SubjectKind) || in.SubjectID == "" {
		return "", "", "", platform.E("validation", 422)
	}
	owner := in.ActorID
	if !in.Staff {
		if err := s.owned(ctx, in.ActorID, in.SubjectKind, in.SubjectID); err != nil {
			return "", "", "", err
		}
	} else {
		party, err := s.partyOf(ctx, in.SubjectKind, in.SubjectID)
		if err != nil {
			return "", "", "", err
		}
		owner = party
	}
	var id string
	err := tx.QueryRow(ctx, `
		SELECT id::text FROM messages.conversations
		WHERE subject_kind = $1 AND subject_id = $2 AND created_by = $3
	`, in.SubjectKind, in.SubjectID, owner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO messages.conversations (id, company_id, subject_kind, subject_id, recipient_locale, created_by)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
			RETURNING id::text
		`, in.CompanyID, in.SubjectKind, in.SubjectID, locale, owner).Scan(&id)
	}
	if err != nil {
		return "", "", "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO messages.participants (conversation_id, user_id, role, added_by)
		VALUES ($1, $2, 'client', $3) ON CONFLICT DO NOTHING
	`, id, owner, in.ActorID); err != nil {
		return "", "", "", err
	}
	if in.Staff {
		if _, err := tx.Exec(ctx, `
			INSERT INTO messages.participants (conversation_id, user_id, role, added_by)
			VALUES ($1, $2, 'staff', $2) ON CONFLICT DO NOTHING
		`, id, in.ActorID); err != nil {
			return "", "", "", err
		}
	}
	var stored string
	if err := tx.QueryRow(ctx, `SELECT recipient_locale FROM messages.conversations WHERE id = $1`, id).Scan(&stored); err != nil {
		return "", "", "", err
	}
	return id, owner, stored, nil
}

func (s Service) partyOf(ctx context.Context, kind, subject string) (string, error) {
	var party string
	var err error
	switch kind {
	case "dispute":
		err = s.Pool.QueryRow(ctx, `SELECT party_user_id::text FROM disputes.cases WHERE id = $1`, subject).Scan(&party)
	default:
		return "", platform.E("forbidden", 403)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", platform.E("not_found", 404)
	}
	return party, err
}

func (s Service) owned(ctx context.Context, actor, kind, subject string) error {
	var ok bool
	var err error
	switch kind {
	case "listing":
		err = s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog.listings WHERE id = $1 AND status = 'published')`, subject).Scan(&ok)
	case "visit":
		err = s.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM visits.appointments v JOIN clients.profiles p ON p.id = v.profile_id
				WHERE v.id = $1 AND p.user_id = $2
			)
		`, subject, actor).Scan(&ok)
	case "booking":
		err = s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM stays.bookings WHERE id = $1 AND actor_id = $2)`, subject, actor).Scan(&ok)
	case "deal":
		err = s.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM deals.deals d JOIN clients.profiles p ON p.id = d.profile_id
				WHERE d.id = $1 AND p.user_id = $2
			)
		`, subject, actor).Scan(&ok)
	case "dispute":
		err = s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM disputes.cases WHERE id = $1 AND party_user_id = $2)`, subject, actor).Scan(&ok)
	default:
		return platform.E("validation", 422)
	}
	if err != nil {
		return err
	}
	if !ok {
		return platform.E("not_found", 404)
	}
	return nil
}

func (s Service) canRead(ctx context.Context, actor, conversationID string, staff bool) error {
	return s.canReadTx(ctx, s.Pool, actor, conversationID, staff)
}

type rowQuery interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s Service) canReadTx(ctx context.Context, q rowQuery, actor, conversationID string, staff bool) error {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM messages.conversations WHERE id = $1)`, conversationID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return platform.E("not_found", 404)
	}
	if staff {
		return nil
	}
	var member bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM messages.participants WHERE conversation_id = $1 AND user_id = $2)
	`, conversationID, actor).Scan(&member); err != nil {
		return err
	}
	if !member {
		return platform.E("not_found", 404)
	}
	return nil
}

func (s Service) canned(ctx context.Context, id string) (string, string, error) {
	var body, locale string
	err := s.Pool.QueryRow(ctx, `SELECT body, locale FROM messages.canned WHERE id = $1 AND active`, id).Scan(&body, &locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", platform.E("not_found", 404)
	}
	return body, locale, err
}

func (s Service) loadMessage(ctx context.Context, id string, reveal bool) (Message, error) {
	var item Message
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, conversation_id::text, sender_id::text, kind,
		       CASE WHEN hidden_at IS NULL OR $2 THEN body ELSE '' END,
		       hidden_at IS NOT NULL, hide_reason, send_status, delivery_status, provider_adopted,
		       COALESCE(corrects_id::text,''), created_at
		FROM messages.messages WHERE id = $1
	`, id, reveal).Scan(&item.ID, &item.ConversationID, &item.SenderID, &item.Kind, &item.Body, &item.Hidden, &item.HideReason, &item.SendStatus, &item.DeliveryStatus, &item.ProviderAdopted, &item.CorrectsID, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, platform.E("not_found", 404)
	}
	return item, err
}

func companyOf(ctx context.Context, s Service, actor string) string {
	var company string
	_ = s.Pool.QueryRow(ctx, `SELECT COALESCE(company_id::text,'') FROM identity.users WHERE id = $1`, actor).Scan(&company)
	return company
}
