package identity

import (
	"context"
	"strings"
)

type Staff struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Status      string `json:"status"`
}

func (s Store) ListStaff(ctx context.Context, companyID, query string) ([]Staff, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT DISTINCT u.id::text, u.display_name, u.status,
		       COALESCE((SELECT c.value FROM identity.contact_methods c WHERE c.user_id = u.id AND c.kind = 'email' AND c.is_primary LIMIT 1), '')
		FROM identity.users u
		JOIN organization.memberships m ON m.user_id = u.id AND m.company_id = $1::uuid
		ORDER BY u.display_name
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Staff, 0)
	needle := strings.ToLower(strings.TrimSpace(query))
	for rows.Next() {
		var item Staff
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Status, &item.Email); err != nil {
			return nil, err
		}
		if needle != "" && !strings.Contains(strings.ToLower(item.DisplayName), needle) && !strings.Contains(strings.ToLower(item.Email), needle) {
			continue
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
