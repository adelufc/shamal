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

func TestReviewPaths(t *testing.T) {
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

	const narjis = "88888888-8888-4888-8888-888888888801"
	status, place := c.get("/api/v1/public/neighborhoods/" + narjis + "?locale=ar")
	if status != 200 || asMap(t, place["neighborhood"])["id"] != narjis {
		t.Fatalf("neighborhood by id: %d %v", status, place)
	}
	status, missing := c.get("/api/v1/public/neighborhoods/not-a-real-slug?locale=ar")
	if status != 404 || missing["code"] != "not_found" {
		t.Fatalf("missing neighborhood: %d %v", status, missing)
	}

	status, search := c.get("/api/v1/public/search?locale=ar&page=2&page_size=25")
	if status != 200 || search["map_scope"] != "all_matches" || search["page"] != float64(2) {
		t.Fatalf("search page: %d %v", status, search["page"])
	}
	listings := asList(t, search["listings"])
	if len(listings) == 0 {
		status, search = c.get("/api/v1/public/search?locale=ar&page=1&page_size=25")
		if status != 200 {
			t.Fatalf("search page 1: %d", status)
		}
		listings = asList(t, search["listings"])
	}
	if len(listings) == 0 {
		t.Fatal("search had no listings")
	}
	first := asMap(t, listings[0])
	if _, ok := first["precise_lon"]; ok {
		t.Fatal("search hit exposed a precise coordinate")
	}
	status, detail := c.get("/api/v1/public/listings/" + str(first, "id") + "?locale=ar")
	if status != 200 || detail["title"] == "" || detail["approximate_location"] != true {
		t.Fatalf("public listing: %d %v", status, detail)
	}
	if _, ok := detail["precise_lon"]; ok {
		t.Fatal("public listing exposed a precise coordinate")
	}
	if _, ok := detail["pending_changes"]; ok {
		t.Fatal("public listing exposed a pending change")
	}

	status, daily := c.get("/api/v1/public/search?locale=ar&transaction=daily&page_size=1")
	if status == 200 && len(asList(t, daily["listings"])) > 0 {
		stayID := str(asMap(t, asList(t, daily["listings"])[0]), "id")
		status, early := c.get("/api/v1/public/stays/quote?listing_id=" + stayID + "&arrival=2027-11-10&departure=2027-11-09&guests=1")
		if status != 422 || !hasField(early, "min_stay") {
			t.Fatalf("departure before arrival: %d %v", status, early)
		}
		status, guests := c.get("/api/v1/public/stays/quote?listing_id=" + stayID + "&arrival=2027-11-10&departure=2027-11-12&guests=0")
		if status != 422 || !hasField(guests, "invalid_guests") {
			t.Fatalf("guest count: %d %v", status, guests)
		}
	}

	status, denied := c.post("/api/v1/reports/visits/export", map[string]any{}, "")
	if status != 401 {
		t.Fatalf("anonymous export: %d %v", status, denied)
	}
	email := fmt.Sprintf("review-%d@example.com", time.Now().UnixNano())
	status, registered := c.post("/api/v1/auth/register", map[string]any{
		"display_name": "عميل المراجعة", "email": email, "password": seed.Password,
	}, "")
	if status != 201 {
		t.Fatalf("register: %d %v", status, registered)
	}
	client := c.loginClient(email, seed.Password)
	status, clientExport := c.post("/api/v1/reports/visits/export", map[string]any{}, client)
	if status != 401 && status != 403 {
		t.Fatalf("client export: %d %v", status, clientExport)
	}
	status, clientReport := c.getAuth("/api/v1/reports/visits", client)
	if clientReport["measures"] != nil || (status != 401 && status != 403) {
		t.Fatalf("client report: %d %v", status, clientReport)
	}

	loginStatus, loginBody := c.post("/api/v1/auth/login", map[string]any{
		"email": "editor@shamal.local", "password": seed.Password, "context": "staff",
	}, "")
	if loginStatus != 200 || loginBody["mfa_required"] != true {
		t.Fatalf("staff login: %d %v", loginStatus, loginBody)
	}
	badStatus, badBody := c.post("/api/v1/auth/mfa", map[string]any{
		"challenge_id": loginBody["challenge_id"], "code": "000000",
	}, "")
	if badStatus != 401 && badStatus != 422 {
		t.Fatalf("bad totp: %d %v", badStatus, badBody)
	}

	editor := c.login("editor@shamal.local", seed.EditorTOTP)
	status, occupancy := c.getAuth("/api/v1/reports/occupancy", editor)
	if status != 200 {
		t.Fatalf("occupancy: %d %v", status, occupancy)
	}
	for _, raw := range asList(t, occupancy["measures"]) {
		measure := asMap(t, raw)
		if measure["code"] == "occupancy_rate" && measure["value"] != nil {
			t.Fatalf("occupancy rate invented: %v", measure)
		}
	}
	status, owner := c.getAuth("/api/v1/owner/performance", editor)
	if status != 200 {
		t.Fatalf("owner: %d %v", status, owner)
	}
	for _, raw := range asList(t, owner["measures"]) {
		measure := asMap(t, raw)
		if measure["code"] == "occupancy_rate" && measure["value"] != nil {
			t.Fatalf("owner occupancy rate invented: %v", measure)
		}
	}
}
