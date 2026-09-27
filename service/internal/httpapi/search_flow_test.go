package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func TestPublicSearchAndMap(t *testing.T) {
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
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)

	cheap := publishTaggedListing(t, c, editor, reviewer, tag, "sale", 15000, 400, 46.650, 24.890, "أرض البحث "+tag, "Search "+tag+" parcel")
	costly := publishTaggedListing(t, c, editor, reviewer, tag+"b", "annual", 900000, 900, 46.648, 24.888, "إيجار البحث "+tag, "Search "+tag+" lease")
	hiddenProp := c.mustProperty(editor, narjisID, "land", 200, floatPtr(46.646), floatPtr(24.886))
	hiddenUnit := c.mustUnit(editor, str(hiddenProp, "id"), "H-"+tag)
	_ = c.mustListing(editor, str(hiddenProp, "id"), str(hiddenUnit, "id"), narjisID, 100, "Hidden "+tag)

	status, hidden := c.get("/api/v1/public/search?locale=en&q=Hidden%20" + tag)
	if status != 200 || num(hidden, "total") != 0 {
		t.Fatalf("draft listing was public: %d %v", status, hidden)
	}

	status, mixed := c.get("/api/v1/public/search?locale=en&price_min_minor=1000")
	if status != 422 || !hasField(mixed, "price_needs_transaction") {
		t.Fatalf("mixed price: %d %v", status, mixed)
	}

	status, found := c.get("/api/v1/public/search?locale=en&transaction=sale&price_min_minor=10000&price_max_minor=20000&q=Search%20" + tag + "%20parcel")
	if status != 200 || num(found, "total") != 1 {
		t.Fatalf("priced sale: %d %v", status, found)
	}
	listings := asList(t, found["listings"])
	markers := asList(t, found["markers"])
	if len(listings) != 1 || len(markers) != 1 {
		t.Fatalf("list and map diverged: %d listings %d markers", len(listings), len(markers))
	}
	listing := asMap(t, listings[0])
	marker := asMap(t, markers[0])
	if listing["id"] != cheap || marker["id"] != cheap || listing["title"] != "Search "+tag+" parcel" {
		t.Fatalf("same hit: listing %v marker %v", listing, marker)
	}
	if listing["precise_lon"] != nil || marker["precise_lon"] != nil {
		t.Fatalf("precise point leaked: %v %v", listing, marker)
	}
	lon, _ := marker["lon"].(float64)
	if math.Abs(lon-46.645) > 0.003 || math.Abs(lon-46.650) < 0.001 {
		t.Fatalf("marker used the property point: %v", lon)
	}
	clusters := asList(t, found["clusters"])
	sum := 0
	for _, raw := range clusters {
		sum += num(asMap(t, raw), "count")
	}
	if sum != num(found, "total") {
		t.Fatalf("clusters %d total %v", sum, found["total"])
	}

	status, ordered := c.get("/api/v1/public/search?locale=en&q=Search%20" + tag + "&sort=price_asc&page_size=1")
	if status != 200 || num(ordered, "total") != 2 || len(asList(t, ordered["listings"])) != 1 || len(asList(t, ordered["markers"])) != 2 {
		t.Fatalf("paged list and full map: %d %v", status, ordered)
	}
	if asMap(t, asList(t, ordered["listings"])[0])["id"] != cheap {
		t.Fatalf("price order: %v", ordered["listings"])
	}
	if asMap(t, asList(t, ordered["markers"])[0])["id"] != cheap || asMap(t, asList(t, ordered["markers"])[1])["id"] != costly {
		t.Fatalf("marker order: %v", ordered["markers"])
	}

	status, arabic := c.get("/api/v1/public/search?locale=ar&q=Search%20" + tag + "%20parcel")
	if status != 200 || num(arabic, "total") != 0 {
		t.Fatalf("arabic search used the english title: %d %v", status, arabic)
	}
	status, englishName := c.get("/api/v1/public/search?locale=ar&q=Al%20Narjis&transaction=sale&price_min_minor=10000&price_max_minor=20000")
	if status != 200 || num(englishName, "total") != 0 {
		t.Fatalf("arabic search used the english neighborhood name: %d %v", status, englishName)
	}
	status, alias := c.get("/api/v1/public/search?locale=ar&q=" + "حي%20النرجس" + "&transaction=sale&price_min_minor=10000&price_max_minor=20000")
	if status != 200 || !idsContain(asList(t, alias["listings"]), cheap) || !idsContain(asList(t, alias["markers"]), cheap) {
		t.Fatalf("arabic alias: %d %v", status, alias)
	}
	status, narjis := c.get("/api/v1/public/search?locale=en&q=Al%20Narjis&transaction=annual&price_min_minor=800000&price_max_minor=1000000")
	if status != 200 || !idsContain(asList(t, narjis["listings"]), costly) {
		t.Fatalf("english neighborhood: %d %v", status, narjis)
	}

	status, ignored := c.get("/api/v1/public/search?locale=en&transaction=sale&price_min_minor=10000&price_max_minor=20000&q=Search%20" + tag + "%20parcel&west=0&south=0&east=1&north=1")
	if status != 200 || num(ignored, "total") != 1 || asMap(t, ignored["query"])["in_area"] != false {
		t.Fatalf("map box changed the search: %d %v", status, ignored)
	}
	status, area := c.get("/api/v1/public/search?locale=en&transaction=sale&price_min_minor=10000&price_max_minor=20000&q=Search%20" + tag + "%20parcel&in_area=1&west=0&south=0&east=1&north=1")
	if status != 200 || num(area, "total") != 0 || len(asList(t, area["markers"])) != 0 || asMap(t, area["query"])["in_area"] != true {
		t.Fatalf("area search: %d %v", status, area)
	}
	echo := asMap(t, area["query"])
	if echo["transaction"] != "sale" || echo["q"] != "Search "+tag+" parcel" {
		t.Fatalf("empty result hid the filters: %v", echo)
	}
}

func publishTaggedListing(t *testing.T, c *apiClient, editor, reviewer, tag, transaction string, price int64, area, lon, lat float64, titleAR, titleEN string) string {
	t.Helper()
	property := c.mustProperty(editor, narjisID, "land", area, floatPtr(lon), floatPtr(lat))
	unit := c.mustUnit(editor, str(property, "id"), "S-"+tag)
	status, item := c.post("/api/v1/catalog/listings", map[string]any{
		"property_id": str(property, "id"), "unit_id": str(unit, "id"), "neighborhood_id": narjisID,
		"transaction_type": transaction, "price_minor": price, "currency": "SAR", "license_ref": "local-test-ref",
		"broker_user_id": seed.ReviewerID,
		"texts": []map[string]string{
			{"locale": "ar", "title": titleAR, "description": "وصف عربي مستقل."},
			{"locale": "en", "title": titleEN, "description": "An independent English description."},
		},
	}, editor)
	if status != 201 {
		t.Fatalf("listing: %d %v", status, item)
	}
	status, item = c.post("/api/v1/catalog/listings/"+str(item, "id")+"/submit", map[string]any{"version": num(item, "version")}, editor)
	if status != 200 {
		t.Fatalf("submit: %d %v", status, item)
	}
	status, item = c.post("/api/v1/catalog/listings/"+str(item, "id")+"/approve", map[string]any{"version": num(item, "version")}, reviewer)
	if status != 200 {
		t.Fatalf("approve: %d %v", status, item)
	}
	status, item = c.post("/api/v1/catalog/listings/"+str(item, "id")+"/publish", map[string]any{"version": num(item, "version")}, reviewer)
	if status != 200 || item["status"] != "published" {
		t.Fatalf("publish: %d %v", status, item)
	}
	return str(item, "id")
}

func idsContain(items []any, id string) bool {
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item["id"] == id {
			return true
		}
	}
	return false
}
