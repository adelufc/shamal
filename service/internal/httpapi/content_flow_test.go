package httpapi

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func TestContentAndPendingListing(t *testing.T) {
	ctx := context.Background()
	cfg := platform.LoadConfig()
	pool, err := platform.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skip(err)
	}
	if err := platform.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	options, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(options)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip(err)
	}
	api := New(pool, rdb, cfg)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	c := &apiClient{t: t, base: srv.URL, http: srv.Client()}
	editor := c.login("editor@shamal.local", seed.EditorTOTP)
	reviewer := c.login("reviewer@shamal.local", seed.ReviewerTOTP)
	slug := fmt.Sprintf("guide-%d", time.Now().UnixNano()%1000000)

	status, hidden := c.get("/api/v1/public/content?locale=ar&slug=" + slug)
	if status != 200 || len(asList(t, hidden["items"])) != 0 {
		t.Fatalf("public before draft: %d %v", status, hidden)
	}
	status, page := c.post("/api/v1/content/pages", map[string]any{
		"kind": "article", "slug": slug, "neighborhood_id": narjisID,
		"title_ar": "دليل النرجس", "body_ar": "نص عربي مستقل عن سجل الحي.",
		"title_en": "Narjis guide", "body_en": "An English page that is not a copy of the neighborhood record.",
	}, editor)
	if status != 201 || page["status"] != "draft" {
		t.Fatalf("create: %d %v", status, page)
	}
	status, preview := c.getAuth("/api/v1/content/pages/"+str(page, "id"), editor)
	if status != 200 || len(asList(t, preview["texts"])) != 2 {
		t.Fatalf("preview: %d %v", status, preview)
	}
	status, submitted := c.post("/api/v1/content/pages/"+str(page, "id")+"/submit", map[string]any{"version": num(page, "version")}, editor)
	if status != 200 || submitted["status"] != "in_review" {
		t.Fatalf("submit: %d %v", status, submitted)
	}
	status, denied := c.post("/api/v1/content/pages/"+str(page, "id")+"/publish", map[string]any{"version": num(submitted, "version")}, editor)
	if status != 403 {
		t.Fatalf("editor publish: %d %v", status, denied)
	}
	status, published := c.post("/api/v1/content/pages/"+str(page, "id")+"/publish", map[string]any{"version": num(submitted, "version")}, reviewer)
	if status != 200 || published["status"] != "published" {
		t.Fatalf("publish: %d %v", status, published)
	}
	status, visible := c.get("/api/v1/public/content?locale=en&slug=" + slug)
	if status != 200 || len(asList(t, visible["items"])) != 1 || asMap(t, asList(t, visible["items"])[0])["title"] != "Narjis guide" {
		t.Fatalf("english public: %d %v", status, visible)
	}
	status, arabic := c.get("/api/v1/public/content?locale=ar&slug=" + slug)
	if status != 200 || asMap(t, asList(t, arabic["items"])[0])["title"] != "دليل النرجس" {
		t.Fatalf("arabic public: %d %v", status, arabic)
	}
	if asMap(t, asList(t, arabic["items"])[0])["body"] == "" {
		t.Fatal("neighborhood text was not kept separate")
	}

	listingID := publishTaggedListing(t, c, editor, reviewer, slug, "sale", 22000, 180, 46.65, 24.89, "إعلان "+slug, "Listing "+slug)
	status, listed := c.getAuth("/api/v1/catalog/listings?status=published", editor)
	if status != 200 || !idsContain(asList(t, listed["items"]), listingID) {
		t.Fatalf("listing list: %d %v", status, listed)
	}
	var version int
	for _, raw := range asList(t, listed["items"]) {
		item := asMap(t, raw)
		if item["id"] == listingID {
			version = num(item, "version")
		}
	}
	status, staged := c.post("/api/v1/catalog/listings/"+listingID+"/pending-change", map[string]any{
		"version": version, "price_minor": 33000, "neighborhood_id": narjisID, "license_ref": "local-test-ref-2", "owner_user_id": seed.ReviewerID,
	}, editor)
	if status != 200 || staged["status"] != "published" || num(staged, "price_minor") == 33000 {
		t.Fatalf("pending change applied live: %d %v", status, staged)
	}
	pending := asMap(t, staged["pending_changes"])
	if num(pending, "price_minor") != 33000 || pending["license_ref"] != "local-test-ref-2" {
		t.Fatalf("staged payload: %v", staged["pending_changes"])
	}
}
