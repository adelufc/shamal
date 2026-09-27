package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"shamal/service/internal/catalog"
	"shamal/service/internal/clients"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
	"shamal/service/internal/stays"
)

func (a *API) stayQuote(w http.ResponseWriter, r *http.Request) {
	source, priced, issues, err := a.stayPrice(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if len(issues) > 0 {
		writeErr(w, r, &platform.Error{Code: "validation", HTTP: 422, Fields: stayIssues(issues)})
		return
	}
	policy, err := a.Stays.Policy(r.Context(), source.Transaction)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	available, err := a.Stays.Available(r.Context(), source.UnitID, priced.Arrival, priced.Departure)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, stayQuoteBody(source, priced, policy, available))
}

func (a *API) accountStay(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.accountStays(w, r)
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
	if refusedStayFields(raw) {
		writeErr(w, r, platform.E("commercial_terms_unapproved", 422))
		return
	}
	var body struct {
		ListingID       string   `json:"listing_id"`
		Arrival         string   `json:"arrival"`
		Departure       string   `json:"departure"`
		Guests          int      `json:"guests"`
		QuotedDue       int64    `json:"quoted_stay_due_minor"`
		PolicyVersion   int      `json:"policy_version"`
		AcceptPolicy    bool     `json:"accept_policy"`
		DiscountCodes   []string `json:"discount_codes"`
		DiscountPercent *int     `json:"discount_percent"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	if len(body.DiscountCodes) > 1 {
		writeErr(w, r, platform.E("one_offer_only", 422))
		return
	}
	if len(body.DiscountCodes) == 1 || body.DiscountPercent != nil {
		writeErr(w, r, platform.E("discount_unapproved", 422))
		return
	}
	source, err := a.stayListing(r, body.ListingID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	profile, err := a.Clients.ProfileByUser(r.Context(), user.ID)
	if err != nil {
		if appErr := platform.As(err); appErr == nil || appErr.Code != "not_found" {
			writeErr(w, r, err)
			return
		}
		profile, err = a.Clients.EnsureProfile(r.Context(), user.ID, requestID(r), clients.Input{
			CompanyID: seed.CompanyID, UserID: user.ID, DisplayName: user.DisplayName, Email: user.Email,
		})
		if err != nil {
			writeErr(w, r, err)
			return
		}
	}
	item, err := a.Stays.Hold(r.Context(), stays.HoldRequest{
		ActorID: user.ID, ProfileID: profile.ID, RequestID: requestID(r),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Listing:        stayListing(source),
		Arrival:        mustParse(body.Arrival), Departure: mustParse(body.Departure),
		Guests: body.Guests, QuotedDue: body.QuotedDue, PolicyVersion: body.PolicyVersion, AcceptPolicy: body.AcceptPolicy,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) accountStays(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	items, err := a.Stays.Mine(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) confirmStay(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	item, err := a.Stays.Confirm(r.Context(), user.ID, requestID(r), r.PathValue("id"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) stayPrice(r *http.Request) (catalog.StaySource, stays.Breakdown, []string, error) {
	query := r.URL.Query()
	listingID := strings.TrimSpace(query.Get("listing_id"))
	if listingID == "" {
		return catalog.StaySource{}, stays.Breakdown{}, nil, platform.E("not_found", 404)
	}
	source, err := a.stayListing(r, listingID)
	if err != nil {
		return catalog.StaySource{}, stays.Breakdown{}, nil, err
	}
	arrival, arrivalOK := stays.ParseDate(query.Get("arrival"))
	departure, departureOK := stays.ParseDate(query.Get("departure"))
	if !arrivalOK || !departureOK {
		return source, stays.Breakdown{}, []string{"invalid_dates"}, nil
	}
	guests := 1
	if raw := strings.TrimSpace(query.Get("guests")); raw != "" {
		value, err := parsePositive(raw)
		if err != nil {
			return source, stays.Breakdown{}, []string{"invalid_guests"}, nil
		}
		guests = value
	}
	priced, issues := stays.Price(stays.Input{
		Transaction: source.Transaction, Arrival: arrival, Departure: departure,
		Guests: guests, Capacity: source.Capacity, UnitMinor: source.PriceMinor, Now: time.Now().UTC(),
	})
	return source, priced, issues, nil
}

func (a *API) stayListing(r *http.Request, id string) (catalog.StaySource, error) {
	source, err := a.Cat.StaySource(r.Context(), id)
	if err != nil {
		return catalog.StaySource{}, err
	}
	if source.Status != "published" {
		return catalog.StaySource{}, platform.E("listing_unpublished", 422)
	}
	if source.Transaction != "daily" && source.Transaction != "monthly" {
		return catalog.StaySource{}, platform.E("invalid_transaction", 422)
	}
	published, err := a.Geo.PublishedIDs(r.Context(), seed.CityID)
	if err != nil {
		return catalog.StaySource{}, err
	}
	found := false
	for _, item := range published {
		if item == source.NeighborhoodID {
			found = true
			break
		}
	}
	if !found {
		return catalog.StaySource{}, platform.E("neighborhood_unpublished", 422)
	}
	return source, nil
}

func stayListing(source catalog.StaySource) stays.Listing {
	return stays.Listing{
		ID: source.ID, UnitID: source.UnitID, CompanyID: source.CompanyID, NeighborhoodID: source.NeighborhoodID,
		Transaction: source.Transaction, Currency: source.Currency, PriceMinor: source.PriceMinor,
		AddressAR: source.AddressAR, AddressEN: source.AddressEN, TitleAR: source.TitleAR, TitleEN: source.TitleEN,
		Capacity: source.Capacity,
	}
}

func stayQuoteBody(source catalog.StaySource, priced stays.Breakdown, policy stays.Policy, available bool) map[string]any {
	return map[string]any{
		"listing_id": source.ID, "transaction_type": priced.Transaction,
		"arrival": priced.Arrival.Format("2006-01-02"), "departure": priced.Departure.Format("2006-01-02"),
		"nights": priced.Nights, "months": priced.Months, "guests": priced.Guests,
		"currency": "SAR", "base_minor": priced.BaseMinor, "discount_minor": priced.DiscountMinor,
		"service_minor": priced.ServiceMinor, "tax_minor": priced.TaxMinor, "deposit_minor": priced.DepositMinor,
		"stay_due_minor": priced.StayDueMinor, "collect_minor": priced.CollectMinor, "revenue_minor": priced.RevenueMinor,
		"lines": priced.Lines, "available": available, "expires_at": priced.ExpiresAt,
		"policy": policy, "non_contractual": true, "payment_status": "not_collected",
		"commission_adopted": false, "deposit_is_revenue": false,
		"title_ar": source.TitleAR, "title_en": source.TitleEN,
		"address_ar": source.AddressAR, "address_en": source.AddressEN,
		"hold_minutes": stays.HoldMinutes, "quote_minutes": stays.QuoteMinutes,
		"includes_ar": "يشمل سعر الإعلان المنشور للمدة فقط. لا عمولة، ولا ضريبة معتمدة، ولا رسم خدمة معتمد. التأمين غير محصل؛ ولو وُجد فهو التزام قابل للرد وليس إيراداً.",
		"includes_en": "The published listing rate for this stay only. No commission, no adopted tax, and no adopted service fee. A deposit is not collected; if one were adopted it would be a refundable obligation, not revenue.",
	}
}

func refusedStayFields(raw []byte) bool {
	var bag map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bag); err != nil {
		return false
	}
	for _, key := range []string{"commission_bps", "commission_minor", "tax_rate", "tax_minor", "cancellation_fee_minor"} {
		if _, ok := bag[key]; ok {
			return true
		}
	}
	return false
}

func stayIssues(issues []string) map[string]string {
	out := map[string]string{}
	for _, issue := range issues {
		out[issue] = issue
	}
	return out
}

func mustParse(raw string) time.Time {
	value, _ := stays.ParseDate(raw)
	return value
}

func parsePositive(raw string) (int, error) {
	value := 0
	for _, item := range raw {
		if item < '0' || item > '9' {
			return 0, platform.E("invalid_guests", 422)
		}
		value = value*10 + int(item-'0')
	}
	if value < 1 {
		return 0, platform.E("invalid_guests", 422)
	}
	return value, nil
}
