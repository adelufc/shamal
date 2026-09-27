package reports

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"shamal/service/internal/platform"
)

type Filter struct {
	BranchID       string
	NeighborhoodID string
	Transaction    string
	Status         string
}

type Measure struct {
	Code         string `json:"code"`
	Value        any    `json:"value"`
	Unit         string `json:"unit"`
	DefinitionAR string `json:"definition_ar"`
	DefinitionEN string `json:"definition_en"`
}

type Report struct {
	Code               string    `json:"code"`
	GeneratedAt        time.Time `json:"generated_at"`
	Timezone           string    `json:"timezone"`
	PeriodStart        string    `json:"period_start"`
	PeriodEnd          string    `json:"period_end"`
	DefinitionAR       string    `json:"definition_ar"`
	DefinitionEN       string    `json:"definition_en"`
	Source             string    `json:"source"`
	ExclusionsAR       string    `json:"exclusions_ar"`
	ExclusionsEN       string    `json:"exclusions_en"`
	FreshnessAR        string    `json:"freshness_ar"`
	FreshnessEN        string    `json:"freshness_en"`
	CommissionIncluded bool      `json:"commission_included"`
	Reconciliation     string    `json:"reconciliation"`
	Measures           []Measure `json:"measures"`
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s Service) Run(ctx context.Context, code, company string, from, to string, filter Filter, now time.Time) (Report, error) {
	period, err := ParsePeriod(from, to, now)
	if err != nil {
		return Report{}, platform.E("invalid_dates", 422)
	}
	report := Report{
		Code: code, GeneratedAt: now.UTC(), Timezone: Timezone,
		PeriodStart: period.FromText, PeriodEnd: period.ToText,
		FreshnessAR:        "القراءة من جداول المصدر عند الطلب، فيدخل التحديث التشغيلي ضمن حد خمس دقائق دون مستودع منفصل.",
		FreshnessEN:        "Figures are read from the source tables on request, so operational freshness stays inside five minutes without a separate warehouse.",
		CommissionIncluded: false,
		Measures:           []Measure{},
	}
	switch code {
	case "visits":
		return s.visits(ctx, report, company, period, filter)
	case "deals":
		return s.deals(ctx, report, company, period, filter)
	case "occupancy":
		return s.occupancy(ctx, report, company, period, filter)
	case "bookings":
		return s.bookings(ctx, report, company, period, filter)
	case "payments":
		return s.payments(ctx, report, company, period, filter)
	default:
		return Report{}, platform.E("not_found", 404)
	}
}

func (s Service) visits(ctx context.Context, report Report, company string, period Period, filter Filter) (Report, error) {
	var count int64
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*)::bigint
		FROM visits.appointments v
		LEFT JOIN catalog.listings l ON l.id = v.listing_id
		WHERE v.company_id = $1::uuid
		  AND v.starts_at >= $2 AND v.starts_at < $3
		  AND ($4::uuid IS NULL OR v.branch_id = $4::uuid)
		  AND ($5::uuid IS NULL OR l.neighborhood_id = $5::uuid)
		  AND ($6 = '' OR l.transaction_type = $6)
		  AND ($7 = '' OR v.status = $7)
	`, company, period.StartAt, period.EndAt, uuidOrNil(filter.BranchID), uuidOrNil(filter.NeighborhoodID), filter.Transaction, filter.Status).Scan(&count)
	if err != nil {
		return Report{}, err
	}
	report.DefinitionAR = "عدد الزيارات التي يبدأ موعدها داخل اليوم بتوقيت الرياض."
	report.DefinitionEN = "Visits whose start falls inside the Riyadh day."
	report.Source = "visits.appointments.starts_at"
	report.ExclusionsAR = "يُستبعد الموعد فقط إذا أخرجه فلتر الفرع أو الحي أو نوع المعاملة أو الحالة."
	report.ExclusionsEN = "A visit drops out only when the branch, neighborhood, transaction, or status filter excludes it."
	report.Reconciliation = "not_applicable"
	report.Measures = []Measure{{
		Code: "count", Value: count, Unit: "count",
		DefinitionAR: "عدد المواعيد داخل النافذة. هذا عداد تشغيلي وليس إيراداً.",
		DefinitionEN: "Appointments inside the window. This is an operational count, not revenue.",
	}}
	return report, nil
}

func (s Service) deals(ctx context.Context, report Report, company string, period Period, filter Filter) (Report, error) {
	var count, agreement int64
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*)::bigint, coalesce(sum(amount_minor), 0)::bigint
		FROM deals.deals d
		WHERE d.company_id = $1::uuid
		  AND d.created_at >= $2 AND d.created_at < $3
		  AND d.is_test = false
		  AND ($4::uuid IS NULL OR d.branch_id = $4::uuid)
		  AND ($5::uuid IS NULL OR d.neighborhood_id = $5::uuid)
		  AND ($6 = '' OR d.transaction_type = $6)
		  AND ($7 = '' OR d.status = $7)
	`, company, period.StartAt, period.EndAt, uuidOrNil(filter.BranchID), uuidOrNil(filter.NeighborhoodID), filter.Transaction, filter.Status).Scan(&count, &agreement)
	if err != nil {
		return Report{}, err
	}
	report.DefinitionAR = "صفقات أُنشئت داخل النافذة، بلا صفقات الاختبار."
	report.DefinitionEN = "Deals created inside the window, excluding test deals."
	report.Source = "deals.deals.created_at"
	report.ExclusionsAR = "الصفقات المعلّمة is_test مستبعدة. مبلغ الاتفاق لقطة للاتفاق وليس إيراداً محصلاً ولا عمولة."
	report.ExclusionsEN = "Rows marked is_test are excluded. The agreement amount is a snapshot of the agreement, not collected revenue and not a commission."
	report.Reconciliation = "not_applicable"
	report.Measures = []Measure{
		{
			Code: "count", Value: count, Unit: "count",
			DefinitionAR: "عدد الصفقات غير التجريبية داخل النافذة.",
			DefinitionEN: "Non-test deals inside the window.",
		},
		{
			Code: "agreement_minor", Value: agreement, Unit: "minor",
			DefinitionAR: "مجموع amount_minor للاتفاق بالهللة. ليس إيراداً ولا عمولة.",
			DefinitionEN: "Sum of agreement amount_minor in halalas. It is not revenue and not a commission.",
		},
	}
	return report, nil
}

func (s Service) occupancy(ctx context.Context, report Report, company string, period Period, filter Filter) (Report, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT b.unit_id::text, b.arrival, b.departure
		FROM stays.bookings b
		JOIN catalog.listings l ON l.id = b.listing_id
		WHERE b.company_id = $1::uuid
		  AND b.status = 'confirmed'
		  AND ($2 = '' OR $2 = 'confirmed')
		  AND b.arrival < $3::date AND b.departure > $4::date
		  AND ($5::uuid IS NULL OR l.branch_id = $5::uuid)
		  AND ($6::uuid IS NULL OR l.neighborhood_id = $6::uuid)
		  AND ($7 = '' OR b.transaction_type = $7)
	`, company, filter.Status, period.End, period.From, uuidOrNil(filter.BranchID), uuidOrNil(filter.NeighborhoodID), filter.Transaction)
	if err != nil {
		return Report{}, err
	}
	defer rows.Close()
	nights := 0
	units := map[string]struct{}{}
	for rows.Next() {
		var unit string
		var arrival, departure time.Time
		if err := rows.Scan(&unit, &arrival, &departure); err != nil {
			return Report{}, err
		}
		overlap := OverlapNights(arrival, departure, period.From, period.End)
		if overlap > 0 {
			nights += overlap
			units[unit] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return Report{}, err
	}
	report.DefinitionAR = "ليالي الإشغال للوحدات المؤكدة التي تتقاطع إقامتها مع النافذة. النسبة غير محسوبة."
	report.DefinitionEN = "Occupied nights of confirmed stays that overlap the window. A rate is not calculated."
	report.Source = "stays.bookings arrival/departure where status is confirmed"
	report.ExclusionsAR = "التثبيت والإلغاء والانتهاء لا تُحسب. نسبة الإشغال تبقى فارغة لأن مقام السعة غير معتمد، ولا تُخلط بعمولة."
	report.ExclusionsEN = "Holds, cancellations, and expirations are not counted. The occupancy rate stays empty because a capacity denominator is not adopted, and commission is not mixed in."
	report.Reconciliation = "not_applicable"
	report.Measures = []Measure{
		{
			Code: "occupied_nights", Value: int64(nights), Unit: "nights",
			DefinitionAR: "تقاطع المدى نصف المفتوح للوصول والمغادرة مع نافذة الرياض، بالليالي.",
			DefinitionEN: "Intersection of the half-open arrival and departure with the Riyadh window, in nights.",
		},
		{
			Code: "distinct_units", Value: int64(len(units)), Unit: "count",
			DefinitionAR: "عدد الوحدات المختلفة التي لها ليلة مشغولة داخل النافذة.",
			DefinitionEN: "Distinct units with an occupied night inside the window.",
		},
		{
			Code: "occupancy_rate", Value: nil, Unit: "ratio",
			DefinitionAR: "النسبة غير معتمدة. لا يُخترع مقام للسعة ولا يُحسب ADR أو RevPAR.",
			DefinitionEN: "The rate is not adopted. No capacity denominator is invented, and ADR or RevPAR are not calculated.",
		},
	}
	return report, nil
}

func (s Service) bookings(ctx context.Context, report Report, company string, period Period, filter Filter) (Report, error) {
	status := filter.Status
	if status == "" {
		status = "confirmed"
	}
	var count, due int64
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*)::bigint, coalesce(sum(b.stay_due_minor), 0)::bigint
		FROM stays.bookings b
		JOIN catalog.listings l ON l.id = b.listing_id
		WHERE b.company_id = $1::uuid
		  AND b.arrival >= $2::date AND b.arrival < $3::date
		  AND b.status = $4
		  AND ($5::uuid IS NULL OR l.branch_id = $5::uuid)
		  AND ($6::uuid IS NULL OR l.neighborhood_id = $6::uuid)
		  AND ($7 = '' OR b.transaction_type = $7)
	`, company, period.From, period.End, status, uuidOrNil(filter.BranchID), uuidOrNil(filter.NeighborhoodID), filter.Transaction).Scan(&count, &due)
	if err != nil {
		return Report{}, err
	}
	report.DefinitionAR = "حجوزات يصل ضيفها داخل النافذة. المجموع الافتراضي للمؤكد فقط."
	report.DefinitionEN = "Bookings whose arrival falls inside the window. The default total is confirmed stays only."
	report.Source = "stays.bookings.arrival"
	report.ExclusionsAR = "غير المؤكد يخرج ما لم يُطلب بالحالة. stay_due_minor لقطة سعر الإعلان وليست إيراداً محصلاً ولا عمولة."
	report.ExclusionsEN = "Anything not confirmed is omitted unless that status is requested. stay_due_minor is the listing price snapshot, not collected revenue and not a commission."
	report.Reconciliation = "not_applicable"
	report.Measures = []Measure{
		{
			Code: "count", Value: count, Unit: "count",
			DefinitionAR: "عدد الحجوزات التي يقع وصولها داخل النافذة وبحالتها المطلوبة.",
			DefinitionEN: "Bookings whose arrival is inside the window and whose status matches the filter.",
		},
		{
			Code: "stay_due_minor", Value: due, Unit: "minor",
			DefinitionAR: "مجموع مستحق الإقامة بالهللة كما ثُبّت على الحجز. ليس تحصيلاً ولا عمولة.",
			DefinitionEN: "Sum of stay due in halalas as stored on the booking. It is not a collection and not a commission.",
		},
	}
	return report, nil
}

func (s Service) payments(ctx context.Context, report Report, company string, period Period, filter Filter) (Report, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT a.status, count(*)::bigint
		FROM finance.attempts a
		JOIN stays.bookings b ON b.id = a.stay_id
		JOIN catalog.listings l ON l.id = b.listing_id
		WHERE b.company_id = $1::uuid
		  AND b.arrival >= $2::date AND b.arrival < $3::date
		  AND ($4 = '' OR a.status = $4)
		  AND ($5::uuid IS NULL OR l.branch_id = $5::uuid)
		  AND ($6::uuid IS NULL OR l.neighborhood_id = $6::uuid)
		  AND ($7 = '' OR b.transaction_type = $7)
		GROUP BY a.status
	`, company, period.From, period.End, filter.Status, uuidOrNil(filter.BranchID), uuidOrNil(filter.NeighborhoodID), filter.Transaction)
	if err != nil {
		return Report{}, err
	}
	defer rows.Close()
	var attempts, pending, mismatch, collected int64
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return Report{}, err
		}
		attempts += count
		switch status {
		case "pending_review":
			pending += count
		case "mismatch_review":
			mismatch += count
		case "collected":
			collected += count
		}
	}
	if err := rows.Err(); err != nil {
		return Report{}, err
	}
	var collectedMinor int64
	err = s.Pool.QueryRow(ctx, `
		SELECT coalesce(sum(ll.amount_minor), 0)::bigint
		FROM finance.ledger_lines ll
		JOIN finance.journals j ON j.id = ll.journal_id
		JOIN finance.attempts a ON a.id = j.attempt_id
		JOIN stays.bookings b ON b.id = a.stay_id
		JOIN catalog.listings l ON l.id = b.listing_id
		WHERE ll.account = 'provider_clearing' AND ll.direction = 'debit'
		  AND b.company_id = $1::uuid
		  AND b.arrival >= $2::date AND b.arrival < $3::date
		  AND ($4 = '' OR a.status = $4)
		  AND ($5::uuid IS NULL OR l.branch_id = $5::uuid)
		  AND ($6::uuid IS NULL OR l.neighborhood_id = $6::uuid)
		  AND ($7 = '' OR b.transaction_type = $7)
	`, company, period.From, period.End, filter.Status, uuidOrNil(filter.BranchID), uuidOrNil(filter.NeighborhoodID), filter.Transaction).Scan(&collectedMinor)
	if err != nil {
		return Report{}, err
	}
	report.DefinitionAR = "محاولات الدفع لحجوزات يصل ضيفها داخل النافذة، والمحصل من دفتر القيود."
	report.DefinitionEN = "Payment attempts for bookings whose arrival is inside the window, and collected cash from the ledger."
	report.Source = "finance.attempts joined to stays.bookings.arrival; collected_minor from finance.ledger_lines provider_clearing debit"
	report.ExclusionsAR = "لا يُجمع حساب العمولة مع المحصل. مستحق الإقامة ليس هذا المقياس. التسوية لم تُشغّل."
	report.ExclusionsEN = "The commission account is not added into collected cash. Stay due is a different measure. Reconciliation has not been run."
	report.Reconciliation = "not_run"
	report.Measures = []Measure{
		{Code: "attempt_count", Value: attempts, Unit: "count", DefinitionAR: "عدد محاولات الدفع داخل النافذة.", DefinitionEN: "Payment attempts inside the window."},
		{Code: "pending_review_count", Value: pending, Unit: "count", DefinitionAR: "محاولات ما زالت قيد التحقق.", DefinitionEN: "Attempts still pending verification."},
		{Code: "mismatch_review_count", Value: mismatch, Unit: "count", DefinitionAR: "محاولات اختلاف المبلغ، بلا قيد تحصيل.", DefinitionEN: "Amount-mismatch attempts, with no collection journal."},
		{Code: "collected_count", Value: collected, Unit: "count", DefinitionAR: "محاولات حالتها collected.", DefinitionEN: "Attempts whose status is collected."},
		{Code: "collected_minor", Value: collectedMinor, Unit: "minor", DefinitionAR: "مجموع مدين provider_clearing بالهللة. هذا تحصيل من الدفتر وليس عمولة ولا مستحق إقامة.", DefinitionEN: "Sum of provider_clearing debits in halalas. This is ledger collection, not a commission and not stay due."},
		{Code: "collected_ratio", Value: Ratio(collected, attempts), Unit: "ratio", DefinitionAR: "عدد المحصل مقسوماً على عدد المحاولات. يفرغ إذا لم توجد محاولة.", DefinitionEN: "Collected attempts divided by attempts. Empty when there are no attempts."},
	}
	return report, nil
}

func uuidOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
