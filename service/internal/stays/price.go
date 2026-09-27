package stays

import (
	"time"
)

const (
	HoldMinutes  = 10
	QuoteMinutes = 10
)

type Line struct {
	Code         string `json:"code"`
	Minor        int64  `json:"minor"`
	CountsAsStay bool   `json:"counts_as_stay"`
}

type Input struct {
	Transaction     string
	Arrival         time.Time
	Departure       time.Time
	Guests          int
	Capacity        *int
	UnitMinor       int64
	Offers          int
	DiscountPercent int
	ServiceMinor    int64
	DepositMinor    int64
	Now             time.Time
}

type Breakdown struct {
	Transaction   string    `json:"transaction_type"`
	Arrival       time.Time `json:"arrival"`
	Departure     time.Time `json:"departure"`
	Nights        int       `json:"nights"`
	Months        int       `json:"months"`
	Guests        int       `json:"guests"`
	BaseMinor     int64     `json:"base_minor"`
	DiscountMinor int64     `json:"discount_minor"`
	ServiceMinor  int64     `json:"service_minor"`
	TaxMinor      int64     `json:"tax_minor"`
	DepositMinor  int64     `json:"deposit_minor"`
	StayDueMinor  int64     `json:"stay_due_minor"`
	CollectMinor  int64     `json:"collect_minor"`
	RevenueMinor  int64     `json:"revenue_minor"`
	Lines         []Line    `json:"lines"`
	ExpiresAt     time.Time `json:"expires_at"`
}

func Price(in Input) (Breakdown, []string) {
	issues := make([]string, 0)
	arrival := dateOnly(in.Arrival)
	departure := dateOnly(in.Departure)
	out := Breakdown{
		Transaction: in.Transaction,
		Arrival:     arrival,
		Departure:   departure,
		Guests:      in.Guests,
		TaxMinor:    0,
	}
	switch in.Transaction {
	case "daily":
		out.Nights = Nights(arrival, departure)
		if out.Nights < 1 {
			issues = append(issues, "min_stay")
		}
	case "monthly":
		months, exact := ExactMonths(arrival, departure)
		if !exact {
			issues = append(issues, "monthly_partial")
		}
		out.Months = months
	default:
		issues = append(issues, "invalid_transaction")
	}
	if in.Guests < 1 {
		issues = append(issues, "invalid_guests")
	}
	if in.Capacity != nil && *in.Capacity > 0 && in.Guests > *in.Capacity {
		issues = append(issues, "over_capacity")
	}
	if in.UnitMinor <= 0 {
		issues = append(issues, "invalid_price")
	}
	if in.Offers > 1 {
		issues = append(issues, "one_offer_only")
	}
	if !in.Now.IsZero() {
		today := riyadhDate(in.Now)
		if arrival.Before(today) {
			issues = append(issues, "arrival_in_past")
		}
	}
	if len(issues) > 0 {
		return out, issues
	}
	if in.Transaction == "daily" {
		out.BaseMinor = int64(out.Nights) * in.UnitMinor
	} else {
		out.BaseMinor = int64(out.Months) * in.UnitMinor
	}
	if in.Offers == 1 && in.DiscountPercent > 0 {
		out.DiscountMinor = percentOf(out.BaseMinor, in.DiscountPercent)
	}
	if out.DiscountMinor > out.BaseMinor {
		return out, []string{"invalid_price"}
	}
	out.ServiceMinor = in.ServiceMinor
	out.DepositMinor = in.DepositMinor
	out.StayDueMinor = out.BaseMinor - out.DiscountMinor + out.ServiceMinor + out.TaxMinor
	out.CollectMinor = out.StayDueMinor + out.DepositMinor
	out.RevenueMinor = out.StayDueMinor
	out.Lines = []Line{
		{Code: "base", Minor: out.BaseMinor, CountsAsStay: true},
		{Code: "discount", Minor: out.DiscountMinor, CountsAsStay: true},
		{Code: "service", Minor: out.ServiceMinor, CountsAsStay: true},
		{Code: "tax", Minor: out.TaxMinor, CountsAsStay: true},
		{Code: "deposit", Minor: out.DepositMinor, CountsAsStay: false},
	}
	if !in.Now.IsZero() {
		out.ExpiresAt = in.Now.UTC().Add(QuoteMinutes * time.Minute)
	}
	return out, nil
}

func percentOf(base int64, percent int) int64 {
	if base <= 0 || percent <= 0 {
		return 0
	}
	return (base*int64(percent) + 50) / 100
}

func Nights(arrival, departure time.Time) int {
	return int(dateOnly(departure).Sub(dateOnly(arrival)).Hours() / 24)
}

func ExactMonths(arrival, departure time.Time) (int, bool) {
	arrival = dateOnly(arrival)
	departure = dateOnly(departure)
	if !departure.After(arrival) {
		return 0, false
	}
	for n := 1; n <= 240; n++ {
		next := AddCalendarMonths(arrival, n)
		if next.Equal(departure) {
			return n, true
		}
		if next.After(departure) {
			return 0, false
		}
	}
	return 0, false
}

func AddCalendarMonths(start time.Time, months int) time.Time {
	start = dateOnly(start)
	index := int(start.Month()) - 1 + months
	year := start.Year() + index/12
	monthIndex := index % 12
	if monthIndex < 0 {
		monthIndex += 12
		year--
	}
	month := time.Month(monthIndex + 1)
	day := start.Day()
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func riyadhDate(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Riyadh")
	if err != nil {
		loc = time.FixedZone("AST", 3*60*60)
	}
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func ParseDate(raw string) (time.Time, bool) {
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}
	return value, true
}
