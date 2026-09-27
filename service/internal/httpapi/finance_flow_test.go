package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"shamal/service/internal/finance"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func TestFinanceSlice(t *testing.T) {
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
	email := fmt.Sprintf("pay-%s@example.com", tag)
	status, registered := c.post("/api/v1/auth/register", map[string]any{"display_name": "دافع", "email": email, "password": seed.Password}, "")
	if status != 201 {
		t.Fatalf("register: %d %v", status, registered)
	}
	client := c.loginClient(email, seed.Password)
	listingID, _ := publishStayListing(t, c, editor, reviewer, "pay"+tag, "daily", 40000)
	stayID := confirmStay(t, c, client, listingID, "2027-12-01", "2027-12-03", 80000, "pay-"+tag)

	status, card := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{
		"method": "mada", "pan": "4111111111111111", "cvv": "123",
	}, client, "card-"+tag)
	if status != 422 || card["code"] != "card_data_refused" {
		t.Fatalf("card: %d %v", status, card)
	}
	var cardColumns int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'finance' AND column_name IN ('pan','card_number','cvv','cvc')
	`).Scan(&cardColumns); err != nil {
		t.Fatal(err)
	}
	if cardColumns != 0 {
		t.Fatal("card columns exist")
	}
	status, priced := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{
		"method": "mada", "commission_bps": 250,
	}, client, "fee-"+tag)
	if status != 422 || priced["code"] != "commercial_terms_unapproved" {
		t.Fatalf("commission: %d %v", status, priced)
	}
	status, wrong := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{
		"method": "mada", "amount_minor": 1,
	}, client, "amount-"+tag)
	if status != 422 || wrong["code"] != "amount_mismatch" {
		t.Fatalf("amount: %d %v", status, wrong)
	}
	status, attempt := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{"method": "mada"}, client, "try-"+tag)
	if status != 201 || attempt["status"] != "pending_review" || attempt["success"] != false || attempt["decisive"] != false || attempt["provider"] != "test_adapter" || attempt["gateway_adopted"] != false {
		t.Fatalf("attempt: %d %v", status, attempt)
	}
	if attempt["label_en"] != "Pending verification" || num(attempt, "amount_minor") != 80000 {
		t.Fatalf("pending label: %v", attempt)
	}
	status, again := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{"method": "mada"}, client, "try-"+tag)
	if status != 201 || again["id"] != attempt["id"] || again["success"] != false {
		t.Fatalf("replay: %d %v", status, again)
	}
	status, returned := c.getAuth("/api/v1/account/payments/"+str(attempt, "id")+"/return", client)
	if status != 200 || returned["status"] != "pending_review" || returned["success"] != false || returned["receipt_id"] != nil && returned["receipt_id"] != "" {
		t.Fatalf("browser return changed the attempt: %d %v", status, returned)
	}
	status, proof := c.postKey("/api/v1/account/payments/"+str(attempt, "id")+"/proof", map[string]any{"note": "مرجع تحويل"}, client, "proof-"+tag)
	if status != 200 || proof["success"] != false || proof["proof_confirms"] != false || proof["status"] != "pending_review" {
		t.Fatalf("proof confirmed payment: %d %v", status, proof)
	}

	status, invoice := c.getAuth("/api/v1/account/stays/"+stayID+"/invoice", client)
	if status != 200 || num(invoice, "total_minor") != 80000 || invoice["seller_adopted"] != false {
		t.Fatalf("invoice: %d %v", status, invoice)
	}
	for _, raw := range asList(t, invoice["lines"]) {
		line := asMap(t, raw)
		if line["code"] == "tax" && num(line, "amount_minor") != 0 {
			t.Fatalf("tax was invented: %v", line)
		}
		if line["code"] == "deposit" && line["counts_as_stay"] != false {
			t.Fatal("deposit counted as stay revenue")
		}
	}
	status, edited := c.send(http.MethodPut, "/api/v1/account/stays/"+stayID+"/invoice", map[string]any{"total_minor": 1}, client)
	if status != 409 || edited["code"] != "invoice_immutable" {
		t.Fatalf("edit invoice: %d %v", status, edited)
	}
	status, notice := c.post("/api/v1/account/stays/"+stayID+"/invoice/corrections", map[string]any{"reason": "بيان ناقص"}, client)
	if status != 201 || num(asMap(t, notice["invoice"]), "total_minor") != 80000 {
		t.Fatalf("correction changed the invoice: %d %v", status, notice)
	}
	status, refund := c.postKey("/api/v1/account/stays/"+stayID+"/refunds", map[string]any{"amount_minor": 80000, "reason": "لم أصل"}, client, "refund-"+tag)
	if status != 422 || refund["code"] != "nothing_collected" {
		t.Fatalf("refund before collection: %d %v", status, refund)
	}
	balanced, err := api.Finance.LedgerBalanced(ctx, stayID)
	if err != nil || !balanced {
		t.Fatalf("ledger: %v %v", balanced, err)
	}
	journals, err := api.Finance.JournalCount(ctx, stayID)
	if err != nil || journals != 1 {
		t.Fatalf("journals before a provider result: %d %v", journals, err)
	}

	_, err = api.Finance.ApplyProviderEvent(ctx, finance.ProviderEvent{ID: "bad-" + tag, AttemptID: str(attempt, "id"), Amount: 1, Currency: "SAR", Outcome: "captured"})
	if err != nil {
		t.Fatal(err)
	}
	status, mismatched := c.getAuth("/api/v1/account/payments/"+str(attempt, "id")+"/return", client)
	if status != 200 || mismatched["status"] != "mismatch_review" || mismatched["success"] != false {
		t.Fatalf("mismatch: %d %v", status, mismatched)
	}
	var stayStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM stays.bookings WHERE id = $1`, stayID).Scan(&stayStatus); err != nil {
		t.Fatal(err)
	}
	if stayStatus != "confirmed" {
		t.Fatalf("mismatch changed the stay: %s", stayStatus)
	}
	if n, err := api.Finance.JournalCount(ctx, stayID); err != nil || n != 1 {
		t.Fatalf("mismatch posted a journal: %d %v", n, err)
	}

	status, second := c.postKey("/api/v1/account/stays/"+stayID+"/payments", map[string]any{"method": "visa"}, client, "try2-"+tag)
	if status != 201 || second["success"] != false {
		t.Fatalf("second attempt: %d %v", status, second)
	}
	collected, err := api.Finance.ApplyProviderEvent(ctx, finance.ProviderEvent{ID: "ok-" + tag, AttemptID: str(second, "id"), Amount: 80000, Currency: "SAR", Outcome: "captured"})
	if err != nil || !collected.Success || collected.ReceiptID == "" {
		t.Fatalf("capture: %v %v", collected, err)
	}
	replay, err := api.Finance.ApplyProviderEvent(ctx, finance.ProviderEvent{ID: "ok-" + tag, AttemptID: str(second, "id"), Amount: 80000, Currency: "SAR", Outcome: "captured"})
	if err != nil || replay.ID != collected.ID {
		t.Fatalf("duplicate event: %v %v", replay, err)
	}
	if n, err := api.Finance.JournalCount(ctx, stayID); err != nil || n != 2 {
		t.Fatalf("duplicate event added a journal: %d %v", n, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE finance.ledger_lines SET amount_minor = 1`); err == nil {
		t.Fatal("ledger line was updated")
	}

	status, asked := c.postKey("/api/v1/account/stays/"+stayID+"/refunds", map[string]any{"amount_minor": 80000, "reason": "إلغاء غير تعاقدي"}, client, "refund2-"+tag)
	if status != 201 || asked["success"] != false || asked["status"] != "under_review" {
		t.Fatalf("refund request: %d %v", status, asked)
	}
	status, listed := c.getAuth("/api/v1/account/stays/"+stayID+"/refunds", client)
	found := false
	for _, raw := range asList(t, listed["items"]) {
		row, _ := raw.(map[string]any)
		if row["id"] == asked["id"] && row["success"] == false && row["status"] == "under_review" {
			found = true
		}
	}
	if status != 200 || !found {
		t.Fatalf("refund list: %d %v", status, listed)
	}
	status, self := c.post("/api/v1/finance/refunds/"+str(asked, "id")+"/approve", map[string]any{}, client)
	if status != 403 {
		t.Fatalf("client approved a refund: %d %v", status, self)
	}
	status, first := c.post("/api/v1/finance/refunds/"+str(asked, "id")+"/approve", map[string]any{}, reviewer)
	if status != 200 || first["status"] != "under_review" || first["success"] != false || num(first, "approvals") != 1 {
		t.Fatalf("first approval: %d %v", status, first)
	}
	status, repeat := c.post("/api/v1/finance/refunds/"+str(asked, "id")+"/approve", map[string]any{}, reviewer)
	if status != 403 || repeat["code"] != "separation_of_duties" {
		t.Fatalf("same approver: %d %v", status, repeat)
	}
	status, secondOK := c.post("/api/v1/finance/refunds/"+str(asked, "id")+"/approve", map[string]any{}, manager)
	if status != 200 || secondOK["status"] != "approved" || secondOK["success"] != false {
		t.Fatalf("second approval looked like success: %d %v", status, secondOK)
	}
	status, extra := c.postKey("/api/v1/account/stays/"+stayID+"/refunds", map[string]any{"amount_minor": 1, "reason": "زيادة"}, client, "refund3-"+tag)
	if status != 422 || extra["code"] != "refund_exceeds_collected" {
		t.Fatalf("second refund: %d %v", status, extra)
	}

	nearID, _ := publishStayListing(t, c, editor, reviewer, "near"+tag, "daily", 40000)
	soon, until := soonStay()
	nearStay := confirmStay(t, c, client, nearID, soon, until, 40000, "near-"+tag)
	status, transfer := c.postKey("/api/v1/account/stays/"+nearStay+"/payments", map[string]any{"method": "bank_transfer"}, client, "xfer-"+tag)
	if status != 422 || transfer["code"] != "transfer_too_soon" {
		t.Fatalf("transfer window: %d %v", status, transfer)
	}
}

func confirmStay(t *testing.T, c *apiClient, client, listingID, arrival, departure string, due int64, key string) string {
	t.Helper()
	status, quote := c.get("/api/v1/public/stays/quote?listing_id=" + listingID + "&arrival=" + arrival + "&departure=" + departure + "&guests=1")
	if status != 200 {
		t.Fatalf("quote: %d %v", status, quote)
	}
	status, held := c.postKey("/api/v1/account/stays", map[string]any{
		"listing_id": listingID, "arrival": arrival, "departure": departure, "guests": 1,
		"quoted_stay_due_minor": due, "policy_version": 1, "accept_policy": true,
	}, client, "hold-"+key)
	if status != 201 {
		t.Fatalf("hold: %d %v", status, held)
	}
	status, confirmed := c.postKey("/api/v1/account/stays/"+str(held, "id")+"/confirm", map[string]any{}, client, "confirm-"+key)
	if status != 200 || confirmed["status"] != "confirmed" {
		t.Fatalf("confirm: %d %v", status, confirmed)
	}
	return str(confirmed, "id")
}

func soonStay() (string, string) {
	loc, err := time.LoadLocation("Asia/Riyadh")
	if err != nil {
		loc = time.FixedZone("AST", 3*60*60)
	}
	start := time.Now().In(loc).Add(20 * time.Hour)
	return start.Format("2006-01-02"), start.Add(24 * time.Hour).Format("2006-01-02")
}
