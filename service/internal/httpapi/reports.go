package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/reports"
	"shamal/service/internal/seed"
)

func (a *API) report(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "reports", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	filter, err := reportFilter(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Reports.Run(r.Context(), r.PathValue("code"), user.CompanyID, r.URL.Query().Get("from"), r.URL.Query().Get("to"), filter, time.Now().UTC())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) reportExport(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authorize(r, "reports", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"}); err != nil {
		writeErr(w, r, err)
		return
	}
	writeErr(w, r, platform.E("export_unapproved", 403))
}

func reportFilter(r *http.Request) (reports.Filter, error) {
	filter := reports.Filter{
		BranchID:       r.URL.Query().Get("branch_id"),
		NeighborhoodID: r.URL.Query().Get("neighborhood_id"),
		Transaction:    r.URL.Query().Get("transaction"),
		Status:         r.URL.Query().Get("status"),
	}
	for _, value := range []string{filter.BranchID, filter.NeighborhoodID} {
		if value == "" {
			continue
		}
		if _, err := uuid.Parse(value); err != nil {
			return reports.Filter{}, platform.E("validation", 422)
		}
	}
	return filter, nil
}
