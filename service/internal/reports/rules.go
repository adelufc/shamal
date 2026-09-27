package reports

import (
	"errors"
	"time"
)

var errInvalidPeriod = errors.New("invalid period")

const Timezone = "Asia/Riyadh"

type Period struct {
	From     time.Time
	End      time.Time
	StartAt  time.Time
	EndAt    time.Time
	FromText string
	ToText   string
}

func ParsePeriod(from, to string, now time.Time) (Period, error) {
	loc := riyadh()
	if from == "" {
		from = now.In(loc).Format("2006-01-02")
	}
	if to == "" {
		to = now.In(loc).Format("2006-01-02")
	}
	startDay, err := time.Parse("2006-01-02", from)
	if err != nil {
		return Period{}, err
	}
	endDay, err := time.Parse("2006-01-02", to)
	if err != nil {
		return Period{}, err
	}
	if endDay.Before(startDay) {
		return Period{}, errInvalidPeriod
	}
	startAt := time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, loc)
	endAt := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	return Period{
		From:     civil(startDay),
		End:      civil(endDay.AddDate(0, 0, 1)),
		StartAt:  startAt,
		EndAt:    endAt,
		FromText: from,
		ToText:   to,
	}, nil
}

func OverlapNights(arrival, departure, winStart, winEnd time.Time) int {
	start := civil(arrival)
	if win := civil(winStart); win.After(start) {
		start = win
	}
	end := civil(departure)
	if win := civil(winEnd); win.Before(end) {
		end = win
	}
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start).Hours() / 24)
}

func Ratio(part, whole int64) any {
	if whole == 0 {
		return nil
	}
	return float64(part) / float64(whole)
}

func civil(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func riyadh() *time.Location {
	loc, err := time.LoadLocation(Timezone)
	if err != nil {
		return time.FixedZone("AST", 3*60*60)
	}
	return loc
}
