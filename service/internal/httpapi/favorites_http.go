package httpapi

import (
	"net/http"
	"strings"
)

func (a *API) listFavorites(w http.ResponseWriter, r *http.Request) {
	user, err := a.clientUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Favorites.List(r.Context(), user.ID, favoriteLocale(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) addFavorite(w http.ResponseWriter, r *http.Request) {
	user, err := a.clientUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body struct {
		ListingID string `json:"listing_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, created, err := a.Favorites.Add(r.Context(), user.ID, body.ListingID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	listed, err := a.Favorites.List(r.Context(), user.ID, favoriteLocale(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for _, row := range listed {
		if row.ListingID == item.ListingID {
			item = row
			break
		}
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, item)
}

func (a *API) removeFavorite(w http.ResponseWriter, r *http.Request) {
	user, err := a.clientUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.Favorites.Remove(r.Context(), user.ID, r.PathValue("id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"removed": true})
}

func favoriteLocale(r *http.Request) string {
	switch r.URL.Query().Get("locale") {
	case "en":
		return "en"
	case "ar":
		return "ar"
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		return "en"
	}
	return "ar"
}
