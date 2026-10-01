import { NextResponse } from "next/server";
import { Pool } from "pg";

export const dynamic = "force-dynamic";

const pool = new Pool({
  connectionString:
    process.env.SHAMAL_DATABASE_URL ??
    "postgres://shamal:shamal_local_dev@127.0.0.1:55432/shamal?sslmode=disable",
});

const where = `
  AND ($4::uuid IS NULL OR l.branch_id = $4::uuid)
  AND ($5::uuid IS NULL OR l.neighborhood_id = $5::uuid)
  AND ($6 = '' OR b.transaction_type = $6)
`;

function riyadhDay(value: Date) {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Riyadh" }).format(value);
}

function nextDay(iso: string) {
  const [year, month, day] = iso.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  date.setUTCDate(date.getUTCDate() + 1);
  return date.toISOString().slice(0, 10);
}

function period(from: string, to: string) {
  if (!from) from = riyadhDay(new Date());
  if (!to) to = riyadhDay(new Date());
  if (!/^\d{4}-\d{2}-\d{2}$/.test(from) || !/^\d{4}-\d{2}-\d{2}$/.test(to) || to < from) return null;
  const end = nextDay(to);
  return {
    from,
    to,
    start: new Date(`${from}T00:00:00+03:00`),
    endAt: new Date(`${end}T00:00:00+03:00`),
    end,
  };
}

function uuidOrNull(value: string) {
  if (!value) return null;
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value) ? value : undefined;
}

async function count(sql: string, params: unknown[]) {
  const result = await pool.query<{ count: number }>(sql, params);
  return Number(result.rows[0]?.count ?? 0);
}

export async function GET(request: Request) {
  const meResponse = await fetch("http://127.0.0.1:4318/api/v1/me", {
    headers: { cookie: request.headers.get("cookie") ?? "" },
    cache: "no-store",
  });
  if (!meResponse.ok) {
    const body = await meResponse.json().catch(() => ({ message: "تعذر التحقق من الجلسة." }));
    return NextResponse.json(body, { status: meResponse.status });
  }
  const me = (await meResponse.json()) as { company_id?: string; actions?: { reports?: string[] } };
  if (!(me.actions?.reports ?? []).includes("view") || !me.company_id) {
    return NextResponse.json({ message: "لا توجد صلاحية لعرض التقارير." }, { status: 403 });
  }
  const query = new URL(request.url).searchParams;
  const window = period(query.get("from") ?? "", query.get("to") ?? "");
  if (!window) return NextResponse.json({ message: "الفترة غير صالحة." }, { status: 422 });
  const branch = uuidOrNull(query.get("branch_id") ?? "");
  const neighborhood = uuidOrNull(query.get("neighborhood_id") ?? "");
  if (branch === undefined || neighborhood === undefined) {
    return NextResponse.json({ message: "المعرف غير صالح." }, { status: 422 });
  }
  const params = [me.company_id, window.start, window.endAt, branch, neighborhood, query.get("transaction") ?? ""];
  try {
    const [movementCount, paymentRecords, refundRecords, arrivalDiffers] = await Promise.all([
      count(
        `SELECT count(*)::int AS count
         FROM finance.journals j
         JOIN stays.bookings b ON b.id = j.stay_id
         JOIN catalog.listings l ON l.id = b.listing_id
         WHERE b.company_id = $1::uuid
           AND j.created_at >= $2 AND j.created_at < $3
           ${where}`,
        params,
      ),
      count(
        `SELECT count(*)::int AS count
         FROM finance.attempts a
         JOIN stays.bookings b ON b.id = a.stay_id
         JOIN catalog.listings l ON l.id = b.listing_id
         WHERE b.company_id = $1::uuid
           AND a.created_at >= $2 AND a.created_at < $3
           ${where}`,
        params,
      ),
      count(
        `SELECT count(*)::int AS count
         FROM finance.refunds r
         JOIN stays.bookings b ON b.id = r.stay_id
         JOIN catalog.listings l ON l.id = b.listing_id
         WHERE b.company_id = $1::uuid
           AND r.created_at >= $2 AND r.created_at < $3
           ${where}`,
        params,
      ),
      count(
        `SELECT count(*)::int AS count
         FROM finance.attempts a
         JOIN stays.bookings b ON b.id = a.stay_id
         JOIN catalog.listings l ON l.id = b.listing_id
         WHERE b.company_id = $1::uuid
           AND a.created_at >= $2 AND a.created_at < $3
           AND (b.arrival < $7::date OR b.arrival >= $8::date)
           ${where}`,
        [...params, window.from, window.end],
      ),
    ]);
    return NextResponse.json({
      code: "movements",
      generated_at: new Date().toISOString(),
      timezone: "Asia/Riyadh",
      period_start: window.from,
      period_end: window.to,
      definition_ar: "حركات دفترية وقع وقت تسجيلها بتاريخ الدفع داخل النافذة بتوقيت الرياض.",
      definition_en: "Ledger movements whose recording time falls in the window by payment date, in Riyadh time.",
      source: "finance.journals.created_at; finance.attempts.created_at; finance.refunds.created_at",
      exclusions_ar: "وصول الضيف لا يختار الصفوف. محاولة الدفع وطلب الاسترداد والتسوية لا تُعلَّم مالاً تحرك.",
      exclusions_en: "Guest arrival does not select the rows. A payment attempt, a refund, and a settlement are not marked as money moved.",
      freshness_ar: "القراءة من جداول المصدر عند الطلب، فيدخل التحديث التشغيلي ضمن حد خمس دقائق دون مستودع منفصل.",
      freshness_en: "Figures are read from the source tables on request, so operational freshness stays inside five minutes without a separate warehouse.",
      commission_included: false,
      reconciliation: "not_applicable",
      measures: [
        {
          code: "movement_count",
          value: movementCount,
          unit: "count",
          definition_ar: "عدد القيود التي وقع وقت تسجيلها داخل النافذة. هذا عدّ بتاريخ الدفع، وليس بتاريخ وصول الضيف.",
          definition_en: "Journals whose recording time is inside the window. The count uses the payment date, not the guest arrival.",
        },
        {
          code: "payment_record_count",
          value: paymentRecords,
          unit: "count",
          definition_ar: "عدد محاولات الدفع بتاريخ تسجيلها. المحاولة ليست مالاً تحرك.",
          definition_en: "Payment attempts recorded inside the window. An attempt is not money moved.",
        },
        {
          code: "refund_record_count",
          value: refundRecords,
          unit: "count",
          definition_ar: "عدد سجلات الاسترداد بتاريخ إنشائها. السجل ليس مالاً تحرك.",
          definition_en: "Refund records created inside the window. A refund record is not money moved.",
        },
        {
          code: "marked_money_moved",
          value: 0,
          unit: "count",
          definition_ar: "لا يُعلَّم أي دفع أو استرداد أو تسوية على أنه مال تحرك.",
          definition_en: "No payment, refund, or settlement is marked as money moved.",
        },
        {
          code: "arrival_differs_count",
          value: arrivalDiffers,
          unit: "count",
          definition_ar: "محاولات دفع داخل النافذة بتاريخ الدفع، ووصول ضيفها خارج النافذة. هذه المحاولات ليست مالاً تحرك.",
          definition_en: "Payment attempts inside the window by payment date whose guest arrival is outside the window. These attempts are not money moved.",
        },
      ],
    });
  } catch {
    return NextResponse.json({ message: "تعذر قراءة الحركات المالية." }, { status: 500 });
  }
}
