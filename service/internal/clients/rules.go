package clients

import "strings"

const (
	StageNew         = "new"
	StageContacted   = "contacted"
	StageInterested  = "interested"
	StageVisit       = "visit"
	StageFollowUp    = "follow_up"
	StageNegotiation = "negotiation"
	StageAgreement   = "agreement"
	StageConverted   = "converted"
	StageUnqualified = "unqualified"
	StageClosed      = "closed"
)

func CanTransition(from, to string) bool {
	if from == to {
		return false
	}
	if to == StageUnqualified || to == StageClosed {
		return openStage(from)
	}
	switch from {
	case StageNew:
		return to == StageContacted
	case StageContacted:
		return to == StageInterested
	case StageInterested:
		return to == StageVisit || to == StageFollowUp
	case StageVisit:
		return to == StageFollowUp || to == StageNegotiation
	case StageFollowUp:
		return to == StageVisit || to == StageNegotiation
	case StageNegotiation:
		return to == StageAgreement
	case StageAgreement:
		return to == StageConverted
	default:
		return false
	}
}

func TransitionIssue(from, to, reason string) string {
	if !CanTransition(from, to) {
		return "invalid_transition"
	}
	if (to == StageUnqualified || to == StageClosed || to == StageConverted) && strings.TrimSpace(reason) == "" && to != StageConverted {
		return "reason_required"
	}
	return ""
}

func openStage(stage string) bool {
	switch stage {
	case StageNew, StageContacted, StageInterested, StageVisit, StageFollowUp, StageNegotiation, StageAgreement:
		return true
	default:
		return false
	}
}

func ClientJourney(requestStage, dealStatus string) string {
	switch dealStatus {
	case "closed_success":
		return "completed"
	case "closed_unsuccessful", "cancelled":
		return "closed_without_agreement"
	case "awaiting_documents":
		return "documents"
	case "preliminary_agreement", "awaiting_external":
		return "agreement"
	case "negotiation":
		return "negotiation"
	}
	switch requestStage {
	case StageNew:
		return "received"
	case StageVisit:
		return "visit"
	case StageNegotiation:
		return "negotiation"
	case StageAgreement:
		return "agreement"
	case StageUnqualified, StageClosed:
		return "closed_without_agreement"
	default:
		return "follow_up"
	}
}

func RequestIssues(transaction, contact, neighborhood, currency string, budget *int64) []string {
	issues := make([]string, 0)
	switch transaction {
	case "sale", "annual", "monthly", "daily":
	default:
		issues = append(issues, "invalid_transaction")
	}
	switch contact {
	case "phone", "email", "either":
	default:
		issues = append(issues, "contact_required")
	}
	if strings.TrimSpace(neighborhood) == "" {
		issues = append(issues, "neighborhood_required")
	}
	if budget != nil {
		if *budget <= 0 {
			issues = append(issues, "invalid_price")
		}
		if currency != "SAR" {
			issues = append(issues, "currency_not_sar")
		}
	}
	return issues
}
