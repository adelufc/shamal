package finance

import (
	"testing"
	"time"
)

func TestCardDataAndTransferWindow(t *testing.T) {
	if !RefusesCard(map[string]string{"pan": "4111111111111111"}) {
		t.Fatal("a full card number was accepted")
	}
	if !RefusesCard(map[string]string{"note": "4111111111111111"}) {
		t.Fatal("a card number in a note was accepted")
	}
	if RefusesCard(map[string]string{"method": "mada"}) {
		t.Fatal("the method name was treated as a card")
	}
	now := time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC)
	soon := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	later := now.Add(TransferLead)
	if TransferOpen(now, soon) {
		t.Fatal("a transfer inside 48 hours was allowed")
	}
	if !TransferOpen(now, later) {
		t.Fatal("a transfer at the 48 hour mark was blocked")
	}
	if !JournalBalanced(80000, 80000) || JournalBalanced(1, 2) || JournalBalanced(0, 0) {
		t.Fatal("journal balance rule failed")
	}
	if RefundRoom(0, 0, 100) || RefundRoom(100, 80, 30) || !RefundRoom(100, 40, 60) {
		t.Fatal("refund room rule failed")
	}
}
