package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"shamal/service/internal/finance"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func TestDeskSlice(t *testing.T) {
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
	status, registered := c.post("/api/v1/auth/register", map[string]any{"display_name": "مراسل", "email": "desk-" + tag + "@example.com", "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("register: %d %v", status, registered)
	}
	clientID := str(registered, "id")
	client := c.loginClient("desk-"+tag+"@example.com", seed.Password)
	status, otherReg := c.post("/api/v1/auth/register", map[string]any{"display_name": "آخر", "email": "other-" + tag + "@example.com", "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("other: %d %v", status, otherReg)
	}
	other := c.loginClient("other-"+tag+"@example.com", seed.Password)
	listingID, _ := publishStayListing(t, c, editor, reviewer, "desk"+tag, "daily", 40000)

	status, preview := c.getAuth("/api/v1/account/messages/preview?subject_kind=listing&subject_id="+listingID, client)
	if status != 200 || preview["created"] != false || num(preview, "received") != 0 {
		t.Fatalf("preview created a message: %d %v", status, preview)
	}
	var conversations int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM messages.conversations WHERE subject_id = $1`, listingID).Scan(&conversations); err != nil {
		t.Fatal(err)
	}
	if conversations != 0 {
		t.Fatal("opening the compose screen stored a conversation")
	}
	status, posted := c.postKey("/api/v1/account/messages", map[string]any{"subject_kind": "listing", "subject_id": listingID, "body": "سؤال عن الإعلان"}, client, "msg-"+tag)
	if status != 201 || posted["delivery_status"] != "not_sent" || posted["provider_adopted"] != false || posted["send_status"] != "stored" {
		t.Fatalf("message: %d %v", status, posted)
	}
	conv := str(posted, "conversation_id")
	status, again := c.getAuth("/api/v1/account/messages/preview?subject_kind=listing&subject_id="+listingID, client)
	if status != 200 || again["created"] != false || num(again, "received") != 1 {
		t.Fatalf("preview after send: %d %v", status, again)
	}
	status, denied := c.getAuth("/api/v1/account/messages/"+conv, other)
	if status != 404 {
		t.Fatalf("outsider read the conversation: %d %v", status, denied)
	}
	status, edited := c.send(http.MethodPut, "/api/v1/account/messages/"+conv, map[string]any{"body": "تعديل صامت"}, client)
	if status != 409 || edited["code"] != "message_immutable" {
		t.Fatalf("silent edit: %d %v", status, edited)
	}
	status, card := c.postKey("/api/v1/account/messages", map[string]any{"subject_kind": "listing", "subject_id": listingID, "body": "4111111111111111"}, client, "card-"+tag)
	if status != 422 || card["code"] != "card_data_refused" {
		t.Fatalf("card in message: %d %v", status, card)
	}

	visit := createAccountVisit(t, c, client, listingID, time.Now().Add(72*time.Hour).UTC(), "تعليمات", "Instructions")
	status, visitMsg := c.postKey("/api/v1/account/messages", map[string]any{"subject_kind": "visit", "subject_id": str(visit, "id"), "body": "عن الزيارة"}, client, "visit-"+tag)
	if status != 201 {
		t.Fatalf("visit message: %d %v", status, visitMsg)
	}
	var before int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM messages.messages m JOIN messages.conversations c ON c.id = m.conversation_id WHERE c.subject_id = $1`, str(visit, "id")).Scan(&before)
	status, peeked := c.getAuth("/api/v1/account/messages/preview?subject_kind=visit&subject_id="+str(visit, "id"), other)
	if status != 404 {
		t.Fatalf("outsider preview: %d %v", status, peeked)
	}
	var afterPeek int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM messages.messages m JOIN messages.conversations c ON c.id = m.conversation_id WHERE c.subject_id = $1`, str(visit, "id")).Scan(&afterPeek)
	if afterPeek != before {
		t.Fatal("a refused preview stored a message")
	}

	stayID := confirmStay(t, c, client, listingID, "2027-06-01", "2027-06-03", 80000, "desk-"+tag)
	status, stayMsg := c.postKey("/api/v1/account/messages", map[string]any{"subject_kind": "booking", "subject_id": stayID, "body": "عن الحجز"}, client, "stay-"+tag)
	if status != 201 {
		t.Fatalf("booking message: %d %v", status, stayMsg)
	}
	var profile string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM clients.profiles WHERE user_id = $1`, clientID).Scan(&profile); err != nil {
		t.Fatal(err)
	}
	dealID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO deals.deals (id, company_id, profile_id, transaction_type, status, created_by, is_test)
		VALUES ($1, $2, $3, 'sale', 'draft', $4, true)
	`, dealID, seed.CompanyID, profile, seed.EditorID); err != nil {
		t.Fatal(err)
	}
	status, dealMsg := c.postKey("/api/v1/account/messages", map[string]any{"subject_kind": "deal", "subject_id": dealID, "body": "عن الصفقة"}, client, "deal-"+tag)
	if status != 201 {
		t.Fatalf("deal message: %d %v", status, dealMsg)
	}

	status, second := c.postKey("/api/v1/account/messages/"+conv, map[string]any{"body": "متابعة"}, client, "msg2-"+tag)
	if status != 201 {
		t.Fatalf("second: %d %v", status, second)
	}
	status, thread := c.getAuth("/api/v1/account/messages/"+conv+"?after="+str(posted, "id"), client)
	items := asList(t, thread["items"])
	if status != 200 || len(items) != 1 || asMap(t, items[0])["id"] != str(second, "id") {
		t.Fatalf("missed messages: %d %v", status, thread)
	}
	status, thread = c.getAuth("/api/v1/account/messages/"+conv+"?after="+str(posted, "id"), client)
	if status != 200 || len(asList(t, thread["items"])) != 1 {
		t.Fatalf("cursor duplicated: %d %v", status, thread)
	}
	var stored int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM messages.messages WHERE conversation_id = $1`, conv).Scan(&stored)
	status, read := c.post("/api/v1/account/messages/"+conv+"/read", map[string]any{}, client)
	if status != 200 || read["unchanged"] != true {
		t.Fatalf("read created a message: %d %v", status, read)
	}
	var storedAfter int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM messages.messages WHERE conversation_id = $1`, conv).Scan(&storedAfter)
	if storedAfter != stored {
		t.Fatal("marking read stored a message")
	}

	status, note := c.postKey("/api/v1/messages/"+conv, map[string]any{"kind": "internal_note", "body": "ملاحظة داخلية"}, editor, "note-"+tag)
	if status != 201 || note["kind"] != "internal_note" {
		t.Fatalf("internal: %d %v", status, note)
	}
	status, clientThread := c.getAuth("/api/v1/account/messages/"+conv, client)
	for _, raw := range asList(t, clientThread["items"]) {
		row := asMap(t, raw)
		if row["kind"] == "internal_note" || row["body"] == "ملاحظة داخلية" {
			t.Fatalf("client saw an internal note: %v", clientThread)
		}
	}
	if status != 200 {
		t.Fatalf("client thread: %d", status)
	}
	status, wrongCanned := c.postKey("/api/v1/messages/"+conv, map[string]any{"canned_id": "55555555-5555-4555-8555-555555555501"}, editor, "canned-ar-"+tag)
	if status != 422 || wrongCanned["code"] != "canned_locale" {
		t.Fatalf("canned locale: %d %v", status, wrongCanned)
	}
	status, canned := c.postKey("/api/v1/messages/"+conv, map[string]any{"canned_id": "55555555-5555-4555-8555-555555555502"}, editor, "canned-en-"+tag)
	if status != 201 || canned["kind"] != "canned" {
		t.Fatalf("canned: %d %v", status, canned)
	}
	status, inbox := c.getAuth("/api/v1/messages", editor)
	if status != 200 || inboxItem(t, inbox, conv)["first_human_reply_at"] != nil {
		t.Fatalf("canned counted as a human reply: %d %v", status, inbox)
	}
	status, human := c.postKey("/api/v1/messages/"+conv, map[string]any{"body": "رد الموظف"}, editor, "human-"+tag)
	if status != 201 {
		t.Fatalf("human: %d %v", status, human)
	}
	status, inbox = c.getAuth("/api/v1/messages", editor)
	if status != 200 || inboxItem(t, inbox, conv)["first_human_reply_at"] == nil {
		t.Fatalf("human reply was not recorded: %d %v", status, inbox)
	}
	status, assigned := c.post("/api/v1/messages/"+conv+"/assign", map[string]any{"assignee_id": seed.ReviewerID}, editor)
	if status != 200 {
		t.Fatalf("assign: %d %v", status, assigned)
	}
	status, hidden := c.post("/api/v1/messages/hide", map[string]any{"message_id": str(posted, "id"), "reason": "إخفاء إشرافي"}, manager)
	if status != 200 || hidden["hidden"] != true || hidden["body"] == "" {
		t.Fatalf("hide kept no audit copy for the archivist: %d %v", status, hidden)
	}
	status, clientThread = c.getAuth("/api/v1/account/messages/"+conv, client)
	for _, raw := range asList(t, clientThread["items"]) {
		row := asMap(t, raw)
		if row["id"] == str(posted, "id") && (row["hidden"] != true || row["body"] != "") {
			t.Fatalf("client still sees the hidden body: %v", row)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE messages.messages SET body = 'x' WHERE id = $1`, str(posted, "id")); err == nil {
		t.Fatal("message body was edited")
	}
	status, reported := c.post("/api/v1/account/messages/"+conv+"/report", map[string]any{"kind": "abuse", "reason": "إساءة"}, client)
	if status != 200 {
		t.Fatalf("report: %d %v", status, reported)
	}
	status, stays := c.getAuth("/api/v1/account/stays", client)
	if status != 200 {
		t.Fatalf("report blocked the booking: %d %v", status, stays)
	}

	when := time.Now().UTC().Format(time.RFC3339)
	status, opened := c.post("/api/v1/account/disputes", map[string]any{
		"subject_kind": "booking", "subject_id": stayID, "type": "cleanliness",
		"description": "الوحدة غير مطابقة للوصف", "occurred_at": when, "requested_amount_minor": 1000,
	}, client)
	if status != 201 || opened["financial_complete"] != false || opened["money_moved"] != false || opened["sla_adopted"] != false || opened["reference"] == "" {
		t.Fatalf("dispute: %d %v", status, opened)
	}
	caseID := str(opened, "id")
	status, outsider := c.getAuth("/api/v1/account/disputes/"+caseID, other)
	if status != 404 {
		t.Fatalf("outsider saw the dispute: %d %v", status, outsider)
	}
	status, jump := c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{"to": "closed", "reason": "إغلاق مباشر"}, reviewer)
	if status != 409 || jump["code"] != "invalid_transition" {
		t.Fatalf("jump to closed: %d %v", status, jump)
	}
	move(t, c, editor, caseID, "classified", "تصنيف")
	move(t, c, editor, caseID, "in_progress", "بدء المعالجة")
	status, evidence := c.upload("/api/v1/account/disputes/"+caseID+"/evidence", client, "note.pdf", "application/pdf", []byte("%PDF-1.4 desk"))
	if status != 201 || evidence["sha256"] == "" {
		t.Fatalf("evidence: %d %v", status, evidence)
	}
	status, duplicate := c.upload("/api/v1/account/disputes/"+caseID+"/evidence", client, "note.pdf", "application/pdf", []byte("%PDF-1.4 desk"))
	if status != 409 || duplicate["code"] != "evidence_exists" {
		t.Fatalf("evidence replaced: %d %v", status, duplicate)
	}
	if _, err := pool.Exec(ctx, `UPDATE disputes.evidence SET size_bytes = 1 WHERE case_id = $1`, caseID); err == nil {
		t.Fatal("evidence row was updated")
	}
	status, beforeAssign := c.getAuth("/api/v1/disputes/"+caseID, editor)
	if status != 200 {
		t.Fatalf("staff dispute: %d %v", status, beforeAssign)
	}
	due := beforeAssign["response_due_at"]
	evidenceID := str(asMap(t, asList(t, beforeAssign["evidence"])[0]), "id")
	status, reassigned := c.post("/api/v1/disputes/"+caseID+"/assign", map[string]any{"assignee_id": seed.ReviewerID}, editor)
	if status != 200 || reassigned["response_due_at"] != due {
		t.Fatalf("assign lost the deadline: %d %v", status, reassigned)
	}
	if str(asMap(t, asList(t, reassigned["evidence"])[0]), "id") != evidenceID {
		t.Fatal("assign lost the evidence")
	}
	status, proposed := c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{
		"to": "proposed", "reason": "تعويض غير تعاقدي", "kind": "compensation", "amount_minor": 1000, "actions": "طلب استرداد فقط",
	}, editor)
	if status != 200 || proposed["status"] != "proposed" {
		t.Fatalf("proposal: %d %v", status, proposed)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO permissions.rules (
			id, company_id, grantee_type, grantee_id, domain, action, effect, layer, scope_kind, scope_node_id, source_label, created_by
		) VALUES (
			gen_random_uuid(), $1, 'user', $2, 'disputes', 'approve', 'allow', 'direct', 'company', $1, 'desk-test', $2
		)
	`, seed.CompanyID, seed.EditorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM permissions.rules WHERE source_label = 'desk-test'`)
	})
	status, same := c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{"to": "resolved", "reason": "اعتماد"}, editor)
	if status != 403 || same["code"] != "separation_of_duties" {
		t.Fatalf("proposer approved the decision: %d %v", status, same)
	}
	status, early := c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{"to": "resolved", "reason": "اعتماد"}, reviewer)
	if status != 422 || early["code"] != "nothing_collected" {
		t.Fatalf("compensation without collection: %d %v", status, early)
	}
	status, still := c.getAuth("/api/v1/disputes/"+caseID, reviewer)
	if status != 200 || still["status"] != "proposed" || still["financial_complete"] != false {
		t.Fatalf("failed compensation completed the case: %d %v", status, still)
	}
	status, attempt := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{"method": "mada", "amount_minor": 80000}, client, "pay-"+tag)
	if status != 201 || attempt["success"] != false {
		t.Fatalf("attempt: %d %v", status, attempt)
	}
	collected, err := api.Finance.ApplyProviderEvent(ctx, finance.ProviderEvent{ID: "desk-" + tag, AttemptID: str(attempt, "id"), Amount: 80000, Currency: "SAR", Outcome: "captured"})
	if err != nil || !collected.Success {
		t.Fatalf("capture: %v %v", collected, err)
	}
	journals, err := api.Finance.JournalCount(ctx, stayID)
	if err != nil {
		t.Fatal(err)
	}
	status, decided := c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{"to": "resolved", "reason": "اعتماد"}, reviewer)
	if status != 200 || decided["status"] != "resolved" || decided["financial_complete"] != false || decided["money_moved"] != false || decided["refund_success"] != false || decided["execution_status"] != "refund_requested" || num(decided, "hold_amount_minor") != 1000 || decided["other_funds_frozen"] != false {
		t.Fatalf("decision moved money: %d %v", status, decided)
	}
	if n, err := api.Finance.JournalCount(ctx, stayID); err != nil || n != journals {
		t.Fatalf("compensation posted a ledger movement: %d %v", n, err)
	}
	var refundStatus string
	var refundSuccess bool
	if err := pool.QueryRow(ctx, `SELECT status FROM finance.refunds WHERE id = $1`, str(decided, "refund_id")).Scan(&refundStatus); err != nil {
		t.Fatal(err)
	}
	if refundStatus != "under_review" {
		t.Fatalf("refund status: %s", refundStatus)
	}
	_ = refundSuccess
	status, secret := c.post("/api/v1/disputes/"+caseID+"/notes", map[string]any{"visibility": "internal", "body": "لا يراها العميل"}, editor)
	if status != 201 {
		t.Fatalf("staff note: %d %v", status, secret)
	}
	status, clientCase := c.getAuth("/api/v1/account/disputes/"+caseID, client)
	for _, raw := range asList(t, clientCase["notes"]) {
		if asMap(t, raw)["body"] == "لا يراها العميل" || asMap(t, raw)["visibility"] == "internal" {
			t.Fatalf("client saw an internal dispute note: %v", clientCase)
		}
	}
	status, closed := c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{"to": "closed", "reason": "إغلاق"}, reviewer)
	if status != 422 || closed["code"] != "notice_required" {
		t.Fatalf("close without notice: %d %v", status, closed)
	}
	status, notice := c.post("/api/v1/disputes/"+caseID+"/notice", map[string]any{"body": "القرار ظاهر داخل القضية ولم يُرسل خارج شمال."}, editor)
	if status != 201 || notice["external_delivery"] != "not_sent" {
		t.Fatalf("notice: %d %v", status, notice)
	}
	status, closed = c.post("/api/v1/disputes/"+caseID+"/move", map[string]any{"to": "closed", "reason": "إغلاق بعد الإشعار"}, reviewer)
	if status != 200 || closed["status"] != "closed" || closed["financial_complete"] != false || closed["money_moved"] != false {
		t.Fatalf("close looked financially complete: %d %v", status, closed)
	}

	if _, err := pool.Exec(ctx, `UPDATE stays.bookings SET arrival = CURRENT_DATE - 1, departure = CURRENT_DATE + 2 WHERE id = $1`, stayID); err != nil {
		t.Fatal(err)
	}
	status, urgent := c.post("/api/v1/account/disputes", map[string]any{
		"subject_kind": "booking", "subject_id": stayID, "type": "entry_failure",
		"description": "تعذر الدخول", "occurred_at": time.Now().UTC().Format(time.RFC3339),
	}, client)
	if status != 201 || urgent["priority"] != "critical" || urgent["sla_adopted"] != false || urgent["contractual"] != false || urgent["status"] != "open" {
		t.Fatalf("urgent case: %d %v", status, urgent)
	}
}

func move(t *testing.T, c *apiClient, cookie, id, to, reason string) {
	t.Helper()
	status, item := c.post("/api/v1/disputes/"+id+"/move", map[string]any{"to": to, "reason": reason}, cookie)
	if status != 200 || item["status"] != to {
		t.Fatalf("move %s: %d %v", to, status, item)
	}
}

func inboxItem(t *testing.T, body map[string]any, id string) map[string]any {
	t.Helper()
	for _, raw := range asList(t, body["items"]) {
		row := asMap(t, raw)
		if row["id"] == id {
			return row
		}
	}
	t.Fatalf("conversation %s missing", id)
	return nil
}

func (c *apiClient) upload(path, cookie, filename, contentType string, body []byte) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	part, err := form.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename)},
		"Content-Type":        {contentType},
	})
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := part.Write(body); err != nil {
		c.t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		c.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Shamal-Request", "1")
	req.Header.Set("Accept-Language", "en")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "shamal_session", Value: cookie})
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		c.t.Fatal(err)
	}
	return res.StatusCode, payload
}
