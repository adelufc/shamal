package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
	"shamal/service/internal/stays"
)

func TestStayBooking(t *testing.T) {
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
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)
	listingID, unitID := publishStayListing(t, c, editor, reviewer, tag, "daily", 40000)
	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stays.bookings`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	status, quote := c.get("/api/v1/public/stays/quote?listing_id=" + listingID + "&arrival=2027-11-10&departure=2027-11-13&guests=1")
	if status != 200 || num(quote, "stay_due_minor") != 120000 || num(quote, "discount_minor") != 0 || num(quote, "collect_minor") != 120000 {
		t.Fatalf("quote: %d %v", status, quote)
	}
	if quote["payment_status"] != "not_collected" || quote["commission_adopted"] != false || quote["deposit_is_revenue"] != false || quote["non_contractual"] != true {
		t.Fatalf("quote invented a commercial term: %v", quote)
	}
	if quote["available"] != true || num(quote, "nights") != 3 || quote["arrival"] != "2027-11-10" || quote["departure"] != "2027-11-13" {
		t.Fatalf("quote dates: %v", quote)
	}
	policy := asMap(t, quote["policy"])
	if policy["contractual"] != false || !containsText(str(policy, "summary_ar"), "غير تعاقدية") || !containsText(str(policy, "summary_en"), "non-contractual") {
		t.Fatalf("policy: %v", policy)
	}
	var after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stays.bookings`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("opening the quote created a stay")
	}
	status, bad := c.get("/api/v1/public/stays/quote?listing_id=" + listingID + "&arrival=2027-11-10&departure=2027-11-10")
	if status != 422 || !hasField(bad, "min_stay") {
		t.Fatalf("one night minimum: %d %v", status, bad)
	}

	email := fmt.Sprintf("stay-%s@example.com", tag)
	status, registered := c.post("/api/v1/auth/register", map[string]any{
		"display_name": "ضيف الإقامة", "email": email, "password": seed.Password,
	}, "")
	if status != 201 {
		t.Fatalf("register: %d %v", status, registered)
	}
	client := c.loginClient(email, seed.Password)
	otherEmail := fmt.Sprintf("stay-b-%s@example.com", tag)
	status, _ = c.post("/api/v1/auth/register", map[string]any{
		"display_name": "ضيف آخر", "email": otherEmail, "password": seed.Password,
	}, "")
	if status != 201 {
		t.Fatalf("second register: %d", status)
	}
	other := c.loginClient(otherEmail, seed.Password)

	holdBody := map[string]any{
		"listing_id": listingID, "arrival": "2027-11-10", "departure": "2027-11-13", "guests": 1,
		"quoted_stay_due_minor": 120000, "policy_version": num(policy, "version"), "accept_policy": true,
	}
	status, priced := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2027-11-10", "departure": "2027-11-13", "guests": 1,
		"quoted_stay_due_minor": 1, "policy_version": 1, "accept_policy": true, "commission_bps": 250,
	}, client, "fee-"+tag)
	if status != 422 || priced["code"] != "commercial_terms_unapproved" {
		t.Fatalf("commission: %d %v", status, priced)
	}
	status, discounted := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2027-11-10", "departure": "2027-11-13", "guests": 1,
		"quoted_stay_due_minor": 108000, "policy_version": 1, "accept_policy": true, "discount_percent": 10,
	}, client, "discount-"+tag)
	if status != 422 || discounted["code"] != "discount_unapproved" {
		t.Fatalf("discount: %d %v", status, discounted)
	}
	status, held := c.postKey("/api/v1/account/stays", holdBody, client, "hold-"+tag)
	if status != 201 || held["status"] != "held" || num(held, "stay_due_minor") != 120000 || held["payment_status"] != "not_collected" {
		t.Fatalf("hold: %d %v", status, held)
	}
	status, again := c.postKey("/api/v1/account/stays", holdBody, client, "hold-"+tag)
	if status != 201 || again["id"] != held["id"] {
		t.Fatalf("idempotent hold: %d %v", status, again)
	}
	changed := map[string]any{}
	for key, value := range holdBody {
		changed[key] = value
	}
	changed["guests"] = 2
	status, mismatch := c.postKey("/api/v1/account/stays", changed, client, "hold-"+tag)
	if status != 409 || mismatch["code"] != "idempotency_mismatch" {
		t.Fatalf("idempotency: %d %v", status, mismatch)
	}
	if _, err := pool.Exec(ctx, `UPDATE catalog.listings SET price_minor = 99999 WHERE id = $1`, listingID); err != nil {
		t.Fatal(err)
	}
	status, confirmed := c.postKey("/api/v1/account/stays/"+str(held, "id")+"/confirm", map[string]any{}, client, "confirm-"+tag)
	if status != 200 || confirmed["status"] != "confirmed" || num(confirmed, "stay_due_minor") != 120000 {
		t.Fatalf("confirm kept the snapshot: %d %v", status, confirmed)
	}
	if confirmed["precise_lon"] != nil || confirmed["commission_adopted"] != false {
		t.Fatalf("confirm payload: %v", confirmed)
	}
	if _, err := pool.Exec(ctx, `UPDATE catalog.listings SET price_minor = 40000 WHERE id = $1`, listingID); err != nil {
		t.Fatal(err)
	}
	status, overlap := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2027-11-12", "departure": "2027-11-14", "guests": 1,
		"quoted_stay_due_minor": 80000, "policy_version": 1, "accept_policy": true,
	}, other, "overlap-"+tag)
	if status != 409 || overlap["code"] != "stay_conflict" {
		t.Fatalf("overlap: %d %v", status, overlap)
	}
	status, adjacent := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2027-11-13", "departure": "2027-11-15", "guests": 1,
		"quoted_stay_due_minor": 80000, "policy_version": 1, "accept_policy": true,
	}, other, "adjacent-"+tag)
	if status != 201 {
		t.Fatalf("adjacent hold: %d %v", status, adjacent)
	}
	status, adjacent = c.postKey("/api/v1/account/stays/"+str(adjacent, "id")+"/confirm", map[string]any{}, other, "adjacent-confirm-"+tag)
	if status != 200 || adjacent["status"] != "confirmed" {
		t.Fatalf("adjacent confirm: %d %v", status, adjacent)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO stays.occupancy (id, unit_id, kind, start_on, end_on)
		VALUES (gen_random_uuid(), $1, 'closure', '2027-11-11', '2027-11-12')
	`, unitID)
	var pgErr *pgconn.PgError
	if err == nil || !errors.As(err, &pgErr) || pgErr.Code != "23P01" {
		t.Fatalf("closure covered a confirmed stay: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO stays.occupancy (id, unit_id, kind, start_on, end_on)
		VALUES (gen_random_uuid(), $1, 'closure', '2028-08-01', '2028-08-04')
	`, unitID); err != nil {
		t.Fatal(err)
	}
	status, blocked := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2028-08-01", "departure": "2028-08-03", "guests": 1,
		"quoted_stay_due_minor": 80000, "policy_version": 1, "accept_policy": true,
	}, client, "closed-"+tag)
	if status != 409 || blocked["code"] != "stay_conflict" {
		t.Fatalf("closure did not block a new hold: %d %v", status, blocked)
	}

	monthlyID, _ := publishStayListing(t, c, editor, reviewer, tag+"m", "monthly", 250000)
	status, month := c.get("/api/v1/public/stays/quote?listing_id=" + monthlyID + "&arrival=2027-01-31&departure=2027-02-28&guests=1")
	if status != 200 || num(month, "months") != 1 || num(month, "stay_due_minor") != 250000 {
		t.Fatalf("calendar month: %d %v", status, month)
	}
	status, partial := c.get("/api/v1/public/stays/quote?listing_id=" + monthlyID + "&arrival=2027-03-01&departure=2027-03-31&guests=1")
	if status != 422 || !hasField(partial, "monthly_partial") {
		t.Fatalf("partial month: %d %v", status, partial)
	}
	saleID, _ := publishStayListing(t, c, editor, reviewer, tag+"s", "sale", 500000)
	status, sale := c.get("/api/v1/public/stays/quote?listing_id=" + saleID + "&arrival=2027-11-10&departure=2027-11-12")
	if status != 422 || sale["code"] != "invalid_transaction" {
		t.Fatalf("sale stay: %d %v", status, sale)
	}

	status, expiring := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2028-05-01", "departure": "2028-05-03", "guests": 1,
		"quoted_stay_due_minor": 80000, "policy_version": 1, "accept_policy": true,
	}, client, "expire-"+tag)
	if status != 201 {
		t.Fatalf("expiring hold: %d %v", status, expiring)
	}
	if _, err := pool.Exec(ctx, `UPDATE stays.bookings SET hold_expires_at = now() - interval '1 minute' WHERE id = $1`, str(expiring, "id")); err != nil {
		t.Fatal(err)
	}
	status, expired := c.postKey("/api/v1/account/stays/"+str(expiring, "id")+"/confirm", map[string]any{}, client, "expire-confirm-"+tag)
	if status != 409 || expired["code"] != "hold_expired" {
		t.Fatalf("expired confirm: %d %v", status, expired)
	}
	status, released := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": "2028-05-01", "departure": "2028-05-03", "guests": 1,
		"quoted_stay_due_minor": 80000, "policy_version": 1, "accept_policy": true,
	}, other, "released-"+tag)
	if status != 201 {
		t.Fatalf("released hold: %d %v", status, released)
	}

	if _, err := pool.Exec(ctx, `UPDATE catalog.units SET capacity = 1 WHERE id = $1`, unitID); err != nil {
		t.Fatal(err)
	}
	status, crowded := c.get("/api/v1/public/stays/quote?listing_id=" + listingID + "&arrival=2028-06-01&departure=2028-06-03&guests=2")
	if status != 422 || !hasField(crowded, "over_capacity") {
		t.Fatalf("capacity: %d %v", status, crowded)
	}

	raceListing(t, api.Stays, listingID, unitID, str(registered, "id"))
}

func raceListing(t *testing.T, svc stays.Service, listingID, unitID, actor string) {
	t.Helper()
	var wins atomic.Int32
	var conflicts atomic.Int32
	var other atomic.Int32
	var wg sync.WaitGroup
	ready := make(chan struct{})
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-ready
			_, err := svc.Hold(context.Background(), stays.HoldRequest{
				ActorID: actor, ProfileID: actor, IdempotencyKey: fmt.Sprintf("race-%s-%d", listingID, i),
				Listing: stays.Listing{
					ID: listingID, UnitID: unitID, CompanyID: seed.CompanyID, Transaction: "daily", Currency: "SAR", PriceMinor: 40000,
				},
				Arrival: mustDay("2028-04-01"), Departure: mustDay("2028-04-03"), Guests: 1,
				QuotedDue: 80000, PolicyVersion: 1, AcceptPolicy: true,
			})
			if err == nil {
				wins.Add(1)
				return
			}
			if appErr := platform.As(err); appErr != nil && appErr.Code == "stay_conflict" {
				conflicts.Add(1)
				return
			}
			other.Add(1)
		}(i)
	}
	close(ready)
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != 99 || other.Load() != 0 {
		t.Fatalf("concurrent holds wins=%d conflicts=%d other=%d", wins.Load(), conflicts.Load(), other.Load())
	}
}

func publishStayListing(t *testing.T, c *apiClient, editor, reviewer, tag, transaction string, price int64) (string, string) {
	t.Helper()
	property := c.mustProperty(editor, narjisID, "land", 180, floatPtr(46.646), floatPtr(24.886))
	unit := c.mustUnit(editor, str(property, "id"), "Stay-"+tag)
	status, item := c.post("/api/v1/catalog/listings", map[string]any{
		"property_id": str(property, "id"), "unit_id": str(unit, "id"), "neighborhood_id": narjisID,
		"transaction_type": transaction, "price_minor": price, "currency": "SAR", "license_ref": "local-test-ref",
		"broker_user_id": seed.ReviewerID,
		"texts": []map[string]string{
			{"locale": "ar", "title": "إقامة " + tag, "description": "وصف عربي مستقل."},
			{"locale": "en", "title": "Stay " + tag, "description": "An independent English description."},
		},
	}, editor)
	if status != 201 {
		t.Fatalf("listing: %d %v", status, item)
	}
	for _, step := range []struct {
		actor, path string
	}{
		{editor, "/submit"},
		{reviewer, "/approve"},
		{reviewer, "/publish"},
	} {
		status, item = c.post("/api/v1/catalog/listings/"+str(item, "id")+step.path, map[string]any{"version": num(item, "version")}, step.actor)
		if status != 200 {
			t.Fatalf("%s: %d %v", step.path, status, item)
		}
	}
	return str(item, "id"), str(unit, "id")
}

func (c *apiClient) postKey(path string, body any, cookie, key string) (int, map[string]any) {
	c.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		c.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("X-Shamal-Request", "1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "shamal_session", Value: cookie})
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	payload := map[string]any{}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		c.t.Fatal(err)
	}
	return res.StatusCode, payload
}

func mustDay(raw string) time.Time {
	value, _ := stays.ParseDate(raw)
	return value
}

func containsText(value, part string) bool {
	return bytes.Contains([]byte(value), []byte(part))
}
