package notify

import (
	"fmt"

	"shamal/service/internal/platform"
)

const (
	SearchLimit = 20
	Channel     = "in_app"
)

func AllowChannel(channel string) error {
	if channel == Channel {
		return nil
	}
	return platform.E("marketing_unapproved", 422)
}

func ReviewStatus(status string) bool {
	return status == "pending_review" || status == "mismatch_review"
}

func DedupListing(userID, listingID, searchID string) string {
	return fmt.Sprintf("%s|listing_match|%s|%s", userID, listingID, searchID)
}

func DedupVisit(userID, visitID, status string, version int) string {
	return fmt.Sprintf("%s|visit_change|%s|%s|%d", userID, visitID, status, version)
}

func DedupPayment(userID, attemptID, status string) string {
	return fmt.Sprintf("%s|payment_review|%s|%s", userID, attemptID, status)
}
