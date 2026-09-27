package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func TestFoundationFlow(t *testing.T) {
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

	status, home := c.get("/api/v1/public/home?locale=en")
	if status != 200 {
		t.Fatalf("home %d", status)
	}
	if home["locale"] != "en" {
		t.Fatalf("locale %v", home["locale"])
	}
	cards := asList(t, home["neighborhoods"])
	if len(cards) < 7 {
		t.Fatalf("expected the seeded neighborhoods, got %d", len(cards))
	}
	seenNarjis := false
	for _, raw := range cards {
		card := asMap(t, raw)
		if card["name"] == "النرجس" {
			t.Fatal("english home used the arabic neighborhood name")
		}
		if card["name"] == "Al Narjis" {
			seenNarjis = true
			if card["boundary_is_official"] != false {
				t.Fatal("seeded boundary was marked official")
			}
		}
	}
	if !seenNarjis {
		t.Fatal("english home is missing Al Narjis")
	}

	status, denied := c.post("/api/v1/auth/login", map[string]any{"email": "editor@shamal.local", "password": "not-the-password", "context": "staff"}, "")
	if status != 401 || denied["code"] != "invalid_credentials" {
		t.Fatalf("bad password: %d %v", status, denied)
	}
	status, missing := c.post("/api/v1/auth/login", map[string]any{"email": "nobody@shamal.local", "password": "not-the-password", "context": "staff"}, "")
	if status != 401 || missing["code"] != "invalid_credentials" || missing["message"] != denied["message"] {
		t.Fatalf("unknown account was distinguishable: %v vs %v", missing, denied)
	}

	editor := c.login("editor@shamal.local", seed.EditorTOTP)
	reviewer := c.login("reviewer@shamal.local", seed.ReviewerTOTP)
	manager := c.login("manager@shamal.local", seed.ManagerTOTP)

	stamp := time.Now().UnixNano()
	tag := fmt.Sprintf("%d", stamp%1000000)
	lon, lat := 46.82, 24.38

	status, official := c.post("/api/v1/geo/neighborhoods", neighborhoodBody("حي رسمي "+tag, "Official Claim "+tag, box(lon, lat, 0.008), true, true), editor)
	if status != 422 || official["code"] != "official_boundary_unapproved" {
		t.Fatalf("official claim: %d %v", status, official)
	}

	status, broken := c.post("/api/v1/geo/neighborhoods", neighborhoodBody("حي مكسور "+tag, "Broken Ring "+tag, `{"type":"Polygon","coordinates":[[[0,0],[1,1],[0,1],[1,0],[0,0]]]}`, false, true), editor)
	if status != 201 || broken["status"] != "draft" || !hasIssue(broken, "invalid_boundary") {
		t.Fatalf("invalid polygon was not kept as a draft: %d %v", status, broken)
	}
	status, submitBroken := c.post("/api/v1/geo/neighborhoods/"+str(broken, "id")+"/submit", map[string]any{"version": num(broken, "version")}, editor)
	if status != 422 || submitBroken["code"] != "validation" || !hasField(submitBroken, "invalid_boundary") {
		t.Fatalf("invalid draft was submitted: %d %v", status, submitBroken)
	}

	dupBody := neighborhoodBody("النرجس", "Duplicate Probe "+tag, box(lon+0.03, lat, 0.008), false, true)
	dupBody["slug_ar"] = "probe-ar-" + tag
	dupBody["slug_en"] = "probe-en-" + tag
	status, dup := c.post("/api/v1/geo/neighborhoods", dupBody, editor)
	if status != 201 || !hasIssue(dup, "duplicate_neighborhood") {
		t.Fatalf("duplicate name: %d %v", status, dup)
	}

	var created map[string]any
	var trialAR, trialEN string
	for attempt := 0; attempt < 16; attempt++ {
		n := stamp + int64(attempt*17)
		lon = 46.82 + float64(int(n%8))*0.055
		lat = 24.38 + float64(int((n/8)%10))*0.028
		trialAR = fmt.Sprintf("حي التجربة %s-%d", tag, attempt)
		trialEN = fmt.Sprintf("Trial Quarter %s-%d", tag, attempt)
		status, created = c.post("/api/v1/geo/neighborhoods", neighborhoodBody(trialAR, trialEN, box(lon, lat, 0.008), false, true), editor)
		if status == 201 && !hasIssue(created, "boundary_overlap") && !hasIssue(created, "outside_city") && !hasIssue(created, "invalid_boundary") {
			break
		}
		created = nil
	}
	if created == nil {
		t.Fatal("no free cell inside the Riyadh test mask")
	}
	status, conflict := c.send("PUT", "/api/v1/geo/neighborhoods/"+str(created, "id"), map[string]any{
		"name_ar": trialAR, "name_en": trialEN,
		"geojson": box(lon, lat, 0.008), "service_sale": true, "coverage": "served", "version": 99,
	}, editor)
	if status != 409 || conflict["code"] != "version_conflict" {
		t.Fatalf("version conflict: %d %v", status, conflict)
	}

	status, submitted := c.post("/api/v1/geo/neighborhoods/"+str(created, "id")+"/submit", map[string]any{"version": num(created, "version")}, editor)
	if status != 200 || submitted["status"] != "in_review" {
		t.Fatalf("submit: %d %v", status, submitted)
	}
	status, own := c.post("/api/v1/geo/neighborhoods/"+str(created, "id")+"/publish", map[string]any{"version": num(submitted, "version")}, editor)
	if status != 403 || !hasField(own, "reason") || field(own, "reason") == "" {
		t.Fatalf("author publish: %d %v", status, own)
	}
	if field(own, "reason") != "The person who created this record cannot approve it." {
		t.Fatalf("expected separation of duties, got %v", own)
	}
	status, published := c.post("/api/v1/geo/neighborhoods/"+str(created, "id")+"/publish", map[string]any{"version": num(submitted, "version")}, reviewer)
	if status != 200 || published["status"] != "published" || published["sync_status"] != "recorded" {
		t.Fatalf("publish: %d %v", status, published)
	}

	var nosale map[string]any
	var saleLon, saleLat float64
	for attempt := 0; attempt < 16; attempt++ {
		n := stamp + 5000 + int64(attempt*19)
		saleLon = 46.845 + float64(int(n%8))*0.055
		saleLat = 24.392 + float64(int((n/8)%10))*0.028
		if saleLon > lon-0.02 && saleLon < lon+0.02 && saleLat > lat-0.02 && saleLat < lat+0.02 {
			continue
		}
		status, nosale = c.post("/api/v1/geo/neighborhoods", neighborhoodBody(fmt.Sprintf("حي بلا بيع %s-%d", tag, attempt), fmt.Sprintf("No Sale %s-%d", tag, attempt), box(saleLon, saleLat, 0.008), false, false), editor)
		if status == 201 && !hasIssue(nosale, "boundary_overlap") && !hasIssue(nosale, "outside_city") {
			break
		}
		nosale = nil
	}
	if nosale == nil {
		t.Fatal("no free cell for the sale-disabled neighborhood")
	}
	status, nosale = c.post("/api/v1/geo/neighborhoods/"+str(nosale, "id")+"/submit", map[string]any{"version": num(nosale, "version")}, editor)
	if status != 200 {
		t.Fatalf("nosale submit: %d %v", status, nosale)
	}
	status, nosale = c.post("/api/v1/geo/neighborhoods/"+str(nosale, "id")+"/publish", map[string]any{"version": num(nosale, "version")}, reviewer)
	if status != 200 || nosale["status"] != "published" {
		t.Fatalf("nosale publish: %d %v", status, nosale)
	}

	status, after := c.get("/api/v1/public/home?locale=ar")
	if status != 200 || !namesContain(asList(t, after["neighborhoods"]), trialAR) {
		t.Fatalf("public home missed the published neighborhood: %v", after["counts"])
	}

	draftID := str(broken, "id")
	property := c.mustProperty(editor, draftID, "land", 400, nil, nil)
	unit := c.mustUnit(editor, str(property, "id"), "A-"+tag)
	listing := c.mustListing(editor, str(property, "id"), str(unit, "id"), draftID, 100, "Blocked "+tag)
	status, blocked := c.post("/api/v1/catalog/listings/"+str(listing, "id")+"/submit", map[string]any{"version": num(listing, "version")}, editor)
	if status != 422 || !hasField(blocked, "neighborhood_unpublished") {
		t.Fatalf("unpublished neighborhood accepted a listing: %d %v", status, blocked)
	}

	outsideProp := c.mustProperty(editor, str(created, "id"), "land", 500, floatPtr(40), floatPtr(20))
	outsideUnit := c.mustUnit(editor, str(outsideProp, "id"), "B-"+tag)
	outside := c.mustListing(editor, str(outsideProp, "id"), str(outsideUnit, "id"), str(created, "id"), 100, "Outside "+tag)
	status, outsideRes := c.post("/api/v1/catalog/listings/"+str(outside, "id")+"/submit", map[string]any{"version": num(outside, "version")}, editor)
	if status != 422 || !hasField(outsideRes, "outside_neighborhood") {
		t.Fatalf("point outside neighborhood: %d %v", status, outsideRes)
	}

	disabledProp := c.mustProperty(editor, str(nosale, "id"), "land", 300, floatPtr(saleLon+0.004), floatPtr(saleLat+0.004))
	disabledUnit := c.mustUnit(editor, str(disabledProp, "id"), "C-"+tag)
	disabled := c.mustListing(editor, str(disabledProp, "id"), str(disabledUnit, "id"), str(nosale, "id"), 100, "No sale "+tag)
	status, disabledRes := c.post("/api/v1/catalog/listings/"+str(disabled, "id")+"/submit", map[string]any{"version": num(disabled, "version")}, editor)
	if status != 422 || !hasField(disabledRes, "service_disabled") {
		t.Fatalf("disabled service: %d %v", status, disabledRes)
	}

	liveProp := c.mustProperty(editor, str(created, "id"), "land", 640, floatPtr(lon+0.004), floatPtr(lat+0.004))
	liveUnit := c.mustUnit(editor, str(liveProp, "id"), "D-"+tag)
	live := c.mustListing(editor, str(liveProp, "id"), str(liveUnit, "id"), str(created, "id"), 100, "Trial land "+tag)
	status, reviewed := c.post("/api/v1/catalog/listings/"+str(live, "id")+"/submit", map[string]any{"version": num(live, "version")}, editor)
	if status != 200 || reviewed["status"] != "in_review" {
		t.Fatalf("listing submit: %d %v", status, reviewed)
	}
	status, approved := c.post("/api/v1/catalog/listings/"+str(live, "id")+"/approve", map[string]any{"version": num(reviewed, "version")}, reviewer)
	if status != 200 || approved["status"] != "approved" {
		t.Fatalf("listing approve: %d %v", status, approved)
	}
	status, livePub := c.post("/api/v1/catalog/listings/"+str(live, "id")+"/publish", map[string]any{"version": num(approved, "version")}, reviewer)
	if status != 200 || livePub["status"] != "published" {
		t.Fatalf("listing publish: %d %v", status, livePub)
	}
	var prices int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM catalog.price_history WHERE listing_id = $1 AND currency = 'SAR'`, str(live, "id")).Scan(&prices); err != nil || prices != 1 {
		t.Fatalf("price history %d err %v", prices, err)
	}
	status, public := c.get("/api/v1/public/home?locale=en")
	found := false
	for _, raw := range asList(t, public["listings"]) {
		item := asMap(t, raw)
		if item["id"] != str(live, "id") {
			continue
		}
		found = true
		if item["precise_lon"] != nil || item["currency"] != "SAR" {
			t.Fatalf("public listing leaked a location or changed currency: %v", item)
		}
		texts := asList(t, item["texts"])
		if len(texts) != 1 || asMap(t, texts[0])["title"] != "Trial land "+tag {
			t.Fatalf("english listing text: %v", item["texts"])
		}
	}
	if status != 200 || !found {
		t.Fatalf("published listing missing from the public home")
	}

	status, nodes := c.getAuth("/api/v1/org/nodes", manager)
	if status != 200 || !nodeNamed(asList(t, nodes["items"]), "شمال العقاري") {
		t.Fatalf("org tree: %d %v", status, nodes)
	}
	status, blockedClose := c.post("/api/v1/org/nodes/"+seed.TeamID+"/close", map[string]any{"version": 1, "reason": "اختبار الإغلاق"}, manager)
	if status != 409 || blockedClose["code"] != "close_blocked" {
		t.Fatalf("team with members closed: %d %v", status, blockedClose)
	}
	status, added := c.post("/api/v1/org/nodes", map[string]any{
		"kind": "team", "parent_id": seed.DeptID, "name_ar": "فريق تجريبي " + tag, "name_en": "Trial team " + tag,
	}, manager)
	if status != 201 {
		t.Fatalf("create team: %d %v", status, added)
	}
	status, editorOrg := c.post("/api/v1/org/nodes", map[string]any{
		"kind": "team", "parent_id": seed.DeptID, "name_ar": "فريق ممنوع " + tag, "name_en": "Denied team " + tag,
	}, editor)
	if status != 403 {
		t.Fatalf("editor created an org node: %d %v", status, editorOrg)
	}
}

func (c *apiClient) mustProperty(cookie, neighborhood, typeCode string, area float64, lon, lat *float64) map[string]any {
	c.t.Helper()
	body := map[string]any{
		"neighborhood_id": neighborhood, "owner_user_id": seed.EditorID, "type_code": typeCode, "area_sqm": area,
		"address_ar": "عنوان اختبار", "address_en": "Test address",
	}
	if lon != nil && lat != nil {
		body["precise_lon"] = *lon
		body["precise_lat"] = *lat
	}
	status, item := c.post("/api/v1/catalog/properties", body, cookie)
	if status != 201 {
		c.t.Fatalf("property: %d %v", status, item)
	}
	return item
}

func (c *apiClient) mustUnit(cookie, propertyID, number string) map[string]any {
	c.t.Helper()
	status, item := c.post("/api/v1/catalog/units", map[string]any{"property_id": propertyID, "unit_number": number}, cookie)
	if status != 201 {
		c.t.Fatalf("unit: %d %v", status, item)
	}
	return item
}

func (c *apiClient) mustListing(cookie, propertyID, unitID, neighborhood string, price int64, title string) map[string]any {
	c.t.Helper()
	status, item := c.post("/api/v1/catalog/listings", map[string]any{
		"property_id": propertyID, "unit_id": unitID, "neighborhood_id": neighborhood,
		"transaction_type": "sale", "price_minor": price, "currency": "SAR", "license_ref": "local-test-ref",
		"broker_user_id": seed.ReviewerID,
		"texts": []map[string]string{
			{"locale": "ar", "title": "أرض اختبار", "description": "وصف عربي مستقل."},
			{"locale": "en", "title": title, "description": "An independent English description."},
		},
	}, cookie)
	if status != 201 {
		c.t.Fatalf("listing: %d %v", status, item)
	}
	return item
}

type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func (c *apiClient) login(email, secret string) string {
	c.t.Helper()
	status, body := c.post("/api/v1/auth/login", map[string]any{"email": email, "password": seed.Password, "context": "staff"}, "")
	if status != 200 || body["mfa_required"] != true {
		c.t.Fatalf("login %s: %d %v", email, status, body)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		c.t.Fatal(err)
	}
	status, session, cookie := c.postCookie("/api/v1/auth/mfa", map[string]any{"challenge_id": body["challenge_id"], "code": code}, "")
	if status != 200 || cookie == "" {
		c.t.Fatalf("mfa %s: %d %v", email, status, session)
	}
	return cookie
}

func (c *apiClient) get(path string) (int, map[string]any) {
	c.t.Helper()
	return c.call(http.MethodGet, path, nil, "")
}

func (c *apiClient) getAuth(path, cookie string) (int, map[string]any) {
	c.t.Helper()
	return c.call(http.MethodGet, path, nil, cookie)
}

func (c *apiClient) post(path string, body any, cookie string) (int, map[string]any) {
	c.t.Helper()
	return c.call(http.MethodPost, path, body, cookie)
}

func (c *apiClient) send(method, path string, body any, cookie string) (int, map[string]any) {
	c.t.Helper()
	return c.call(method, path, body, cookie)
}

func (c *apiClient) postCookie(path string, body any, cookie string) (int, map[string]any, string) {
	c.t.Helper()
	status, payload, set := c.callFull(http.MethodPost, path, body, cookie)
	return status, payload, set
}

func (c *apiClient) call(method, path string, body any, cookie string) (int, map[string]any) {
	c.t.Helper()
	status, payload, _ := c.callFull(method, path, body, cookie)
	return status, payload
}

func (c *apiClient) callFull(method, path string, body any, cookie string) (int, map[string]any, string) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("X-Shamal-Request", "1")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
	set := ""
	for _, item := range res.Cookies() {
		if item.Name == "shamal_session" && item.Value != "" {
			set = item.Value
		}
	}
	return res.StatusCode, payload, set
}

func neighborhoodBody(nameAR, nameEN, geo string, official, sale bool) map[string]any {
	return map[string]any{
		"name_ar": nameAR, "name_en": nameEN, "slug_ar": nameAR, "slug_en": slugEN(nameEN),
		"description_ar": "وصف حي للاختبار المحلي.", "description_en": "A local test neighborhood description.",
		"geojson": geo, "service_sale": sale, "service_annual": true, "service_monthly": true, "service_daily": false,
		"coverage": "served", "show_in_directory": true, "show_on_map": true,
		"boundary_note_ar": "حد تقريبي للاختبار.", "boundary_note_en": "Approximate test boundary.",
		"boundary_is_official": official,
	}
}

func box(lon, lat, d float64) string {
	return fmt.Sprintf(
		`{"type":"Polygon","coordinates":[[[%.6f,%.6f],[%.6f,%.6f],[%.6f,%.6f],[%.6f,%.6f],[%.6f,%.6f]]]}`,
		lon, lat, lon+d, lat, lon+d, lat+d, lon, lat+d, lon, lat,
	)
}

func slugEN(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == ' ' {
			out = append(out, '-')
			continue
		}
		if r >= 'A' && r <= 'Z' {
			out = append(out, r+32)
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func hasIssue(body map[string]any, code string) bool {
	for _, item := range asListRaw(body["issues"]) {
		if item == code {
			return true
		}
	}
	return false
}

func hasField(body map[string]any, key string) bool {
	fields, _ := body["fields"].(map[string]any)
	_, ok := fields[key]
	return ok
}

func field(body map[string]any, key string) string {
	fields, _ := body["fields"].(map[string]any)
	value, _ := fields[key].(string)
	return value
}

func namesContain(items []any, name string) bool {
	for _, raw := range items {
		card, _ := raw.(map[string]any)
		if card["name"] == name {
			return true
		}
	}
	return false
}

func nodeNamed(items []any, name string) bool {
	for _, raw := range items {
		card, _ := raw.(map[string]any)
		if card["name_ar"] == name {
			return true
		}
	}
	return false
}

func asList(t *testing.T, raw any) []any {
	t.Helper()
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected a list, got %T", raw)
	}
	return items
}

func asListRaw(raw any) []any {
	items, _ := raw.([]any)
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func asMap(t *testing.T, raw any) map[string]any {
	t.Helper()
	item, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected an object, got %T", raw)
	}
	return item
}

func str(body map[string]any, key string) string {
	value, _ := body[key].(string)
	return value
}

func num(body map[string]any, key string) int {
	value, _ := body[key].(float64)
	return int(value)
}

func floatPtr(v float64) *float64 { return &v }
