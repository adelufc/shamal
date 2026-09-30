package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"shamal/service/internal/catalog"
	"shamal/service/internal/geography"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func (a *API) publicSearch(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	if locale != "en" {
		locale = "ar"
	}
	query := r.URL.Query()
	transaction := strings.TrimSpace(query.Get("transaction"))
	sort := strings.TrimSpace(query.Get("sort"))
	text := strings.TrimSpace(query.Get("q"))
	neighborhoodID := strings.TrimSpace(query.Get("neighborhood_id"))
	inArea := query.Get("in_area") == "1"
	priceMin, minOK := optionalMinor(query.Get("price_min_minor"))
	priceMax, maxOK := optionalMinor(query.Get("price_max_minor"))
	var west, south, east, north *float64
	if inArea {
		west = optionalFloat(query.Get("west"))
		south = optionalFloat(query.Get("south"))
		east = optionalFloat(query.Get("east"))
		north = optionalFloat(query.Get("north"))
	}
	issues := catalog.SearchIssues(transaction, sort, priceMin, priceMax, inArea, west, south, east, north)
	if !minOK || !maxOK {
		issues = append(issues, "invalid_price")
	}
	rooms, roomIssues := catalog.ParseBedrooms(query.Get("bedrooms"))
	issues = append(issues, roomIssues...)
	furnished, cooling, internet, parking, bedding, entry, amenityIssues := catalog.AmenityFlags(query.Get("amenities"))
	issues = append(issues, amenityIssues...)
	verified := query.Get("verified") == "1"
	if query.Get("verified") != "" && query.Get("verified") != "1" && query.Get("verified") != "0" {
		issues = append(issues, "validation")
	}
	if len(issues) > 0 {
		fields := map[string]string{}
		for _, issue := range issues {
			fields[issue] = issue
		}
		writeErr(w, r, &platform.Error{Code: "validation", HTTP: 422, Fields: fields})
		return
	}
	published, err := a.Geo.PublishedIDs(r.Context(), seed.CityID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	allowed := published
	if neighborhoodID != "" {
		allowed = intersectIDs(allowed, []string{neighborhoodID})
	}
	if inArea {
		areaIDs, err := a.Geo.IDsInArea(r.Context(), seed.CityID, *west, *south, *east, *north)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		allowed = intersectIDs(allowed, areaIDs)
	}
	nameIDs := []string{}
	if text != "" {
		nameIDs, err = a.Geo.IDsByName(r.Context(), locale, seed.CityID, geography.Normalize(text))
		if err != nil {
			writeErr(w, r, err)
			return
		}
	}
	places, err := a.Geo.MapPlaces(r.Context(), locale, seed.CityID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, size := platform.Page(query.Get("page"), query.Get("page_size"))
	result, err := a.Cat.Search(r.Context(), catalog.SearchQuery{
		Locale: locale, NeighborhoodID: neighborhoodID, Transaction: transaction,
		PriceMin: priceMin, PriceMax: priceMax, Sort: sort, Text: text,
		AllowedIDs: allowed, NameIDs: nameIDs, Page: page, Size: size,
		RoomsMin: rooms, Furnished: furnished, Cooling: cooling, Internet: internet, Parking: parking, Bedding: bedding, SelfEntry: entry, Verified: verified,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	points := map[string]geography.MapPlace{}
	publicPlaces := make([]map[string]any, 0, len(places))
	for _, place := range places {
		points[place.ID] = place
		publicPlaces = append(publicPlaces, map[string]any{
			"id": place.ID, "name": place.Name, "slug": place.Slug,
			"lon": place.Lon, "lat": place.Lat, "geojson": place.GeoJSON,
			"boundary_is_official": place.Official,
		})
	}
	markers := make([]catalog.SearchMarker, 0, len(result.Markers))
	for _, hit := range result.Markers {
		place, ok := points[hit.NeighborhoodID]
		if !ok {
			continue
		}
		markers = append(markers, catalog.SearchMarker{
			ID: hit.ID, NeighborhoodID: hit.NeighborhoodID, TransactionType: hit.TransactionType,
			PriceMinor: hit.PriceMinor, PriceUnit: hit.PriceUnit, Currency: hit.Currency,
			Title: hit.Title, Lon: place.Lon, Lat: place.Lat,
		})
	}
	for i := range result.Clusters {
		if place, ok := points[result.Clusters[i].NeighborhoodID]; ok {
			result.Clusters[i].Lon = place.Lon
			result.Clusters[i].Lat = place.Lat
		}
	}
	mode := "markers"
	if catalog.UseClusters(result.Total) {
		mode = "clusters"
		markers = []catalog.SearchMarker{}
	}
	if result.Listings == nil {
		result.Listings = []catalog.SearchHit{}
	}
	writeJSON(w, 200, map[string]any{
		"locale": locale,
		"query": map[string]any{
			"q": text, "neighborhood_id": neighborhoodID, "transaction": transaction,
			"price_min_minor": priceMin, "price_max_minor": priceMax, "sort": sortOrNewest(sort),
			"in_area": inArea, "west": west, "south": south, "east": east, "north": north,
			"bedrooms": rooms, "amenities": strings.TrimSpace(query.Get("amenities")), "verified": verified,
		},
		"listings": result.Listings, "markers": markers, "clusters": result.Clusters,
		"marker_mode": mode, "marker_cap": catalog.MarkerCap,
		"map_scope": "all_matches",
		"total":     result.Total, "page": page, "page_size": size,
		"places": publicPlaces,
	})
}

func sortOrNewest(sort string) string {
	if sort == "" {
		return "newest"
	}
	return sort
}

func optionalMinor(raw string) (*int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, false
	}
	return &value, true
}

func optionalFloat(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

func intersectIDs(left, right []string) []string {
	keep := map[string]bool{}
	for _, id := range right {
		keep[id] = true
	}
	out := make([]string, 0)
	for _, id := range left {
		if keep[id] {
			out = append(out, id)
		}
	}
	return out
}
