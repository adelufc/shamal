package organization

import (
	"context"
	"time"

	"github.com/google/uuid"
	"shamal/service/internal/platform"
)

type Node struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	Kind          string    `json:"kind"`
	ParentID      string    `json:"parent_id"`
	NameAR        string    `json:"name_ar"`
	NameEN        string    `json:"name_en"`
	Status        string    `json:"status"`
	ManagerUserID string    `json:"manager_user_id"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
}

type Membership struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	NodeID    string     `json:"node_id"`
	CompanyID string     `json:"company_id"`
	Primary   bool       `json:"is_primary"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
}

type Store struct {
	DB platform.DB
}

func (s Store) List(ctx context.Context, companyID string) ([]Node, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id::text, company_id::text, kind, COALESCE(parent_id::text,''), name_ar, name_en, status,
		       COALESCE(manager_user_id::text,''), version, created_at
		FROM organization.nodes
		WHERE company_id = $1
		ORDER BY kind, name_ar
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Node, 0)
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.CompanyID, &node.Kind, &node.ParentID, &node.NameAR, &node.NameEN, &node.Status, &node.ManagerUserID, &node.Version, &node.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (s Store) Create(ctx context.Context, node Node) (Node, error) {
	if node.ID == "" {
		node.ID = uuid.NewString()
	}
	err := s.DB.QueryRow(ctx, `
		INSERT INTO organization.nodes (id, company_id, kind, parent_id, name_ar, name_en, status, manager_user_id)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, $5, $6, 'active', NULLIF($7,'')::uuid)
		RETURNING version, created_at
	`, node.ID, node.CompanyID, node.Kind, node.ParentID, node.NameAR, node.NameEN, node.ManagerUserID).Scan(&node.Version, &node.CreatedAt)
	node.Status = "active"
	return node, err
}

func (s Store) Close(ctx context.Context, id string, version int) error {
	tag, err := s.DB.Exec(ctx, `
		UPDATE organization.nodes
		SET status = 'closed', version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2 AND status = 'active'
	`, id, version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return platform.E("version_conflict", 409)
	}
	return nil
}

func (s Store) ActiveChildren(ctx context.Context, id string) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT count(*) FROM organization.nodes WHERE parent_id = $1 AND status = 'active'`, id).Scan(&n)
	return n, err
}

func (s Store) ActiveMemberships(ctx context.Context, id string, now time.Time) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `
		SELECT count(*) FROM organization.memberships
		WHERE node_id = $1 AND starts_at <= $2 AND (ends_at IS NULL OR ends_at > $2)
	`, id, now).Scan(&n)
	return n, err
}

func (s Store) Memberships(ctx context.Context, userID string) ([]Membership, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id::text, user_id::text, node_id::text, company_id::text, is_primary, starts_at, ends_at
		FROM organization.memberships
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Membership, 0)
	for rows.Next() {
		var item Membership
		if err := rows.Scan(&item.ID, &item.UserID, &item.NodeID, &item.CompanyID, &item.Primary, &item.StartsAt, &item.EndsAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) AddMembership(ctx context.Context, item Membership) (Membership, error) {
	item.ID = uuid.NewString()
	if item.StartsAt.IsZero() {
		item.StartsAt = time.Now().UTC()
	}
	_, err := s.DB.Exec(ctx, `
		INSERT INTO organization.memberships (id, user_id, node_id, company_id, is_primary, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, item.ID, item.UserID, item.NodeID, item.CompanyID, item.Primary, item.StartsAt, item.EndsAt)
	return item, err
}
