package httpapi

import (
	"io"
	"net/http"
	"path"

	"github.com/google/uuid"
	"shamal/service/internal/catalog"
	"shamal/service/internal/files"
	"shamal/service/internal/geography"
	"shamal/service/internal/geom"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func (a *API) cities(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id::text, name_ar, name_en, currency, timezone, boundary_authority, boundary_note_ar, boundary_note_en
		FROM geography.cities ORDER BY name_ar
	`)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, ar, en, currency, tz, authority, noteAR, noteEN string
		if err := rows.Scan(&id, &ar, &en, &currency, &tz, &authority, &noteAR, &noteEN); err != nil {
			writeErr(w, r, err)
			return
		}
		items = append(items, map[string]any{"id": id, "name_ar": ar, "name_en": en, "currency": currency, "timezone": tz, "boundary_authority": authority, "boundary_note_ar": noteAR, "boundary_note_en": noteEN})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) neighborhoods(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "geography", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, size := platform.Page(r.URL.Query().Get("page"), r.URL.Query().Get("page_size"))
	items, total, err := a.Geo.List(r.Context(), user.CompanyID, r.URL.Query().Get("status"), page, size)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
}

func (a *API) neighborhood(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authorize(r, "geography", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"}); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Geo.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) createNeighborhood(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "geography", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	in, err := readNeighborhood(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Geo.Create(r.Context(), user.ID, requestID(r), seed.CityID, user.CompanyID, seed.BranchID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) updateNeighborhood(w http.ResponseWriter, r *http.Request) {
	current, err := a.Geo.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	user, err := a.authorize(r, "geography", permissions.ActionUpdate, &permissions.Resource{CompanyID: current.CompanyID, BranchID: current.BranchID, CreatedBy: current.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	in, err := readNeighborhood(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Geo.Update(r.Context(), user.ID, requestID(r), current.ID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) submitNeighborhood(w http.ResponseWriter, r *http.Request) {
	a.neighborhoodCommand(w, r, permissions.ActionUpdate, func(userID, id string, version int, reason string) (geography.Neighborhood, error) {
		return a.Geo.Submit(r.Context(), userID, requestID(r), id, version)
	})
}

func (a *API) requestNeighborhoodChanges(w http.ResponseWriter, r *http.Request) {
	a.neighborhoodCommand(w, r, permissions.ActionApprove, func(userID, id string, version int, reason string) (geography.Neighborhood, error) {
		return a.Geo.RequestChanges(r.Context(), userID, requestID(r), id, reason, version)
	})
}

func (a *API) publishNeighborhood(w http.ResponseWriter, r *http.Request) {
	a.neighborhoodCommand(w, r, permissions.ActionPublish, func(userID, id string, version int, reason string) (geography.Neighborhood, error) {
		item, err := a.Geo.Publish(r.Context(), userID, requestID(r), id, version)
		if err == nil && item.SyncJobID != "" {
			_ = a.Redis.LPush(r.Context(), "queue:geo.sync", item.SyncJobID).Err()
		}
		return item, err
	})
}

func (a *API) suspendNeighborhood(w http.ResponseWriter, r *http.Request) {
	a.neighborhoodCommand(w, r, permissions.ActionArchive, func(userID, id string, version int, reason string) (geography.Neighborhood, error) {
		return a.Geo.Suspend(r.Context(), userID, requestID(r), id, reason, version)
	})
}

func (a *API) reactivateNeighborhood(w http.ResponseWriter, r *http.Request) {
	a.neighborhoodCommand(w, r, permissions.ActionPublish, func(userID, id string, version int, reason string) (geography.Neighborhood, error) {
		return a.Geo.Reactivate(r.Context(), userID, requestID(r), id, reason, version)
	})
}

func (a *API) neighborhoodCommand(w http.ResponseWriter, r *http.Request, action permissions.Action, fn func(userID, id string, version int, reason string) (geography.Neighborhood, error)) {
	current, err := a.Geo.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	createdBy := current.CreatedBy
	if action == permissions.ActionPublish && current.PendingAuthor != "" {
		createdBy = current.PendingAuthor
	}
	user, err := a.authorize(r, "geography", action, &permissions.Resource{CompanyID: current.CompanyID, BranchID: current.BranchID, CreatedBy: createdBy})
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
	if body.Version == 0 {
		body.Version = current.Version
	}
	item, err := fn(user.ID, current.ID, body.Version, body.Reason)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func readNeighborhood(r *http.Request) (geography.Input, error) {
	var body struct {
		NameAR          string   `json:"name_ar"`
		NameEN          string   `json:"name_en"`
		SlugAR          string   `json:"slug_ar"`
		SlugEN          string   `json:"slug_en"`
		DescriptionAR   string   `json:"description_ar"`
		DescriptionEN   string   `json:"description_en"`
		AliasesAR       []string `json:"aliases_ar"`
		AliasesEN       []string `json:"aliases_en"`
		GeoJSON         string   `json:"geojson"`
		ServiceSale     bool     `json:"service_sale"`
		ServiceAnnual   bool     `json:"service_annual"`
		ServiceMonthly  bool     `json:"service_monthly"`
		ServiceDaily    bool     `json:"service_daily"`
		Coverage        string   `json:"coverage"`
		ShowInDirectory bool     `json:"show_in_directory"`
		ShowOnMap       bool     `json:"show_on_map"`
		NoteAR          string   `json:"boundary_note_ar"`
		NoteEN          string   `json:"boundary_note_en"`
		Official        bool     `json:"boundary_is_official"`
		Version         int      `json:"version"`
	}
	if err := decode(r, &body); err != nil {
		return geography.Input{}, err
	}
	return geography.Input{
		NameAR: body.NameAR, NameEN: body.NameEN, SlugAR: body.SlugAR, SlugEN: body.SlugEN,
		DescriptionAR: body.DescriptionAR, DescriptionEN: body.DescriptionEN, AliasesAR: body.AliasesAR, AliasesEN: body.AliasesEN,
		GeoJSON: body.GeoJSON, ServiceSale: body.ServiceSale, ServiceAnnual: body.ServiceAnnual, ServiceMonthly: body.ServiceMonthly, ServiceDaily: body.ServiceDaily,
		Coverage: body.Coverage, ShowInDirectory: body.ShowInDirectory, ShowOnMap: body.ShowOnMap, NoteAR: body.NoteAR, NoteEN: body.NoteEN,
		BoundaryOfficial: body.Official, Version: body.Version,
	}, nil
}

func (a *API) propertyTypes(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authorize(r, "catalog", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"}); err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Cat.Types(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) properties(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, size := platform.Page(r.URL.Query().Get("page"), r.URL.Query().Get("page_size"))
	items, total, err := a.Cat.ListProperties(r.Context(), user.CompanyID, page, size)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
}

func (a *API) createProperty(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body catalog.Property
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	body.CompanyID = user.CompanyID
	if body.CityID == "" {
		body.CityID = seed.CityID
	}
	if body.BranchID == "" {
		body.BranchID = seed.BranchID
	}
	item, err := a.Cat.CreateProperty(r.Context(), user.ID, requestID(r), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) createUnit(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body catalog.Unit
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Cat.CreateUnit(r.Context(), user.ID, requestID(r), user.CompanyID, body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) createListing(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body catalog.Listing
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	body.CompanyID = user.CompanyID
	if body.Currency == "" {
		body.Currency = "SAR"
	}
	if body.PriceUnit == "" {
		body.PriceUnit = priceUnit(body.TransactionType)
	}
	item, err := a.Cat.CreateListing(r.Context(), user.ID, requestID(r), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}

func (a *API) listListings(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "catalog", permissions.ActionView, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := a.Cat.ListCards(r.Context(), user.CompanyID, r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) pendingListingChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version        int    `json:"version"`
		PriceMinor     int64  `json:"price_minor"`
		NeighborhoodID string `json:"neighborhood_id"`
		LicenseRef     string `json:"license_ref"`
		OwnerUserID    string `json:"owner_user_id"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.PriceMinor <= 0 || body.LicenseRef == "" {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	if _, err := uuid.Parse(body.NeighborhoodID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	if _, err := uuid.Parse(body.OwnerUserID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	item, err := a.Cat.GetListing(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if item.Status != "published" {
		writeErr(w, r, platform.E("invalid_transition", 409))
		return
	}
	user, err := a.authorize(r, "catalog", permissions.ActionUpdate, &permissions.Resource{CompanyID: item.CompanyID, CreatedBy: item.CreatedBy})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Version == 0 {
		body.Version = item.Version
	}
	item.Version = body.Version
	updated, err := a.Cat.StageSensitiveChange(r.Context(), user.ID, requestID(r), item, body.PriceMinor, body.NeighborhoodID, body.LicenseRef, body.OwnerUserID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, updated)
}

func (a *API) listing(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authorize(r, "catalog", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"}); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.Cat.GetListing(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) listingMove(to string, action permissions.Action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := a.Cat.GetListing(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, r, err)
			return
		}
		user, err := a.authorize(r, "catalog", action, &permissions.Resource{CompanyID: item.CompanyID, BranchID: item.BranchID, CreatedBy: item.CreatedBy})
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
		if body.Version == 0 {
			body.Version = item.Version
		}
		issues, err := a.listingIssues(r, item)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		updated, err := a.Cat.Transition(r.Context(), user.ID, requestID(r), item.ID, body.Version, to, issues)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if to == "published" {
			if err := a.Notify.ListingPublished(r.Context(), updated.ID, updated.NeighborhoodID, updated.TransactionType); err != nil {
				writeErr(w, r, err)
				return
			}
		}
		writeJSON(w, 200, updated)
	}
}

func (a *API) listingIssues(r *http.Request, item catalog.Listing) ([]string, error) {
	place, err := a.Geo.Place(r.Context(), item.NeighborhoodID)
	if err != nil {
		return nil, err
	}
	var requires bool
	_ = a.Pool.QueryRow(r.Context(), `SELECT requires_rooms FROM catalog.property_types WHERE code = $1`, item.TypeCode).Scan(&requires)
	inside := false
	if item.PreciseLon != nil && item.PreciseLat != nil && place.GeoJSON != "" {
		shape, err := geom.Parse([]byte(place.GeoJSON))
		if err == nil {
			inside = geom.ContainsPoint(shape, *item.PreciseLon, *item.PreciseLat)
		}
	}
	var waiver int
	_ = a.Pool.QueryRow(r.Context(), `SELECT count(*) FROM catalog.location_waivers WHERE listing_id = $1`, item.ID).Scan(&waiver)
	titleAR, titleEN, descAR, descEN := "", "", "", ""
	for _, text := range item.Texts {
		if text.Locale == "ar" {
			titleAR, descAR = text.Title, text.Description
		}
		if text.Locale == "en" {
			titleEN, descEN = text.Title, text.Description
		}
	}
	enabled := false
	switch item.TransactionType {
	case "sale":
		enabled = place.Sale
	case "annual":
		enabled = place.Annual
	case "monthly":
		enabled = place.Monthly
	case "daily":
		enabled = place.Daily
	}
	return catalog.ListingIssues(catalog.ListingDraft{
		Transaction: item.TransactionType, TitleAR: titleAR, TitleEN: titleEN, DescriptionAR: descAR, DescriptionEN: descEN,
		PriceMinor: item.PriceMinor, Currency: item.Currency, LicenseRef: item.LicenseRef, LicenseExceptionRef: item.LicenseExceptionRef,
		BrokerID: item.BrokerUserID, OwnerID: item.OwnerUserID, RequiresRooms: requires, Rooms: item.Rooms, AreaSqm: item.AreaSqm,
		NeighborhoodPublished: place.Status == "published", ServiceEnabled: enabled, PointInside: inside, HasLocationWaiver: waiver > 0,
	}), nil
}

func (a *API) home(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	if locale != "en" {
		locale = "ar"
	}
	neighborhoods, err := a.Geo.Public(r.Context(), locale, seed.CityID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	listings, err := a.Cat.PublicList(r.Context(), locale, r.URL.Query().Get("neighborhood_id"), r.URL.Query().Get("transaction"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	total, by, err := a.Cat.CountPublished(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	cards := make([]map[string]any, 0, len(neighborhoods))
	for _, item := range neighborhoods {
		name, slug := "", ""
		if len(item.Texts) > 0 {
			name, slug = item.Texts[0].Name, item.Texts[0].Slug
		}
		cards = append(cards, map[string]any{
			"id": item.ID, "name": name, "slug": slug, "listing_count": by[item.ID],
			"service_sale": item.ServiceSale, "service_annual": item.ServiceAnnual, "service_monthly": item.ServiceMonthly, "service_daily": item.ServiceDaily,
			"boundary_is_official": item.BoundaryOfficial,
		})
	}
	cityName := "الرياض"
	if locale == "en" {
		cityName = "Riyadh"
	}
	blocks, err := a.Content.Published(r.Context(), locale, "", "")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"locale": locale, "city": map[string]string{"name": cityName, "currency": "SAR"},
		"neighborhoods": cards, "listings": listings,
		"counts":  map[string]int{"published_neighborhoods": len(cards), "published_listings": total},
		"content": blocks,
	})
}

func (a *API) publicNeighborhood(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	if locale != "en" {
		locale = "ar"
	}
	item, err := a.Geo.BySlug(r.Context(), locale, r.PathValue("slug"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	listings, err := a.Cat.PublicList(r.Context(), locale, item.ID, "")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"neighborhood": item, "listings": listings})
}

func (a *API) publicListing(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	if locale != "en" {
		locale = "ar"
	}
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		writeErr(w, r, platform.E("not_found", 404))
		return
	}
	item, err := a.Cat.PublicOne(r.Context(), locale, r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "files", permissions.ActionCreate, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	area := r.FormValue("area")
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, r, platform.E("invalid_json", 400))
		return
	}
	defer file.Close()
	kind := files.KindFromType(header.Header.Get("Content-Type"))
	if code := files.ValidateUpload(area, kind, header.Size, 0); code != "" {
		writeErr(w, r, platform.E(code, 422))
		return
	}
	key := uuid.NewString() + path.Ext(header.Filename)
	if err := a.Files.Put(area, key, file); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := a.Pool.Exec(r.Context(), `
		INSERT INTO files.objects (id, area, storage_key, content_type, size_bytes, original_name, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.NewString(), area, key, header.Header.Get("Content-Type"), header.Size, header.Filename, user.ID); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"area": area, "key": key})
}

func (a *API) download(w http.ResponseWriter, r *http.Request) {
	area := r.PathValue("area")
	key := r.PathValue("key")
	if area == "private" {
		if _, err := a.authorize(r, "files", permissions.ActionView, &permissions.Resource{CompanyID: seed.CompanyID, CreatedBy: "system"}); err != nil {
			writeErr(w, r, err)
			return
		}
	} else if area != "public" {
		writeErr(w, r, platform.E("unknown_storage", 404))
		return
	}
	file, err := a.Files.Open(area, key)
	if err != nil {
		writeErr(w, r, platform.E("not_found", 404))
		return
	}
	defer file.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, file)
}

func priceUnit(transaction string) string {
	switch transaction {
	case "sale":
		return "total"
	case "annual":
		return "yearly"
	case "monthly":
		return "monthly"
	case "daily":
		return "nightly"
	default:
		return ""
	}
}
