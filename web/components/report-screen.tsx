"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money } from "@/lib/chrome";
import type { Locale } from "@/lib/types";

type Measure = {
  code: string;
  value: number | null;
  unit: string;
  definition_ar: string;
  definition_en: string;
};

type Report = {
  code: string;
  generated_at: string;
  timezone: string;
  period_start: string;
  period_end: string;
  definition_ar: string;
  definition_en: string;
  source: string;
  exclusions_ar: string;
  exclusions_en: string;
  freshness_ar: string;
  freshness_en: string;
  commission_included: boolean;
  reconciliation: string;
  measures: Measure[];
};

const codes = ["visits", "deals", "occupancy", "bookings", "payments"] as const;

const copy = {
  ar: {
    title: "تقارير التشغيل",
    lead: "كل رقم معه تعريفه ووقت توليده. العمولة لا تدخل في أي مقياس، ونسبة الإشغال غير معتمدة. التصدير ليس جزءاً من صلاحية العرض.",
    code: "التقرير",
    from: "من",
    to: "إلى",
    branch: "الفرع",
    neighborhood: "الحي",
    transaction: "نوع المعاملة",
    status: "الحالة",
    today: "اليوم",
    week: "الأسبوع",
    month: "الشهر",
    method: "تفاصيل المنهجية",
    any: "الكل",
    run: "عرض التقرير",
    generated: "وقت التوليد",
    period: "الفترة بتوقيت الرياض",
    source: "المصدر",
    exclusions: "ما يُستبعد",
    freshness: "حداثة الرقم",
    commission: "العمولة غير داخلة في المقياس",
    reconciliation: "التسوية",
    login: "هذه الشاشة لموظف لديه صلاحية العرض.",
    desk: "دخول المكتب",
    english: "English",
    names: {
      visits: "الزيارات",
      deals: "الصفقات",
      occupancy: "الإشغال",
      bookings: "الحجوزات",
      payments: "المدفوعات",
    },
    transactions: { "": "الكل", sale: "بيع", annual: "سنوي", monthly: "شهري", daily: "يومي" },
    undefined: "غير معرّف",
    basis: {
      visits: "أساس التاريخ هو بداية موعد الزيارة بتوقيت الرياض. ليس تاريخ وصول الضيف ولا تاريخ الدفع.",
      deals: "أساس التاريخ هو وقت إنشاء الصفقة. ليس تاريخ وصول الضيف ولا تاريخ الدفع. صفقات الاختبار غير داخلة.",
      occupancy: "تُحسب ليالي الوحدات المؤكدة التي تتقاطع إقامتها مع الفترة. الوصول والمغادرة يحددان التقاطع. نسبة الإشغال غير محسوبة.",
      bookings: "أساس التاريخ هو تاريخ وصول الضيف. ترك الحالة على الافتراضي يحسب المؤكد فقط.",
      payments:
        "هذه مدفوعات حجوزات يقع وصول ضيفها داخل الفترة، وليست حركات مالية بتاريخ الدفع. ملف المتطلبات لم يحسم الوصول مقابل تاريخ الدفع، لذلك بقي الحساب كما هو.",
    },
    placesError: "تعذر تحميل الأحياء. القائمة لا تُعرض كأنها فارغة.",
    placesCap: "تظهر أول 100 حي حسب صلاحية العرض.",
  },
  en: {
    title: "Operational reports",
    lead: "Each figure carries its definition and the time it was generated. Commission is not part of any measure, and an occupancy rate is not adopted. Export is not included in view permission.",
    code: "Report",
    from: "From",
    to: "To",
    branch: "Branch",
    neighborhood: "Neighborhood",
    transaction: "Transaction",
    status: "Status",
    today: "Today",
    week: "Week",
    month: "Month",
    method: "Method details",
    any: "Any",
    run: "Show the report",
    generated: "Generated at",
    period: "Riyadh period",
    source: "Source",
    exclusions: "Exclusions",
    freshness: "Freshness",
    commission: "Commission is not included in the measure",
    reconciliation: "Reconciliation",
    login: "This screen is for a staff member who can view reports.",
    desk: "Work desk sign-in",
    english: "العربية",
    names: {
      visits: "Visits",
      deals: "Deals",
      occupancy: "Occupancy",
      bookings: "Bookings",
      payments: "Payments",
    },
    transactions: { "": "Any", sale: "Sale", annual: "Annual", monthly: "Monthly", daily: "Daily" },
    undefined: "Not defined",
    basis: {
      visits: "The date basis is the visit start in Riyadh time. It is not the guest arrival and not a payment date.",
      deals: "The date basis is when the deal was created. It is not the guest arrival and not a payment date. Test deals are excluded.",
      occupancy: "Occupied nights are confirmed stays that overlap the period. Arrival and departure set the overlap. An occupancy rate is not calculated.",
      bookings: "The date basis is the guest arrival. Leaving the status on the default counts confirmed bookings only.",
      payments:
        "These are payments for bookings whose guest arrival falls in the period, not money movements by payment date. The requirements file does not settle arrival versus payment date, so the calculation was left unchanged.",
    },
    placesError: "Neighborhoods could not be loaded. The list is not shown as if it were empty.",
    placesCap: "The first 100 neighborhoods available to this view permission are shown.",
  },
} as const;

export function ReportScreen({ locale, framed = false }: { locale: Locale; framed?: boolean }) {
  const text = copy[locale];
  const [code, setCode] = useState<(typeof codes)[number]>("visits");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [transaction, setTransaction] = useState("");
  const [status, setStatus] = useState("");
  const [places, setPlaces] = useState<{ id: string; name: string }[]>([]);
  const [placesNote, setPlacesNote] = useState("");
  const [branches, setBranches] = useState<{ id: string; name_ar: string; name_en: string }[]>([]);
    const [report, setReport] = useState<Report | null>(null);
    const measureName: Record<string, string> = {
      count: locale === "en" ? "Count" : "العدد",
      attempt_count: locale === "en" ? "Attempts" : "عدد المحاولات",
      collected_minor: locale === "en" ? "Collected" : "المحصّل",
      not_run: locale === "en" ? "Not run" : "لم يُشغَّل",
      agreement_minor: locale === "en" ? "Agreement amount" : "مبلغ الاتفاق",
      stay_due_minor: locale === "en" ? "Stay amount due" : "مستحق الإقامة",
      collected_count: locale === "en" ? "Collected attempts" : "عدد المحصّل",
      pending_review_count: locale === "en" ? "Pending review" : "قيد المراجعة",
      mismatch_review_count: locale === "en" ? "Amount mismatch" : "اختلاف المبلغ",
      collected_ratio: locale === "en" ? "Collected share" : "نسبة المحصّل",
      distinct_units: locale === "en" ? "Distinct units" : "وحدات مختلفة",
      occupancy_rate: locale === "en" ? "Occupancy rate" : "نسبة الإشغال",
      occupied_nights: locale === "en" ? "Occupied nights" : "ليالي الإشغال",
    };
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);

  async function load(next = code) {
    setLoading(true);
    setError(null);
    const query = new URLSearchParams();
    if (from) query.set("from", from);
    if (to) query.set("to", to);
    if (branch) query.set("branch_id", branch);
    if (neighborhood) query.set("neighborhood_id", neighborhood);
    if (transaction) query.set("transaction", transaction);
    if (status) query.set("status", status);
    const suffix = query.toString() ? `?${query}` : "";
    try {
      const body = await api<Report>(`/api/v1/reports/${next}${suffix}`, { headers: { "Accept-Language": locale } });
      setReport(body);
      if (!from && body.period_start) setFrom(body.period_start.slice(0, 10));
      if (!to && body.period_end) setTo(body.period_end.slice(0, 10));
    } catch (err) {
      setReport(null);
      setError(err);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    api<{ items: { id: string; texts?: { locale: string; name: string }[] }[]; total: number }>("/api/v1/geo/neighborhoods?page_size=100")
      .then((body) => {
        setPlaces(
          body.items.map((item) => ({
            id: item.id,
            name: item.texts?.find((text) => text.locale === locale)?.name || item.texts?.[0]?.name || item.id,
          })),
        );
        setPlacesNote(body.total > body.items.length ? text.placesCap : "");
      })
      .catch(() => {
        setPlaces([]);
        setPlacesNote(text.placesError);
      });
    api<{ nodes: { id: string; kind: string; name_ar: string; name_en: string; status: string }[] }>("/api/v1/org/structure")
      .then((structure) => setBranches(structure.nodes.filter((node) => node.kind === "branch" && node.status === "active")))
      .catch(() => setBranches([]));
    void load("visits");
    // The first read uses the server's Riyadh day when the date fields are empty.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [locale]);

  function fillPeriod(daysBack: number) {
    const end = new Date();
    const start = new Date(end.getTime() - daysBack * 24 * 60 * 60 * 1000);
    const stamp = (value: Date) => value.toLocaleDateString("en-CA", { timeZone: "Asia/Riyadh" });
    setFrom(stamp(start));
    setTo(stamp(end));
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    void load(code);
  }

  const body = (
    <section className="grid gap-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        {framed ? null : (
          <a className="text-sm underline" href="/en/reports">
            {text.english}
          </a>
        )}
      </div>
      <p className="text-sm text-ink/80">{text.lead}</p>
      <form className="grid gap-3 rounded-lg border border-line bg-card p-4 sm:grid-cols-2" onSubmit={submit}>
        <Label>
          {text.code}
          <select
            className="rounded-md border border-line bg-paper px-2 py-2"
            value={code}
            onChange={(event) => {
              setCode(event.target.value as (typeof codes)[number]);
              setStatus("");
            }}
          >
            {codes.map((item) => (
              <option key={item} value={item}>
                {text.names[item]}
              </option>
            ))}
          </select>
        </Label>
        <Label>
          {text.transaction}
          <select className="rounded-md border border-line bg-paper px-2 py-2" value={transaction} onChange={(event) => setTransaction(event.target.value)}>
            {Object.entries(text.transactions).map(([value, label]) => (
              <option key={value || "any"} value={value}>
                {label}
              </option>
            ))}
          </select>
        </Label>
        <Label>
          {text.from}
          <Input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
        </Label>
        <Label>
          {text.to}
          <Input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
        </Label>
        <Label>
          {text.neighborhood}
          <select className="rounded-md border border-line bg-paper px-2 py-2" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)}>
            <option value="">{text.any}</option>
            {places.map((place) => (
              <option key={place.id} value={place.id}>
                {place.name}
              </option>
            ))}
          </select>
          {placesNote ? <span className="text-sm text-ink/80">{placesNote}</span> : null}
        </Label>
        <Label>
          {text.status}
          <select className="min-h-11 rounded-lg border border-[#7B877D] bg-card px-3" value={status} onChange={(event) => setStatus(event.target.value)}>
            {statusChoices(code).map((item) => (
              <option key={`${code}-${item.value || "any"}`} value={item.value}>
                {locale === "en" ? item.en : item.ar}
              </option>
            ))}
          </select>
        </Label>
        <Label>
          {text.branch}
          <select className="rounded-md border border-line bg-paper px-2 py-2" value={branch} onChange={(event) => setBranch(event.target.value)}>
            <option value="">{text.any}</option>
            {branches.map((item) => (
              <option key={item.id} value={item.id}>
                {locale === "en" ? item.name_en : item.name_ar}
              </option>
            ))}
          </select>
        </Label>
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="ghost" onClick={() => fillPeriod(0)}>
            {text.today}
          </Button>
          <Button type="button" variant="ghost" onClick={() => fillPeriod(6)}>
            {text.week}
          </Button>
          <Button type="button" variant="ghost" onClick={() => fillPeriod(29)}>
            {text.month}
          </Button>
        </div>
        <p className="text-sm text-muted sm:col-span-2">{text.basis[code]}</p>
        <Button type="submit">{text.run}</Button>
      </form>
      {loading ? <p className="text-sm">{locale === "en" ? "Loading…" : "جارٍ التحميل…"}</p> : null}
      {error ? (
        <div className="grid gap-2">
          <Failure error={error} />
          {error instanceof ApiError && (error.status === 401 || error.status === 403) ? (
            <a className="text-sm underline" href="/admin/login">
              {text.desk}
            </a>
          ) : null}
          <p className="text-sm text-ink/70">{text.login}</p>
        </div>
      ) : null}
      {report ? (
        <article className="grid gap-3 rounded-lg border border-line bg-card p-4">
          <h2 className="text-lg font-semibold">{text.names[report.code as (typeof codes)[number]] ?? report.code}</h2>
          <p>{readable(locale === "en" ? report.definition_en : report.definition_ar, locale)}</p>
          <p className="text-sm text-ink/80">{text.basis[report.code as (typeof codes)[number]] ?? text.basis[code]}</p>
          <dl className="grid gap-2 text-sm">
            <div>
              <dt className="text-muted">{text.generated}</dt>
              <dd>{new Date(report.generated_at).toLocaleString(locale === "en" ? "en-SA" : "ar-SA", { timeZone: "Asia/Riyadh" })} · {report.timezone}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.period}</dt>
              <dd>
                {report.period_start} – {report.period_end} ({report.timezone})
              </dd>
            </div>
            <details>
              <summary>{text.method}</summary>
              <p>
                {text.source}: {readable(report.source, locale)}
              </p>
            </details>
            <div>
              <dt className="text-ink/70">{text.exclusions}</dt>
              <dd>{readable(locale === "en" ? report.exclusions_en : report.exclusions_ar, locale)}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.freshness}</dt>
              <dd>{locale === "en" ? report.freshness_en : report.freshness_ar}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.reconciliation}</dt>
              <dd>{readable(report.reconciliation, locale)}</dd>
            </div>
          </dl>
          <p className="text-sm font-semibold">{text.commission}</p>
          <ul className="grid gap-3">
            {report.measures.map((item) => (
              <li key={item.code} className="rounded-md border border-line p-3">
                <p className="font-semibold">{measureName[item.code] ?? (locale === "en" ? item.definition_en : item.definition_ar)}</p>
                <p className="text-lg">{formatMeasure(item, locale, text.undefined)}</p>
                <p className="text-sm text-ink/80">{readable(locale === "en" ? item.definition_en : item.definition_ar, locale)}</p>
              </li>
            ))}
          </ul>
        </article>
      ) : null}
    </section>
  );

  if (!framed) return body;
  return (
    <PublicFrame locale={locale} path="/en/reports">
      {body}
    </PublicFrame>
  );
}

function statusChoices(code: (typeof codes)[number]) {
  if (code === "visits") {
    return [
      { value: "", ar: "الكل", en: "Any" },
      { value: "new", ar: "جديد", en: "New" },
      { value: "awaiting_broker", ar: "بانتظار الوسيط", en: "Awaiting the broker" },
      { value: "awaiting_owner", ar: "بانتظار المالك", en: "Awaiting the owner" },
      { value: "confirmed", ar: "مؤكد", en: "Confirmed" },
      { value: "completed", ar: "مكتمل", en: "Completed" },
      { value: "cancelled", ar: "ملغى", en: "Cancelled" },
      { value: "no_show", ar: "لم يحضر", en: "No show" },
    ];
  }
  if (code === "deals") {
    return [
      { value: "", ar: "الكل", en: "Any" },
      { value: "draft", ar: "مسودة", en: "Draft" },
      { value: "qualified", ar: "مؤهل", en: "Qualified" },
      { value: "negotiation", ar: "تفاوض", en: "Negotiation" },
      { value: "awaiting_documents", ar: "بانتظار المستندات", en: "Awaiting documents" },
      { value: "preliminary_agreement", ar: "اتفاق مبدئي", en: "Preliminary agreement" },
      { value: "awaiting_external", ar: "بانتظار جهة خارجية", en: "Awaiting an external party" },
      { value: "closed_success", ar: "أُغلق بنجاح", en: "Closed successfully" },
      { value: "closed_unsuccessful", ar: "أُغلق دون إتمام", en: "Closed without completion" },
      { value: "cancelled", ar: "ملغى", en: "Cancelled" },
    ];
  }
  if (code === "occupancy") return [{ value: "", ar: "المؤكد فقط", en: "Confirmed only" }];
  if (code === "bookings") {
    return [
      { value: "", ar: "المؤكد (الافتراضي)", en: "Confirmed (default)" },
      { value: "confirmed", ar: "مؤكد", en: "Confirmed" },
      { value: "held", ar: "تثبيت", en: "Hold" },
      { value: "expired", ar: "منتهٍ", en: "Expired" },
    ];
  }
  return [
    { value: "", ar: "الكل", en: "Any" },
    { value: "pending_review", ar: "قيد المراجعة", en: "Pending review" },
    { value: "mismatch_review", ar: "اختلاف المبلغ", en: "Amount mismatch" },
    { value: "collected", ar: "محصّل", en: "Collected" },
  ];
}

function readable(text: string, locale: Locale) {
  const ar = locale !== "en";
  const pairs: [string, string][] = [
    [
      "finance.attempts joined to stays.bookings.arrival; collected_minor from finance.ledger_lines provider_clearing debit",
      ar ? "محاولات الدفع لحجوزات الوصول في الفترة، والمحصّل من قيد حساب التحصيل" : "Payment attempts for arrivals in the period, and collected cash from the collection account",
    ],
    ["stays.bookings arrival/departure where status is confirmed", ar ? "وصول ومغادرة الحجوزات المؤكدة" : "Arrival and departure of confirmed bookings"],
    ["visits.appointments.starts_at", ar ? "موعد بداية الزيارة" : "Visit start"],
    ["deals.deals.created_at", ar ? "وقت إنشاء الصفقة" : "Deal creation time"],
    ["stays.bookings.arrival", ar ? "تاريخ وصول الحجز" : "Booking arrival date"],
    ["provider_clearing", ar ? "حساب التحصيل" : "the collection account"],
    ["pending_review", ar ? "قيد المراجعة" : "pending review"],
    ["mismatch_review", ar ? "اختلاف المبلغ" : "amount mismatch"],
    ["collected_minor", ar ? "المحصّل" : "collected amount"],
    ["stay_due_minor", ar ? "مستحق الإقامة" : "stay amount due"],
    ["amount_minor", ar ? "مبلغ الاتفاق" : "agreement amount"],
    ["is_test", ar ? "صفقة اختبار" : "test deal"],
    ["not_applicable", ar ? "لا تنطبق" : "Not applicable"],
    ["not_run", ar ? "لم تُشغَّل" : "Not run"],
    ["collected", ar ? "محصّل" : "collected"],
    ["ADR أو RevPAR", ar ? "مؤشرات سعر الغرفة" : "ADR أو RevPAR"],
    ["ADR or RevPAR", ar ? "ADR or RevPAR" : "room-rate indicators"],
  ];
  return pairs.reduce((value, [from, to]) => value.split(from).join(to), text);
}

function formatMeasure(item: Measure, locale: Locale, undefinedLabel: string) {
  if (item.value === null || item.value === undefined) return undefinedLabel;
  if (item.unit === "minor") return money(item.value, locale);
  if (item.unit === "ratio") return new Intl.NumberFormat(locale === "en" ? "en" : "ar", { maximumFractionDigits: 2 }).format(item.value);
  return new Intl.NumberFormat(locale === "en" ? "en" : "ar").format(item.value);
}
