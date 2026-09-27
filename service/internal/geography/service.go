package geography

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/audit"
	"shamal/service/internal/geom"
	"shamal/service/internal/platform"
)

type Text struct {
	Locale            string   `json:"locale"`
	Name              string   `json:"name"`
	Slug              string   `json:"slug"`
	Description       string   `json:"description"`
	Title             string   `json:"title"`
	SearchDescription string   `json:"search_description"`
	Aliases           []string `json:"aliases"`
}

type Neighborhood struct {
	ID                string    `json:"id"`
	CityID            string    `json:"city_id"`
	CompanyID         string    `json:"company_id"`
	BranchID          string    `json:"branch_id"`
	Status            string    `json:"status"`
	Version           int       `json:"version"`
	ServiceSale       bool      `json:"service_sale"`
	ServiceAnnual     bool      `json:"service_annual"`
	ServiceMonthly    bool      `json:"service_monthly"`
	ServiceDaily      bool      `json:"service_daily"`
	Coverage          string    `json:"coverage"`
	ShowInDirectory   bool      `json:"show_in_directory"`
	ShowOnMap         bool      `json:"show_on_map"`
	GeoJSON           string    `json:"geojson"`
	BoundarySource    string    `json:"boundary_source"`
	BoundaryOfficial  bool      `json:"boundary_is_official"`
	NoteAR            string    `json:"boundary_note_ar"`
	NoteEN            string    `json:"boundary_note_en"`
	Issues            []string  `json:"issues"`
	CreatedBy         string    `json:"created_by"`
	UpdatedBy         string    `json:"updated_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Texts             []Text    `json:"texts"`
	PendingRevisionID string    `json:"pending_revision_id,omitempty"`
	PendingAuthor     string    `json:"pending_author,omitempty"`
	SyncJobID         string    `json:"sync_job_id,omitempty"`
	SyncStatus        string    `json:"sync_status,omitempty"`
}

type Input struct {
	NameAR           string
	NameEN           string
	SlugAR           string
	SlugEN           string
	DescriptionAR    string
	DescriptionEN    string
	AliasesAR        []string
	AliasesEN        []string
	GeoJSON          string
	ServiceSale      bool
	ServiceAnnual    bool
	ServiceMonthly   bool
	ServiceDaily     bool
	Coverage         string
	ShowInDirectory  bool
	ShowOnMap        bool
	NoteAR           string
	NoteEN           string
	BoundaryOfficial bool
	Version          int
}

type Place struct {
	ID        string
	CompanyID string
	Status    string
	Sale      bool
	Annual    bool
	Monthly   bool
	Daily     bool
	GeoJSON   string
}

type Service struct {
	Pool        *pgxpool.Pool
	AllowApprox bool
}

func (s Service) List(ctx context.Context, companyID, status string, page, size int) ([]Neighborhood, int, error) {
	var total int
	if err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM geography.neighborhoods
		WHERE company_id = $1 AND ($2 = '' OR status = $2)
	`, companyID, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text FROM geography.neighborhoods
		WHERE company_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY updated_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, companyID, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
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
	items := make([]Neighborhood, 0, len(ids))
	for _, id := range ids {
		item, err := s.Get(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (s Service) Get(ctx context.Context, id string) (Neighborhood, error) {
	var item Neighborhood
	var issues []byte
	var geo *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, city_id::text, company_id::text, COALESCE(branch_id::text,''), status, version,
		       service_sale, service_annual, service_monthly, service_daily, coverage, show_in_directory, show_on_map,
		       ST_AsGeoJSON(boundary), COALESCE(boundary_source,''), boundary_is_official, boundary_note_ar, boundary_note_en,
		       validation_issues, created_by::text, updated_by::text, created_at, updated_at
		FROM geography.neighborhoods WHERE id = $1
	`, id).Scan(&item.ID, &item.CityID, &item.CompanyID, &item.BranchID, &item.Status, &item.Version,
		&item.ServiceSale, &item.ServiceAnnual, &item.ServiceMonthly, &item.ServiceDaily, &item.Coverage, &item.ShowInDirectory, &item.ShowOnMap,
		&geo, &item.BoundarySource, &item.BoundaryOfficial, &item.NoteAR, &item.NoteEN, &issues, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Neighborhood{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Neighborhood{}, err
	}
	if geo != nil {
		item.GeoJSON = *geo
	}
	_ = json.Unmarshal(issues, &item.Issues)
	if item.Issues == nil {
		item.Issues = []string{}
	}
	texts, err := s.texts(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	item.Texts = texts
	_ = s.Pool.QueryRow(ctx, `
		SELECT id::text, created_by::text FROM geography.revisions
		WHERE neighborhood_id = $1 AND status IN ('draft','in_review','changes_requested')
		ORDER BY created_at DESC LIMIT 1
	`, id).Scan(&item.PendingRevisionID, &item.PendingAuthor)
	_ = s.Pool.QueryRow(ctx, `
		SELECT id::text, status FROM geography.sync_jobs
		WHERE neighborhood_id = $1 ORDER BY created_at DESC LIMIT 1
	`, id).Scan(&item.SyncJobID, &item.SyncStatus)
	return item, nil
}

func (s Service) texts(ctx context.Context, id string) ([]Text, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT locale, name, slug, description, title, search_description
		FROM geography.translations WHERE neighborhood_id = $1 ORDER BY locale
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Text, 0)
	for rows.Next() {
		var text Text
		if err := rows.Scan(&text.Locale, &text.Name, &text.Slug, &text.Description, &text.Title, &text.SearchDescription); err != nil {
			return nil, err
		}
		out = append(out, text)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		arows, err := s.Pool.Query(ctx, `SELECT alias FROM geography.aliases WHERE neighborhood_id = $1 AND locale = $2 ORDER BY alias`, id, out[i].Locale)
		if err != nil {
			return nil, err
		}
		for arows.Next() {
			var alias string
			if err := arows.Scan(&alias); err != nil {
				arows.Close()
				return nil, err
			}
			out[i].Aliases = append(out[i].Aliases, alias)
		}
		arows.Close()
		if out[i].Aliases == nil {
			out[i].Aliases = []string{}
		}
	}
	return out, nil
}

func (s Service) Create(ctx context.Context, actor, requestID, cityID, companyID, branchID string, in Input) (Neighborhood, error) {
	if in.BoundaryOfficial {
		return Neighborhood{}, platform.E("official_boundary_unapproved", 422)
	}
	if in.Coverage == "" {
		in.Coverage = "directory_only"
	}
	issues, err := s.assess(ctx, cityID, "", in)
	if err != nil {
		return Neighborhood{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Neighborhood{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	issueJSON, _ := json.Marshal(issues)
	err = tx.QueryRow(ctx, `
		INSERT INTO geography.neighborhoods (
			id, city_id, company_id, branch_id, status, service_sale, service_annual, service_monthly, service_daily,
			coverage, show_in_directory, show_on_map, boundary, boundary_source, boundary_is_official, boundary_note_ar, boundary_note_en,
			validation_issues, created_by, updated_by
		) VALUES (
			gen_random_uuid(), $1, $2, NULLIF($3,'')::uuid, 'draft', $4, $5, $6, $7,
			$8, $9, $10, CASE WHEN $11 = '' THEN NULL ELSE ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($11), 4326)) END,
			'editor', false, $12, $13, $14, $15, $15
		) RETURNING id::text
	`, cityID, companyID, branchID, in.ServiceSale, in.ServiceAnnual, in.ServiceMonthly, in.ServiceDaily, in.Coverage, in.ShowInDirectory, in.ShowOnMap, in.GeoJSON, in.NoteAR, in.NoteEN, issueJSON, actor).Scan(&id)
	if err != nil {
		return Neighborhood{}, mapGeoErr(err)
	}
	if err := s.writeTexts(ctx, tx, id, cityID, in); err != nil {
		return Neighborhood{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{ActorID: actor, Action: "neighborhood.create", EntityType: "neighborhood", EntityID: id, CompanyID: companyID, RequestID: requestID, Result: "ok", After: in}); err != nil {
		return Neighborhood{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Neighborhood{}, err
	}
	return s.Get(ctx, id)
}

func (s Service) Update(ctx context.Context, actor, requestID, id string, in Input) (Neighborhood, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	if current.Version != in.Version {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	if current.Status == "published" || current.Status == "suspended" {
		return s.saveRevision(ctx, actor, requestID, current, in)
	}
	if current.Status != "draft" && current.Status != "changes_requested" {
		return Neighborhood{}, platform.E("invalid_transition", 409)
	}
	issues, err := s.assess(ctx, current.CityID, id, in)
	if err != nil {
		return Neighborhood{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Neighborhood{}, err
	}
	defer tx.Rollback(ctx)
	issueJSON, _ := json.Marshal(issues)
	tag, err := tx.Exec(ctx, `
		UPDATE geography.neighborhoods SET
			service_sale = $3, service_annual = $4, service_monthly = $5, service_daily = $6,
			coverage = $7, show_in_directory = $8, show_on_map = $9,
			boundary = CASE WHEN $10 = '' THEN NULL ELSE ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($10), 4326)) END,
			boundary_note_ar = $11, boundary_note_en = $12, validation_issues = $13,
			status = 'draft', updated_by = $14, updated_at = now(), version = version + 1
		WHERE id = $1 AND version = $2
	`, id, in.Version, in.ServiceSale, in.ServiceAnnual, in.ServiceMonthly, in.ServiceDaily, coverageOr(in.Coverage), in.ShowInDirectory, in.ShowOnMap, in.GeoJSON, in.NoteAR, in.NoteEN, issueJSON, actor)
	if err != nil {
		return Neighborhood{}, mapGeoErr(err)
	}
	if tag.RowsAffected() == 0 {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM geography.translations WHERE neighborhood_id = $1`, id); err != nil {
		return Neighborhood{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM geography.aliases WHERE neighborhood_id = $1`, id); err != nil {
		return Neighborhood{}, err
	}
	if err := s.writeTexts(ctx, tx, id, current.CityID, in); err != nil {
		return Neighborhood{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{ActorID: actor, Action: "neighborhood.update", EntityType: "neighborhood", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok"}); err != nil {
		return Neighborhood{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Neighborhood{}, err
	}
	return s.Get(ctx, id)
}

func (s Service) saveRevision(ctx context.Context, actor, requestID string, current Neighborhood, in Input) (Neighborhood, error) {
	raw, _ := json.Marshal(in)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Neighborhood{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM geography.revisions WHERE neighborhood_id = $1 AND status IN ('draft','in_review','changes_requested')`, current.ID); err != nil {
		return Neighborhood{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO geography.revisions (id, neighborhood_id, status, snapshot, created_by)
		VALUES (gen_random_uuid(), $1, 'draft', $2, $3)
	`, current.ID, raw, actor); err != nil {
		return Neighborhood{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{ActorID: actor, Action: "neighborhood.revision", EntityType: "neighborhood", EntityID: current.ID, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok"}); err != nil {
		return Neighborhood{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Neighborhood{}, err
	}
	return s.Get(ctx, current.ID)
}

func (s Service) Submit(ctx context.Context, actor, requestID, id string, version int) (Neighborhood, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	if current.Version != version {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	if current.PendingRevisionID != "" && (current.Status == "published" || current.Status == "suspended") {
		if _, err := s.Pool.Exec(ctx, `UPDATE geography.revisions SET status = 'in_review' WHERE id = $1`, current.PendingRevisionID); err != nil {
			return Neighborhood{}, err
		}
		_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "neighborhood.submit_revision", EntityType: "neighborhood", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok"})
		return s.Get(ctx, id)
	}
	if !CanTransition(current.Status, "in_review") && current.Status != "changes_requested" {
		return Neighborhood{}, platform.E("invalid_transition", 409)
	}
	in := inputFrom(current)
	issues, err := s.assess(ctx, current.CityID, id, in)
	if err != nil {
		return Neighborhood{}, err
	}
	if len(issues) > 0 {
		raw, _ := json.Marshal(issues)
		_, _ = s.Pool.Exec(ctx, `UPDATE geography.neighborhoods SET validation_issues = $2, updated_at = now() WHERE id = $1`, id, raw)
		return Neighborhood{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueFields(issues)}
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE geography.neighborhoods
		SET status = 'in_review', validation_issues = '[]'::jsonb, version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, actor)
	if err != nil {
		return Neighborhood{}, err
	}
	if tag.RowsAffected() == 0 {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "neighborhood.submit", EntityType: "neighborhood", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok"})
	return s.Get(ctx, id)
}

func (s Service) RequestChanges(ctx context.Context, actor, requestID, id, reason string, version int) (Neighborhood, error) {
	if reason == "" {
		return Neighborhood{}, platform.E("reason_required", 422)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	if current.Version != version || !CanTransition(current.Status, "changes_requested") {
		if current.Version != version {
			return Neighborhood{}, platform.E("version_conflict", 409)
		}
		return Neighborhood{}, platform.E("invalid_transition", 409)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE geography.neighborhoods
		SET status = 'changes_requested', version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND version = $2
	`, id, version, actor)
	if err != nil {
		return Neighborhood{}, err
	}
	if tag.RowsAffected() == 0 {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: "neighborhood.request_changes", EntityType: "neighborhood", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason})
	return s.Get(ctx, id)
}

func (s Service) Publish(ctx context.Context, actor, requestID, id string, version int) (Neighborhood, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	if current.Version != version {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	if current.PendingRevisionID != "" {
		return s.publishRevision(ctx, actor, requestID, current)
	}
	if !CanTransition(current.Status, "published") {
		return Neighborhood{}, platform.E("invalid_transition", 409)
	}
	in := inputFrom(current)
	issues, err := s.assess(ctx, current.CityID, id, in)
	if err != nil {
		return Neighborhood{}, err
	}
	if len(issues) > 0 {
		return Neighborhood{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueFields(issues)}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Neighborhood{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE geography.neighborhoods
		SET status = 'published', validation_issues = '[]'::jsonb, version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND version = $2 AND status = 'in_review'
	`, id, version, actor)
	if err != nil {
		return Neighborhood{}, err
	}
	if tag.RowsAffected() == 0 {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	var jobID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO geography.sync_jobs (id, neighborhood_id, status) VALUES (gen_random_uuid(), $1, 'recorded') RETURNING id::text
	`, id).Scan(&jobID); err != nil {
		return Neighborhood{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{ActorID: actor, Action: "neighborhood.publish", EntityType: "neighborhood", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", After: map[string]string{"sync_job_id": jobID, "sync_status": "recorded"}}); err != nil {
		return Neighborhood{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Neighborhood{}, err
	}
	item, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	item.SyncJobID = jobID
	item.SyncStatus = "recorded"
	return item, nil
}

func (s Service) publishRevision(ctx context.Context, actor, requestID string, current Neighborhood) (Neighborhood, error) {
	var status string
	var raw []byte
	if err := s.Pool.QueryRow(ctx, `SELECT status, snapshot FROM geography.revisions WHERE id = $1`, current.PendingRevisionID).Scan(&status, &raw); err != nil {
		return Neighborhood{}, err
	}
	if status != "in_review" {
		return Neighborhood{}, platform.E("invalid_transition", 409)
	}
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return Neighborhood{}, err
	}
	issues, err := s.assess(ctx, current.CityID, current.ID, in)
	if err != nil {
		return Neighborhood{}, err
	}
	if len(issues) > 0 {
		return Neighborhood{}, &platform.Error{Code: "validation", HTTP: 422, Fields: issueFields(issues)}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Neighborhood{}, err
	}
	defer tx.Rollback(ctx)
	for _, text := range current.Texts {
		next := in.SlugAR
		if text.Locale == "en" {
			next = in.SlugEN
		}
		if text.Slug != "" && text.Slug != next {
			_, _ = tx.Exec(ctx, `
				INSERT INTO geography.slug_redirects (city_id, locale, from_slug, neighborhood_id)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (city_id, locale, from_slug) DO UPDATE SET neighborhood_id = EXCLUDED.neighborhood_id
			`, current.CityID, text.Locale, text.Slug, current.ID)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM geography.translations WHERE neighborhood_id = $1`, current.ID); err != nil {
		return Neighborhood{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM geography.aliases WHERE neighborhood_id = $1`, current.ID); err != nil {
		return Neighborhood{}, err
	}
	if err := s.writeTexts(ctx, tx, current.ID, current.CityID, in); err != nil {
		return Neighborhood{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE geography.neighborhoods SET
			service_sale = $2, service_annual = $3, service_monthly = $4, service_daily = $5,
			coverage = $6, show_in_directory = $7, show_on_map = $8,
			boundary = CASE WHEN $9 = '' THEN boundary ELSE ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($9), 4326)) END,
			boundary_note_ar = $10, boundary_note_en = $11, status = 'published', version = version + 1, updated_by = $12, updated_at = now()
		WHERE id = $1
	`, current.ID, in.ServiceSale, in.ServiceAnnual, in.ServiceMonthly, in.ServiceDaily, coverageOr(in.Coverage), in.ShowInDirectory, in.ShowOnMap, in.GeoJSON, in.NoteAR, in.NoteEN, actor); err != nil {
		return Neighborhood{}, mapGeoErr(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE geography.revisions SET status = 'applied' WHERE id = $1`, current.PendingRevisionID); err != nil {
		return Neighborhood{}, err
	}
	var jobID string
	if err := tx.QueryRow(ctx, `INSERT INTO geography.sync_jobs (id, neighborhood_id, status) VALUES (gen_random_uuid(), $1, 'recorded') RETURNING id::text`, current.ID).Scan(&jobID); err != nil {
		return Neighborhood{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{ActorID: actor, Action: "neighborhood.publish_revision", EntityType: "neighborhood", EntityID: current.ID, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", After: map[string]string{"sync_job_id": jobID}}); err != nil {
		return Neighborhood{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Neighborhood{}, err
	}
	item, err := s.Get(ctx, current.ID)
	if err != nil {
		return Neighborhood{}, err
	}
	item.SyncJobID = jobID
	item.SyncStatus = "recorded"
	return item, nil
}

func (s Service) Suspend(ctx context.Context, actor, requestID, id, reason string, version int) (Neighborhood, error) {
	return s.setStatus(ctx, actor, requestID, id, reason, version, "published", "suspended", "neighborhood.suspend")
}

func (s Service) Reactivate(ctx context.Context, actor, requestID, id, reason string, version int) (Neighborhood, error) {
	return s.setStatus(ctx, actor, requestID, id, reason, version, "suspended", "published", "neighborhood.reactivate")
}

func (s Service) setStatus(ctx context.Context, actor, requestID, id, reason string, version int, from, to, action string) (Neighborhood, error) {
	if reason == "" {
		return Neighborhood{}, platform.E("reason_required", 422)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	if current.Version != version {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	if current.Status != from || !CanTransition(from, to) {
		return Neighborhood{}, platform.E("invalid_transition", 409)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE geography.neighborhoods
		SET status = $4, version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND version = $2 AND status = $5
	`, id, version, actor, to, from)
	if err != nil {
		return Neighborhood{}, err
	}
	if tag.RowsAffected() == 0 {
		return Neighborhood{}, platform.E("version_conflict", 409)
	}
	_ = audit.Record(ctx, s.Pool, audit.Event{ActorID: actor, Action: action, EntityType: "neighborhood", EntityID: id, CompanyID: current.CompanyID, RequestID: requestID, Result: "ok", Reason: reason})
	return s.Get(ctx, id)
}

func (s Service) Place(ctx context.Context, id string) (Place, error) {
	var place Place
	var geo *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, company_id::text, status, service_sale, service_annual, service_monthly, service_daily, ST_AsGeoJSON(boundary)
		FROM geography.neighborhoods WHERE id = $1
	`, id).Scan(&place.ID, &place.CompanyID, &place.Status, &place.Sale, &place.Annual, &place.Monthly, &place.Daily, &geo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Place{}, platform.E("not_found", 404)
	}
	if geo != nil {
		place.GeoJSON = *geo
	}
	return place, err
}

func (s Service) Public(ctx context.Context, locale, cityID string) ([]Neighborhood, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT n.id::text
		FROM geography.neighborhoods n
		JOIN geography.translations t ON t.neighborhood_id = n.id AND t.locale = $1
		WHERE n.status = 'published' AND n.show_in_directory AND ($2::uuid IS NULL OR n.city_id = $2::uuid)
		ORDER BY t.name
	`, locale, nullable(cityID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Neighborhood, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		filtered := make([]Text, 0, 1)
		for _, text := range item.Texts {
			if text.Locale == locale {
				filtered = append(filtered, text)
			}
		}
		item.Texts = filtered
		item.GeoJSON = ""
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) BySlug(ctx context.Context, locale, slug string) (Neighborhood, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `
		SELECT n.id::text
		FROM geography.neighborhoods n
		JOIN geography.translations t ON t.neighborhood_id = n.id
		WHERE t.locale = $1 AND t.slug = $2 AND n.status = 'published'
	`, locale, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.Pool.QueryRow(ctx, `
			SELECT neighborhood_id::text FROM geography.slug_redirects WHERE locale = $1 AND from_slug = $2
		`, locale, slug).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Neighborhood{}, platform.E("not_found", 404)
	}
	if err != nil {
		return Neighborhood{}, err
	}
	item, err := s.Get(ctx, id)
	if err != nil {
		return Neighborhood{}, err
	}
	filtered := make([]Text, 0, 1)
	for _, text := range item.Texts {
		if text.Locale == locale {
			filtered = append(filtered, text)
		}
	}
	item.Texts = filtered
	return item, nil
}

func (s Service) RecordedJobs(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text FROM geography.sync_jobs WHERE status = 'recorded' ORDER BY created_at LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s Service) CompleteJob(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE geography.sync_jobs
		SET status = 'succeeded', updated_at = now(), attempts = attempts + 1
		WHERE id = $1 AND status IN ('recorded','running')
	`, id)
	return err
}

func (s Service) assess(ctx context.Context, cityID, selfID string, in Input) ([]string, error) {
	var cityJSON, authority string
	if err := s.Pool.QueryRow(ctx, `SELECT ST_AsGeoJSON(boundary), boundary_authority FROM geography.cities WHERE id = $1`, cityID).Scan(&cityJSON, &authority); err != nil {
		return nil, err
	}
	city, err := geom.Parse([]byte(cityJSON))
	if err != nil {
		return nil, err
	}
	draft := Draft{
		NameAR: in.NameAR, NameEN: in.NameEN, AllowApprox: s.AllowApprox, CityOfficial: authority == "official",
		OfficialClaim: in.BoundaryOfficial,
	}
	if in.GeoJSON != "" {
		draft.HasBoundary = true
		shape, err := geom.Parse([]byte(in.GeoJSON))
		if err != nil {
			draft.BoundaryValid = false
		} else if err := geom.Validate(shape); err != nil {
			draft.BoundaryValid = false
		} else if !geom.CoveredBy(shape, city) {
			draft.BoundaryValid = true
			draft.InsideCity = false
		} else {
			draft.BoundaryValid = true
			draft.InsideCity = true
			var valid bool
			var inside bool
			if err := s.Pool.QueryRow(ctx, `
				SELECT ST_IsValid(g), ST_CoveredBy(g, c.boundary)
				FROM geography.cities c
				CROSS JOIN LATERAL (SELECT ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($1), 4326)) AS g) q
				WHERE c.id = $2
			`, in.GeoJSON, cityID).Scan(&valid, &inside); err != nil || !valid {
				draft.BoundaryValid = false
			} else {
				draft.InsideCity = inside
			}
			var overlaps int
			if err := s.Pool.QueryRow(ctx, `
				SELECT count(*)
				FROM geography.neighborhoods n
				WHERE n.city_id = $2 AND n.status = 'published' AND ($3::uuid IS NULL OR n.id <> $3::uuid) AND n.boundary IS NOT NULL
				  AND ($3::uuid IS NULL OR NOT EXISTS (
				    SELECT 1 FROM geography.overlap_exceptions e WHERE e.neighborhood_id = $3::uuid AND e.other_id = n.id
				  ))
				  AND ST_Area(ST_Intersection(n.boundary::geography, ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($1), 4326))::geography)) > 1
			`, in.GeoJSON, cityID, nullable(selfID)).Scan(&overlaps); err == nil && overlaps > 0 {
				draft.Overlaps = true
			}
		}
	}
	names := []string{Normalize(in.NameAR), Normalize(in.NameEN)}
	for _, alias := range append(append([]string{}, in.AliasesAR...), in.AliasesEN...) {
		names = append(names, Normalize(alias))
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		var count int
		if err := s.Pool.QueryRow(ctx, `
			SELECT count(*) FROM (
				SELECT n.id FROM geography.neighborhoods n
				JOIN geography.translations t ON t.neighborhood_id = n.id
				WHERE n.city_id = $1 AND ($2::uuid IS NULL OR n.id <> $2::uuid) AND n.status NOT IN ('archived','merged') AND t.normalized_name = $3
				UNION
				SELECT n.id FROM geography.neighborhoods n
				JOIN geography.aliases a ON a.neighborhood_id = n.id
				WHERE n.city_id = $1 AND ($2::uuid IS NULL OR n.id <> $2::uuid) AND n.status NOT IN ('archived','merged') AND a.normalized = $3
			) d
		`, cityID, nullable(selfID), name).Scan(&count); err != nil {
			return nil, err
		}
		if count > 0 {
			draft.Duplicate = true
		}
	}
	return Issues(draft), nil
}

func (s Service) writeTexts(ctx context.Context, db platform.DB, id, cityID string, in Input) error {
	pairs := []Text{
		{Locale: "ar", Name: in.NameAR, Slug: slugOr(in.SlugAR, in.NameAR, id), Description: in.DescriptionAR, Aliases: in.AliasesAR},
		{Locale: "en", Name: in.NameEN, Slug: slugOr(in.SlugEN, in.NameEN, id), Description: in.DescriptionEN, Aliases: in.AliasesEN},
	}
	for _, text := range pairs {
		if text.Name == "" {
			text.Name = text.Slug
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO geography.translations (neighborhood_id, city_id, locale, name, normalized_name, slug, description, title, search_description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $4, '')
		`, id, cityID, text.Locale, text.Name, Normalize(text.Name), text.Slug, text.Description); err != nil {
			return mapGeoErr(err)
		}
		for _, alias := range text.Aliases {
			if _, err := db.Exec(ctx, `
				INSERT INTO geography.aliases (id, neighborhood_id, locale, alias, normalized)
				VALUES (gen_random_uuid(), $1, $2, $3, $4)
			`, id, text.Locale, alias, Normalize(alias)); err != nil {
				return err
			}
		}
	}
	return nil
}

func inputFrom(item Neighborhood) Input {
	in := Input{
		ServiceSale: item.ServiceSale, ServiceAnnual: item.ServiceAnnual, ServiceMonthly: item.ServiceMonthly, ServiceDaily: item.ServiceDaily,
		Coverage: item.Coverage, ShowInDirectory: item.ShowInDirectory, ShowOnMap: item.ShowOnMap, GeoJSON: item.GeoJSON,
		NoteAR: item.NoteAR, NoteEN: item.NoteEN, BoundaryOfficial: item.BoundaryOfficial, Version: item.Version,
	}
	for _, text := range item.Texts {
		if text.Locale == "ar" {
			in.NameAR, in.SlugAR, in.DescriptionAR, in.AliasesAR = text.Name, text.Slug, text.Description, text.Aliases
		}
		if text.Locale == "en" {
			in.NameEN, in.SlugEN, in.DescriptionEN, in.AliasesEN = text.Name, text.Slug, text.Description, text.Aliases
		}
	}
	return in
}

func slugOr(slug, name, id string) string {
	if slug != "" {
		return slug
	}
	if name != "" {
		return name
	}
	if len(id) > 8 {
		return "draft-" + id[:8]
	}
	return "draft"
}

type MapPlace struct {
	ID       string
	Name     string
	Slug     string
	Lon      float64
	Lat      float64
	GeoJSON  string
	Official bool
}

func (s Service) PublishedIDs(ctx context.Context, cityID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text FROM geography.neighborhoods
		WHERE status = 'published' AND ($1::uuid IS NULL OR city_id = $1::uuid)
		ORDER BY id
	`, nullable(cityID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (s Service) IDsInArea(ctx context.Context, cityID string, west, south, east, north float64) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text FROM geography.neighborhoods
		WHERE status = 'published' AND ($1::uuid IS NULL OR city_id = $1::uuid)
		  AND boundary IS NOT NULL
		  AND ST_Intersects(boundary, ST_MakeEnvelope($2, $3, $4, $5, 4326))
		ORDER BY id
	`, nullable(cityID), west, south, east, north)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (s Service) IDsByName(ctx context.Context, locale, cityID, normalized string) ([]string, error) {
	if normalized == "" {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT n.id::text
		FROM geography.neighborhoods n
		JOIN geography.translations t ON t.neighborhood_id = n.id AND t.locale = $1
		LEFT JOIN geography.aliases a ON a.neighborhood_id = n.id AND a.locale = $1
		WHERE n.status = 'published' AND ($2::uuid IS NULL OR n.city_id = $2::uuid)
		  AND (
		    t.normalized_name = $3 OR strpos(t.normalized_name, $3) > 0
		    OR (char_length(t.normalized_name) >= 2 AND strpos($3, t.normalized_name) > 0)
		    OR a.normalized = $3 OR strpos(COALESCE(a.normalized, ''), $3) > 0
		    OR (char_length(COALESCE(a.normalized, '')) >= 2 AND strpos($3, a.normalized) > 0)
		  )
		ORDER BY 1
	`, locale, nullable(cityID), normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (s Service) MapPlaces(ctx context.Context, locale, cityID string) ([]MapPlace, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT n.id::text, t.name, t.slug,
		       COALESCE(ST_X(ST_Centroid(n.boundary)), 0), COALESCE(ST_Y(ST_Centroid(n.boundary)), 0),
		       COALESCE(ST_AsGeoJSON(n.boundary), ''), n.boundary_is_official
		FROM geography.neighborhoods n
		JOIN geography.translations t ON t.neighborhood_id = n.id AND t.locale = $1
		WHERE n.status = 'published' AND n.show_on_map AND ($2::uuid IS NULL OR n.city_id = $2::uuid)
		ORDER BY t.name
	`, locale, nullable(cityID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MapPlace, 0)
	for rows.Next() {
		var item MapPlace
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Lon, &item.Lat, &item.GeoJSON, &item.Official); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanIDs(rows pgx.Rows) ([]string, error) {
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func nullable(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func coverageOr(value string) string {
	if value == "" {
		return "directory_only"
	}
	return value
}

func issueFields(issues []string) map[string]string {
	fields := map[string]string{}
	for _, issue := range issues {
		fields[issue] = issue
	}
	return fields
}

func mapGeoErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return platform.E("slug_taken", 409)
	}
	return err
}
