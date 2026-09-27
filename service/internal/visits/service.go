package visits

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

type Visit struct {
	ID                string     `json:"id"`
	CompanyID         string     `json:"company_id"`
	BranchID          string     `json:"branch_id"`
	ProfileID         string     `json:"profile_id"`
	ListingID         string     `json:"listing_id"`
	PropertyID        string     `json:"property_id"`
	UnitID            string     `json:"unit_id"`
	BrokerUserID      string     `json:"broker_user_id"`
	HostUserID        string     `json:"host_user_id"`
	StartsAt          time.Time  `json:"starts_at"`
	EndsAt            time.Time  `json:"ends_at"`
	Attendees         int        `json:"attendees"`
	Status            string     `json:"status"`
	BrokerConfirmedAt *time.Time `json:"broker_confirmed_at,omitempty"`
	OwnerConfirmedAt  *time.Time `json:"owner_confirmed_at,omitempty"`
	Result            string     `json:"result,omitempty"`
	ResultNote        string     `json:"result_note,omitempty"`
	InstructionsAR    string     `json:"instructions_ar"`
	InstructionsEN    string     `json:"instructions_en"`
	CancelReason      string     `json:"cancel_reason,omitempty"`
	EscalateAt        *time.Time `json:"escalate_at,omitempty"`
	Overdue           bool       `json:"overdue"`
	Version           int        `json:"version"`
	CreatedBy         string     `json:"created_by"`
	CreatedAt         time.Time  `json:"created_at"`
	DisplayName       string     `json:"display_name"`
}

type Draft struct {
	CompanyID      string
	BranchID       string
	ProfileID      string
	ListingID      string
	PropertyID     string
	UnitID         string
	BrokerUserID   string
	HostUserID     string
	StartsAt       time.Time
	EndsAt         time.Time
	Attendees      int
	InstructionsAR string
	InstructionsEN string
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Create(ctx context.Context, actor, requestID string, in Draft) (Visit, error) {
	if in.ProfileID == "" || in.StartsAt.IsZero() {
		return Visit{}, platform.E("validation", 422)
	}
	if in.Attendees <= 0 {
		in.Attendees = 1
	}
	in.EndsAt = EndOrDefault(in.StartsAt, in.EndsAt)
	status := StatusNew
	if in.BrokerUserID != "" && (in.ListingID != "" || in.PropertyID != "") {
		status = StatusAwaitingBroker
	}
	escalate := in.StartsAt
	if status == StatusAwaitingBroker {
		escalate = time.Now().UTC().Add(EscalateAfter)
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO visits.appointments (
			id, company_id, branch_id, profile_id, listing_id, property_id, unit_id, broker_user_id, host_user_id,
			starts_at, ends_at, attendees, status, instructions_ar, instructions_en, escalate_at, created_by
		) VALUES (
			gen_random_uuid(), $1, NULLIF($2,'')::uuid, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid, NULLIF($6,'')::uuid,
			NULLIF($7,'')::uuid, NULLIF($8,'')::uuid, $9, $10, $11, $12, $13, $14, $15, NULLIF($16,'')::uuid
		) RETURNING id::text
	`, in.CompanyID, in.BranchID, in.ProfileID, in.ListingID, in.PropertyID, in.UnitID, in.BrokerUserID, in.HostUserID, in.StartsAt, in.EndsAt, in.Attendees, status, in.InstructionsAR, in.InstructionsEN, escalate, actor).Scan(&id)
	if err != nil {
		return Visit{}, err
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.create", EntityType: "visit", EntityID: id, CompanyID: in.CompanyID, RequestID: requestID, Result: "ok", After: status})
	return s.Get(ctx, id)
}

func (s Service) Get(ctx context.Context, id string) (Visit, error) {
	var item Visit
	err := s.Pool.QueryRow(ctx, `
		SELECT v.id::text, v.company_id::text, COALESCE(v.branch_id::text,''), v.profile_id::text,
		       COALESCE(v.listing_id::text,''), COALESCE(v.property_id::text,''), COALESCE(v.unit_id::text,''),
		       COALESCE(v.broker_user_id::text,''), COALESCE(v.host_user_id::text,''), v.starts_at, v.ends_at, v.attendees, v.status,
		       v.broker_confirmed_at, v.owner_confirmed_at, v.result, v.result_note, v.instructions_ar, v.instructions_en,
		       v.cancel_reason, v.escalate_at, v.version, COALESCE(v.created_by::text,''), v.created_at, p.display_name
		FROM visits.appointments v
		JOIN clients.profiles p ON p.id = v.profile_id
		WHERE v.id = $1
	`, id).Scan(&item.ID, &item.CompanyID, &item.BranchID, &item.ProfileID, &item.ListingID, &item.PropertyID, &item.UnitID, &item.BrokerUserID, &item.HostUserID, &item.StartsAt, &item.EndsAt, &item.Attendees, &item.Status, &item.BrokerConfirmedAt, &item.OwnerConfirmedAt, &item.Result, &item.ResultNote, &item.InstructionsAR, &item.InstructionsEN, &item.CancelReason, &item.EscalateAt, &item.Version, &item.CreatedBy, &item.CreatedAt, &item.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Visit{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Visit{}, err
	}
	if item.EscalateAt != nil && time.Now().UTC().After(*item.EscalateAt) && (item.Status == StatusAwaitingBroker || item.Status == StatusAwaitingOwner) {
		item.Overdue = true
	}
	return item, nil
}

func (s Service) List(ctx context.Context, companyID, status, view string, page, size int) ([]Visit, int, error) {
	const viewSQL = `
		AND (
			$3 = ''
			OR ($3 = 'today' AND (starts_at AT TIME ZONE 'Asia/Riyadh')::date = (now() AT TIME ZONE 'Asia/Riyadh')::date)
			OR ($3 = 'overdue' AND escalate_at IS NOT NULL AND escalate_at < now() AND status IN ('awaiting_broker','awaiting_owner'))
			OR ($3 = 'unassigned' AND broker_user_id IS NULL)
			OR ($3 = 'results' AND result <> '')
		)`
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM visits.appointments WHERE company_id = $1 AND ($2 = '' OR status = $2) `+viewSQL, companyID, status, view).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text FROM visits.appointments
		WHERE company_id = $1 AND ($2 = '' OR status = $2) `+viewSQL+`
		ORDER BY starts_at ASC, id ASC LIMIT $4 OFFSET $5
	`, companyID, status, view, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Visit, 0)
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

func (s Service) Mine(ctx context.Context, userID string) ([]Visit, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT v.id::text FROM visits.appointments v
		JOIN clients.profiles p ON p.id = v.profile_id
		WHERE p.user_id = $1 ORDER BY v.starts_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Visit, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		item.Result, item.ResultNote = "", ""
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) BrokerConfirm(ctx context.Context, actor, requestID, id string, version int) (Visit, error) {
	return s.simple(ctx, actor, requestID, id, version, StatusAwaitingOwner, "visit.broker_confirm", "")
}

func (s Service) OwnerConfirm(ctx context.Context, actor, requestID, id string, version int, now time.Time) (Visit, error) {
	current, err := s.lockAndCheck(ctx, id, version, StatusConfirmed)
	if err != nil {
		return Visit{}, err
	}
	var conflicts int
	windowStart := current.StartsAt.Add(-SlotGap)
	windowEnd := current.EndsAt.Add(SlotGap)
	if err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM visits.appointments
		WHERE id <> $1 AND status = 'confirmed'
		  AND starts_at < $2 AND ends_at > $3
		  AND (
		    (broker_user_id = NULLIF($4,'')::uuid AND NULLIF($4,'') IS NOT NULL)
		    OR (unit_id = NULLIF($5,'')::uuid AND NULLIF($5,'') IS NOT NULL)
		    OR profile_id = $6
		  )
	`, id, windowEnd, windowStart, current.BrokerUserID, current.UnitID, current.ProfileID).Scan(&conflicts); err != nil {
		return Visit{}, err
	}
	if conflicts > 0 {
		return Visit{}, platform.E("visit_conflict", 409)
	}
	if _, err := s.Pool.Exec(ctx, `
		UPDATE visits.appointments
		SET status = 'confirmed', owner_confirmed_at = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, now); err != nil {
		return Visit{}, err
	}
	for _, kind := range ReminderKinds(now, current.StartsAt) {
		fire := current.StartsAt.Add(-2 * time.Hour)
		if kind == "day_before" {
			fire = current.StartsAt.Add(-24 * time.Hour)
		}
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO visits.reminders (id, visit_id, kind, status, fire_at) VALUES (gen_random_uuid(), $1, $2, 'recorded', $3)
		`, id, kind, fire); err != nil {
			return Visit{}, err
		}
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.owner_confirm", EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", After: StatusConfirmed})
	return s.Get(ctx, id)
}

func (s Service) OwnerReject(ctx context.Context, actor, requestID, id, reason string, version int, alternative time.Time) (Visit, error) {
	if strings.TrimSpace(reason) == "" {
		return Visit{}, platform.E("reason_required", 422)
	}
	current, err := s.lockAndCheck(ctx, id, version, StatusAwaitingBroker)
	if err != nil {
		return Visit{}, err
	}
	if current.Status != StatusAwaitingOwner {
		return Visit{}, platform.E("invalid_transition", 409)
	}
	starts, ends := current.StartsAt, current.EndsAt
	if !alternative.IsZero() {
		starts = alternative
		ends = alternative.Add(DefaultDuration)
	}
	if err := s.archive(ctx, current, actor, reason); err != nil {
		return Visit{}, err
	}
	if _, err := s.Pool.Exec(ctx, `
		UPDATE visits.appointments
		SET status = 'awaiting_broker', starts_at = $3, ends_at = $4, owner_confirmed_at = NULL, broker_confirmed_at = NULL,
		    cancel_reason = '', version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, starts, ends); err != nil {
		return Visit{}, err
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.owner_reject", EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason})
	return s.Get(ctx, id)
}

func (s Service) Complete(ctx context.Context, actor, requestID, id, result, note, reason string, version int, now time.Time, correction bool) (Visit, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Visit{}, err
	}
	if current.Version != version || !CanTransition(current.Status, StatusCompleted) {
		if current.Version != version {
			return Visit{}, platform.E("version_conflict", 409)
		}
		return Visit{}, platform.E("invalid_transition", 409)
	}
	if code := CompleteIssue(now, current.EndsAt, result, note, correction, reason); code != "" {
		status := 422
		if code == "too_early" {
			status = 409
		}
		return Visit{}, platform.E(code, status)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE visits.appointments
		SET status = 'completed', result = $3, result_note = $4, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2 AND status = 'confirmed'
	`, id, version, result, note)
	if err != nil {
		return Visit{}, err
	}
	if tag.RowsAffected() == 0 {
		return Visit{}, platform.E("version_conflict", 409)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE visits.completion_counter SET total = total + 1 WHERE id = 'shamal'`); err != nil {
		return Visit{}, err
	}
	if correction {
		if _, err := s.Pool.Exec(ctx, `UPDATE visits.completion_counter SET total = total - 1 WHERE id = 'shamal' AND total > 0`); err != nil {
			return Visit{}, err
		}
		_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.counter_reversed", EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason})
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.complete", EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason, After: result})
	return s.Get(ctx, id)
}

func (s Service) Cancel(ctx context.Context, actor, requestID, id, reason string, version int) (Visit, error) {
	if strings.TrimSpace(reason) == "" {
		return Visit{}, platform.E("reason_required", 422)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Visit{}, err
	}
	if current.Version != version || !CanTransition(current.Status, StatusCancelled) {
		if current.Version != version {
			return Visit{}, platform.E("version_conflict", 409)
		}
		return Visit{}, platform.E("invalid_transition", 409)
	}
	if _, err := s.Pool.Exec(ctx, `
		UPDATE visits.appointments SET status = 'cancelled', cancel_reason = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, reason); err != nil {
		return Visit{}, err
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE visits.reminders SET status = 'cancelled' WHERE visit_id = $1 AND status = 'recorded'`, id); err != nil {
		return Visit{}, err
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.cancel", EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason})
	return s.Get(ctx, id)
}

func (s Service) NoShow(ctx context.Context, actor, requestID, id, reason string, version int) (Visit, error) {
	if strings.TrimSpace(reason) == "" {
		return Visit{}, platform.E("reason_required", 422)
	}
	return s.simple(ctx, actor, requestID, id, version, StatusNoShow, "visit.no_show", reason)
}

func (s Service) Reschedule(ctx context.Context, actor, requestID, id, reason string, version int, start, end time.Time) (Visit, error) {
	if strings.TrimSpace(reason) == "" || start.IsZero() {
		return Visit{}, platform.E("reason_required", 422)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Visit{}, err
	}
	if current.Version != version || !CanTransition(current.Status, StatusAwaitingBroker) {
		if current.Version != version {
			return Visit{}, platform.E("version_conflict", 409)
		}
		return Visit{}, platform.E("invalid_transition", 409)
	}
	end = EndOrDefault(start, end)
	if err := s.archive(ctx, current, actor, reason); err != nil {
		return Visit{}, err
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE visits.reminders SET status = 'cancelled' WHERE visit_id = $1 AND status = 'recorded'`, id); err != nil {
		return Visit{}, err
	}
	if _, err := s.Pool.Exec(ctx, `
		UPDATE visits.appointments
		SET status = 'awaiting_broker', starts_at = $3, ends_at = $4, broker_confirmed_at = NULL, owner_confirmed_at = NULL,
		    version = version + 1, updated_at = now(), escalate_at = now() + interval '2 hours'
		WHERE id = $1 AND version = $2
	`, id, version, start, end); err != nil {
		return Visit{}, err
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "visit.reschedule", EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason})
	return s.Get(ctx, id)
}

func (s Service) RecordedReminders(ctx context.Context, id string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM visits.reminders WHERE visit_id = $1 AND status = 'recorded'`, id).Scan(&n)
	return n, err
}

func (s Service) CompletionTotal(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT total FROM visits.completion_counter WHERE id = 'shamal'`).Scan(&n)
	return n, err
}

func (s Service) simple(ctx context.Context, actor, requestID, id string, version int, to, action, reason string) (Visit, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Visit{}, err
	}
	if current.Version != version || !CanTransition(current.Status, to) {
		if current.Version != version {
			return Visit{}, platform.E("version_conflict", 409)
		}
		return Visit{}, platform.E("invalid_transition", 409)
	}
	query := `UPDATE visits.appointments SET status = $3, version = version + 1, updated_at = now()`
	if to == StatusAwaitingOwner {
		query += `, broker_confirmed_at = now()`
	}
	if to == StatusNoShow {
		query += `, cancel_reason = $4`
	}
	query += ` WHERE id = $1 AND version = $2`
	var tag interface{ RowsAffected() int64 }
	var errExec error
	if to == StatusNoShow {
		tag, errExec = s.Pool.Exec(ctx, query, id, version, to, reason)
	} else {
		tag, errExec = s.Pool.Exec(ctx, query, id, version, to)
	}
	if errExec != nil {
		return Visit{}, errExec
	}
	if tag.RowsAffected() == 0 {
		return Visit{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: action, EntityType: "visit", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", After: to})
	return s.Get(ctx, id)
}

func (s Service) lockAndCheck(ctx context.Context, id string, version int, to string) (Visit, error) {
	var status string
	var gotVersion int
	err := s.Pool.QueryRow(ctx, `SELECT status, version FROM visits.appointments WHERE id = $1 FOR UPDATE`, id).Scan(&status, &gotVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Visit{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Visit{}, err
	}
	if gotVersion != version {
		return Visit{}, platform.E("version_conflict", 409)
	}
	if !CanTransition(status, to) {
		return Visit{}, platform.E("invalid_transition", 409)
	}
	return s.Get(ctx, id)
}

func (s Service) archive(ctx context.Context, current Visit, actor, reason string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO visits.history (id, visit_id, starts_at, ends_at, previous_status, reason, actor_id)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NULLIF($6,'')::uuid)
	`, current.ID, current.StartsAt, current.EndsAt, current.Status, reason, actor)
	return err
}
