package permissions

import (
	"context"

	"github.com/google/uuid"
	"shamal/service/internal/platform"
)

type Store struct {
	DB platform.DB
}

func (s Store) Load(ctx context.Context, userID string, nodeIDs []string) ([]Rule, []Break, error) {
	if nodeIDs == nil {
		nodeIDs = []string{}
	}
	rows, err := s.DB.Query(ctx, `
		SELECT id::text, domain, action, effect, layer, scope_kind, COALESCE(scope_node_id::text,''),
		       starts_at, ends_at, source_label, grantee_id::text, grantee_type
		FROM permissions.rules
		WHERE (grantee_type = 'user' AND grantee_id = $1)
		   OR (grantee_type = 'node' AND grantee_id = ANY($2::uuid[]))
	`, userID, nodeIDs)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	rules := make([]Rule, 0)
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Domain, &rule.Action, &rule.Effect, &rule.Layer, &rule.Scope.Kind, &rule.Scope.NodeID, &rule.StartsAt, &rule.EndsAt, &rule.Source, &rule.GranteeID, &rule.AttachedTo); err != nil {
			return nil, nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	breaks := make([]Break, 0)
	brows, err := s.DB.Query(ctx, `
		SELECT node_id::text, domain, action
		FROM permissions.inheritance_breaks
		WHERE node_id = ANY($1::uuid[])
	`, nodeIDs)
	if err != nil {
		return nil, nil, err
	}
	defer brows.Close()
	for brows.Next() {
		var item Break
		if err := brows.Scan(&item.NodeID, &item.Domain, &item.Action); err != nil {
			return nil, nil, err
		}
		breaks = append(breaks, item)
	}
	return rules, breaks, brows.Err()
}

func (s Store) Create(ctx context.Context, companyID, actor string, rule Rule) (Rule, error) {
	rule.ID = uuid.NewString()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO permissions.rules (
			id, company_id, grantee_type, grantee_id, domain, action, effect, layer, scope_kind, scope_node_id, starts_at, ends_at, source_label, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10,'')::uuid, $11, $12, $13, NULLIF($14,'')::uuid
		)
	`, rule.ID, companyID, rule.AttachedTo, rule.GranteeID, rule.Domain, rule.Action, rule.Effect, rule.Layer, rule.Scope.Kind, rule.Scope.NodeID, rule.StartsAt, rule.EndsAt, rule.Source, actor)
	return rule, err
}

func (s Store) CreateBreak(ctx context.Context, companyID, actor string, item Break, reason string) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO permissions.inheritance_breaks (id, company_id, node_id, domain, action, reason, approved_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.NewString(), companyID, item.NodeID, item.Domain, item.Action, reason, actor)
	return err
}

func AncestorIDs(nodes []Node, start string) []string {
	parent := map[string]string{}
	for _, node := range nodes {
		parent[node.ID] = node.ParentID
	}
	return ancestors(start, parent)
}
