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
  const [branches, setBranches] = useState<{ id: string; name_ar: string; name_en: string }[]>([]);
  const [report, setReport] = useState<Report | null>(null);
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
      setReport(await api<Report>(`/api/v1/reports/${next}${suffix}`, { headers: { "Accept-Language": locale } }));
    } catch (err) {
      setReport(null);
      setError(err);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    api<{ neighborhoods: { id: string; name: string }[] }>(`/api/v1/public/home?locale=${locale}`)
      .then((home) => setPlaces(home.neighborhoods))
      .catch(() => setPlaces([]));
    api<{ nodes: { id: string; kind: string; name_ar: string; name_en: string; status: string }[] }>("/api/v1/org/structure")
      .then((structure) => setBranches(structure.nodes.filter((node) => node.kind === "branch" && node.status === "active")))
      .catch(() => setBranches([]));
    void load("visits");
    // The first read uses the server's Riyadh day when the date fields are empty.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [locale]);

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
          <select className="rounded-md border border-line bg-paper px-2 py-2" value={code} onChange={(event) => setCode(event.target.value as (typeof codes)[number])}>
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
        </Label>
        <Label>
          {text.status}
          <Input value={status} onChange={(event) => setStatus(event.target.value)} />
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
          <p>{locale === "en" ? report.definition_en : report.definition_ar}</p>
          <dl className="grid gap-2 text-sm">
            <div>
              <dt className="text-ink/70">{text.generated}</dt>
              <dd>{report.generated_at}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.period}</dt>
              <dd>
                {report.period_start} – {report.period_end} ({report.timezone})
              </dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.source}</dt>
              <dd>{report.source}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.exclusions}</dt>
              <dd>{locale === "en" ? report.exclusions_en : report.exclusions_ar}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.freshness}</dt>
              <dd>{locale === "en" ? report.freshness_en : report.freshness_ar}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.reconciliation}</dt>
              <dd>{report.reconciliation}</dd>
            </div>
          </dl>
          <p className="text-sm font-semibold">{text.commission}</p>
          <ul className="grid gap-3">
            {report.measures.map((item) => (
              <li key={item.code} className="rounded-md border border-line p-3">
                <p className="font-semibold">{item.code}</p>
                <p className="text-lg">{formatMeasure(item, locale, text.undefined)}</p>
                <p className="text-sm text-ink/80">{locale === "en" ? item.definition_en : item.definition_ar}</p>
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

function formatMeasure(item: Measure, locale: Locale, undefinedLabel: string) {
  if (item.value === null || item.value === undefined) return undefinedLabel;
  if (item.unit === "minor") return money(item.value, locale);
  if (item.unit === "ratio") return new Intl.NumberFormat(locale === "en" ? "en" : "ar", { maximumFractionDigits: 2 }).format(item.value);
  return new Intl.NumberFormat(locale === "en" ? "en" : "ar").format(item.value);
}
