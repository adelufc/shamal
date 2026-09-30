package catalog

import "context"

type SearchResult struct {
	Listings []SearchHit
	Markers  []SearchHit
	Clusters []SearchCluster
	Total    int
}

func (s Service) Search(ctx context.Context, q SearchQuery) (SearchResult, error) {
	if len(q.AllowedIDs) == 0 {
		return SearchResult{Listings: []SearchHit{}, Markers: []SearchHit{}, Clusters: []SearchCluster{}}, nil
	}
	if q.NameIDs == nil {
		q.NameIDs = []string{}
	}
	where := `
		FROM catalog.listings l
		JOIN catalog.properties p ON p.id = l.property_id
		JOIN catalog.units u ON u.id = l.unit_id
		LEFT JOIN catalog.listing_translations t ON t.listing_id = l.id AND t.locale = $7
		WHERE l.status = 'published'
		  AND l.currency = 'SAR'
		  AND l.neighborhood_id::text = ANY($1::text[])
		  AND ($2 = '' OR l.transaction_type = $2)
		  AND ($3::bigint IS NULL OR l.price_minor >= $3)
		  AND ($4::bigint IS NULL OR l.price_minor <= $4)
		  AND (
		    $5 = ''
		    OR l.neighborhood_id::text = ANY($6::text[])
		    OR strpos(lower(COALESCE(t.title, '')), lower($5)) > 0
		  )
		  AND ($8::int IS NULL OR COALESCE(u.rooms, 0) >= $8)
		  AND ($9::bool = false OR u.furnished IS TRUE)
		  AND ($10::bool = false OR u.air_conditioning IS TRUE)
		  AND ($11::bool = false OR u.internet IS TRUE)
		  AND ($12::bool = false OR u.parking IS TRUE)
		  AND ($13::bool = false OR u.bedding IS TRUE)
		  AND ($14::bool = false OR u.self_entry IS TRUE)
		  AND ($15::bool = false OR EXISTS (
		    SELECT 1 FROM identity.contact_methods c
		    WHERE c.user_id = p.owner_user_id AND c.verified_at IS NOT NULL
		  ))`
	args := []any{q.AllowedIDs, q.Transaction, q.PriceMin, q.PriceMax, q.Text, q.NameIDs, q.Locale, q.RoomsMin, q.Furnished, q.Cooling, q.Internet, q.Parking, q.Bedding, q.SelfEntry, q.Verified}
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) `+where, args...).Scan(&total); err != nil {
		return SearchResult{}, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT l.neighborhood_id::text, count(*), COALESCE(min(l.price_minor), 0), COALESCE(max(l.price_minor), 0)
	`+where+` GROUP BY l.neighborhood_id ORDER BY l.neighborhood_id`, args...)
	if err != nil {
		return SearchResult{}, err
	}
	clusters := make([]SearchCluster, 0)
	for rows.Next() {
		var item SearchCluster
		if err := rows.Scan(&item.NeighborhoodID, &item.Count, &item.PriceMin, &item.PriceMax); err != nil {
			rows.Close()
			return SearchResult{}, err
		}
		clusters = append(clusters, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SearchResult{}, err
	}
	rows.Close()

	listSQL := `
		SELECT l.id::text, l.neighborhood_id::text, l.transaction_type, COALESCE(l.price_minor, 0), l.price_unit, l.currency,
		       COALESCE(t.title, ''), COALESCE(t.description, ''), COALESCE(p.area_sqm, 0),
		       u.rooms, p.type_code,
		       EXISTS (SELECT 1 FROM identity.contact_methods c WHERE c.user_id = p.owner_user_id AND c.verified_at IS NOT NULL)
	` + where + `
		ORDER BY ` + OrderBy(q.Sort) + `
		LIMIT $16 OFFSET $17`
	pageArgs := append(append([]any{}, args...), q.Size, (q.Page-1)*q.Size)
	rows, err = s.Pool.Query(ctx, listSQL, pageArgs...)
	if err != nil {
		return SearchResult{}, err
	}
	listings, err := scanHits(rows)
	rows.Close()
	if err != nil {
		return SearchResult{}, err
	}
	markers := []SearchHit{}
	if !UseClusters(total) {
		markerSQL := `
			SELECT l.id::text, l.neighborhood_id::text, l.transaction_type, COALESCE(l.price_minor, 0), l.price_unit, l.currency,
			       COALESCE(t.title, ''), '', 0, u.rooms, p.type_code, false
		` + where + `
			ORDER BY ` + OrderBy(q.Sort) + `
			LIMIT $16`
		rows, err = s.Pool.Query(ctx, markerSQL, append(append([]any{}, args...), MarkerCap)...)
		if err != nil {
			return SearchResult{}, err
		}
		markers, err = scanHits(rows)
		rows.Close()
		if err != nil {
			return SearchResult{}, err
		}
	}
	return SearchResult{Listings: listings, Markers: markers, Clusters: clusters, Total: total}, nil
}

func scanHits(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]SearchHit, error) {
	out := make([]SearchHit, 0)
	for rows.Next() {
		var item SearchHit
		if err := rows.Scan(&item.ID, &item.NeighborhoodID, &item.TransactionType, &item.PriceMinor, &item.PriceUnit, &item.Currency, &item.Title, &item.Description, &item.AreaSqm, &item.Rooms, &item.TypeCode, &item.OwnerVerified); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
