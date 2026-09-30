package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"shamal/service/internal/audit"
	"shamal/service/internal/catalog"
	"shamal/service/internal/clients"
	"shamal/service/internal/content"
	"shamal/service/internal/deals"
	"shamal/service/internal/disputes"
	"shamal/service/internal/files"
	"shamal/service/internal/finance"
	"shamal/service/internal/geography"
	"shamal/service/internal/identity"
	"shamal/service/internal/messages"
	"shamal/service/internal/notify"
	"shamal/service/internal/organization"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/reports"
	"shamal/service/internal/seed"
	"shamal/service/internal/stays"
	"shamal/service/internal/visits"
)

type API struct {
	Pool         *pgxpool.Pool
	Redis        *redis.Client
	Identity     identity.Store
	Org          organization.Store
	Perm         permissions.Store
	Geo          geography.Service
	Cat          catalog.Service
	Content      content.Service
	Clients      clients.Service
	Visits       visits.Service
	Deals        deals.Service
	Stays        stays.Service
	Finance      finance.Service
	Messages     messages.Service
	Disputes     disputes.Service
	Reports      reports.Service
	Notify       notify.Service
	Files        files.Local
	AllowApprox  bool
	PublicOrigin string
	dummyHash    []byte
}

type session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CompanyID string    `json:"company_id"`
	Kind      string    `json:"kind"`
	Remember  bool      `json:"remember"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
	StepUpAt  time.Time `json:"step_up_at"`
}

func New(pool *pgxpool.Pool, rdb *redis.Client, cfg platform.Config) *API {
	hash, _ := bcrypt.GenerateFromPassword([]byte("not-a-real-account"), bcrypt.DefaultCost)
	return &API{
		Pool: pool, Redis: rdb,
		Identity:     identity.Store{DB: pool},
		Org:          organization.Store{DB: pool},
		Perm:         permissions.Store{DB: pool},
		Geo:          geography.Service{Pool: pool, AllowApprox: cfg.AllowApprox},
		Cat:          catalog.Service{Pool: pool},
		Content:      content.Service{Pool: pool},
		Clients:      clients.Service{DB: pool},
		Visits:       visits.Service{Pool: pool},
		Deals:        deals.Service{Pool: pool},
		Stays:        stays.Service{Pool: pool},
		Finance:      finance.Service{Pool: pool},
		Messages:     messages.Service{Pool: pool},
		Disputes:     disputes.Service{Pool: pool, Files: files.Local{Root: cfg.StorageDir}, Finance: finance.Service{Pool: pool}},
		Reports:      reports.Service{Pool: pool},
		Notify:       notify.Service{Pool: pool},
		Files:        files.Local{Root: cfg.StorageDir},
		AllowApprox:  cfg.AllowApprox,
		PublicOrigin: cfg.PublicOrigin,
		dummyHash:    hash,
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/mfa", a.mfa)
	mux.HandleFunc("POST /api/v1/auth/step-up", a.stepUp)
	mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
	mux.HandleFunc("GET /api/v1/me", a.me)
	mux.HandleFunc("GET /api/v1/org/nodes", a.orgNodes)
	mux.HandleFunc("POST /api/v1/org/nodes", a.createNode)
	mux.HandleFunc("POST /api/v1/org/nodes/{id}/close", a.closeNode)
	mux.HandleFunc("GET /api/v1/org/structure", a.orgStructure)
	mux.HandleFunc("POST /api/v1/org/memberships", a.createMembership)
	mux.HandleFunc("GET /api/v1/geo/cities", a.cities)
	mux.HandleFunc("GET /api/v1/geo/neighborhoods", a.neighborhoods)
	mux.HandleFunc("POST /api/v1/geo/neighborhoods", a.createNeighborhood)
	mux.HandleFunc("GET /api/v1/geo/neighborhoods/{id}", a.neighborhood)
	mux.HandleFunc("PUT /api/v1/geo/neighborhoods/{id}", a.updateNeighborhood)
	mux.HandleFunc("POST /api/v1/geo/neighborhoods/{id}/submit", a.submitNeighborhood)
	mux.HandleFunc("POST /api/v1/geo/neighborhoods/{id}/request-changes", a.requestNeighborhoodChanges)
	mux.HandleFunc("POST /api/v1/geo/neighborhoods/{id}/publish", a.publishNeighborhood)
	mux.HandleFunc("POST /api/v1/geo/neighborhoods/{id}/suspend", a.suspendNeighborhood)
	mux.HandleFunc("POST /api/v1/geo/neighborhoods/{id}/reactivate", a.reactivateNeighborhood)
	mux.HandleFunc("GET /api/v1/catalog/property-types", a.propertyTypes)
	mux.HandleFunc("GET /api/v1/catalog/properties", a.properties)
	mux.HandleFunc("POST /api/v1/catalog/properties", a.createProperty)
	mux.HandleFunc("POST /api/v1/catalog/units", a.createUnit)
	mux.HandleFunc("POST /api/v1/catalog/listings", a.createListing)
	mux.HandleFunc("GET /api/v1/catalog/listings", a.listListings)
	mux.HandleFunc("GET /api/v1/catalog/listings/{id}", a.listing)
	mux.HandleFunc("POST /api/v1/catalog/listings/{id}/pending-change", a.pendingListingChange)
	mux.HandleFunc("POST /api/v1/catalog/listings/{id}/submit", a.listingMove("in_review", permissions.ActionUpdate))
	mux.HandleFunc("POST /api/v1/catalog/listings/{id}/approve", a.listingMove("approved", permissions.ActionApprove))
	mux.HandleFunc("POST /api/v1/catalog/listings/{id}/publish", a.listingMove("published", permissions.ActionPublish))
	mux.HandleFunc("POST /api/v1/permissions/preview", a.preview)
	mux.HandleFunc("GET /api/v1/permissions/account", a.permissionAccount)
	mux.HandleFunc("POST /api/v1/permissions/grants", a.createGrant)
	mux.HandleFunc("POST /api/v1/permissions/breaks", a.createBreak)
	mux.HandleFunc("GET /api/v1/public/home", a.home)
	mux.HandleFunc("GET /api/v1/public/content", a.publicContent)
	mux.HandleFunc("GET /api/v1/content/pages", a.contentPages)
	mux.HandleFunc("POST /api/v1/content/pages", a.createContentPage)
	mux.HandleFunc("GET /api/v1/content/pages/{id}", a.contentPage)
	mux.HandleFunc("PUT /api/v1/content/pages/{id}", a.updateContentPage)
	mux.HandleFunc("POST /api/v1/content/pages/{id}/submit", a.submitContentPage)
	mux.HandleFunc("POST /api/v1/content/pages/{id}/publish", a.publishContentPage)
	mux.HandleFunc("GET /api/v1/public/search", a.publicSearch)
	mux.HandleFunc("GET /api/v1/public/stays/quote", a.stayQuote)
	mux.HandleFunc("GET /api/v1/account/stays", a.accountStay)
	mux.HandleFunc("POST /api/v1/account/stays", a.accountStay)
	mux.HandleFunc("POST /api/v1/account/stays/{id}/confirm", a.confirmStay)
	mux.HandleFunc("GET /api/v1/account/stays/{id}/payments", a.stayPayment)
	mux.HandleFunc("POST /api/v1/account/stays/{id}/payments", a.stayPayment)
	mux.HandleFunc("GET /api/v1/account/payments/{id}/return", a.paymentReturn)
	mux.HandleFunc("POST /api/v1/account/payments/{id}/proof", a.paymentProof)
	mux.HandleFunc("GET /api/v1/account/stays/{id}/invoice", a.stayInvoice)
	mux.HandleFunc("PUT /api/v1/account/stays/{id}/invoice", a.stayInvoice)
	mux.HandleFunc("POST /api/v1/account/stays/{id}/invoice/corrections", a.correctInvoice)
	mux.HandleFunc("GET /api/v1/account/stays/{id}/refunds", a.listStayRefunds)
	mux.HandleFunc("POST /api/v1/account/stays/{id}/refunds", a.stayRefund)
	mux.HandleFunc("POST /api/v1/account/refunds/{id}/cancel", a.cancelRefund)
	mux.HandleFunc("GET /api/v1/finance/refunds", a.staffRefunds)
	mux.HandleFunc("POST /api/v1/finance/refunds/{id}/approve", a.approveRefund)
	mux.HandleFunc("GET /api/v1/account/messages/preview", a.accountMessagePreview)
	mux.HandleFunc("GET /api/v1/account/messages", a.accountMessages)
	mux.HandleFunc("POST /api/v1/account/messages", a.accountMessages)
	mux.HandleFunc("GET /api/v1/account/messages/{id}", a.accountMessageThread)
	mux.HandleFunc("POST /api/v1/account/messages/{id}", a.accountMessageThread)
	mux.HandleFunc("PUT /api/v1/account/messages/{id}", a.accountMessageThread)
	mux.HandleFunc("PATCH /api/v1/account/messages/{id}", a.accountMessageThread)
	mux.HandleFunc("POST /api/v1/account/messages/{id}/read", a.accountMessageRead)
	mux.HandleFunc("POST /api/v1/account/messages/{id}/report", a.accountMessageReport)
	mux.HandleFunc("GET /api/v1/messages/canned", a.staffCanned)
	mux.HandleFunc("GET /api/v1/messages", a.staffMessages)
	mux.HandleFunc("GET /api/v1/messages/{id}", a.staffMessageThread)
	mux.HandleFunc("POST /api/v1/messages/{id}", a.staffMessageThread)
	mux.HandleFunc("PUT /api/v1/messages/{id}", a.staffMessageThread)
	mux.HandleFunc("POST /api/v1/messages/{id}/assign", a.staffMessageAssign)
	mux.HandleFunc("POST /api/v1/messages/hide", a.staffMessageHide)
	mux.HandleFunc("GET /api/v1/account/disputes", a.accountDisputes)
	mux.HandleFunc("POST /api/v1/account/disputes", a.accountDisputes)
	mux.HandleFunc("GET /api/v1/account/disputes/{id}", a.accountDispute)
	mux.HandleFunc("POST /api/v1/account/disputes/{id}/evidence", a.accountDisputeEvidence)
	mux.HandleFunc("POST /api/v1/account/disputes/{id}/notes", a.accountDisputeNote)
	mux.HandleFunc("POST /api/v1/account/disputes/{id}/review", a.accountDisputeReview)
	mux.HandleFunc("GET /api/v1/disputes", a.staffDisputes)
	mux.HandleFunc("POST /api/v1/disputes", a.staffDisputes)
	mux.HandleFunc("GET /api/v1/disputes/{id}", a.staffDispute)
	mux.HandleFunc("POST /api/v1/disputes/{id}/move", a.staffDisputeMove)
	mux.HandleFunc("POST /api/v1/disputes/{id}/assign", a.staffDisputeAssign)
	mux.HandleFunc("POST /api/v1/disputes/{id}/notes", a.staffDisputeNote)
	mux.HandleFunc("POST /api/v1/disputes/{id}/notice", a.staffDisputeNotice)
	mux.HandleFunc("POST /api/v1/disputes/{id}/evidence", a.staffDisputeEvidence)
	mux.HandleFunc("GET /api/v1/public/neighborhoods/{slug}", a.publicNeighborhood)
	mux.HandleFunc("POST /api/v1/public/requests", a.publicRequest)
	mux.HandleFunc("GET /api/v1/account", a.account)
	mux.HandleFunc("POST /api/v1/account/visits", a.accountVisit)
	mux.HandleFunc("POST /api/v1/account/visits/{id}/cancel", a.accountCancelVisit)
	mux.HandleFunc("GET /api/v1/clients/requests", a.staffRequests)
	mux.HandleFunc("POST /api/v1/clients/requests", a.staffRequests)
	mux.HandleFunc("POST /api/v1/clients/requests/{id}/move", a.moveRequest)
	mux.HandleFunc("POST /api/v1/clients/requests/{id}/assign", a.assignRequest)
	mux.HandleFunc("POST /api/v1/clients/requests/{id}/convert", a.convertRequest)
	mux.HandleFunc("GET /api/v1/visits", a.staffVisits)
	mux.HandleFunc("POST /api/v1/visits", a.staffVisits)
	mux.HandleFunc("POST /api/v1/visits/{id}/broker-confirm", a.brokerConfirm)
	mux.HandleFunc("POST /api/v1/visits/{id}/owner-confirm", a.ownerConfirm)
	mux.HandleFunc("POST /api/v1/visits/{id}/owner-reject", a.ownerReject)
	mux.HandleFunc("POST /api/v1/visits/{id}/complete", a.completeVisit)
	mux.HandleFunc("POST /api/v1/visits/{id}/cancel", a.cancelVisit)
	mux.HandleFunc("POST /api/v1/visits/{id}/no-show", a.noShowVisit)
	mux.HandleFunc("POST /api/v1/visits/{id}/reschedule", a.rescheduleVisit)
	mux.HandleFunc("GET /api/v1/deals", a.staffDeals)
	mux.HandleFunc("POST /api/v1/deals/{id}/move", a.moveDeal)
	mux.HandleFunc("POST /api/v1/deals/{id}/request-close", a.requestClose)
	mux.HandleFunc("POST /api/v1/deals/{id}/approve-close", a.approveClose)
	mux.HandleFunc("POST /api/v1/files", a.upload)
	mux.HandleFunc("GET /api/v1/files/{area}/{key}", a.download)
	mux.HandleFunc("GET /api/v1/reports/{code}", a.report)
	mux.HandleFunc("POST /api/v1/reports/{code}/export", a.reportExport)
	mux.HandleFunc("GET /api/v1/account/searches", a.listSearches)
	mux.HandleFunc("POST /api/v1/account/searches", a.saveSearch)
	mux.HandleFunc("POST /api/v1/account/searches/{id}/mute", a.muteSearch)
	mux.HandleFunc("POST /api/v1/account/notifications/mute", a.muteChannel)
	mux.HandleFunc("GET /api/v1/account/notifications", a.notifications)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if r.Header.Get("X-Shamal-Request") != "1" {
				writeErr(w, r, platform.E("csrf", 403))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := "up"
	if err := a.Pool.Ping(ctx); err != nil {
		db = "down"
	}
	cache := "up"
	if err := a.Redis.Ping(ctx).Err(); err != nil {
		cache = "down"
	}
	status := http.StatusOK
	if db != "up" || cache != "up" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]string{"postgres": db, "redis": cache, "storage": "local"})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Context  string `json:"context"`
		Remember bool   `json:"remember"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Context == "" {
		body.Context = "staff"
	}
	key := "ratelimit:login:" + strings.ToLower(body.Email)
	count, _ := a.Redis.Incr(r.Context(), key).Result()
	if count == 1 {
		_ = a.Redis.Expire(r.Context(), key, 15*time.Minute).Err()
	}
	user, err := a.Identity.FindByEmail(r.Context(), body.Email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(body.Password))
		writeErr(w, r, platform.E("invalid_credentials", 401))
		return
	}
	now := time.Now().UTC()
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		writeErr(w, r, platform.E("invalid_credentials", 401))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)) != nil {
		attempts, first, locked := identity.NextLockout(user.FailedAttempts, user.FirstFailedAt, now)
		_ = a.Identity.SaveFailure(r.Context(), user.ID, attempts, first, locked)
		_ = audit.Record(r.Context(), a.Pool, audit.Event{ActorID: user.ID, Action: "auth.login", EntityType: "user", EntityID: user.ID, Result: "denied", RequestID: requestID(r)})
		writeErr(w, r, platform.E("invalid_credentials", 401))
		return
	}
	if user.Status != "active" {
		writeErr(w, r, platform.E("invalid_credentials", 401))
		return
	}
	if body.Context == "staff" {
		memberships, err := a.Org.Memberships(r.Context(), user.ID)
		if err != nil || len(activeMemberships(memberships, now)) == 0 {
			writeErr(w, r, platform.E("invalid_credentials", 401))
			return
		}
	}
	_ = a.Identity.SaveSuccess(r.Context(), user.ID)
	if user.MFARequired {
		challenge := uuid.NewString()
		payload, _ := json.Marshal(map[string]any{"user_id": user.ID, "context": body.Context, "remember": body.Remember})
		if err := a.Redis.Set(r.Context(), "mfa:"+challenge, payload, 5*time.Minute).Err(); err != nil {
			writeErr(w, r, platform.E("unavailable", 503))
			return
		}
		writeJSON(w, 200, map[string]any{"mfa_required": true, "challenge_id": challenge})
		return
	}
	a.startSession(w, r, user, body.Context, body.Remember)
}

func (a *API) mfa(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	raw, err := a.Redis.Get(r.Context(), "mfa:"+body.ChallengeID).Bytes()
	if err != nil {
		writeErr(w, r, platform.E("mfa_invalid", 401))
		return
	}
	var payload struct {
		UserID   string `json:"user_id"`
		Context  string `json:"context"`
		Remember bool   `json:"remember"`
	}
	_ = json.Unmarshal(raw, &payload)
	user, err := a.Identity.FindByID(r.Context(), payload.UserID)
	if err != nil || !totp.Validate(body.Code, user.MFASecret) {
		writeErr(w, r, platform.E("mfa_invalid", 401))
		return
	}
	_ = a.Redis.Del(r.Context(), "mfa:"+body.ChallengeID).Err()
	a.startSession(w, r, user, payload.Context, payload.Remember)
}

func (a *API) stepUp(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if !totp.Validate(body.Code, user.MFASecret) {
		writeErr(w, r, platform.E("mfa_invalid", 401))
		return
	}
	sess.StepUpAt = time.Now().UTC()
	if err := a.saveSession(r, sess); err != nil {
		writeErr(w, r, platform.E("unavailable", 503))
		return
	}
	writeJSON(w, 200, map[string]any{"step_up": true})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("shamal_session"); err == nil {
		_ = a.Redis.Del(r.Context(), "session:"+cookie.Value).Err()
	}
	http.SetCookie(w, &http.Cookie{Name: "shamal_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	actions := a.actionMap(r, user, sess)
	writeJSON(w, 200, map[string]any{
		"id": user.ID, "display_name": user.DisplayName, "email": user.Email,
		"company_id": user.CompanyID, "context": sess.Kind,
		"step_up": time.Since(sess.StepUpAt) <= 5*time.Minute,
		"actions": actions,
	})
}

func (a *API) startSession(w http.ResponseWriter, r *http.Request, user identity.User, kind string, remember bool) {
	now := time.Now().UTC()
	sess := session{ID: uuid.NewString(), UserID: user.ID, CompanyID: user.CompanyID, Kind: kind, Remember: remember, CreatedAt: now, LastSeen: now, StepUpAt: now}
	if err := a.saveSession(r, sess); err != nil {
		writeErr(w, r, platform.E("unavailable", 503))
		return
	}
	sum := sha256.Sum256([]byte(r.UserAgent()))
	_ = a.Identity.AddDevice(r.Context(), uuid.NewString(), user.ID, "browser", hex.EncodeToString(sum[:8]), now)
	_ = audit.Record(r.Context(), a.Pool, audit.Event{ActorID: user.ID, Action: "auth.login", EntityType: "user", EntityID: user.ID, CompanyID: user.CompanyID, RequestID: requestID(r), Result: "ok"})
	http.SetCookie(w, &http.Cookie{Name: "shamal_session", Value: sess.ID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]any{"mfa_required": false, "user_id": user.ID})
}

func (a *API) saveSession(r *http.Request, sess session) error {
	raw, _ := json.Marshal(sess)
	ttl := identity.IdleTTL(sess.Kind, sess.Remember, sess.CreatedAt, time.Now().UTC())
	if ttl <= 0 {
		return platform.E("session_expired", 401)
	}
	return a.Redis.Set(r.Context(), "session:"+sess.ID, raw, ttl).Err()
}

func (a *API) sessionUser(r *http.Request) (session, identity.User, error) {
	cookie, err := r.Cookie("shamal_session")
	if err != nil {
		return session{}, identity.User{}, platform.E("session_expired", 401)
	}
	raw, err := a.Redis.Get(r.Context(), "session:"+cookie.Value).Bytes()
	if err != nil {
		return session{}, identity.User{}, platform.E("session_expired", 401)
	}
	var sess session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return session{}, identity.User{}, platform.E("session_expired", 401)
	}
	now := time.Now().UTC()
	if !identity.SessionOpen(sess.Kind, sess.Remember, sess.CreatedAt, sess.LastSeen, now) {
		_ = a.Redis.Del(r.Context(), "session:"+sess.ID).Err()
		return session{}, identity.User{}, platform.E("session_expired", 401)
	}
	sess.LastSeen = now
	if err := a.saveSession(r, sess); err != nil {
		return session{}, identity.User{}, platform.E("unavailable", 503)
	}
	user, err := a.Identity.FindByID(r.Context(), sess.UserID)
	if err != nil {
		return session{}, identity.User{}, platform.E("session_expired", 401)
	}
	return sess, user, nil
}

func (a *API) authorize(r *http.Request, domain string, action permissions.Action, resource *permissions.Resource) (identity.User, error) {
	sess, user, err := a.sessionUser(r)
	if err != nil {
		return identity.User{}, err
	}
	decision, err := a.decide(r, sess, user, domain, action, resource)
	if err != nil {
		return identity.User{}, err
	}
	if !decision.Allowed {
		return identity.User{}, &platform.Error{Code: "forbidden", HTTP: 403, Fields: map[string]string{"reason": decision.Code, "layer": string(decision.Layer)}}
	}
	return user, nil
}

func (a *API) decide(r *http.Request, sess session, user identity.User, domain string, action permissions.Action, resource *permissions.Resource) (permissions.Decision, error) {
	nodes, err := a.Org.List(r.Context(), user.CompanyID)
	if err != nil {
		return permissions.Decision{}, err
	}
	memberships, err := a.Org.Memberships(r.Context(), user.ID)
	if err != nil {
		return permissions.Decision{}, err
	}
	permNodes := make([]permissions.Node, 0, len(nodes))
	nodeIDs := make([]string, 0)
	for _, node := range nodes {
		permNodes = append(permNodes, permissions.Node{ID: node.ID, ParentID: node.ParentID})
		nodeIDs = append(nodeIDs, node.ID)
	}
	rules, breaks, err := a.Perm.Load(r.Context(), user.ID, nodeIDs)
	if err != nil {
		return permissions.Decision{}, err
	}
	now := time.Now().UTC()
	active := make([]permissions.Membership, 0)
	for _, item := range memberships {
		active = append(active, permissions.Membership{NodeID: item.NodeID, Starts: item.StartsAt, Ends: item.EndsAt})
	}
	collected := permissions.Collect(user.ID, permNodes, active, rules, breaks, now)
	subject := permissions.Subject{UserID: user.ID, CompanyID: user.CompanyID, Status: user.Status, StepUp: now.Sub(sess.StepUpAt) <= 5*time.Minute}
	return permissions.Decide(now, subject, domain, action, resource, collected, false), nil
}

func (a *API) actionMap(r *http.Request, user identity.User, sess session) map[string][]string {
	domains := []string{"geography", "catalog", "organization", "permissions", "files", "identity", "clients", "visits", "deals", "finance", "messages", "disputes", "reports"}
	actions := []permissions.Action{permissions.ActionView, permissions.ActionCreate, permissions.ActionUpdate, permissions.ActionApprove, permissions.ActionPublish, permissions.ActionArchive, permissions.ActionDeleteDraft}
	resource := &permissions.Resource{CompanyID: user.CompanyID, CreatedBy: "someone-else"}
	out := map[string][]string{}
	for _, domain := range domains {
		for _, action := range actions {
			decision, err := a.decide(r, sess, user, domain, action, resource)
			if err == nil && decision.Allowed {
				out[domain] = append(out[domain], string(action))
			}
		}
	}
	return out
}

func (a *API) preview(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		UserID   string `json:"user_id"`
		Domain   string `json:"domain"`
		Action   string `json:"action"`
		Resource struct {
			CompanyID    string `json:"company_id"`
			BranchID     string `json:"branch_id"`
			DepartmentID string `json:"department_id"`
			TeamID       string `json:"team_id"`
			OwnerID      string `json:"owner_id"`
			AssigneeID   string `json:"assignee_id"`
			CreatedBy    string `json:"created_by"`
		} `json:"resource"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	target := user
	targetSess := sess
	if body.UserID != "" && body.UserID != user.ID {
		if _, err := a.authorize(r, "permissions", permissions.ActionView, &permissions.Resource{CompanyID: user.CompanyID, CreatedBy: "system"}); err != nil {
			writeErr(w, r, err)
			return
		}
		target, err = a.Identity.FindByID(r.Context(), body.UserID)
		if err != nil {
			writeErr(w, r, platform.E("not_found", 404))
			return
		}
	}
	resource := &permissions.Resource{
		CompanyID: body.Resource.CompanyID, BranchID: body.Resource.BranchID, DepartmentID: body.Resource.DepartmentID,
		TeamID: body.Resource.TeamID, OwnerID: body.Resource.OwnerID, AssigneeID: body.Resource.AssigneeID, CreatedBy: body.Resource.CreatedBy,
	}
	if resource.CompanyID == "" {
		resource.CompanyID = target.CompanyID
	}
	decision, err := a.decide(r, targetSess, target, body.Domain, permissions.Action(body.Action), resource)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"allowed": decision.Allowed, "layer": decision.Layer, "code": decision.Code, "deny_source": decision.DenySource, "at": time.Now().UTC()})
}

func (a *API) orgNodes(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "organization", permissions.ActionView, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	nodes, err := a.Org.List(r.Context(), user.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": nodes})
}

func (a *API) createNode(w http.ResponseWriter, r *http.Request) {
	var body organization.Node
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "organization", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	body.CompanyID = user.CompanyID
	nodes, err := a.Org.List(r.Context(), user.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	parent := map[string]string{}
	for _, node := range nodes {
		parent[node.ID] = node.ParentID
	}
	if permissions.WouldCycle(parent, body.ID, body.ParentID) {
		writeErr(w, r, platform.E("cycle", 422))
		return
	}
	created, err := a.Org.Create(r.Context(), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), a.Pool, audit.Event{ActorID: user.ID, Action: "org.create", EntityType: "org_node", EntityID: created.ID, CompanyID: user.CompanyID, RequestID: requestID(r), Result: "ok"})
	writeJSON(w, 201, created)
}

func (a *API) closeNode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version int    `json:"version"`
		Reason  string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Reason == "" {
		writeErr(w, r, platform.E("reason_required", 422))
		return
	}
	user, err := a.authorize(r, "organization", permissions.ActionArchive, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	id := r.PathValue("id")
	children, _ := a.Org.ActiveChildren(r.Context(), id)
	members, _ := a.Org.ActiveMemberships(r.Context(), id, time.Now().UTC())
	if organization.CloseBlocked(children, members, 0) {
		writeErr(w, r, platform.E("close_blocked", 409))
		return
	}
	if err := a.Org.Close(r.Context(), id, body.Version); err != nil {
		writeErr(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), a.Pool, audit.Event{ActorID: user.ID, Action: "org.close", EntityType: "org_node", EntityID: id, CompanyID: user.CompanyID, RequestID: requestID(r), Result: "ok", Reason: body.Reason})
	writeJSON(w, 200, map[string]any{"id": id, "status": "closed"})
}

func activeMemberships(items []organization.Membership, now time.Time) []organization.Membership {
	out := make([]organization.Membership, 0)
	for _, item := range items {
		if permissions.MembershipActive(permissions.Membership{NodeID: item.NodeID, Starts: item.StartsAt, Ends: item.EndsAt}, now) {
			out = append(out, item)
		}
	}
	return out
}

func decode(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil && !errors.Is(err, io.EOF) {
		return platform.E("invalid_json", 400)
	}
	return nil
}

func requestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return uuid.NewString()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	trace := requestID(r)
	locale := "ar"
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "en") || r.URL.Query().Get("locale") == "en" {
		locale = "en"
	}
	appErr := platform.As(err)
	if appErr == nil {
		if errors.Is(err, pgx.ErrNoRows) {
			appErr = platform.E("not_found", 404)
		} else {
			log.Printf("trace %s: %v", trace, err)
			appErr = platform.E("internal", 500)
		}
	}
	fields := map[string]string{}
	for key, value := range appErr.Fields {
		fields[key] = text(value, locale)
	}
	writeJSON(w, appErr.HTTP, map[string]any{
		"code": appErr.Code, "message": text(appErr.Code, locale), "fields": fields, "trace_id": trace, "retryable": appErr.HTTP >= 500,
	})
}

func text(code, locale string) string {
	ar := map[string]string{
		"invalid_credentials":          "بيانات الدخول غير صحيحة.",
		"mfa_invalid":                  "رمز التحقق غير صحيح.",
		"session_expired":              "انتهت الجلسة.",
		"csrf":                         "تعذر تأكيد الطلب.",
		"forbidden":                    "هذا الإجراء غير مسموح.",
		"not_found":                    "السجل غير موجود.",
		"version_conflict":             "تغير السجل لدى مستخدم آخر. أعد التحميل قبل الحفظ.",
		"invalid_transition":           "لا يمكن تنفيذ هذا الانتقال من الحالة الحالية.",
		"validation":                   "تعذر الإرسال بسبب أخطاء في البيانات.",
		"invalid_json":                 "تعذر قراءة الطلب.",
		"internal":                     "تعذر إكمال الطلب.",
		"unavailable":                  "الخدمة غير متاحة مؤقتا.",
		"reason_required":              "السبب مطلوب.",
		"close_blocked":                "لا يمكن الإغلاق قبل نقل السجلات أو العضويات النشطة.",
		"cycle":                        "لا يمكن نقل العقدة تحت نفسها.",
		"slug_taken":                   "الرابط مستخدم داخل المدينة.",
		"official_boundary_unapproved": "لا يُقبل ادّعاء أن الحد رسمي قبل اعتماد مصدر الحدود.",
		"name_required":                "الاسم العربي والإنجليزي مطلوبان للنشر، من حرفين إلى ثمانين.",
		"invalid_boundary":             "الحد الجغرافي غير صالح.",
		"outside_city":                 "الحد خارج النطاق المعتمد للاختبار داخل الرياض.",
		"boundary_overlap":             "الحد يتداخل مع حي منشور.",
		"duplicate_neighborhood":       "الاسم يطابق حياً موجوداً.",
		"city_boundary_unapproved":     "نشر الحدود موقف حتى اعتماد مصدر حد الرياض.",
		"missing_translation":          "العنوان والوصف مطلوبان بالعربية والإنجليزية.",
		"invalid_price":                "السعر يجب أن يكون أكبر من صفر.",
		"currency_not_sar":             "العملة المعتمدة هي الريال السعودي.",
		"missing_license":              "يلزم ترخيص أو سند استثناء.",
		"missing_broker":               "الوسيط مطلوب.",
		"missing_owner":                "المالك مطلوب.",
		"invalid_area":                 "المساحة يجب أن تكون أكبر من صفر.",
		"rooms_required":               "عدد الغرف مطلوب لهذا النوع.",
		"service_disabled":             "الخدمة غير مفعلة في الحي.",
		"neighborhood_unpublished":     "الحي غير منشور.",
		"outside_neighborhood":         "الموقع خارج حد الحي.",
		"separation_of_duties":         "لا يعتمد الشخص العملية التي أنشأها.",
		"step_up_required":             "يلزم تحقق إضافي.",
		"default_deny":                 "لا توجد صلاحية لهذا الإجراء.",
		"explicit_deny":                "يوجد منع صريح.",
		"outside_scope":                "السجل خارج النطاق المسموح.",
		"account_inactive":             "الحساب غير نشط.",
		"company_mismatch":             "السجل يتبع شركة أخرى.",
		"file_too_large":               "حجم الملف يتجاوز حد الاختبار.",
		"unsupported_type":             "نوع الملف غير مدعوم.",
		"unknown_storage":              "مساحة التخزين غير معروفة.",
		"email_taken":                  "البريد مستخدم.",
		"invalid_password":             "كلمة المرور تحتاج 12 حرفاً بحرف ورقم، وليست من الكلمات الشائعة.",
		"invalid_transaction":          "نوع المعاملة غير مدعوم.",
		"contact_required":             "وسيلة التواصل مطلوبة.",
		"neighborhood_required":        "الحي مطلوب.",
		"visit_conflict":               "الوقت يتعارض مع زيارة مؤكدة للوسيط أو الوحدة أو العميل.",
		"too_early":                    "لا تُستكمل الزيارة قبل نهاية الموعد إلا بتصحيح مسبب من المدير.",
		"result_required":              "نتيجة الزيارة يجب أن تكون مهتم، غير مهتم، متابعة، أو تفاوض.",
		"agreement_required":           "مرجع الاتفاق مطلوب لإغلاق الصفقة.",
		"parties_required":             "العميل والمالك والوسيط مطلوبون.",
		"close_date_required":          "تاريخ الإغلاق مطلوب.",
		"commercial_terms_unapproved":  "لا تُعتمد عمولة أو ضريبة أو رسم إلغاء قبل اعتماد قاعدتها.",
		"listing_unpublished":          "الإعلان غير منشور.",
		"property_change_unapproved":   "تغيير العقار في صفقة متقدمة يحتاج نسخة موافقة أو صفقة بديلة، وهذا المسار غير معتمد بعد.",
		"price_needs_transaction":      "حد السعر يُقارن داخل نوع معاملة واحد، حتى لا يُخلط إجمالي البيع بسعر ليلة.",
		"invalid_sort":                 "ترتيب النتائج غير مدعوم.",
		"stay_conflict":                "هذه المدة تتداخل مع تثبيت أو إقامة مؤكدة على الوحدة نفسها.",
		"hold_expired":                 "انتهت مدة التثبيت. أعد التسعير قبل تثبيت جديد.",
		"idempotency_required":         "مفتاح منع التكرار مطلوب.",
		"idempotency_mismatch":         "مفتاح منع التكرار مستخدم لمحتوى مختلف.",
		"policy_required":              "يلزم قبول نسخة شروط الإقامة قبل التثبيت.",
		"policy_version":               "نسخة الشروط لم تعد هي النسخة المعروضة.",
		"policy_unadopted":             "لا توجد سياسة إقامة معتمدة لهذا النوع.",
		"quote_changed":                "تغير السعر أو التوفر. راجع العرض قبل التثبيت.",
		"one_offer_only":               "لا يُجمع أكثر من عرض واحد في هذا الإصدار.",
		"discount_unapproved":          "لا يوجد عرض خصم معتمد.",
		"monthly_partial":              "الإيجار الشهري يحسب أشهراً تقويمية كاملة، ولا يُقسم الشهر على ٣٠.",
		"min_stay":                     "الإيجار اليومي يبدأ من ليلة واحدة.",
		"over_capacity":                "عدد الضيوف يتجاوز سعة الوحدة.",
		"invalid_guests":               "عدد الضيوف يجب أن يكون واحداً على الأقل.",
		"arrival_in_past":              "تاريخ الوصول فات بتوقيت الرياض.",
		"invalid_dates":                "تاريخ الوصول والمغادرة مطلوبان بصيغة اليوم.",
		"card_data_refused":            "لا يُحفظ رقم البطاقة ولا رمز التحقق في شمال.",
		"stay_not_confirmed":           "الدفع متاح لإقامة مؤكدة فقط.",
		"invalid_method":               "وسيلة الدفع غير مدعومة.",
		"amount_mismatch":              "المبلغ يجب أن يطابق مستحق الإقامة المثبت.",
		"transfer_too_soon":            "التحويل غير متاح إذا كان الوصول خلال أقل من ٤٨ ساعة. هذه مهلة تشغيلية غير تعاقدية.",
		"invoice_immutable":            "الفاتورة الصادرة لا تُعدّل. التصحيح إشعار مرتبط.",
		"nothing_collected":            "لا يوجد مبلغ محصل يمكن استرداده.",
		"refund_exceeds_collected":     "الاسترداد والمعلق يتجاوزان صافي المحصل.",
		"unbalanced":                   "القيد غير متوازن.",
		"message_immutable":            "الرسالة المرسلة لا تُعدّل. التصحيح رسالة لاحقة.",
		"body_too_long":                "الرسالة أطول من خمسة آلاف حرف.",
		"canned_locale":                "الرد الجاهز لا يطابق لغة المستلم.",
		"notice_required":              "قبل الإغلاق يلزم إشعار ظاهر للطرف داخل القضية. لا يُعد ذلك إرسالاً خارجياً.",
		"evidence_exists":              "هذه البصمة مرفوعة مسبقاً ولا تُستبدل.",
		"compensation_needs_booking":   "قرار التعويض يحتاج حجزاً مرتبطاً، ولا يحرّك مالاً بنفسه.",
		"export_unapproved":            "التصدير غير معتمد. صلاحية العرض لا تشمل ملفاً ولا تصدير Access.",
		"search_limit":                 "حد البحوث المحفوظة عشرون بحثاً للحساب.",
		"invalid_amenity":              "المرفق غير معروف. المرافق المعتمدة: مفروش، تكييف، إنترنت، موقف، مفارش، دخول ذاتي.",
		"invalid_rooms":                "عدد غرف النوم يجب أن يكون بين 1 و 30.",
		"marketing_unapproved":         "لا يُرسل بريد تسويقي. القناة المعتمدة هنا هي الإشعار داخل التطبيق فقط.",
	}
	en := map[string]string{
		"invalid_credentials":          "The sign-in details are not valid.",
		"mfa_invalid":                  "The verification code is not valid.",
		"session_expired":              "The session has ended.",
		"csrf":                         "The request could not be confirmed.",
		"forbidden":                    "This action is not allowed.",
		"not_found":                    "The record was not found.",
		"version_conflict":             "Someone else changed this record. Reload before saving.",
		"invalid_transition":           "This status change is not available.",
		"validation":                   "The draft is saved, but it cannot move forward until the listed issues are fixed.",
		"invalid_json":                 "The request could not be read.",
		"internal":                     "The request could not be completed.",
		"unavailable":                  "The service is temporarily unavailable.",
		"reason_required":              "A reason is required.",
		"close_blocked":                "Move active memberships before closing this unit.",
		"cycle":                        "A unit cannot be moved under itself.",
		"slug_taken":                   "That address is already used in the city.",
		"official_boundary_unapproved": "An official boundary cannot be claimed before its source is adopted.",
		"name_required":                "Arabic and English names are required before publishing, from 2 to 80 characters.",
		"invalid_boundary":             "The boundary is not valid.",
		"outside_city":                 "The boundary is outside the Riyadh test mask.",
		"boundary_overlap":             "The boundary overlaps a published neighborhood.",
		"duplicate_neighborhood":       "The name matches an existing neighborhood.",
		"city_boundary_unapproved":     "Publishing is paused until a Riyadh boundary source is adopted.",
		"missing_translation":          "A title and description are required in Arabic and English.",
		"invalid_price":                "The price must be greater than zero.",
		"currency_not_sar":             "The currency is SAR.",
		"missing_license":              "A license or an approved exception reference is required.",
		"missing_broker":               "A broker is required.",
		"missing_owner":                "An owner is required.",
		"invalid_area":                 "The area must be greater than zero.",
		"rooms_required":               "A room count is required for this type.",
		"service_disabled":             "That service is not enabled in the neighborhood.",
		"neighborhood_unpublished":     "The neighborhood is not published.",
		"outside_neighborhood":         "The location is outside the neighborhood boundary.",
		"separation_of_duties":         "The person who created this record cannot approve it.",
		"step_up_required":             "An extra verification is required.",
		"default_deny":                 "There is no permission for this action.",
		"explicit_deny":                "An explicit denial applies.",
		"outside_scope":                "The record is outside the allowed scope.",
		"account_inactive":             "The account is not active.",
		"company_mismatch":             "The record belongs to another company.",
		"file_too_large":               "The file exceeds the test size limit.",
		"unsupported_type":             "This file type is not supported.",
		"unknown_storage":              "The storage area is unknown.",
		"email_taken":                  "That email is already used.",
		"invalid_password":             "The password needs 12 characters, a letter and a digit, and must not be a common password.",
		"invalid_transaction":          "That transaction type is not supported.",
		"contact_required":             "A contact method is required.",
		"neighborhood_required":        "A neighborhood is required.",
		"visit_conflict":               "That time conflicts with a confirmed visit for the broker, the unit, or the client.",
		"too_early":                    "The visit cannot be completed before it ends unless a manager records a reason.",
		"result_required":              "The visit result must be interested, not interested, follow-up, or negotiation.",
		"agreement_required":           "An agreement reference is required to close the deal.",
		"parties_required":             "The client, owner, and broker are required.",
		"close_date_required":          "A close date is required.",
		"commercial_terms_unapproved":  "A commission, tax, or cancellation fee cannot be applied before its rule is adopted.",
		"listing_unpublished":          "The listing is not published.",
		"property_change_unapproved":   "Changing the property on an advanced deal needs an approval copy or an alternate deal, and that path is not adopted yet.",
		"price_needs_transaction":      "A price bound applies inside one transaction type, so a sale total is not compared with a nightly price.",
		"invalid_sort":                 "That result order is not supported.",
		"stay_conflict":                "Those dates overlap a hold or a confirmed stay on the same unit.",
		"hold_expired":                 "The hold has ended. Price the stay again before a new hold.",
		"idempotency_required":         "An idempotency key is required.",
		"idempotency_mismatch":         "That idempotency key was already used for different content.",
		"policy_required":              "Accept the stay terms copy before the hold.",
		"policy_version":               "That terms copy is no longer the one on screen.",
		"policy_unadopted":             "No stay policy is adopted for this transaction.",
		"quote_changed":                "The price or the availability changed. Review the quote before holding.",
		"one_offer_only":               "Only one offer can be selected in this release.",
		"discount_unapproved":          "No discount offer is adopted.",
		"monthly_partial":              "A monthly stay uses whole calendar months and does not divide a month by 30.",
		"min_stay":                     "A daily stay starts at one night.",
		"over_capacity":                "The guest count is above the unit capacity.",
		"invalid_guests":               "The guest count must be at least one.",
		"arrival_in_past":              "The arrival date is already past in Riyadh.",
		"invalid_dates":                "Arrival and departure dates are required.",
		"card_data_refused":            "A full card number or security code is not stored.",
		"stay_not_confirmed":           "Payment is available for a confirmed stay.",
		"invalid_method":               "That payment method is not supported.",
		"amount_mismatch":              "The amount has to match the confirmed stay total.",
		"transfer_too_soon":            "A bank transfer is not available when arrival is within 48 hours. That window is an operational default, not a contract.",
		"invoice_immutable":            "An issued invoice is not edited. A correction is a linked notice.",
		"nothing_collected":            "Nothing has been collected to refund.",
		"refund_exceeds_collected":     "The refund plus pending refunds exceed the net collected amount.",
		"unbalanced":                   "The ledger entry is not balanced.",
		"message_immutable":            "A sent message is not edited. A correction is a later message.",
		"body_too_long":                "A message is limited to 5,000 characters.",
		"canned_locale":                "The canned reply does not match the recipient language.",
		"notice_required":              "Closing needs an in-case notice visible to the party. That is not an external delivery.",
		"evidence_exists":              "This fingerprint is already stored and is not replaced.",
		"compensation_needs_booking":   "A compensation decision needs a linked booking and does not move money by itself.",
		"export_unapproved":            "Export is not adopted. View permission does not include a file or an Access export.",
		"search_limit":                 "An account can keep twenty saved searches.",
		"invalid_amenity":              "That amenity is unknown. The known ones are furnished, air conditioning, internet, parking, bedding, and self entry.",
		"invalid_rooms":                "The bedroom count must be from 1 to 30.",
		"marketing_unapproved":         "Marketing mail is not sent. The channel here is an in-app notice only.",
	}
	table := ar
	if locale == "en" {
		table = en
	}
	if value, ok := table[code]; ok {
		return value
	}
	return code
}
