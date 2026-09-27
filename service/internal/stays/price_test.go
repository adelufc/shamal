package stays

import (
	"testing"
	"time"
)

func TestDocumentedStayExampleIsNotRevenueForTheDeposit(t *testing.T) {
	// Requirement 49 calculation example. These amounts are a non-contractual test, not a fee schedule.
	out, issues := Price(Input{
		Transaction:     "daily",
		Arrival:         mustDate(t, "2026-10-01"),
		Departure:       mustDate(t, "2026-10-04"),
		Guests:          2,
		UnitMinor:       40000,
		Offers:          1,
		DiscountPercent: 10,
		ServiceMinor:    10000,
		DepositMinor:    50000,
	})
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	if out.Nights != 3 || out.BaseMinor != 120000 || out.DiscountMinor != 12000 {
		t.Fatalf("base example: %+v", out)
	}
	if out.StayDueMinor != 118000 || out.RevenueMinor != 118000 {
		t.Fatalf("stay due should be 1180 SAR after the sample cleaning fee: %+v", out)
	}
	if out.CollectMinor != 168000 {
		t.Fatalf("collected total includes the sample deposit: %d", out.CollectMinor)
	}
	if out.RevenueMinor == out.CollectMinor || out.DepositMinor == 0 {
		t.Fatal("the sample deposit was treated as stay revenue")
	}
	deposit := out.Lines[len(out.Lines)-1]
	if deposit.Code != "deposit" || deposit.CountsAsStay {
		t.Fatalf("deposit line: %+v", deposit)
	}
}

func TestOneOfferAndCalendarMonths(t *testing.T) {
	_, issues := Price(Input{
		Transaction: "daily", Arrival: mustDate(t, "2026-10-01"), Departure: mustDate(t, "2026-10-04"),
		Guests: 1, UnitMinor: 40000, Offers: 2, DiscountPercent: 10,
	})
	if !has(issues, "one_offer_only") {
		t.Fatal(issues)
	}
	_, issues = Price(Input{
		Transaction: "weekly", Arrival: mustDate(t, "2026-10-01"), Departure: mustDate(t, "2026-10-08"),
		Guests: 1, UnitMinor: 40000,
	})
	if !has(issues, "invalid_transaction") {
		t.Fatal("weekly was accepted as inventory")
	}
	if _, ok := ExactMonths(mustDate(t, "2027-01-01"), mustDate(t, "2027-01-31")); ok {
		t.Fatal("a 30-day span was treated as a calendar month")
	}
	out, issues := Price(Input{
		Transaction: "monthly", Arrival: mustDate(t, "2027-01-31"), Departure: mustDate(t, "2027-02-28"),
		Guests: 1, UnitMinor: 300000,
	})
	if len(issues) > 0 || out.Months != 1 || out.StayDueMinor != 300000 {
		t.Fatalf("january 31 to february 28: %+v %v", out, issues)
	}
	divided := int64(28) * 300000 / 30
	if out.StayDueMinor == divided {
		t.Fatal("the month was divided by 30")
	}
	out, issues = Price(Input{
		Transaction: "monthly", Arrival: mustDate(t, "2028-01-31"), Departure: mustDate(t, "2028-02-29"),
		Guests: 1, UnitMinor: 100,
	})
	if len(issues) > 0 || out.Months != 1 {
		t.Fatalf("leap day: %+v %v", out, issues)
	}
	_, issues = Price(Input{
		Transaction: "monthly", Arrival: mustDate(t, "2027-03-01"), Departure: mustDate(t, "2027-03-31"),
		Guests: 1, UnitMinor: 100,
	})
	if !has(issues, "monthly_partial") {
		t.Fatal(issues)
	}
	_, issues = Price(Input{
		Transaction: "daily", Arrival: mustDate(t, "2026-10-01"), Departure: mustDate(t, "2026-10-01"),
		Guests: 1, UnitMinor: 100,
	})
	if !has(issues, "min_stay") {
		t.Fatal(issues)
	}
	capacity := 2
	_, issues = Price(Input{
		Transaction: "daily", Arrival: mustDate(t, "2026-10-01"), Departure: mustDate(t, "2026-10-02"),
		Guests: 3, Capacity: &capacity, UnitMinor: 100,
	})
	if !has(issues, "over_capacity") {
		t.Fatal(issues)
	}
}

func mustDate(t *testing.T, raw string) time.Time {
	t.Helper()
	value, ok := ParseDate(raw)
	if !ok {
		t.Fatal(raw)
	}
	return value
}

func has(issues []string, code string) bool {
	for _, item := range issues {
		if item == code {
			return true
		}
	}
	return false
}
