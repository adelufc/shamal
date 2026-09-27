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

const narjisID = "88888888-8888-4888-8888-888888888801"

func TestCommercialFlow(t *testing.T) {
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

	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	shift := time.Duration(time.Now().UnixNano()%100000) * time.Minute
	future := time.Now().UTC().Add(400 * 24 * time.Hour).Add(shift).Truncate(time.Minute)
	past := time.Now().UTC().Add(-400 * 24 * time.Hour).Add(shift).Truncate(time.Minute)
	listingID, propertyID := publishNarjisListing(t, c, editor, reviewer, tag)
	email := fmt.Sprintf("client-%s@example.com", tag)
	status, registered := c.post("/api/v1/auth/register", map[string]any{
		"display_name": "عميل التجربة", "email": email, "password": seed.Password, "phone": "0500000000",
	}, "")
	if status != 201 || str(registered, "id") == "" {
		t.Fatalf("register: %d %v", status, registered)
	}
	status, taken := c.post("/api/v1/auth/register", map[string]any{
		"display_name": "عميل التجربة", "email": email, "password": seed.Password,
	}, "")
	if status != 409 || taken["code"] != "email_taken" {
		t.Fatalf("duplicate register: %d %v", status, taken)
	}
	client := c.loginClient(email, seed.Password)
	status, staffAccount := c.getAuth("/api/v1/account", editor)
	if status != 403 || staffAccount["code"] != "forbidden" {
		t.Fatalf("staff used the client account: %d %v", status, staffAccount)
	}
	status, clientStaff := c.getAuth("/api/v1/clients/requests", client)
	if status != 403 {
		t.Fatalf("client opened the staff queue: %d %v", status, clientStaff)
	}

	status, request := c.post("/api/v1/public/requests", map[string]any{
		"display_name": "عميل التجربة", "email": email, "phone": "0500000000",
		"transaction_type": "sale", "neighborhood_id": narjisID, "timing_note": "خلال شهر",
		"specs_ar": "مواصفات عربية مستقلة", "specs_en": "Independent English specs",
		"contact_method": "either",
	}, client)
	if status != 201 || request["stage"] != "new" || request["assignee_id"] != "" || request["specs_en"] != "Independent English specs" {
		t.Fatalf("request: %d %v", status, request)
	}
	if request["budget_minor"] != nil {
		t.Fatalf("budget was assumed: %v", request["budget_minor"])
	}
	requestID := str(request, "id")
	status, jumped := c.post("/api/v1/clients/requests/"+requestID+"/move", map[string]any{"version": num(request, "version"), "to": "converted"}, editor)
	if status != 409 || jumped["code"] != "invalid_transition" {
		t.Fatalf("jump to converted: %d %v", status, jumped)
	}

	spare := staffRequest(t, c, editor, "spare-"+tag+"@example.com", "عميل إضافي")
	status, bare := c.post("/api/v1/clients/requests/"+str(spare, "id")+"/move", map[string]any{"version": num(spare, "version"), "to": "unqualified"}, editor)
	if status != 422 || bare["code"] != "reason_required" {
		t.Fatalf("unqualified without reason: %d %v", status, bare)
	}
	status, closedLead := c.post("/api/v1/clients/requests/"+str(spare, "id")+"/move", map[string]any{"version": num(spare, "version"), "to": "unqualified", "reason": "خارج النطاق"}, editor)
	if status != 200 || closedLead["stage"] != "unqualified" {
		t.Fatalf("unqualified: %d %v", status, closedLead)
	}

	request = walkRequest(t, c, editor, request, []string{"contacted", "interested", "visit", "negotiation", "agreement"})
	if request["stage"] != "agreement" {
		t.Fatalf("agreement: %v", request["stage"])
	}
	status, deal := c.post("/api/v1/clients/requests/"+requestID+"/convert", map[string]any{"version": num(request, "version")}, editor)
	if status != 201 || deal["status"] != "draft" || deal["is_test"] != false {
		t.Fatalf("convert: %d %v", status, deal)
	}
	dealID := str(deal, "id")
	status, priced := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{
		"version": num(deal, "version"), "to": "qualified", "commission_bps": 250,
	}, editor)
	if status != 422 || priced["code"] != "commercial_terms_unapproved" {
		t.Fatalf("commission: %d %v", status, priced)
	}
	status, qualified := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{
		"version": num(deal, "version"), "to": "qualified",
		"listing_id": listingID, "property_id": propertyID,
		"owner_user_id": seed.EditorID, "broker_user_id": seed.ReviewerID,
	}, editor)
	if status != 200 || qualified["status"] != "qualified" {
		t.Fatalf("qualified: %d %v", status, qualified)
	}
	status, negotiation := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{"version": num(qualified, "version"), "to": "negotiation"}, editor)
	if status != 200 || negotiation["status"] != "negotiation" {
		t.Fatalf("negotiation: %d %v", status, negotiation)
	}
	status, changed := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{
		"version": num(negotiation, "version"), "to": "awaiting_documents",
		"property_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	}, editor)
	if status != 422 || changed["code"] != "property_change_unapproved" {
		t.Fatalf("property change: %d %v", status, changed)
	}
	beforeHistory := num(negotiation, "history_count")
	status, back := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{
		"version": num(negotiation, "version"), "to": "qualified", "reason": "ناقص مستند",
	}, editor)
	if status != 200 || back["status"] != "qualified" || num(back, "history_count") <= beforeHistory {
		t.Fatalf("backward move: %d %v", status, back)
	}
	status, negotiation = c.post("/api/v1/deals/"+dealID+"/move", map[string]any{"version": num(back, "version"), "to": "negotiation"}, editor)
	if status != 200 {
		t.Fatalf("return to negotiation: %d %v", status, negotiation)
	}
	status, docs := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{"version": num(negotiation, "version"), "to": "awaiting_documents"}, editor)
	if status != 200 || docs["status"] != "awaiting_documents" {
		t.Fatalf("documents: %d %v", status, docs)
	}
	status, earlyAgree := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{"version": num(docs, "version"), "to": "preliminary_agreement"}, editor)
	if status != 422 || !hasField(earlyAgree, "invalid_price") {
		t.Fatalf("preliminary without amount: %d %v", status, earlyAgree)
	}
	status, preliminary := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{
		"version": num(docs, "version"), "to": "preliminary_agreement", "amount_minor": 250000, "currency": "SAR",
	}, editor)
	if status != 200 || preliminary["status"] != "preliminary_agreement" {
		t.Fatalf("preliminary: %d %v", status, preliminary)
	}
	status, external := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{"version": num(preliminary, "version"), "to": "awaiting_external"}, editor)
	if status != 200 || external["status"] != "awaiting_external" {
		t.Fatalf("external: %d %v", status, external)
	}
	status, taxed := c.post("/api/v1/deals/"+dealID+"/request-close", map[string]any{
		"version": num(external, "version"), "agreement_ref": "local-" + tag, "amount_minor": 250000, "currency": "SAR",
		"close_date": time.Now().UTC(), "tax_rate": 15,
	}, editor)
	if status != 422 || taxed["code"] != "commercial_terms_unapproved" {
		t.Fatalf("tax: %d %v", status, taxed)
	}
	status, closing := c.post("/api/v1/deals/"+dealID+"/request-close", map[string]any{
		"version": num(external, "version"), "agreement_ref": "local-" + tag, "amount_minor": 250000, "currency": "SAR",
		"close_date": time.Now().UTC(),
	}, editor)
	if status != 200 || closing["status"] != "awaiting_external" || closing["agreement_ref"] != "local-"+tag {
		t.Fatalf("request close: %d %v", status, closing)
	}
	status, again := c.post("/api/v1/deals/"+dealID+"/request-close", map[string]any{
		"version": num(closing, "version"), "agreement_ref": "local-again-" + tag, "amount_minor": 250000, "currency": "SAR",
		"close_date": time.Now().UTC(),
	}, editor)
	if status != 200 || again["status"] != "awaiting_external" || again["agreement_ref"] != "local-again-"+tag {
		t.Fatalf("second close request: %d %v", status, again)
	}
	var invoices int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'deals' AND table_name = 'invoices'`).Scan(&invoices); err != nil || invoices != 0 {
		t.Fatalf("a close request created an invoice table: %d %v", invoices, err)
	}
	status, ownClose := c.post("/api/v1/deals/"+dealID+"/approve-close", map[string]any{"version": num(again, "version")}, editor)
	if status != 403 || field(ownClose, "reason") != "The person who created this record cannot approve it." {
		t.Fatalf("author close: %d %v", status, ownClose)
	}
	status, closed := c.post("/api/v1/deals/"+dealID+"/approve-close", map[string]any{"version": num(again, "version")}, manager)
	if status != 200 || closed["status"] != "closed_success" {
		t.Fatalf("manager close: %d %v", status, closed)
	}
	status, afterClose := c.post("/api/v1/deals/"+dealID+"/move", map[string]any{"version": num(closed, "version"), "to": "cancelled", "reason": "بعد الإغلاق"}, editor)
	if status != 409 || afterClose["code"] != "invalid_transition" {
		t.Fatalf("closed deal moved: %d %v", status, afterClose)
	}

	left := staffRequest(t, c, editor, "left-"+tag+"@example.com", "عميل التعارض أ")
	right := staffRequest(t, c, editor, "right-"+tag+"@example.com", "عميل التعارض ب")
	slot := future
	visitA := createVisit(t, c, editor, str(left, "profile_id"), listingID, slot, "تعليمات عربية", "Independent English instructions")
	visitB := createVisit(t, c, editor, str(right, "profile_id"), listingID, slot, "تعليمات", "Instructions")
	visitA = postVisit(t, c, editor, visitA, "broker-confirm", nil)
	visitB = postVisit(t, c, editor, visitB, "broker-confirm", nil)
	if visitA["status"] != "awaiting_owner" || visitB["status"] != "awaiting_owner" {
		t.Fatalf("broker confirm: %v %v", visitA["status"], visitB["status"])
	}
	visitA = postVisit(t, c, reviewer, visitA, "owner-confirm", nil)
	if visitA["status"] != "confirmed" {
		t.Fatalf("owner confirm: %v", visitA)
	}
	status, clash := c.post("/api/v1/visits/"+str(visitB, "id")+"/owner-confirm", map[string]any{"version": num(visitB, "version")}, reviewer)
	if status != 409 || clash["code"] != "visit_conflict" {
		t.Fatalf("second confirm: %d %v", status, clash)
	}
	status, early := c.post("/api/v1/visits/"+str(visitA, "id")+"/complete", map[string]any{"version": num(visitA, "version"), "result": "interested"}, editor)
	if status != 409 || early["code"] != "too_early" {
		t.Fatalf("early complete: %d %v", status, early)
	}

	base, err := api.Visits.CompletionTotal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pastA := createVisit(t, c, editor, str(left, "profile_id"), listingID, past, "عربي", "English")
	pastB := createVisit(t, c, editor, str(right, "profile_id"), listingID, past.Add(3*time.Hour), "عربي", "English")
	pastA = confirmVisit(t, c, editor, reviewer, pastA)
	pastB = confirmVisit(t, c, editor, reviewer, pastB)
	pastA = postVisit(t, c, editor, pastA, "complete", map[string]any{"result": "follow_up", "note": "ملاحظة داخلية"})
	if pastA["status"] != "completed" || pastA["result"] != "follow_up" {
		t.Fatalf("real complete: %v", pastA)
	}
	clientPast := createAccountVisit(t, c, client, listingID, past.Add(6*time.Hour), "موعد العميل", "Client visit note")
	clientPast = confirmVisit(t, c, editor, reviewer, clientPast)
	clientPast = postVisit(t, c, editor, clientPast, "complete", map[string]any{"result": "negotiation", "note": "لا تظهر للعميل"})
	visitA = postVisit(t, c, manager, visitA, "complete", map[string]any{"result": "interested", "reason": "تصحيح إداري قبل نهاية الموعد", "correction": true})
	if visitA["status"] != "completed" {
		t.Fatalf("correction: %v", visitA)
	}
	after, err := api.Visits.CompletionTotal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after != base+2 {
		t.Fatalf("counter base %d after %d", base, after)
	}
	var reversals int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit.events WHERE entity_id = $1 AND action = 'visit.counter_reversed'`, str(visitA, "id")).Scan(&reversals); err != nil || reversals != 1 {
		t.Fatalf("counter reversal log %d %v", reversals, err)
	}

	cancelAt := future.Add(3 * time.Hour)
	cancelVisit := createVisit(t, c, editor, str(left, "profile_id"), listingID, cancelAt, "عربي", "English")
	cancelVisit = confirmVisit(t, c, editor, reviewer, cancelVisit)
	recorded, err := api.Visits.RecordedReminders(ctx, str(cancelVisit, "id"))
	if err != nil || recorded != 2 {
		t.Fatalf("reminders %d %v", recorded, err)
	}
	cancelVisit = postVisit(t, c, editor, cancelVisit, "cancel", map[string]any{"reason": "اعتذر العميل"})
	if cancelVisit["status"] != "cancelled" {
		t.Fatalf("cancel: %v", cancelVisit)
	}
	recorded, err = api.Visits.RecordedReminders(ctx, str(cancelVisit, "id"))
	if err != nil || recorded != 0 {
		t.Fatalf("reminders after cancel %d %v", recorded, err)
	}
	status, secondCancel := c.post("/api/v1/visits/"+str(cancelVisit, "id")+"/cancel", map[string]any{"version": num(cancelVisit, "version"), "reason": "مرة أخرى"}, editor)
	if status != 409 || secondCancel["code"] != "invalid_transition" {
		t.Fatalf("second cancel: %d %v", status, secondCancel)
	}

	rejectAt := future.Add(6 * time.Hour)
	rejected := createVisit(t, c, editor, str(right, "profile_id"), listingID, rejectAt, "عربي", "English")
	rejected = postVisit(t, c, editor, rejected, "broker-confirm", nil)
	alternative := rejectAt.Add(2 * time.Hour)
	rejected = postVisit(t, c, reviewer, rejected, "owner-reject", map[string]any{"reason": "المالك غير متاح", "starts_at": alternative})
	if rejected["status"] != "awaiting_broker" {
		t.Fatalf("owner reject: %v", rejected)
	}
	gotSlot, err := time.Parse(time.RFC3339, str(rejected, "starts_at"))
	if err != nil || !gotSlot.Equal(alternative) {
		t.Fatalf("alternative slot: %v", rejected["starts_at"])
	}

	moveAt := future.Add(9 * time.Hour)
	moved := createVisit(t, c, editor, str(left, "profile_id"), listingID, moveAt, "عربي", "English")
	moved = confirmVisit(t, c, editor, reviewer, moved)
	nextSlot := moveAt.Add(4 * time.Hour)
	moved = postVisit(t, c, editor, moved, "reschedule", map[string]any{"reason": "تغيير الموعد", "starts_at": nextSlot})
	if moved["status"] != "awaiting_broker" {
		t.Fatalf("reschedule: %v", moved)
	}
	var history int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM visits.history WHERE visit_id = $1`, str(moved, "id")).Scan(&history); err != nil || history < 1 {
		t.Fatalf("reschedule history %d %v", history, err)
	}
	recorded, err = api.Visits.RecordedReminders(ctx, str(moved, "id"))
	if err != nil || recorded != 0 {
		t.Fatalf("reminders after reschedule %d %v", recorded, err)
	}

	status, account := c.getAuth("/api/v1/account", client)
	if status != 200 {
		t.Fatalf("account: %d %v", status, account)
	}
	foundRequest := false
	for _, raw := range asList(t, account["requests"]) {
		item := asMap(t, raw)
		if item["id"] != requestID {
			continue
		}
		foundRequest = true
		if item["client_stage"] != "completed" || item["specs_ar"] != "مواصفات عربية مستقلة" || item["specs_en"] != "Independent English specs" {
			t.Fatalf("client journey: %v", item)
		}
	}
	if !foundRequest {
		t.Fatalf("account missed the request: %v", account["requests"])
	}
	foundVisit := false
	for _, raw := range asList(t, account["visits"]) {
		item := asMap(t, raw)
		if item["id"] != str(clientPast, "id") {
			continue
		}
		foundVisit = true
		if item["result"] != nil && item["result"] != "" || item["result_note"] != nil && item["result_note"] != "" {
			t.Fatalf("client saw a staff result: %v", item)
		}
	}
	if !foundVisit {
		t.Fatal("account missed the completed visit")
	}
	for _, raw := range asList(t, account["deals"]) {
		item := asMap(t, raw)
		if item["loss_reason"] != nil && item["loss_reason"] != "" {
			t.Fatalf("client saw a loss reason: %v", item)
		}
	}

	own := createAccountVisit(t, c, client, listingID, future.Add(12*time.Hour), "إلغاء", "Cancel")
	status, own = c.post("/api/v1/account/visits/"+str(own, "id")+"/cancel", map[string]any{"version": num(own, "version"), "reason": "لم أعد أرغب"}, client)
	if status != 200 || own["status"] != "cancelled" {
		t.Fatalf("client cancel: %d %v", status, own)
	}

	status, home := c.get("/api/v1/public/home?locale=ar")
	counts := asMap(t, home["counts"])
	if status != 200 || counts["deals"] != nil {
		t.Fatalf("public home exposed deals: %v", home["counts"])
	}
}

func (c *apiClient) loginClient(email, password string) string {
	c.t.Helper()
	status, body, cookie := c.postCookie("/api/v1/auth/login", map[string]any{"email": email, "password": password, "context": "client"}, "")
	if status != 200 || cookie == "" || body["mfa_required"] == true {
		c.t.Fatalf("client login: %d %v", status, body)
	}
	return cookie
}

func publishNarjisListing(t *testing.T, c *apiClient, editor, reviewer, tag string) (string, string) {
	t.Helper()
	lon, lat := 46.645, 24.885
	property := c.mustProperty(editor, narjisID, "land", 400, floatPtr(lon), floatPtr(lat))
	unit := c.mustUnit(editor, str(property, "id"), "V-"+tag)
	listing := c.mustListing(editor, str(property, "id"), str(unit, "id"), narjisID, 100, "Visit land "+tag)
	status, reviewed := c.post("/api/v1/catalog/listings/"+str(listing, "id")+"/submit", map[string]any{"version": num(listing, "version")}, editor)
	if status != 200 {
		t.Fatalf("listing submit: %d %v", status, reviewed)
	}
	status, approved := c.post("/api/v1/catalog/listings/"+str(listing, "id")+"/approve", map[string]any{"version": num(reviewed, "version")}, reviewer)
	if status != 200 {
		t.Fatalf("listing approve: %d %v", status, approved)
	}
	status, published := c.post("/api/v1/catalog/listings/"+str(listing, "id")+"/publish", map[string]any{"version": num(approved, "version")}, reviewer)
	if status != 200 || published["status"] != "published" {
		t.Fatalf("listing publish: %d %v", status, published)
	}
	return str(listing, "id"), str(property, "id")
}

func staffRequest(t *testing.T, c *apiClient, editor, email, name string) map[string]any {
	t.Helper()
	status, item := c.post("/api/v1/clients/requests", map[string]any{
		"display_name": name, "email": email, "transaction_type": "sale", "neighborhood_id": narjisID,
		"specs_ar": "طلب موظف", "specs_en": "Staff request", "contact_method": "email",
	}, editor)
	if status != 201 || str(item, "profile_id") == "" {
		t.Fatalf("staff request: %d %v", status, item)
	}
	return item
}

func walkRequest(t *testing.T, c *apiClient, editor string, current map[string]any, stages []string) map[string]any {
	t.Helper()
	for _, stage := range stages {
		status, next := c.post("/api/v1/clients/requests/"+str(current, "id")+"/move", map[string]any{
			"version": num(current, "version"), "to": stage,
		}, editor)
		if status != 200 || next["stage"] != stage {
			t.Fatalf("move request to %s: %d %v", stage, status, next)
		}
		current = next
	}
	return current
}

func createVisit(t *testing.T, c *apiClient, cookie, profile, listing string, start time.Time, ar, en string) map[string]any {
	t.Helper()
	status, item := c.post("/api/v1/visits", map[string]any{
		"profile_id": profile, "listing_id": listing, "starts_at": start, "attendees": 1,
		"instructions_ar": ar, "instructions_en": en,
	}, cookie)
	if status != 201 {
		t.Fatalf("visit: %d %v", status, item)
	}
	return item
}

func createAccountVisit(t *testing.T, c *apiClient, cookie, listing string, start time.Time, ar, en string) map[string]any {
	t.Helper()
	status, item := c.post("/api/v1/account/visits", map[string]any{
		"listing_id": listing, "starts_at": start, "attendees": 1,
		"instructions_ar": ar, "instructions_en": en,
	}, cookie)
	if status != 201 {
		t.Fatalf("account visit: %d %v", status, item)
	}
	return item
}

func postVisit(t *testing.T, c *apiClient, cookie string, current map[string]any, action string, extra map[string]any) map[string]any {
	t.Helper()
	body := map[string]any{"version": num(current, "version")}
	for key, value := range extra {
		body[key] = value
	}
	status, item := c.post("/api/v1/visits/"+str(current, "id")+"/"+action, body, cookie)
	if status != 200 {
		t.Fatalf("%s: %d %v", action, status, item)
	}
	return item
}

func confirmVisit(t *testing.T, c *apiClient, editor, reviewer string, current map[string]any) map[string]any {
	t.Helper()
	current = postVisit(t, c, editor, current, "broker-confirm", nil)
	return postVisit(t, c, reviewer, current, "owner-confirm", nil)
}
