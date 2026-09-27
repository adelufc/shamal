package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"shamal/service/internal/messages"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func (a *API) accountMessagePreview(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	item, err := a.Messages.Preview(r.Context(), user.ID, r.URL.Query().Get("subject_kind"), r.URL.Query().Get("subject_id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) accountMessages(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if r.Method == http.MethodGet {
		items, err := a.Messages.Inbox(r.Context(), user.ID, false)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	a.postMessage(w, r, user.ID, user.CompanyID, false, "")
}

func (a *API) accountMessageThread(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		writeErr(w, r, platform.E("message_immutable", 409))
		return
	}
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if r.Method == http.MethodGet {
		items, err := a.Messages.Thread(r.Context(), user.ID, r.PathValue("id"), r.URL.Query().Get("after"), false, false)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	a.postMessage(w, r, user.ID, user.CompanyID, false, r.PathValue("id"))
}

func (a *API) accountMessageRead(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	before, err := a.Messages.Thread(r.Context(), user.ID, r.PathValue("id"), "", false, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Messages.MarkRead(r.Context(), user.ID, r.PathValue("id"), false); err != nil {
		writeErr(w, r, err)
		return
	}
	after, err := a.Messages.Thread(r.Context(), user.ID, r.PathValue("id"), "", false, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"read": true, "messages": len(after), "unchanged": len(before) == len(after)})
}

func (a *API) accountMessageReport(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Messages.Report(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body.Kind, body.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"status": "under_review", "booking_access": "unchanged"})
}

func (a *API) staffMessages(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "messages", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Messages.Inbox(r.Context(), user.ID, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) staffCanned(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authorize(r, "messages", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"}); err != nil {
		writeErr(w, r, err)
		return
	}
	locale := "ar"
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "en") || r.URL.Query().Get("locale") == "en" {
		locale = "en"
	}
	if q := r.URL.Query().Get("locale"); q == "ar" || q == "en" {
		locale = q
	}
	items, err := a.Messages.Canned(r.Context(), locale)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) staffMessageThread(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		writeErr(w, r, platform.E("message_immutable", 409))
		return
	}
	user, err := a.authorize(r, "messages", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if r.Method == http.MethodGet {
		reveal := a.allowed(r, "messages", permissions.ActionArchive)
		items, err := a.Messages.Thread(r.Context(), user.ID, r.PathValue("id"), r.URL.Query().Get("after"), true, reveal)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	if _, err := a.authorize(r, "messages", permissions.ActionUpdate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"}); err != nil {
		writeErr(w, r, err)
		return
	}
	a.postMessage(w, r, user.ID, user.CompanyID, true, r.PathValue("id"))
}

func (a *API) staffMessageAssign(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "messages", permissions.ActionUpdate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		AssigneeID string `json:"assignee_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Messages.Assign(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body.AssigneeID); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"assignee_id": body.AssigneeID})
}

func (a *API) staffMessageHide(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "messages", permissions.ActionArchive, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		MessageID string `json:"message_id"`
		Reason    string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Messages.Hide(r.Context(), user.ID, requestID(r), user.CompanyID, body.MessageID, body.Reason)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) postMessage(w http.ResponseWriter, r *http.Request, actor, company string, staff bool, conversationID string) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	fields := stringFields(raw)
	var body struct {
		SubjectKind    string `json:"subject_kind"`
		SubjectID      string `json:"subject_id"`
		ConversationID string `json:"conversation_id"`
		Body           string `json:"body"`
		Kind           string `json:"kind"`
		CorrectsID     string `json:"corrects_id"`
		CannedID       string `json:"canned_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	if conversationID == "" {
		conversationID = body.ConversationID
	}
	locale := "ar"
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		locale = "en"
	}
	item, err := a.Messages.Post(r.Context(), messages.PostInput{
		ActorID: actor, Staff: staff, CompanyID: company, RequestID: requestID(r),
		Key: r.Header.Get("Idempotency-Key"), SubjectKind: body.SubjectKind, SubjectID: body.SubjectID,
		ConversationID: conversationID, Body: body.Body, Kind: body.Kind, CorrectsID: body.CorrectsID,
		CannedID: body.CannedID, Locale: locale, Fields: fields,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) allowed(r *http.Request, domain string, action permissions.Action) bool {
	_, err := a.authorize(r, domain, action, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	return err == nil
}
