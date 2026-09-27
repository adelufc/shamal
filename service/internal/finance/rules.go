package finance

import (
	"regexp"
	"time"
)

const (
	ProviderTest    = "test_adapter"
	TransferHold    = 24 * time.Hour
	TransferLead    = 48 * time.Hour
	StatusReview    = "pending_review"
	StatusMismatch  = "mismatch_review"
	StatusCollected = "collected"
)

var cardKeys = map[string]bool{
	"pan": true, "card_number": true, "cardnumber": true, "cvv": true, "cvc": true, "security_code": true,
}

var cardNumber = regexp.MustCompile(`\d{13,19}`)

func RefusesCard(fields map[string]string) bool {
	for key, value := range fields {
		if cardKeys[lower(key)] || cardNumber.MatchString(value) {
			return true
		}
	}
	return false
}

func TransferOpen(now, arrival time.Time) bool {
	return !arrival.Before(now.Add(TransferLead))
}

func JournalBalanced(debits, credits int64) bool {
	return debits > 0 && debits == credits
}

func RefundRoom(collected, pending, amount int64) bool {
	return amount > 0 && collected > 0 && pending+amount <= collected
}

func lower(value string) string {
	out := make([]byte, len(value))
	for i := 0; i < len(value); i++ {
		item := value[i]
		if item >= 'A' && item <= 'Z' {
			item += 'a' - 'A'
		}
		out[i] = item
	}
	return string(out)
}
