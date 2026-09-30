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

func TestOwnerPerformance(t *testing.T) {
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
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	editor := c.login("editor@shamal.local", seed.EditorTOTP)
	reviewer := c.login("reviewer@shamal.local", seed.ReviewerTOTP)
	ownerEmail := "owner-" + tag + "@example.com"
	status, registered := c.post("/api/v1/auth/register", map[string]any{"display_name": "مالك وحدة", "email": ownerEmail, "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("register owner: %d %v", status, registered)
	}
	ownerID := str(registered, "id")
	owner := c.loginClient(ownerEmail, seed.Password)
	guest := c.loginClient(registerOther(t, c, "guest-"+tag+"@example.com"), seed.Password)

	status, denied := c.get("/api/v1/owner/performance")
	if status != 401 {
		t.Fatalf("guest: %d %v", status, denied)
	}
	property := map[string]any{
		"neighborhood_id": narjisID, "owner_user_id": ownerID, "type_code": "land", "area_sqm": 150,
		"address_ar": "عنوان المالك", "address_en": "Owner address",
		"precise_lon": 46.649, "precise_lat": 24.889,
	}
	status, place := c.post("/api/v1/catalog/properties", property, editor)
	if status != 201 {
		t.Fatalf("property: %d %v", status, place)
	}
	unit := c.mustUnit(editor, str(place, "id"), "Own-"+tag)
	status, listing := c.post("/api/v1/catalog/listings", map[string]any{
		"property_id": str(place, "id"), "unit_id": str(unit, "id"), "neighborhood_id": narjisID,
		"transaction_type": "daily", "price_minor": 40000, "currency": "SAR", "license_ref": "local-test-ref",
		"broker_user_id": seed.ReviewerID,
		"texts": []map[string]string{
			{"locale": "ar", "title": "وحدة " + tag, "description": "وصف عربي مستقل."},
			{"locale": "en", "title": "Unit " + tag, "description": "An independent English description."},
		},
	}, editor)
	if status != 201 {
		t.Fatalf("listing: %d %v", status, listing)
	}
	for _, step := range []struct{ actor, path string }{{editor, "/submit"}, {reviewer, "/approve"}, {reviewer, "/publish"}} {
		status, listing = c.post("/api/v1/catalog/listings/"+str(listing, "id")+step.path, map[string]any{"version": num(listing, "version")}, step.actor)
		if status != 200 {
			t.Fatalf("%s: %d %v", step.path, status, listing)
		}
	}
	confirmStay(t, c, guest, str(listing, "id"), "2032-06-01", "2032-06-03", 80000, "own-"+tag)

	status, view := c.getAuth("/api/v1/owner/performance?from=2032-06-01&to=2032-06-02", owner)
	if status != 200 || view["commission_included"] != false || view["definition_ar"] == view["definition_en"] {
		t.Fatalf("owner view: %d %v", status, view)
	}
	if got, ok := measureValue(t, view, "units"); !ok || got != 1 {
		t.Fatalf("units %v %v", got, ok)
	}
	if got, ok := measureValue(t, view, "listings_total"); !ok || got != 1 {
		t.Fatalf("listings %v", got)
	}
	if got, ok := measureValue(t, view, "listings_published"); !ok || got != 1 {
		t.Fatalf("published %v", got)
	}
	if got, ok := measureValue(t, view, "occupied_nights"); !ok || got != 2 {
		t.Fatalf("nights %v", got)
	}
	if got, ok := measureValue(t, view, "distinct_units"); !ok || got != 1 {
		t.Fatalf("occupied units %v", got)
	}
	if _, ok := measureValue(t, view, "occupancy_rate"); ok {
		t.Fatal("occupancy rate was calculated")
	}
	found := false
	for _, raw := range asList(t, view["listings"]) {
		item := asMap(t, raw)
		if item["id"] == str(listing, "id") {
			found = true
			if item["title_ar"] != "وحدة "+tag || item["title_en"] != "Unit "+tag || item["title_ar"] == item["title_en"] {
				t.Fatalf("titles %v", item)
			}
		}
	}
	if !found {
		t.Fatalf("owner missed the listing %v", view["listings"])
	}
	status, other := c.getAuth("/api/v1/owner/performance?from=2032-06-01&to=2032-06-02", guest)
	if status != 200 {
		t.Fatalf("guest owner view: %d %v", status, other)
	}
	if got, ok := measureValue(t, other, "units"); !ok || got != 0 {
		t.Fatalf("guest saw units %v", got)
	}
	for _, raw := range asList(t, other["listings"]) {
		if asMap(t, raw)["id"] == str(listing, "id") {
			t.Fatal("guest saw the owner's listing")
		}
	}
	status, staff := c.getAuth("/api/v1/owner/performance?from=2032-06-01&to=2032-06-02", editor)
	if status != 200 {
		t.Fatalf("editor owner view: %d %v", status, staff)
	}
	for _, raw := range asList(t, staff["listings"]) {
		if asMap(t, raw)["id"] == str(listing, "id") {
			t.Fatal("editor saw a unit owned by the client")
		}
	}
}
