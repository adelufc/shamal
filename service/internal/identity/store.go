package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"shamal/service/internal/platform"
)

type User struct {
	ID             string
	CompanyID      string
	Status         string
	DisplayName    string
	PasswordHash   string
	MFARequired    bool
	MFASecret      string
	FailedAttempts int
	FirstFailedAt  *time.Time
	LockedUntil    *time.Time
	Email          string
	Version        int
}

type Store struct {
	DB platform.DB
}

func (s Store) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.DB.QueryRow(ctx, `
		SELECT u.id::text, COALESCE(u.company_id::text,''), u.status, u.display_name,
		       COALESCE(u.password_hash,''), u.mfa_required, COALESCE(u.mfa_secret,''),
		       u.failed_attempts, u.first_failed_at, u.locked_until, c.value, u.version
		FROM identity.users u
		JOIN identity.contact_methods c ON c.user_id = u.id AND c.kind = 'email'
		WHERE lower(c.value) = lower($1)
	`, email).Scan(&user.ID, &user.CompanyID, &user.Status, &user.DisplayName, &user.PasswordHash, &user.MFARequired, &user.MFASecret, &user.FailedAttempts, &user.FirstFailedAt, &user.LockedUntil, &user.Email, &user.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, pgx.ErrNoRows
	}
	return user, err
}

func (s Store) FindByID(ctx context.Context, id string) (User, error) {
	var user User
	err := s.DB.QueryRow(ctx, `
		SELECT u.id::text, COALESCE(u.company_id::text,''), u.status, u.display_name,
		       COALESCE(u.password_hash,''), u.mfa_required, COALESCE(u.mfa_secret,''),
		       u.failed_attempts, u.first_failed_at, u.locked_until,
		       COALESCE((SELECT value FROM identity.contact_methods c WHERE c.user_id = u.id AND c.kind = 'email' LIMIT 1), ''),
		       u.version
		FROM identity.users u
		WHERE u.id = $1
	`, id).Scan(&user.ID, &user.CompanyID, &user.Status, &user.DisplayName, &user.PasswordHash, &user.MFARequired, &user.MFASecret, &user.FailedAttempts, &user.FirstFailedAt, &user.LockedUntil, &user.Email, &user.Version)
	return user, err
}

func (s Store) SaveFailure(ctx context.Context, id string, attempts int, first, locked *time.Time) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE identity.users
		SET failed_attempts = $2, first_failed_at = $3, locked_until = $4, updated_at = now(), version = version + 1
		WHERE id = $1
	`, id, attempts, first, locked)
	return err
}

func (s Store) SaveSuccess(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE identity.users
		SET failed_attempts = 0, first_failed_at = NULL, locked_until = NULL, updated_at = now()
		WHERE id = $1
	`, id)
	return err
}

func (s Store) CreateClient(ctx context.Context, db platform.DB, id, companyID, name, email, passwordHash string) error {
	if db == nil {
		db = s.DB
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO identity.users (id, company_id, status, display_name, password_hash, mfa_required)
		VALUES ($1, $2, 'active', $3, $4, false)
	`, id, companyID, name, passwordHash); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO identity.contact_methods (id, user_id, kind, value, verified_at, is_primary)
		VALUES (gen_random_uuid(), $1, 'email', $2, now(), true)
	`, id, email)
	return err
}

func (s Store) AddDevice(ctx context.Context, id, userID, label, fingerprint string, seen time.Time) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO identity.devices (id, user_id, label, fingerprint_hash, last_seen_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id, userID, label, fingerprint, seen)
	return err
}
