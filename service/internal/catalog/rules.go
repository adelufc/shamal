package catalog

type ListingDraft struct {
	Transaction            string
	TitleAR                string
	TitleEN                string
	DescriptionAR          string
	DescriptionEN          string
	PriceMinor             int64
	Currency               string
	LicenseRef             string
	LicenseExceptionRef    string
	BrokerID               string
	OwnerID                string
	RequiresRooms          bool
	Rooms                  *int
	AreaSqm                float64
	NeighborhoodPublished  bool
	ServiceEnabled         bool
	PointInside            bool
	HasLocationWaiver      bool
}

func ListingIssues(d ListingDraft) []string {
	issues := make([]string, 0)
	if d.TitleAR == "" || d.TitleEN == "" || d.DescriptionAR == "" || d.DescriptionEN == "" {
		issues = append(issues, "missing_translation")
	}
	if d.PriceMinor <= 0 {
		issues = append(issues, "invalid_price")
	}
	if d.Currency != "SAR" {
		issues = append(issues, "currency_not_sar")
	}
	if d.LicenseRef == "" && d.LicenseExceptionRef == "" {
		issues = append(issues, "missing_license")
	}
	if d.BrokerID == "" {
		issues = append(issues, "missing_broker")
	}
	if d.OwnerID == "" {
		issues = append(issues, "missing_owner")
	}
	if d.AreaSqm <= 0 {
		issues = append(issues, "invalid_area")
	}
	if d.RequiresRooms && (d.Rooms == nil || *d.Rooms < 0) {
		issues = append(issues, "rooms_required")
	}
	if d.Rooms != nil && *d.Rooms < 0 {
		issues = append(issues, "invalid_rooms")
	}
	if !d.NeighborhoodPublished {
		issues = append(issues, "neighborhood_unpublished")
	}
	if !d.ServiceEnabled {
		issues = append(issues, "service_disabled")
	}
	if !d.PointInside && !d.HasLocationWaiver {
		issues = append(issues, "outside_neighborhood")
	}
	switch d.Transaction {
	case "sale", "annual", "monthly", "daily":
	default:
		issues = append(issues, "invalid_transaction")
	}
	return issues
}

func CanTransition(from, to string) bool {
	switch from {
	case "draft":
		return to == "in_review"
	case "in_review":
		return to == "changes_requested" || to == "approved"
	case "changes_requested":
		return to == "in_review"
	case "approved":
		return to == "published"
	case "published":
		return to == "suspended" || to == "ended" || to == "in_review"
	case "suspended":
		return to == "published"
	default:
		return false
	}
}

func SensitiveChange(price, neighborhood, license, owner bool) bool {
	return price || neighborhood || license || owner
}
