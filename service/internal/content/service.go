package content

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/platform"
)

type Text struct {
	Locale string `json:"locale"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type Page struct {
	ID             string `json:"id"`
	CompanyID      string `json:"company_id"`
	Kind           string `json:"kind"`
	Slug           string `json:"slug"`
	NeighborhoodID string `json:"neighborhood_id"`
	Status         string `json:"status"`
	Version        int    `json:"version"`
	CreatedBy      string `json:"created_by"`
	Texts          []Text `json:"texts"`
}

type PublicPage struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Slug           string `json:"slug"`
	NeighborhoodID string `json:"neighborhood_id"`
	Locale         string `json:"locale"`
	Title          string `json:"title"`
	Body           string `json:"body"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func KnownKind(kind string) bool {
	switch kind {
	case "home", "featured", "offer", "article", "faq", "policy", "service":
		return true
	default:
		return false
	}
}

func ValidSlug(slug string) bool {
	if len(slug) < 2 || len(slug) > 80 {
		return false
	}
	for _, r := range slug {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func NormalizePair(ar, en Text) (Text, Text, error) {
	ar.Locale, en.Locale = "ar", "en"
	ar.Title, en.Title = strings.TrimSpace(ar.Title), strings.TrimSpace(en.Title)
	ar.Body, en.Body = strings.TrimSpace(ar.Body), strings.TrimSpace(en.Body)
	if utf8.RuneCountInString(ar.Title) < 2 || utf8.RuneCountInString(en.Title) < 2 || ar.Body == "" || en.Body == "" {
		return Text{}, Text{}, platform.E("missing_translation", 422)
	}
	return ar, en, nil
}

func (s Service) Create(ctx context.Context, companyID, actor, kind, slug, neighborhoodID string, ar, en Text) (Page, error) {
	if !KnownKind(kind) || !ValidSlug(slug) {
		return Page{}, platform.E("validation", 422)
	}
	ar, en, err := NormalizePair(ar, en)
	if err != nil {
		return Page{}, err
	}
	if neighborhoodID != "" {
		if _, err := uuid.Parse(neighborhoodID); err != nil {
			return Page{}, platform.E("validation", 422)
		}
		var exists bool
		if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM geography.neighborhoods WHERE id = $1::uuid)`, neighborhoodID).Scan(&exists); err != nil {
			return Page{}, err
		}
		if !exists {
			return Page{}, platform.E("not_found", 404)
		}
	}
	id := uuid.NewString()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO content.pages (id, company_id, kind, slug, neighborhood_id, status, created_by, updated_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,'')::uuid, 'draft', $6, $6)
	`, id, companyID, kind, slug, neighborhoodID, actor)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return Page{}, platform.E("slug_taken", 409)
		}
		return Page{}, err
	}
	if err := writeTexts(ctx, tx, id, ar, en); err != nil {
		return Page{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Page{}, err
	}
	return s.Get(ctx, companyID, id)
}

func (s Service) Update(ctx context.Context, companyID, actor, id string, version int, ar, en Text) (Page, error) {
	ar, en, err := NormalizePair(ar, en)
	if err != nil {
		return Page{}, err
	}
	current, err := s.Get(ctx, companyID, id)
	if err != nil {
		return Page{}, err
	}
	if current.Status != "draft" {
		return Page{}, platform.E("invalid_transition", 409)
	}
	if current.Version != version {
		return Page{}, platform.E("version_conflict", 409)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE content.pages SET version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND company_id = $2 AND version = $4 AND status = 'draft'
	`, id, companyID, actor, version)
	if err != nil {
		return Page{}, err
	}
	if tag.RowsAffected() == 0 {
		return Page{}, platform.E("version_conflict", 409)
	}
	if err := writeTexts(ctx, tx, id, ar, en); err != nil {
		return Page{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Page{}, err
	}
	return s.Get(ctx, companyID, id)
}

func (s Service) Move(ctx context.Context, companyID, actor, id, from, to string, version int) (Page, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE content.pages SET status = $5, version = version + 1, updated_by = $3, updated_at = now()
		WHERE id = $1 AND company_id = $2 AND version = $4 AND status = $6
	`, id, companyID, actor, version, to, from)
	if err != nil {
		return Page{}, err
	}
	if tag.RowsAffected() == 0 {
		return Page{}, platform.E("version_conflict", 409)
	}
	return s.Get(ctx, companyID, id)
}

func (s Service) List(ctx context.Context, companyID string) ([]Page, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text FROM content.pages WHERE company_id = $1 ORDER BY kind, slug
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Page, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for _, id := range ids {
		item, err := s.Get(ctx, companyID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s Service) Get(ctx context.Context, companyID, id string) (Page, error) {
	var item Page
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, company_id::text, kind, slug, COALESCE(neighborhood_id::text, ''), status, version, created_by::text
		FROM content.pages WHERE id = $1 AND company_id = $2
	`, id, companyID).Scan(&item.ID, &item.CompanyID, &item.Kind, &item.Slug, &item.NeighborhoodID, &item.Status, &item.Version, &item.CreatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Page{}, platform.E("not_found", 404)
		}
		return Page{}, err
	}
	item.Texts, err = s.texts(ctx, id)
	return item, err
}

func (s Service) Published(ctx context.Context, locale, kind, slug string) ([]PublicPage, error) {
	if locale != "en" {
		locale = "ar"
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT p.id::text, p.kind, p.slug, COALESCE(p.neighborhood_id::text, ''), t.title, t.body
		FROM content.pages p
		JOIN content.translations t ON t.page_id = p.id AND t.locale = $1
		WHERE p.status = 'published'
		  AND ($2 = '' OR p.kind = $2)
		  AND ($3 = '' OR p.slug = $3)
		ORDER BY p.kind, p.slug
	`, locale, kind, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PublicPage, 0)
	for rows.Next() {
		var item PublicPage
		if err := rows.Scan(&item.ID, &item.Kind, &item.Slug, &item.NeighborhoodID, &item.Title, &item.Body); err != nil {
			return nil, err
		}
		item.Locale = locale
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) texts(ctx context.Context, id string) ([]Text, error) {
	rows, err := s.Pool.Query(ctx, `SELECT locale, title, body FROM content.translations WHERE page_id = $1 ORDER BY locale`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Text, 0, 2)
	for rows.Next() {
		var item Text
		if err := rows.Scan(&item.Locale, &item.Title, &item.Body); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func writeTexts(ctx context.Context, tx pgx.Tx, id string, ar, en Text) error {
	if _, err := tx.Exec(ctx, `DELETE FROM content.translations WHERE page_id = $1`, id); err != nil {
		return err
	}
	for _, item := range []Text{ar, en} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO content.translations (page_id, locale, title, body) VALUES ($1, $2, $3, $4)
		`, id, item.Locale, item.Title, item.Body); err != nil {
			return err
		}
	}
	return nil
}
