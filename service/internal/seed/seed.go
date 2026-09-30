package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"shamal/service/internal/geography"
)

const (
	CompanyID    = "11111111-1111-4111-8111-111111111111"
	BranchID     = "22222222-2222-4222-8222-222222222222"
	DeptID       = "33333333-3333-4333-8333-333333333333"
	TeamID       = "44444444-4444-4444-8444-444444444444"
	EditorID     = "55555555-5555-4555-8555-555555555555"
	ReviewerID   = "66666666-6666-4666-8666-666666666666"
	ManagerID    = "99999999-9999-4999-8999-999999999999"
	CityID       = "77777777-7777-4777-8777-777777777777"
	Password     = "Shamal-Test-12"
	EditorTOTP   = "JBSWY3DPEHPK3PXP"
	ReviewerTOTP = "JBSWY3DPEHPK3PXQ"
	ManagerTOTP  = "JBSWY3DPEHPK3PXR"
)

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM organization.nodes WHERE id = $1`, CompanyID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return EnsureCommercial(ctx, pool)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO organization.nodes (id, company_id, kind, name_ar, name_en, status) VALUES ($1, $1, 'company', 'شمال العقاري', 'Shamal Real Estate', 'active')`, []any{CompanyID}},
		{`INSERT INTO organization.nodes (id, company_id, kind, parent_id, name_ar, name_en, status) VALUES ($1, $2, 'branch', $2, 'شمال الرياض', 'North Riyadh', 'active')`, []any{BranchID, CompanyID}},
		{`INSERT INTO organization.nodes (id, company_id, kind, parent_id, name_ar, name_en, status) VALUES ($1, $2, 'department', $3, 'التشغيل', 'Operations', 'active')`, []any{DeptID, CompanyID, BranchID}},
		{`INSERT INTO organization.nodes (id, company_id, kind, parent_id, name_ar, name_en, status) VALUES ($1, $2, 'team', $3, 'فريق المحتوى', 'Content team', 'active')`, []any{TeamID, CompanyID, DeptID}},
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			return err
		}
	}
	users := []struct {
		id, name, email, secret string
	}{
		{EditorID, "محرر المحتوى", "editor@shamal.local", EditorTOTP},
		{ReviewerID, "مراجع المحتوى", "reviewer@shamal.local", ReviewerTOTP},
		{ManagerID, "مدير البرنامج", "manager@shamal.local", ManagerTOTP},
	}
	for _, user := range users {
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity.users (id, company_id, status, display_name, password_hash, mfa_required, mfa_secret)
			VALUES ($1, $2, 'active', $3, $4, true, $5)
		`, user.id, CompanyID, user.name, string(hash), user.secret); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity.contact_methods (id, user_id, kind, value, verified_at, is_primary)
			VALUES (gen_random_uuid(), $1, 'email', $2, now(), true)
		`, user.id, user.email); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization.memberships (id, user_id, node_id, company_id, is_primary, starts_at)
			VALUES (gen_random_uuid(), $1, $2, $3, true, now())
		`, user.id, TeamID, CompanyID); err != nil {
			return err
		}
	}
	if err := grant(ctx, tx, EditorID, []string{"view", "create", "update"}, []string{"geography", "catalog"}); err != nil {
		return err
	}
	if err := grant(ctx, tx, ReviewerID, []string{"view", "approve", "publish", "archive"}, []string{"geography", "catalog"}); err != nil {
		return err
	}
	if err := grant(ctx, tx, ReviewerID, []string{"view"}, []string{"organization", "files"}); err != nil {
		return err
	}
	if err := grant(ctx, tx, ManagerID, []string{"view", "create", "update", "archive"}, []string{"organization"}); err != nil {
		return err
	}
	if err := grant(ctx, tx, ManagerID, []string{"view"}, []string{"permissions", "geography", "catalog", "identity", "files"}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO geography.cities (id, country_code, name_ar, name_en, timezone, currency, boundary, boundary_authority, boundary_note_ar, boundary_note_en)
		VALUES ($1, 'SA', 'الرياض', 'Riyadh', 'Asia/Riyadh', 'SAR',
			ST_GeomFromText('MULTIPOLYGON(((46.2 24.3, 47.4 24.3, 47.4 25.2, 46.2 25.2, 46.2 24.3)))', 4326),
			'approximate_test',
			'مرجع تشغيلي تقريبي للاختبار، وليس حداً رسمياً معتمداً.',
			'Approximate operational mask for testing. It is not an adopted official boundary.')
	`, CityID); err != nil {
		return err
	}
	neighborhoods := []struct {
		id, ar, en string
		lat, lon   float64
		alias      string
	}{
		{"88888888-8888-4888-8888-888888888801", "النرجس", "Al Narjis", 24.885, 46.645, "حي النرجس"},
		{"88888888-8888-4888-8888-888888888802", "العارض", "Al Arid", 24.905, 46.605, "حي العارض"},
		{"88888888-8888-4888-8888-888888888803", "القيروان", "Al Qirawan", 24.845, 46.595, "حي القيروان"},
		{"88888888-8888-4888-8888-888888888804", "الياسمين", "Al Yasmin", 24.825, 46.645, "حي الياسمين"},
		{"88888888-8888-4888-8888-888888888805", "الملقا", "Al Malqa", 24.805, 46.605, "حي الملقا"},
		{"88888888-8888-4888-8888-888888888806", "حطين", "Hittin", 24.765, 46.605, "حي حطين"},
		{"88888888-8888-4888-8888-888888888807", "الصحافة", "Al Sahafa", 24.795, 46.655, "حي الصحافة"},
	}
	for _, item := range neighborhoods {
		geo := square(item.lon, item.lat, 0.008)
		if _, err := tx.Exec(ctx, `
			INSERT INTO geography.neighborhoods (
				id, city_id, company_id, branch_id, status, service_sale, service_annual, service_monthly, service_daily,
				coverage, show_in_directory, show_on_map, boundary, boundary_source, boundary_is_official,
				boundary_note_ar, boundary_note_en, created_by, updated_by
			) VALUES (
				$1, $2, $3, $4, 'published', true, true, true, true, 'served', true, true,
				ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($5), 4326)), 'seed_approximate_for_test', false,
				'حد تقريبي لبيانات الاختبار.', 'Approximate test boundary.', $6, $7
			)
		`, item.id, CityID, CompanyID, BranchID, geo, EditorID, ReviewerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO geography.translations (neighborhood_id, city_id, locale, name, normalized_name, slug, description)
			VALUES ($1, $2, 'ar', $3, $4, $3, 'حي في شمال الرياض.'),
			       ($1, $2, 'en', $5, $6, $7, 'A neighborhood in North Riyadh.')
		`, item.id, CityID, item.ar, geography.Normalize(item.ar), item.en, geography.Normalize(item.en), slug(item.en)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO geography.aliases (id, neighborhood_id, locale, alias, normalized) VALUES (gen_random_uuid(), $1, 'ar', $2, $3)
		`, item.id, item.alias, geography.Normalize(item.alias)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO geography.sync_jobs (id, neighborhood_id, status) VALUES (gen_random_uuid(), $1, 'succeeded')
		`, item.id); err != nil {
			return err
		}
	}
	types := []struct {
		code, ar, en string
		rooms        bool
	}{
		{"villa", "فيلا", "Villa", true},
		{"apartment", "شقة", "Apartment", true},
		{"floor", "دور", "Floor", true},
		{"land", "أرض", "Land", false},
		{"townhouse", "منزل متلاصق", "Townhouse", true},
		{"small_unit", "وحدة صغيرة", "Small unit", true},
		{"chalet", "شاليه", "Chalet", true},
	}
	for _, item := range types {
		if _, err := tx.Exec(ctx, `INSERT INTO catalog.property_types (code, name_ar, name_en, requires_rooms) VALUES ($1, $2, $3, $4)`, item.code, item.ar, item.en, item.rooms); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return EnsureCommercial(ctx, pool)
}

func EnsureCommercial(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		DELETE FROM permissions.rules
		WHERE grantee_type = 'user' AND grantee_id = $1
		  AND (
		    domain IN ('clients','visits','deals','finance','messages','disputes','files','organization')
		    OR (domain IN ('geography','catalog') AND action = 'delete_draft')
		    OR (domain = 'reports' AND action = 'view' AND source_label = 'seed')
		  )
	`, EditorID); err != nil {
		return err
	}
	grants := []struct{ user, action, domain string }{
		{ReviewerID, "view", "visits"}, {ReviewerID, "approve", "visits"},
		{ReviewerID, "view", "clients"}, {ReviewerID, "view", "deals"},
		{ManagerID, "view", "deals"}, {ManagerID, "approve", "deals"},
		{ManagerID, "view", "visits"}, {ManagerID, "archive", "visits"},
		{ManagerID, "view", "clients"}, {ManagerID, "archive", "clients"},
		{ReviewerID, "view", "finance"}, {ReviewerID, "approve", "finance"},
		{ManagerID, "view", "finance"}, {ManagerID, "approve", "finance"},
		{ReviewerID, "view", "messages"},
		{ManagerID, "view", "messages"}, {ManagerID, "archive", "messages"},
		{ReviewerID, "view", "disputes"}, {ReviewerID, "approve", "disputes"},
		{ManagerID, "view", "disputes"}, {ManagerID, "approve", "disputes"},
		{ReviewerID, "view", "reports"}, {ManagerID, "view", "reports"},
		{ManagerID, "create", "permissions"},
	}
	for _, item := range grants {
		if _, err := pool.Exec(ctx, `
			INSERT INTO permissions.rules (
				id, company_id, grantee_type, grantee_id, domain, action, effect, layer, scope_kind, scope_node_id, source_label, created_by
			)
			SELECT gen_random_uuid(), $1, 'user', $2, $3, $4, 'allow', 'direct', 'company', $1, 'seed', $2
			WHERE NOT EXISTS (
				SELECT 1 FROM permissions.rules
				WHERE grantee_id = $2 AND domain = $3 AND action = $4 AND effect = 'allow' AND layer = 'direct'
			)
		`, CompanyID, item.user, item.domain, item.action); err != nil {
			return err
		}
	}
	return nil
}

type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func grant(ctx context.Context, tx execer, userID string, actions, domains []string) error {
	for _, domain := range domains {
		for _, action := range actions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO permissions.rules (
					id, company_id, grantee_type, grantee_id, domain, action, effect, layer, scope_kind, scope_node_id, source_label, created_by
				) VALUES (
					gen_random_uuid(), $1, 'user', $2, $3, $4, 'allow', 'direct', 'company', $1, 'seed', $2
				)
			`, CompanyID, userID, domain, action); err != nil {
				return err
			}
		}
	}
	return nil
}

func square(lon, lat, d float64) string {
	return fmt.Sprintf(
		`{"type":"Polygon","coordinates":[[[%.5f,%.5f],[%.5f,%.5f],[%.5f,%.5f],[%.5f,%.5f],[%.5f,%.5f]]]}`,
		lon-d, lat-d, lon+d, lat-d, lon+d, lat+d, lon-d, lat+d, lon-d, lat-d,
	)
}

func slug(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}
