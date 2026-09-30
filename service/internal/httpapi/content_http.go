package httpapi

import (
	"net/http"

	"github.com/google/uuid"
	"shamal/service/internal/content"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
)

func (a *API) contentPages(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionView, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Content.List(r.Context(), user.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) createContentPage(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionCreate, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body contentBody
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Content.Create(r.Context(), user.CompanyID, user.ID, body.Kind, body.Slug, body.NeighborhoodID, body.arabic(), body.english())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) contentPage(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionView, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	item, err := a.Content.Get(r.Context(), user.CompanyID, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) updateContentPage(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionUpdate, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body contentBody
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Content.Update(r.Context(), user.CompanyID, user.ID, r.PathValue("id"), body.Version, body.arabic(), body.english())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) submitContentPage(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionUpdate, nil)
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
	item, err := a.Content.Move(r.Context(), user.CompanyID, user.ID, r.PathValue("id"), "draft", "in_review", body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) publishContentPage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version int `json:"version"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	_, user, err := a.sessionUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, err := a.Content.Get(r.Context(), user.CompanyID, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := a.authorize(r, "catalog", permissions.ActionPublish, &permissions.Resource{CompanyID: user.CompanyID, CreatedBy: page.CreatedBy}); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Content.Move(r.Context(), user.CompanyID, user.ID, page.ID, "in_review", "published", body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) publicContent(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	items, err := a.Content.Published(r.Context(), locale, r.URL.Query().Get("kind"), r.URL.Query().Get("slug"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

type contentBody struct {
	Kind           string `json:"kind"`
	Slug           string `json:"slug"`
	NeighborhoodID string `json:"neighborhood_id"`
	Version        int    `json:"version"`
	TitleAR        string `json:"title_ar"`
	BodyAR         string `json:"body_ar"`
	TitleEN        string `json:"title_en"`
	BodyEN         string `json:"body_en"`
}

func (b contentBody) arabic() content.Text {
	return content.Text{Locale: "ar", Title: b.TitleAR, Body: b.BodyAR}
}

func (b contentBody) english() content.Text {
	return content.Text{Locale: "en", Title: b.TitleEN, Body: b.BodyEN}
}
