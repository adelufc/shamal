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

func TestClientFavorites(t *testing.T) {
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
	listingID, _ := publishNarjisListing(t, c, editor, reviewer, "fav"+tag)
	email := "fav-" + tag + "@example.com"
	status, registered := c.post("/api/v1/auth/register", map[string]any{"display_name": "صاحب مفضلة", "email": email, "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("register: %d %v", status, registered)
	}
	client := c.loginClient(email, seed.Password)
	other := c.loginClient(registerOther(t, c, "other-"+tag+"@example.com"), seed.Password)

	status, denied := c.post("/api/v1/account/favorites", map[string]any{"listing_id": listingID}, editor)
	if status != 403 {
		t.Fatalf("staff favorite: %d %v", status, denied)
	}
	property := c.mustProperty(editor, narjisID, "land", 190, floatPtr(46.642), floatPtr(24.882))
	unit := c.mustUnit(editor, str(property, "id"), "F-"+tag)
	unpublished := c.mustListing(editor, str(property, "id"), str(unit, "id"), narjisID, 80, "Unpublished "+tag)
	status, blocked := c.post("/api/v1/account/favorites?locale=ar", map[string]any{"listing_id": str(unpublished, "id")}, client)
	if status != 422 || blocked["code"] != "listing_not_published" {
		t.Fatalf("unpublished favorite: %d %v", status, blocked)
	}
	status, saved := c.post("/api/v1/account/favorites?locale=en", map[string]any{"listing_id": listingID}, client)
	if status != 201 || saved["title"] != "Visit land fav"+tag || saved["title"] == "أرض اختبار" {
		t.Fatalf("save english title: %d %v", status, saved)
	}
	status, again := c.post("/api/v1/account/favorites?locale=ar", map[string]any{"listing_id": listingID}, client)
	if status != 200 || again["title"] != "أرض اختبار" {
		t.Fatalf("repeat save: %d %v", status, again)
	}
	status, list := c.getAuth("/api/v1/account/favorites?locale=ar", client)
	if status != 200 || len(asList(t, list["items"])) != 1 {
		t.Fatalf("list: %d %v", status, list)
	}
	status, hidden := c.getAuth("/api/v1/account/favorites", other)
	if status != 200 || len(asList(t, hidden["items"])) != 0 {
		t.Fatalf("other client saw the favorite: %d %v", status, hidden)
	}
	status, searches := c.getAuth("/api/v1/account/searches", client)
	if status != 200 || len(asList(t, searches["items"])) != 0 {
		t.Fatalf("favorite created a saved search: %d %v", status, searches)
	}
	status, removed := c.send("DELETE", "/api/v1/account/favorites/"+listingID, nil, client)
	if status != 200 || removed["removed"] != true {
		t.Fatalf("remove: %d %v", status, removed)
	}
	status, empty := c.getAuth("/api/v1/account/favorites", client)
	if status != 200 || len(asList(t, empty["items"])) != 0 {
		t.Fatalf("after remove: %d %v", status, empty)
	}
	status, missing := c.send("DELETE", "/api/v1/account/favorites/"+listingID, nil, client)
	if status != 404 {
		t.Fatalf("second remove: %d %v", status, missing)
	}
}

func registerOther(t *testing.T, c *apiClient, email string) string {
	t.Helper()
	status, body := c.post("/api/v1/auth/register", map[string]any{"display_name": "عميل آخر", "email": email, "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("other register: %d %v", status, body)
	}
	return email
}
