package httpapi

import (
	"net/http"

	"shamal/service/internal/platform"
	"shamal/service/internal/visits"
)

func (a *API) saveSearch(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Name            string `json:"name"`
		NeighborhoodID  string `json:"neighborhood_id"`
		TransactionType string `json:"transaction_type"`
		Locale          string `json:"locale"`
		Channel         string `json:"channel"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Notify.SaveSearch(r.Context(), user.ID, body.Name, body.NeighborhoodID, body.TransactionType, body.Locale, body.Channel)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) listSearches(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	items, err := a.Notify.ListSearches(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) muteSearch(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	if err := a.Notify.MuteSearch(r.Context(), user.ID, r.PathValue("id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"muted": true})
}

func (a *API) muteChannel(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	var body struct {
		Channel string `json:"channel"`
		Muted   bool   `json:"muted"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Notify.MuteChannel(r.Context(), user.ID, body.Channel, body.Muted); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"channel": body.Channel, "muted": body.Muted})
}

func (a *API) notifications(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil || sess.Kind != "client" {
		writeErr(w, r, platform.E("forbidden", 403))
		return
	}
	items, err := a.Notify.Inbox(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	muted, err := a.Notify.ChannelMuted(r.Context(), user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "channel_muted": muted})
}

func (a *API) writeVisit(w http.ResponseWriter, r *http.Request, status int, item visits.Visit, err error) {
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Notify.VisitChanged(r.Context(), item.ProfileID, item.ID, item.Status, item.Version); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, status, item)
}
