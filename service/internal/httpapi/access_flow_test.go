package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func TestStaffAccess(t *testing.T) {
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
	manager := c.login("manager@shamal.local", seed.ManagerTOTP)

	status, structure := c.getAuth("/api/v1/org/structure?q=editor", editor)
	if status != 200 {
		t.Fatalf("structure: %d %v", status, structure)
	}
	if !staffNamed(t, structure["staff"], "editor@shamal.local") {
		t.Fatalf("editor missing from staff search: %v", structure["staff"])
	}
	if !nodeNamed(asList(t, structure["nodes"]), "شمال العقاري") {
		t.Fatal("company missing from structure")
	}

	status, account := c.getAuth("/api/v1/permissions/account", editor)
	if status != 200 || !hasRule(t, account["direct"], "organization", "view") {
		t.Fatalf("editor account: %d %v", status, account)
	}
	status, other := c.getAuth("/api/v1/permissions/account?user_id="+seed.ManagerID, editor)
	if status != 403 {
		t.Fatalf("editor read another account: %d %v", status, other)
	}
	status, denied := c.post("/api/v1/permissions/grants", map[string]any{
		"user_id": seed.EditorID, "domain": "reports", "action": "archive", "effect": "allow", "scope_kind": "company",
	}, editor)
	if status != 403 {
		t.Fatalf("editor granted: %d %v", status, denied)
	}
	rawBulk := `[{"user_id":"` + seed.EditorID + `","domain":"reports","action":"archive","effect":"allow","scope_kind":"company"}]`
	bulkStatus, bulkBody := postRaw(t, srv.Client(), srv.URL+"/api/v1/permissions/grants", rawBulk, manager)
	if bulkStatus != 400 || bulkBody["code"] != "invalid_json" {
		t.Fatalf("bulk grant: %d %v", bulkStatus, bulkBody)
	}

	ends := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	status, granted := c.post("/api/v1/permissions/grants", map[string]any{
		"user_id": seed.EditorID, "domain": "reports", "action": "archive", "effect": "allow", "scope_kind": "company", "ends_at": ends,
	}, manager)
	if status != 201 {
		t.Fatalf("grant: %d %v", status, granted)
	}
	status, account = c.getAuth("/api/v1/permissions/account?user_id="+seed.EditorID, manager)
	if status != 200 || !hasRule(t, account["temporary"], "reports", "archive") || !hasRule(t, account["direct"], "reports", "archive") {
		t.Fatalf("temporary grant missing: %d %v", status, account)
	}
	status, preview := c.post("/api/v1/permissions/preview", map[string]any{
		"user_id": seed.EditorID, "domain": "reports", "action": "archive",
		"resource": map[string]any{"company_id": seed.CompanyID, "created_by": seed.ManagerID},
	}, manager)
	if status != 200 || preview["allowed"] != true || preview["layer"] != "direct" {
		t.Fatalf("preview: %d %v", status, preview)
	}
	status, broken := c.post("/api/v1/permissions/breaks", map[string]any{
		"node_id": seed.TeamID, "domain": "files", "action": "create", "reason": "قطع وراثة واحد للاختبار",
	}, manager)
	if status != 201 && status != 422 {
		t.Fatalf("break: %d %v", status, broken)
	}
	status, editorBreak := c.post("/api/v1/permissions/breaks", map[string]any{
		"node_id": seed.TeamID, "domain": "files", "action": "update", "reason": "لا",
	}, editor)
	if status != 403 {
		t.Fatalf("editor break: %d %v", status, editorBreak)
	}
	status, account = c.getAuth("/api/v1/permissions/account?user_id="+seed.EditorID, manager)
	if status != 200 || !hasBreak(t, account["breaks"], seed.TeamID, "files", "create") {
		t.Fatalf("break missing: %d %v", status, account["breaks"])
	}
}

func postRaw(t *testing.T, client *http.Client, url, body, cookie string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shamal-Request", "1")
	req.Header.Set("Accept-Language", "en")
	req.AddCookie(&http.Cookie{Name: "shamal_session", Value: cookie})
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	payload := map[string]any{}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, payload
}

func staffNamed(t *testing.T, raw any, email string) bool {
	t.Helper()
	for _, item := range asList(t, raw) {
		if asMap(t, item)["email"] == email {
			return true
		}
	}
	return false
}

func hasRule(t *testing.T, raw any, domain, action string) bool {
	t.Helper()
	for _, item := range asList(t, raw) {
		rule := asMap(t, item)
		if rule["domain"] == domain && rule["action"] == action {
			return true
		}
	}
	return false
}

func hasBreak(t *testing.T, raw any, node, domain, action string) bool {
	t.Helper()
	for _, item := range asList(t, raw) {
		rule := asMap(t, item)
		if rule["node_id"] == node && rule["domain"] == domain && rule["action"] == action {
			return true
		}
	}
	return false
}
