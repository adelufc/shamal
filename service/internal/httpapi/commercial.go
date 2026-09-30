package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"shamal/service/internal/audit"
	"shamal/service/internal/catalog"
	"shamal/service/internal/clients"
	"shamal/service/internal/deals"
	"shamal/service/internal/identity"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
	"shamal/service/internal/visits"
)

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		Phone       string `json:"phone"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if strings.TrimSpace(body.DisplayName) == "" || strings.TrimSpace(body.Email) == "" {
		writeErr(w, r, platform.E("name_required", 422))
		return
	}
	if !identity.PasswordAccepted(body.Password) {
		writeErr(w, r, platform.E("invalid_password", 422))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, r, platform.E("internal", 500))
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := uuid.NewString()
	if err := a.Identity.CreateClient(r.Context(), tx, id, seed.CompanyID, strings.TrimSpace(body.DisplayName), strings.TrimSpace(body.Email), string(hash)); err != nil {
		writeErr(w, r, mapUnique(err))
		return
	}
	if _, err := a.Clients.Use(tx).EnsureProfile(r.Context(), id, requestID(r), clients.Input{
		DisplayName: body.DisplayName, Email: body.Email, Phone: body.Phone, CompanyID: seed.CompanyID, UserID: id,
	}); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := audit.Record(r.Context(), tx, audit.Event{ActorID: id, Action: "auth.register", EntityType: "user", EntityID: id, CompanyID: seed.CompanyID, RequestID: requestID(r), Result: "ok"}); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

func (a *API) publicRequest(w http.ResponseWriter, r *http.Request) {
	in, err := readRequest(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	place, err := a.Geo.Get(r.Context(), in.NeighborhoodID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if place.Status != "published" {
		writeErr(w, r, platform.E("neighborhood_unpublished", 422))
		return
	}
	actor := ""
	if sess, user, err := a.sessionUser(r); err == nil && sess.Kind == "client" {
		actor = user.ID
		in.UserID = user.ID
		if in.Email == "" {
			in.Email = user.Email
		}
		if in.DisplayName == "" {
			in.DisplayName = user.DisplayName
		}
	}
	in.CompanyID = seed.CompanyID
	in.BranchID = seed.BranchID
	item, err := a.Clients.CreateRequest(r.Context(), actor, requestID(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) account(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	requests, err := a.Clients.Mine(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	visitItems, err := a.Visits.Mine(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	dealItems, err := a.Deals.Mine(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": user.ID, "display_name": user.DisplayName, "email": user.Email,
		"requests": requests, "visits": visitItems, "deals": dealItems,
	})
}

func (a *API) accountVisit(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	profile, err := a.Clients.ProfileByUser(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		ListingID      string    `json:"listing_id"`
		StartsAt       time.Time `json:"starts_at"`
		EndsAt         time.Time `json:"ends_at"`
		Attendees      int       `json:"attendees"`
		InstructionsAR string    `json:"instructions_ar"`
		InstructionsEN string    `json:"instructions_en"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	listing, err := a.publishedListing(r, body.ListingID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Visits.Create(r.Context(), user.ID, requestID(r), visits.Draft{
		CompanyID: seed.CompanyID, BranchID: seed.BranchID, ProfileID: profile.ID, ListingID: listing.ID,
		PropertyID: listing.PropertyID, UnitID: listing.UnitID, BrokerUserID: listing.BrokerUserID, HostUserID: listing.BrokerUserID,
		StartsAt: body.StartsAt, EndsAt: body.EndsAt, Attendees: body.Attendees,
		InstructionsAR: body.InstructionsAR, InstructionsEN: body.InstructionsEN,
	})
	a.writeVisit(w, r, 201, item, err)
}

func (a *API) accountCancelVisit(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	current, err := a.Visits.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	profile, err := a.Clients.ProfileByUser(r.Context(), user.ID)
	if err != nil || profile.ID != current.ProfileID {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Visits.Cancel(r.Context(), user.ID, requestID(r), current.ID, body.Reason, body.Version)
	a.writeVisit(w, r, 200, item, err)
}

func (a *API) staffRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.createStaffRequest(w, r)
		return
	}
	user, err := a.authorize(r, "clients", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, size := platform.Page(r.URL.Query().Get("page"), r.URL.Query().Get("page_size"))
	items, total, err := a.Clients.List(r.Context(), user.CompanyID, r.URL.Query().Get("stage"), r.URL.Query().Get("q"), r.URL.Query().Get("unassigned") == "1", page, size)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
}

func (a *API) createStaffRequest(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "clients", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	in, err := readRequest(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	place, err := a.Geo.Get(r.Context(), in.NeighborhoodID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if place.Status != "published" {
		writeErr(w, r, platform.E("neighborhood_unpublished", 422))
		return
	}
	in.CompanyID = user.CompanyID
	in.BranchID = seed.BranchID
	item, err := a.Clients.CreateRequest(r.Context(), user.ID, requestID(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) moveRequest(w http.ResponseWriter, r *http.Request) {
	current, err := a.Clients.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "clients", permissions.ActionUpdate, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version int    `json:"version"`
		To      string `json:"to"`
		Reason  string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Clients.Move(r.Context(), user.ID, requestID(r), current.ID, body.To, body.Reason, body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) assignRequest(w http.ResponseWriter, r *http.Request) {
	current, err := a.Clients.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "clients", permissions.ActionUpdate, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version    int    `json:"version"`
		AssigneeID string `json:"assignee_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Clients.Assign(r.Context(), user.ID, requestID(r), current.ID, body.AssigneeID, body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) convertRequest(w http.ResponseWriter, r *http.Request) {
	current, err := a.Clients.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "deals", permissions.ActionCreate, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version int  `json:"version"`
		Test    bool `json:"is_test"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if current.Stage != clients.StageAgreement {
		writeErr(w, r, platform.E("invalid_transition", 409))
		return
	}
	deal, err := a.Deals.Open(r.Context(), user.ID, requestID(r), current.ID, current.ProfileID, current.CompanyID, current.BranchID, current.TransactionType, current.NeighborhoodID, body.Test)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Clients.AttachDeal(r.Context(), current.ID, deal.ID, body.Version); err != nil {
		writeErr(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), a.Pool, audit.Event{ActorID: user.ID, Action: "request.converted", EntityType: "client_request", EntityID: current.ID, CompanyID: current.CompanyID, RequestID: requestID(r), Result: "ok", After: deal.ID})
	writeJSON(w, 201, deal)
}

func (a *API) staffVisits(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.createVisit(w, r)
		return
	}
	user, err := a.authorize(r, "visits", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, size := platform.Page(r.URL.Query().Get("page"), r.URL.Query().Get("page_size"))
	items, total, err := a.Visits.List(r.Context(), user.CompanyID, r.URL.Query().Get("status"), r.URL.Query().Get("view"), page, size)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
}

func (a *API) createVisit(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "visits", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		ProfileID      string    `json:"profile_id"`
		ListingID      string    `json:"listing_id"`
		StartsAt       time.Time `json:"starts_at"`
		EndsAt         time.Time `json:"ends_at"`
		Attendees      int       `json:"attendees"`
		InstructionsAR string    `json:"instructions_ar"`
		InstructionsEN string    `json:"instructions_en"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	listing, err := a.publishedListing(r, body.ListingID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Visits.Create(r.Context(), user.ID, requestID(r), visits.Draft{
		CompanyID: user.CompanyID, BranchID: seed.BranchID, ProfileID: body.ProfileID, ListingID: listing.ID,
		PropertyID: listing.PropertyID, UnitID: listing.UnitID, BrokerUserID: listing.BrokerUserID, HostUserID: listing.BrokerUserID,
		StartsAt: body.StartsAt, EndsAt: body.EndsAt, Attendees: body.Attendees,
		InstructionsAR: body.InstructionsAR, InstructionsEN: body.InstructionsEN,
	})
	a.writeVisit(w, r, 201, item, err)
}

func (a *API) brokerConfirm(w http.ResponseWriter, r *http.Request) {
	a.visitCommand(w, r, "visits", permissions.ActionUpdate, func(actor, id string, version int, reason string, when time.Time) (visits.Visit, error) {
		return a.Visits.BrokerConfirm(r.Context(), actor, requestID(r), id, version)
	})
}

func (a *API) ownerConfirm(w http.ResponseWriter, r *http.Request) {
	a.visitCommand(w, r, "visits", permissions.ActionApprove, func(actor, id string, version int, reason string, when time.Time) (visits.Visit, error) {
		return a.Visits.OwnerConfirm(r.Context(), actor, requestID(r), id, version, when)
	})
}

func (a *API) ownerReject(w http.ResponseWriter, r *http.Request) {
	current, err := a.Visits.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "visits", permissions.ActionApprove, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version  int       `json:"version"`
		Reason   string    `json:"reason"`
		StartsAt time.Time `json:"starts_at"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Visits.OwnerReject(r.Context(), user.ID, requestID(r), current.ID, body.Reason, body.Version, body.StartsAt)
	a.writeVisit(w, r, 200, item, err)
}

func (a *API) completeVisit(w http.ResponseWriter, r *http.Request) {
	current, err := a.Visits.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version    int    `json:"version"`
		Result     string `json:"result"`
		Note       string `json:"note"`
		Reason     string `json:"reason"`
		Correction bool   `json:"correction"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	action := permissions.ActionUpdate
	if body.Correction {
		action = permissions.ActionArchive
	}
	user, err := a.authorize(r, "visits", action, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Visits.Complete(r.Context(), user.ID, requestID(r), current.ID, body.Result, body.Note, body.Reason, body.Version, time.Now().UTC(), body.Correction)
	a.writeVisit(w, r, 200, item, err)
}

func (a *API) cancelVisit(w http.ResponseWriter, r *http.Request) {
	a.visitCommand(w, r, "visits", permissions.ActionUpdate, func(actor, id string, version int, reason string, when time.Time) (visits.Visit, error) {
		return a.Visits.Cancel(r.Context(), actor, requestID(r), id, reason, version)
	})
}

func (a *API) noShowVisit(w http.ResponseWriter, r *http.Request) {
	a.visitCommand(w, r, "visits", permissions.ActionUpdate, func(actor, id string, version int, reason string, when time.Time) (visits.Visit, error) {
		return a.Visits.NoShow(r.Context(), actor, requestID(r), id, reason, version)
	})
}

func (a *API) rescheduleVisit(w http.ResponseWriter, r *http.Request) {
	current, err := a.Visits.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "visits", permissions.ActionUpdate, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version  int       `json:"version"`
		Reason   string    `json:"reason"`
		StartsAt time.Time `json:"starts_at"`
		EndsAt   time.Time `json:"ends_at"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Visits.Reschedule(r.Context(), user.ID, requestID(r), current.ID, body.Reason, body.Version, body.StartsAt, body.EndsAt)
	a.writeVisit(w, r, 200, item, err)
}

func (a *API) visitCommand(w http.ResponseWriter, r *http.Request, domain string, action permissions.Action, fn func(actor, id string, version int, reason string, when time.Time) (visits.Visit, error)) {
	current, err := a.Visits.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, domain, action, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := fn(user.ID, current.ID, body.Version, body.Reason, time.Now().UTC())
	a.writeVisit(w, r, 200, item, err)
}

func (a *API) staffDeals(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "deals", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, size := platform.Page(r.URL.Query().Get("page"), r.URL.Query().Get("page_size"))
	items, total, err := a.Deals.List(r.Context(), user.CompanyID, r.URL.Query().Get("status"), page, size)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
}

func (a *API) moveDeal(w http.ResponseWriter, r *http.Request) {
	current, err := a.Deals.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "deals", permissions.ActionUpdate, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version     int    `json:"version"`
		To          string `json:"to"`
		Reason      string `json:"reason"`
		ListingID   string `json:"listing_id"`
		PropertyID  string `json:"property_id"`
		OwnerID     string `json:"owner_user_id"`
		BrokerID    string `json:"broker_user_id"`
		Currency    string `json:"currency"`
		AmountMinor *int64 `json:"amount_minor"`
		Commission  *int   `json:"commission_bps"`
		Tax         *int   `json:"tax_rate"`
		CancelFee   *int64 `json:"cancellation_fee_minor"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Commission != nil || body.Tax != nil || body.CancelFee != nil {
		writeErr(w, r, platform.E("commercial_terms_unapproved", 422))
		return
	}
	item, err := a.Deals.Move(r.Context(), user.ID, requestID(r), current.ID, body.To, body.Reason, body.Version, body.ListingID, body.PropertyID, body.OwnerID, body.BrokerID, body.Currency, body.AmountMinor)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) requestClose(w http.ResponseWriter, r *http.Request) {
	current, err := a.Deals.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "deals", permissions.ActionUpdate, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version      int       `json:"version"`
		AgreementRef string    `json:"agreement_ref"`
		AmountMinor  int64     `json:"amount_minor"`
		Currency     string    `json:"currency"`
		CloseDate    time.Time `json:"close_date"`
		Commission   *int      `json:"commission_bps"`
		Tax          *int      `json:"tax_rate"`
		CancelFee    *int64    `json:"cancellation_fee_minor"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Deals.RequestClose(r.Context(), user.ID, requestID(r), current.ID, body.Version, deals.CloseRequest{
		AgreementRef: body.AgreementRef, AmountMinor: body.AmountMinor, Currency: body.Currency, CloseDate: body.CloseDate,
		HasCommission: body.Commission != nil, HasTax: body.Tax != nil, HasCancelFee: body.CancelFee != nil,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) approveClose(w http.ResponseWriter, r *http.Request) {
	current, err := a.Deals.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "deals", permissions.ActionApprove, &permissions.Resource{CompanyID: current.CompanyID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Version int `json:"version"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Deals.ApproveClose(r.Context(), user.ID, requestID(r), current.ID, body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) publishedListing(r *http.Request, id string) (catalog.Listing, error) {
	item, err := a.Cat.GetListing(r.Context(), id)
	if err != nil {
		return catalog.Listing{}, err
	}
	if item.Status != "published" {
		return catalog.Listing{}, platform.E("listing_unpublished", 422)
	}
	return item, nil
}

func readRequest(r *http.Request) (clients.Input, error) {
	var body struct {
		DisplayName    string `json:"display_name"`
		Email          string `json:"email"`
		Phone          string `json:"phone"`
		Transaction    string `json:"transaction_type"`
		BudgetMinor    *int64 `json:"budget_minor"`
		Currency       string `json:"currency"`
		NeighborhoodID string `json:"neighborhood_id"`
		TimingNote     string `json:"timing_note"`
		SpecsAR        string `json:"specs_ar"`
		SpecsEN        string `json:"specs_en"`
		ContactMethod  string `json:"contact_method"`
	}
	if err := decode(r, &body); err != nil {
		return clients.Input{}, err
	}
	return clients.Input{
		DisplayName: body.DisplayName, Email: body.Email, Phone: body.Phone, Transaction: body.Transaction,
		BudgetMinor: body.BudgetMinor, Currency: body.Currency, NeighborhoodID: body.NeighborhoodID,
		TimingNote: body.TimingNote, SpecsAR: body.SpecsAR, SpecsEN: body.SpecsEN, ContactMethod: body.ContactMethod,
	}, nil
}

func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return platform.E("email_taken", 409)
	}
	return err
}
