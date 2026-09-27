package httpapi

import (
	"io"
	"net/http"
	"time"

	"shamal/service/internal/disputes"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func (a *API) accountDisputes(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if r.Method == http.MethodGet {
		items, err := a.Disputes.List(r.Context(), user.ID, false)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	var body struct {
		SubjectKind     string `json:"subject_kind"`
		SubjectID       string `json:"subject_id"`
		Type            string `json:"type"`
		Description     string `json:"description"`
		OccurredAt      string `json:"occurred_at"`
		RequestedAmount *int64 `json:"requested_amount_minor"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	when, err := time.Parse(time.RFC3339, body.OccurredAt)
	if err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	item, err := a.Disputes.Open(r.Context(), disputes.OpenInput{
		ActorID: user.ID, PartyID: user.ID, CompanyID: user.CompanyID, RequestID: requestID(r),
		SubjectKind: body.SubjectKind, SubjectID: body.SubjectID, Type: body.Type,
		Description: body.Description, OccurredAt: when, RequestedAmount: body.RequestedAmount,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) accountDispute(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	item, err := a.Disputes.Get(r.Context(), r.PathValue("id"), user.ID, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) accountDisputeNote(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Disputes.Note(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), "party", body.Body, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) accountDisputeReview(w http.ResponseWriter, r *http.Request) {
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
	item, err := a.Disputes.Review(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body.Reason)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) accountDisputeEvidence(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	a.saveEvidence(w, r, user.ID, user.CompanyID, false)
}

func (a *API) staffDisputes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		user, err := a.authorize(r, "disputes", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
		if err != nil {
			writeErr(w, r, err)
			return
		}
		items, err := a.Disputes.List(r.Context(), user.ID, true)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	user, err := a.authorize(r, "disputes", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		PartyID         string `json:"party_user_id"`
		SubjectKind     string `json:"subject_kind"`
		SubjectID       string `json:"subject_id"`
		Type            string `json:"type"`
		Description     string `json:"description"`
		OccurredAt      string `json:"occurred_at"`
		RequestedAmount *int64 `json:"requested_amount_minor"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	when, err := time.Parse(time.RFC3339, body.OccurredAt)
	if err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	item, err := a.Disputes.Open(r.Context(), disputes.OpenInput{
		ActorID: user.ID, PartyID: body.PartyID, CompanyID: user.CompanyID, RequestID: requestID(r),
		SubjectKind: body.SubjectKind, SubjectID: body.SubjectID, Type: body.Type,
		Description: body.Description, OccurredAt: when, RequestedAmount: body.RequestedAmount,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) staffDispute(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "disputes", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Disputes.Get(r.Context(), r.PathValue("id"), user.ID, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) staffDisputeMove(w http.ResponseWriter, r *http.Request) {
	var body disputes.MoveInput
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	action := permissions.ActionUpdate
	if body.To == "resolved" || body.To == "closed" || body.To == "reopened" {
		action = permissions.ActionApprove
	}
	user, err := a.authorize(r, "disputes", action, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Disputes.Move(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) staffDisputeAssign(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "disputes", permissions.ActionUpdate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
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
	item, err := a.Disputes.Assign(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body.AssigneeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) staffDisputeNote(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "disputes", permissions.ActionUpdate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Body       string `json:"body"`
		Visibility string `json:"visibility"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Disputes.Note(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body.Visibility, body.Body, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) staffDisputeNotice(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "disputes", permissions.ActionUpdate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Disputes.Notice(r.Context(), user.ID, requestID(r), user.CompanyID, r.PathValue("id"), body.Body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) staffDisputeEvidence(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "disputes", permissions.ActionUpdate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "client"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	a.saveEvidence(w, r, user.ID, user.CompanyID, true)
}

func (a *API) saveEvidence(w http.ResponseWriter, r *http.Request, actor, company string, staff bool) {
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, messagesAttachmentLimit()+1))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Disputes.AddEvidence(r.Context(), actor, requestID(r), company, r.PathValue("id"), header.Header.Get("Content-Type"), header.Filename, body, staff)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func messagesAttachmentLimit() int64 {
	return 10 << 20
}
