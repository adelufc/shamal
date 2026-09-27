package visits

import (
	"strings"
	"time"
)

const (
	StatusNew            = "new"
	StatusAwaitingBroker = "awaiting_broker"
	StatusAwaitingOwner  = "awaiting_owner"
	StatusConfirmed      = "confirmed"
	StatusCompleted      = "completed"
	StatusCancelled      = "cancelled"
	StatusNoShow         = "no_show"

	// Proposed test defaults from the requirements, not a fee schedule.
	DefaultDuration = 30 * time.Minute
	SlotGap         = 15 * time.Minute
	EscalateAfter   = 2 * time.Hour
)

func CanTransition(from, to string) bool {
	switch from + ">" + to {
	case "new>awaiting_broker", "new>cancelled",
		"awaiting_broker>awaiting_owner", "awaiting_broker>cancelled", "awaiting_broker>awaiting_broker",
		"awaiting_owner>confirmed", "awaiting_owner>awaiting_broker", "awaiting_owner>cancelled",
		"confirmed>completed", "confirmed>cancelled", "confirmed>no_show", "confirmed>awaiting_broker":
		return true
	default:
		return false
	}
}

func EndOrDefault(start, end time.Time) time.Time {
	if end.IsZero() || !end.After(start) {
		return start.Add(DefaultDuration)
	}
	return end
}

func TooClose(aStart, aEnd, bStart, bEnd time.Time, gap time.Duration) bool {
	return aStart.Before(bEnd.Add(gap)) && bStart.Before(aEnd.Add(gap))
}

func CompleteIssue(now, ends time.Time, result, note string, correction bool, reason string) string {
	_ = note
	if !correction && now.Before(ends) {
		return "too_early"
	}
	if correction && strings.TrimSpace(reason) == "" {
		return "reason_required"
	}
	switch result {
	case "interested", "not_interested", "follow_up", "negotiation":
		return ""
	default:
		return "result_required"
	}
}

func ReminderKinds(now, start time.Time) []string {
	out := make([]string, 0, 2)
	if !start.Before(now.Add(24 * time.Hour)) {
		out = append(out, "day_before")
	}
	if !start.Before(now.Add(2 * time.Hour)) {
		out = append(out, "two_hours")
	}
	return out
}

func NeedsReason(command string) bool {
	switch command {
	case "cancel", "no_show", "reschedule", "owner_reject":
		return true
	default:
		return false
	}
}
