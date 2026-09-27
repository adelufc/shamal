package disputes

import "time"

const (
	ObjectionDays = 7
	CriticalWait  = 30 * time.Minute
	NormalWait    = 24 * time.Hour
)

func KnownType(kind string) bool {
	switch kind {
	case "entry_failure", "mismatch", "cleanliness", "unit_issue", "service", "compensation", "refund":
		return true
	default:
		return false
	}
}

func KnownSubject(kind string) bool {
	switch kind {
	case "booking", "payment", "service":
		return true
	default:
		return false
	}
}

func Allowed(from, to string) bool {
	switch from {
	case "open":
		return to == "classified"
	case "classified":
		return to == "in_progress"
	case "in_progress":
		return to == "awaiting_client" || to == "awaiting_landlord" || to == "escalated" || to == "proposed"
	case "awaiting_client", "awaiting_landlord":
		return to == "in_progress" || to == "escalated"
	case "escalated":
		return to == "in_progress" || to == "proposed"
	case "proposed":
		return to == "resolved" || to == "in_progress"
	case "resolved":
		return to == "closed"
	case "closed":
		return to == "reopened"
	case "reopened":
		return to == "in_progress"
	default:
		return false
	}
}

func Critical(kind string, arrival, departure, now time.Time) bool {
	if kind != "entry_failure" || arrival.IsZero() || departure.IsZero() {
		return false
	}
	return !now.Before(arrival) && now.Before(departure)
}

func WithinObjection(resolved, now time.Time) bool {
	if resolved.IsZero() {
		return false
	}
	return now.Before(resolved.Add(ObjectionDays * 24 * time.Hour))
}
