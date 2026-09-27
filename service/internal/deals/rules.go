package deals

import (
	"strings"
	"time"
)

const (
	Draft            = "draft"
	Qualified        = "qualified"
	Negotiation      = "negotiation"
	AwaitingDocs     = "awaiting_documents"
	Preliminary      = "preliminary_agreement"
	AwaitingExternal = "awaiting_external"
	ClosedSuccess    = "closed_success"
	ClosedLost       = "closed_unsuccessful"
	Cancelled        = "cancelled"
)

func CanTransition(from, to string) bool {
	if from == to || terminal(from) {
		return false
	}
	if to == ClosedLost || to == Cancelled {
		return true
	}
	switch from + ">" + to {
	case "draft>qualified",
		"qualified>negotiation", "qualified>draft",
		"negotiation>awaiting_documents", "negotiation>qualified",
		"awaiting_documents>preliminary_agreement", "awaiting_documents>negotiation",
		"preliminary_agreement>awaiting_external", "preliminary_agreement>awaiting_documents",
		"awaiting_external>closed_success", "awaiting_external>preliminary_agreement":
		return true
	default:
		return false
	}
}

func Backward(from, to string) bool {
	switch from + ">" + to {
	case "qualified>draft", "negotiation>qualified", "awaiting_documents>negotiation",
		"preliminary_agreement>awaiting_documents", "awaiting_external>preliminary_agreement":
		return true
	default:
		return false
	}
}

func MoveIssue(from, to, reason string) string {
	if !CanTransition(from, to) {
		return "invalid_transition"
	}
	if (Backward(from, to) || to == ClosedLost || to == Cancelled) && strings.TrimSpace(reason) == "" {
		return "reason_required"
	}
	return ""
}

type CloseInput struct {
	AgreementRef  string
	AmountMinor   int64
	Currency      string
	ClientID      string
	OwnerID       string
	BrokerID      string
	CloseDate     time.Time
	RequestedBy   string
	ApproverID    string
	HasCommission bool
	HasTax        bool
	HasCancelFee  bool
}

func CloseIssues(in CloseInput) []string {
	if in.HasCommission || in.HasTax || in.HasCancelFee {
		return []string{"commercial_terms_unapproved"}
	}
	issues := make([]string, 0)
	if strings.TrimSpace(in.AgreementRef) == "" {
		issues = append(issues, "agreement_required")
	}
	if in.AmountMinor <= 0 {
		issues = append(issues, "invalid_price")
	}
	if in.Currency != "SAR" {
		issues = append(issues, "currency_not_sar")
	}
	if in.ClientID == "" || in.OwnerID == "" || in.BrokerID == "" {
		issues = append(issues, "parties_required")
	}
	if in.CloseDate.IsZero() {
		issues = append(issues, "close_date_required")
	}
	if in.RequestedBy == "" || in.RequestedBy == in.ApproverID {
		issues = append(issues, "separation_of_duties")
	}
	return issues
}

func AudienceExcluded(status string, test bool) bool {
	return test || status == Cancelled || status == ClosedLost
}

func terminal(status string) bool {
	return status == ClosedSuccess || status == ClosedLost || status == Cancelled
}
