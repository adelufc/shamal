package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"shamal/service/internal/finance"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func (a *API) stayPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.listStayPayments(w, r)
		return
	}
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	fields := stringFields(raw)
	if finance.RefusesCard(fields) || refusedCommercial(raw) {
		code := "card_data_refused"
		if refusedCommercial(raw) {
			code = "commercial_terms_unapproved"
		}
		writeErr(w, r, platform.E(code, 422))
		return
	}
	var body struct {
		Method string `json:"method"`
		Amount *int64 `json:"amount_minor"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	bill, err := a.stayBill(r, user.ID, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Finance.OpenAttempt(r.Context(), user.ID, requestID(r), r.Header.Get("Idempotency-Key"), bill, body.Method, body.Amount, fields)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Notify.PaymentReview(r.Context(), user.ID, item.ID, item.Status); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) listStayPayments(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if _, err := a.stayBill(r, user.ID, r.PathValue("id")); err != nil {
		writeErr(w, r, err)
		return
	}
	rows, err := a.Pool.Query(r.Context(), `SELECT id::text FROM finance.attempts WHERE stay_id = $1 AND actor_id = $2 ORDER BY created_at`, r.PathValue("id"), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]finance.Attempt, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			writeErr(w, r, err)
			return
		}
		item, err := a.Finance.GetAttempt(r.Context(), id, user.ID)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) paymentReturn(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	item, err := a.Finance.GetAttempt(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) paymentProof(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Finance.RecordProof(r.Context(), user.ID, r.PathValue("id"), body.Note)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) stayInvoice(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		writeErr(w, r, platform.E("invoice_immutable", 409))
		return
	}
	item, err := a.Finance.Invoice(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) correctInvoice(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	invoice, err := a.Finance.Invoice(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	before := invoice.TotalMinor
	if err := a.Finance.CorrectInvoice(r.Context(), user.ID, invoice.ID, body.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	invoice, err = a.Finance.Invoice(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if invoice.TotalMinor != before {
		writeErr(w, r, platform.E("invoice_immutable", 409))
		return
	}
	writeJSON(w, 201, map[string]any{"invoice": invoice, "correction": "notice", "total_minor": invoice.TotalMinor})
}

func (a *API) listStayRefunds(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if _, err := a.stayBill(r, user.ID, r.PathValue("id")); err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Finance.ListStayRefunds(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) stayRefund(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	if refusedCommercial(raw) {
		writeErr(w, r, platform.E("commercial_terms_unapproved", 422))
		return
	}
	var body struct {
		Amount int64  `json:"amount_minor"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	bill, err := a.stayBill(r, user.ID, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Finance.RequestRefund(r.Context(), user.ID, requestID(r), r.Header.Get("Idempotency-Key"), bill, body.Amount, body.Reason)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) cancelRefund(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	item, err := a.Finance.CancelRefund(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) staffRefunds(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authorize(r, "finance", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"}); err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Finance.ListRefunds(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) approveRefund(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "finance", permissions.ActionApprove, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Finance.ApproveRefund(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) stayBill(r *http.Request, actor, stayID string) (finance.StayBill, error) {
	booking, companyID, err := a.Stays.Owned(r.Context(), stayID, actor)
	if err != nil {
		return finance.StayBill{}, err
	}
	return finance.StayBill{
		ID: booking.ID, ActorID: actor, CompanyID: companyID, Transaction: booking.Transaction,
		Arrival: booking.Arrival, Status: booking.Status, Nights: booking.Nights, Months: booking.Months,
		StayDue: booking.StayDueMinor,
	}, nil
}

func stringFields(raw []byte) map[string]string {
	var bag map[string]any
	if err := json.Unmarshal(raw, &bag); err != nil {
		return nil
	}
	out := map[string]string{}
	for key, value := range bag {
		switch typed := value.(type) {
		case string:
			out[key] = typed
		default:
			out[key] = ""
		}
	}
	return out
}

func refusedCommercial(raw []byte) bool {
	var bag map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bag); err != nil {
		return false
	}
	for _, key := range []string{"commission_bps", "commission_minor", "tax_rate", "tax_minor", "cancellation_fee_minor"} {
		if _, ok := bag[strings.ToLower(key)]; ok {
			return true
		}
		if _, ok := bag[key]; ok {
			return true
		}
	}
	return false
}
