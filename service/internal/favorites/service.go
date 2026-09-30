package favorites

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/platform"
)

type Item struct {
	ListingID       string    `json:"listing_id"`
	Status          string    `json:"status"`
	TransactionType string    `json:"transaction_type"`
	PriceMinor      int64     `json:"price_minor"`
	PriceUnit       string    `json:"price_unit"`
	Currency        string    `json:"currency"`
	Title           string    `json:"title"`
	CreatedAt       time.Time `json:"created_at"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Add(ctx context.Context, userID, listingID string) (Item, bool, error) {
	if _, err := uuid.Parse(listingID); err != nil {
		return Item{}, false, platform.E("validation", 422)
	}
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM catalog.listings WHERE id = $1`, listingID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, false, platform.E("not_found", 404)
	}
	if err != nil {
		return Item{}, false, err
	}
	if status != "published" {
		return Item{}, false, platform.E("listing_not_published", 422)
	}
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO catalog.favorites (user_id, listing_id) VALUES ($1, $2)
		ON CONFLICT (user_id, listing_id) DO NOTHING
	`, userID, listingID)
	if err != nil {
		return Item{}, false, err
	}
	item, err := s.one(ctx, userID, listingID, "ar")
	return item, tag.RowsAffected() == 1, err
}

func (s Service) Remove(ctx context.Context, userID, listingID string) error {
	if _, err := uuid.Parse(listingID); err != nil {
		return platform.E("validation", 422)
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM catalog.favorites WHERE user_id = $1 AND listing_id = $2`, userID, listingID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return platform.E("not_found", 404)
	}
	return nil
}

func (s Service) List(ctx context.Context, userID, locale string) ([]Item, error) {
	if locale != "en" {
		locale = "ar"
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT f.listing_id::text, l.status, l.transaction_type, COALESCE(l.price_minor, 0), l.price_unit, l.currency,
		       COALESCE(t.title, ''), f.created_at
		FROM catalog.favorites f
		JOIN catalog.listings l ON l.id = f.listing_id
		LEFT JOIN catalog.listing_translations t ON t.listing_id = l.id AND t.locale = $2
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC, f.listing_id
	`, userID, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ListingID, &item.Status, &item.TransactionType, &item.PriceMinor, &item.PriceUnit, &item.Currency, &item.Title, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Service) one(ctx context.Context, userID, listingID, locale string) (Item, error) {
	items, err := s.List(ctx, userID, locale)
	if err != nil {
		return Item{}, err
	}
	for _, item := range items {
		if item.ListingID == listingID {
			return item, nil
		}
	}
	return Item{}, platform.E("not_found", 404)
}
