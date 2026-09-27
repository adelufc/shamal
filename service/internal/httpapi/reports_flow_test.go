package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

const aridID = "88888888-8888-4888-8888-888888888802"

func TestReportsAndNotifications(t *testing.T) {
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
	manager := c.login("manager@shamal.local", seed.ManagerTOTP)
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)
	email := "ops-" + tag + "@example.com"
	status, registered := c.post("/api/v1/auth/register", map[string]any{"display_name": "متابع", "email": email, "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("register: %d %v", status, registered)
	}
	client := c.loginClient(email, seed.Password)

	status, denied := c.getAuth("/api/v1/reports/visits?from=2032-04-02&to=2032-04-04", client)
	if status != 403 || denied["code"] != "forbidden" {
		t.Fatalf("client report: %d %v", status, denied)
	}
	status, open := c.get("/api/v1/reports/visits")
	if status != 401 {
		t.Fatalf("anonymous report: %d %v", status, open)
	}
	status, exported := c.post("/api/v1/reports/visits/export", map[string]any{}, manager)
	if status != 403 || exported["code"] != "export_unapproved" {
		t.Fatalf("export: %d %v", status, exported)
	}
	status, clientExport := c.post("/api/v1/reports/visits/export", map[string]any{}, client)
	if status != 403 || clientExport["code"] == "export_unapproved" {
		t.Fatalf("client export: %d %v", status, clientExport)
	}

	status, search := c.post("/api/v1/account/searches", map[string]any{
		"name": "نرجس يومي", "neighborhood_id": narjisID, "transaction_type": "daily", "locale": "ar", "channel": "in_app",
	}, client)
	if status != 201 {
		t.Fatalf("search: %d %v", status, search)
	}
	status, mailed := c.post("/api/v1/account/searches", map[string]any{
		"name": "بريد", "neighborhood_id": narjisID, "transaction_type": "daily", "channel": "email",
	}, client)
	if status != 422 || mailed["code"] != "marketing_unapproved" {
		t.Fatalf("email search: %d %v", status, mailed)
	}

	listingID, _ := publishStayListing(t, c, editor, reviewer, "ops-"+tag, "daily", 40000)
	if got := countKind(t, c, client, "listing_match"); got != 1 {
		t.Fatalf("listing matches %d", got)
	}
	notice := kindItem(t, c, client, "listing_match")
	if notice["delivery_status"] != "not_sent" || notice["provider_adopted"] != false || notice["title_ar"] == "" || notice["title_en"] == "" || notice["title_ar"] == notice["title_en"] {
		t.Fatalf("notice %v", notice)
	}
	item, err := api.Cat.GetListing(ctx, listingID)
	if err != nil {
		t.Fatal(err)
	}
	if err := api.Notify.ListingPublished(ctx, item.ID, item.NeighborhoodID, item.TransactionType); err != nil {
		t.Fatal(err)
	}
	if got := countKind(t, c, client, "listing_match"); got != 1 {
		t.Fatalf("duplicate listing match %d", got)
	}
	publishNarjisListing(t, c, editor, reviewer, "ops-sale-"+tag)
	publishDailyAt(t, c, editor, reviewer, aridID, 46.606, 24.906, "arid-"+tag)
	if got := countKind(t, c, client, "listing_match"); got != 1 {
		t.Fatalf("unrelated listing notified %d", got)
	}

	before := map[string]map[string]any{}
	for _, code := range []string{"visits", "deals", "occupancy", "bookings", "payments"} {
		before[code] = fetchReport(t, c, manager, code)
		assertReportShape(t, before[code])
	}

	loc, err := time.LoadLocation("Asia/Riyadh")
	if err != nil {
		t.Fatal(err)
	}
	visit := createAccountVisit(t, c, client, listingID, time.Date(2032, 4, 3, 10, 0, 0, 0, loc), "موعد تقرير", "Report visit")
	if _, err := pool.Exec(ctx, `
		INSERT INTO deals.deals (
			id, company_id, branch_id, profile_id, neighborhood_id, transaction_type, status,
			amount_minor, currency, is_test, version, created_by, created_at
		) VALUES
			(gen_random_uuid(), $1, $2, $3, $4, 'sale', 'draft', 250000, 'SAR', false, 1, $5, $6),
			(gen_random_uuid(), $1, $2, $3, $4, 'sale', 'draft', 999, 'SAR', true, 1, $5, $6)
	`, seed.CompanyID, seed.BranchID, str(visit, "profile_id"), narjisID, seed.EditorID, time.Date(2032, 4, 3, 7, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if got := countKind(t, c, client, "visit_change"); got != 1 {
		t.Fatalf("visit notices %d", got)
	}
	status, muted := c.post("/api/v1/account/notifications/mute", map[string]any{"channel": "in_app", "muted": true}, client)
	if status != 200 || muted["muted"] != true {
		t.Fatalf("mute: %d %v", status, muted)
	}
	status, mailMute := c.post("/api/v1/account/notifications/mute", map[string]any{"channel": "email", "muted": true}, client)
	if status != 422 || mailMute["code"] != "marketing_unapproved" {
		t.Fatalf("email mute: %d %v", status, mailMute)
	}
	postVisit(t, c, editor, visit, "broker-confirm", nil)
	if got := countKind(t, c, client, "visit_change"); got != 1 {
		t.Fatalf("muted visit stayed visible %d", got)
	}
	var suppressed int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notify.events WHERE entity_id = $1 AND kind = 'visit_change' AND suppressed = true`, str(visit, "id")).Scan(&suppressed); err != nil {
		t.Fatal(err)
	}
	if suppressed != 1 {
		t.Fatalf("suppressed visit events %d", suppressed)
	}
	status, unmuted := c.post("/api/v1/account/notifications/mute", map[string]any{"channel": "in_app", "muted": false}, client)
	if status != 200 || unmuted["muted"] != false {
		t.Fatalf("unmute: %d %v", status, unmuted)
	}

	stayID := confirmStay(t, c, client, listingID, "2032-04-03", "2032-04-05", 80000, "ops-"+tag)
	status, attempt := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{"method": "mada", "amount_minor": 80000}, client, "ops-pay-"+tag)
	if status != 201 || attempt["status"] != "pending_review" || attempt["success"] != false {
		t.Fatalf("payment: %d %v", status, attempt)
	}
	if got := countKind(t, c, client, "payment_review"); got != 1 {
		t.Fatalf("payment notices %d", got)
	}

	afterVisits := fetchReport(t, c, manager, "visits")
	afterDeals := fetchReport(t, c, manager, "deals")
	afterOccupancy := fetchReport(t, c, manager, "occupancy")
	afterBookings := fetchReport(t, c, manager, "bookings")
	afterPayments := fetchReport(t, c, manager, "payments")
	assertDelta(t, before["visits"], afterVisits, "count", 1)
	assertDelta(t, before["deals"], afterDeals, "count", 1)
	assertDelta(t, before["deals"], afterDeals, "agreement_minor", 250000)
	assertDelta(t, before["occupancy"], afterOccupancy, "occupied_nights", 2)
	assertDelta(t, before["occupancy"], afterOccupancy, "distinct_units", 1)
	if _, ok := measureValue(t, afterOccupancy, "occupancy_rate"); ok {
		t.Fatal("occupancy rate was calculated")
	}
	assertDelta(t, before["bookings"], afterBookings, "count", 1)
	assertDelta(t, before["bookings"], afterBookings, "stay_due_minor", 80000)
	assertDelta(t, before["payments"], afterPayments, "attempt_count", 1)
	assertDelta(t, before["payments"], afterPayments, "pending_review_count", 1)
	assertDelta(t, before["payments"], afterPayments, "collected_minor", 0)
	attempts, _ := measureValue(t, afterPayments, "attempt_count")
	collected, _ := measureValue(t, afterPayments, "collected_count")
	ratio, ok := measureValue(t, afterPayments, "collected_ratio")
	if attempts == 0 || !ok || math.Abs(ratio-collected/attempts) > 0.000001 {
		t.Fatalf("collected ratio %v ok=%v attempts=%v collected=%v", ratio, ok, attempts, collected)
	}
	for _, code := range []string{"visits", "deals", "occupancy", "bookings", "payments"} {
		assertReportShape(t, fetchReport(t, c, manager, code))
	}

	for i := 0; i < 19; i++ {
		status, item := c.post("/api/v1/account/searches", map[string]any{
			"name": fmt.Sprintf("بحث %d", i), "neighborhood_id": narjisID, "transaction_type": "daily",
		}, client)
		if status != 201 {
			t.Fatalf("search %d: %d %v", i, status, item)
		}
	}
	status, limited := c.post("/api/v1/account/searches", map[string]any{
		"name": "بحث زائد", "neighborhood_id": narjisID, "transaction_type": "daily",
	}, client)
	if status != 422 || limited["code"] != "search_limit" {
		t.Fatalf("search limit: %d %v", status, limited)
	}
}

func fetchReport(t *testing.T, c *apiClient, cookie, code string) map[string]any {
	t.Helper()
	status, body := c.getAuth("/api/v1/reports/"+code+"?from=2032-04-02&to=2032-04-04", cookie)
	if status != 200 {
		t.Fatalf("report %s: %d %v", code, status, body)
	}
	return body
}

func assertReportShape(t *testing.T, report map[string]any) {
	t.Helper()
	if report["generated_at"] == "" || report["timezone"] != "Asia/Riyadh" || report["definition_ar"] == "" || report["definition_en"] == "" {
		t.Fatalf("report shape %v", report)
	}
	if report["definition_ar"] == report["definition_en"] || report["commission_included"] != false {
		t.Fatalf("definitions or commission %v", report)
	}
	if report["code"] == "payments" && report["reconciliation"] != "not_run" {
		t.Fatalf("payment reconciliation %v", report["reconciliation"])
	}
	if report["code"] != "payments" && report["reconciliation"] != "not_applicable" {
		t.Fatalf("reconciliation %v", report["reconciliation"])
	}
	for _, raw := range asList(t, report["measures"]) {
		item := asMap(t, raw)
		code := str(item, "code")
		if strings.Contains(code, "commission") || item["definition_ar"] == "" || item["definition_en"] == "" {
			t.Fatalf("measure %v", item)
		}
	}
}

func assertDelta(t *testing.T, before, after map[string]any, code string, delta float64) {
	t.Helper()
	left, okLeft := measureValue(t, before, code)
	right, okRight := measureValue(t, after, code)
	if !okLeft || !okRight || math.Abs(right-(left+delta)) > 0.001 {
		t.Fatalf("%s before=%v after=%v delta=%v", code, left, right, delta)
	}
}

func measureValue(t *testing.T, report map[string]any, code string) (float64, bool) {
	t.Helper()
	for _, raw := range asList(t, report["measures"]) {
		item := asMap(t, raw)
		if item["code"] != code {
			continue
		}
		if item["value"] == nil {
			return 0, false
		}
		return item["value"].(float64), true
	}
	t.Fatalf("missing measure %s", code)
	return 0, false
}

func countKind(t *testing.T, c *apiClient, cookie, kind string) int {
	t.Helper()
	status, body := c.getAuth("/api/v1/account/notifications", cookie)
	if status != 200 {
		t.Fatalf("inbox: %d %v", status, body)
	}
	count := 0
	for _, raw := range asList(t, body["items"]) {
		if asMap(t, raw)["kind"] == kind {
			count++
		}
	}
	return count
}

func kindItem(t *testing.T, c *apiClient, cookie, kind string) map[string]any {
	t.Helper()
	status, body := c.getAuth("/api/v1/account/notifications", cookie)
	if status != 200 {
		t.Fatalf("inbox: %d %v", status, body)
	}
	for _, raw := range asList(t, body["items"]) {
		item := asMap(t, raw)
		if item["kind"] == kind {
			return item
		}
	}
	t.Fatalf("missing %s", kind)
	return nil
}

func publishDailyAt(t *testing.T, c *apiClient, editor, reviewer, neighborhood string, lon, lat float64, tag string) {
	t.Helper()
	property := c.mustProperty(editor, neighborhood, "land", 160, floatPtr(lon), floatPtr(lat))
	unit := c.mustUnit(editor, str(property, "id"), "Daily-"+tag)
	status, item := c.post("/api/v1/catalog/listings", map[string]any{
		"property_id": str(property, "id"), "unit_id": str(unit, "id"), "neighborhood_id": neighborhood,
		"transaction_type": "daily", "price_minor": 40000, "currency": "SAR", "license_ref": "local-test-ref",
		"broker_user_id": seed.ReviewerID,
		"texts": []map[string]string{
			{"locale": "ar", "title": "إقامة " + tag, "description": "وصف عربي مستقل."},
			{"locale": "en", "title": "Stay " + tag, "description": "An independent English description."},
		},
	}, editor)
	if status != 201 {
		t.Fatalf("listing: %d %v", status, item)
	}
	for _, step := range []struct{ actor, path string }{{editor, "/submit"}, {reviewer, "/approve"}, {reviewer, "/publish"}} {
		status, item = c.post("/api/v1/catalog/listings/"+str(item, "id")+step.path, map[string]any{"version": num(item, "version")}, step.actor)
		if status != 200 {
			t.Fatalf("%s: %d %v", step.path, status, item)
		}
	}
}
