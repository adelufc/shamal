package notify

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/platform"
)

type Search struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	NeighborhoodID  string `json:"neighborhood_id"`
	TransactionType string `json:"transaction_type"`
	Locale          string `json:"locale"`
	Channel         string `json:"channel"`
	Muted           bool   `json:"muted"`
}

type Event struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	EntityID        string `json:"entity_id"`
	TitleAR         string `json:"title_ar"`
	TitleEN         string `json:"title_en"`
	BodyAR          string `json:"body_ar"`
	BodyEN          string `json:"body_en"`
	Channel         string `json:"channel"`
	DeliveryStatus  string `json:"delivery_status"`
	ProviderAdopted bool   `json:"provider_adopted"`
	CreatedAt       string `json:"created_at"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) SaveSearch(ctx context.Context, userID string, name, neighborhoodID, transaction, locale, channel string) (Search, error) {
	if name == "" || neighborhoodID == "" {
		return Search{}, platform.E("validation", 422)
	}
	if channel == "" {
		channel = Channel
	}
	if err := AllowChannel(channel); err != nil {
		return Search{}, err
	}
	if transaction != "" && transaction != "sale" && transaction != "annual" && transaction != "monthly" && transaction != "daily" {
		return Search{}, platform.E("invalid_transaction", 422)
	}
	if locale != "en" {
		locale = "ar"
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Search{}, err
	}
	defer tx.Rollback(ctx)
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM notify.searches WHERE user_id = $1::uuid`, userID).Scan(&count); err != nil {
		return Search{}, err
	}
	if count >= SearchLimit {
		return Search{}, platform.E("search_limit", 422)
	}
	var item Search
	err = tx.QueryRow(ctx, `
		INSERT INTO notify.searches (id, user_id, name, neighborhood_id, transaction_type, locale, channel)
		VALUES (gen_random_uuid(), $1::uuid, $2, $3::uuid, $4, $5, $6)
		RETURNING id::text, name, neighborhood_id::text, transaction_type, locale, channel, muted
	`, userID, name, neighborhoodID, transaction, locale, channel).Scan(
		&item.ID, &item.Name, &item.NeighborhoodID, &item.TransactionType, &item.Locale, &item.Channel, &item.Muted,
	)
	if err != nil {
		return Search{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Search{}, err
	}
	return item, nil
}

func (s Service) ListSearches(ctx context.Context, userID string) ([]Search, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, name, neighborhood_id::text, transaction_type, locale, channel, muted
		FROM notify.searches WHERE user_id = $1::uuid ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Search, 0)
	for rows.Next() {
		var item Search
		if err := rows.Scan(&item.ID, &item.Name, &item.NeighborhoodID, &item.TransactionType, &item.Locale, &item.Channel, &item.Muted); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Service) MuteSearch(ctx context.Context, userID, searchID string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE notify.searches SET muted = true WHERE id = $1::uuid AND user_id = $2::uuid`, searchID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return platform.E("not_found", 404)
	}
	return nil
}

func (s Service) MuteChannel(ctx context.Context, userID, channel string, muted bool) error {
	if err := AllowChannel(channel); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notify.mutes (user_id, channel, muted)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (user_id, channel) DO UPDATE SET muted = EXCLUDED.muted, updated_at = now()
	`, userID, channel, muted)
	return err
}

func (s Service) ChannelMuted(ctx context.Context, userID string) (bool, error) {
	var muted bool
	err := s.Pool.QueryRow(ctx, `SELECT muted FROM notify.mutes WHERE user_id = $1::uuid AND channel = 'in_app'`, userID).Scan(&muted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return muted, err
}

func (s Service) Inbox(ctx context.Context, userID string) ([]Event, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, kind, entity_id::text, title_ar, title_en, body_ar, body_en, channel, delivery_status, provider_adopted, created_at::text
		FROM notify.events
		WHERE user_id = $1::uuid AND suppressed = false
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Kind, &item.EntityID, &item.TitleAR, &item.TitleEN, &item.BodyAR, &item.BodyEN, &item.Channel, &item.DeliveryStatus, &item.ProviderAdopted, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Service) ListingPublished(ctx context.Context, listingID, neighborhoodID, transaction string) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, user_id::text, muted
		FROM notify.searches
		WHERE neighborhood_id = $1::uuid
		  AND (transaction_type = '' OR transaction_type = $2)
	`, neighborhoodID, transaction)
	if err != nil {
		return err
	}
	defer rows.Close()
	type hit struct {
		id, user string
		muted    bool
	}
	hits := make([]hit, 0)
	for rows.Next() {
		var item hit
		if err := rows.Scan(&item.id, &item.user, &item.muted); err != nil {
			return err
		}
		hits = append(hits, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range hits {
		if err := s.emit(ctx, item.user, "listing_match", listingID, item.id, DedupListing(item.user, listingID, item.id), item.muted,
			"إعلان مطابق لبحث محفوظ",
			"A listing matches a saved search",
			"نُشر إعلان جديد في الحي ونوع المعاملة المحفوظين. لم يُرسل بريد.",
			"A newly published listing matches the saved neighborhood and transaction. No mail was sent.",
		); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) VisitChanged(ctx context.Context, profileID, visitID, status string, version int) error {
	var userID string
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(user_id::text, '') FROM clients.profiles WHERE id = $1::uuid`, profileID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) || userID == "" {
		return nil
	}
	if err != nil {
		return err
	}
	return s.emit(ctx, userID, "visit_change", visitID, "", DedupVisit(userID, visitID, status, version), false,
		"تغيّر موعد زيارة",
		"A visit changed",
		"تغيّرت حالة الزيارة إلى "+status+". الإشعار داخل شمال فقط.",
		"The visit status is now "+status+". The notice stays inside Shamal.",
	)
}

func (s Service) PaymentReview(ctx context.Context, userID, attemptID, status string) error {
	if !ReviewStatus(status) {
		return nil
	}
	return s.emit(ctx, userID, "payment_review", attemptID, "", DedupPayment(userID, attemptID, status), false,
		"مراجعة دفعة",
		"A payment is in review",
		"المحاولة بقيت قيد المراجعة ولم يُرسل إشعار إلى مزود.",
		"The attempt stays in review. No notice was handed to a provider.",
	)
}

func (s Service) emit(ctx context.Context, userID, kind, entityID, searchID, dedup string, searchMuted bool, titleAR, titleEN, bodyAR, bodyEN string) error {
	suppressed := searchMuted
	reason := ""
	if suppressed {
		reason = "muted"
	} else {
		muted, err := s.ChannelMuted(ctx, userID)
		if err != nil {
			return err
		}
		if muted {
			suppressed = true
			reason = "muted"
		}
	}
	var search any
	if searchID != "" {
		search = searchID
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notify.events (
			id, user_id, kind, entity_id, search_id, dedup_key,
			title_ar, title_en, body_ar, body_en,
			channel, delivery_status, provider_adopted, suppressed, suppress_reason
		) VALUES (
			gen_random_uuid(), $1::uuid, $2, $3::uuid, $4::uuid, $5,
			$6, $7, $8, $9,
			'in_app', 'not_sent', false, $10, $11
		)
		ON CONFLICT (dedup_key) DO NOTHING
	`, userID, kind, entityID, search, dedup, titleAR, titleEN, bodyAR, bodyEN, suppressed, reason)
	return err
}
