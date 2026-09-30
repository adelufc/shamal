"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money } from "@/lib/chrome";
import type { Locale } from "@/lib/types";

type Line = { code: string; minor: number; counts_as_stay: boolean };
type Policy = { version: number; summary_ar: string; summary_en: string; contractual: boolean };
type Quote = {
  listing_id: string;
  transaction_type: string;
  arrival: string;
  departure: string;
  nights: number;
  months: number;
  guests: number;
  stay_due_minor: number;
  collect_minor: number;
  revenue_minor: number;
  lines: Line[];
  available: boolean;
  policy: Policy;
  non_contractual: boolean;
  payment_status: string;
  commission_adopted: boolean;
  deposit_is_revenue: boolean;
  title_ar: string;
  title_en: string;
  includes_ar: string;
  includes_en: string;
  hold_minutes: number;
  quote_minutes: number;
};
type Booking = Quote & {
  id: string;
  status: string;
  policy_summary_ar: string;
  policy_summary_en: string;
  policy_version: number;
};

const copy = {
  ar: {
    title: "تسعير الإقامة وتثبيتها",
    lead: "اختيار الفترة لا يحجز الوحدة. التثبيت مؤقت من وقت الخادم، والتأكيد هنا يثبت الإشغال دون تحصيل.",
    missing: "اختر إعلاناً يومياً أو شهرياً من البحث.",
    unit: "الوحدة المختارة",
    approximate: "موقع تقريبي داخل الحي",
    noPhoto: "لا توجد صورة",
    arrival: "الوصول",
    departure: "المغادرة",
    guests: "الضيوف",
    price: "اعرض السعر",
    nights: "ليلة",
    months: "شهر تقويمي",
    due: "مستحق الإقامة",
    collect: "المبلغ الظاهر للتحصيل",
    revenue: "إيراد الإقامة",
    unavailable: "المدة متداخلة مع تثبيت أو إقامة مؤكدة. غيّر التواريخ.",
    accept: "أقبل نسخة الشروط غير التعاقدية المعروضة",
    hold: "تثبيت مؤقت",
    confirm: "تأكيد الإشغال",
    held: "الوحدة مثبتة مؤقتاً.",
    confirmed: "الإشغال مؤكد. لم يُحصّل مبلغ.",
    pay: "الدفع والفاتورة",
    message: "راسل عن الإقامة",
    dispute: "افتح نزاعاً",
    login: "ادخل إلى الحساب قبل التثبيت",
    lines: { base: "سعر المدة", discount: "خصم", service: "رسم خدمة", tax: "ضريبة", deposit: "تأمين" } as Record<string, string>,
    notAdopted: "غير معتمد",
    notRevenue: "ليس إيراداً",
    sample: "الأرقام التشغيلية في وثيقة المتطلبات مثال اختباري غير تعاقدي، ولا تُفرض على هذا الحجز.",
    mine: "إقاماتي",
    emptyMine: "لا توجد إقامات على هذا الحساب.",
  },
  en: {
    title: "Price and hold a stay",
    lead: "Choosing dates does not book the unit. The hold is temporary from the server clock, and confirmation here records occupancy without collecting money.",
    missing: "Choose a daily or monthly listing from search.",
    unit: "Selected unit",
    approximate: "Approximate location inside the neighborhood",
    noPhoto: "No photo",
    arrival: "Arrival",
    departure: "Departure",
    guests: "Guests",
    price: "Show the price",
    nights: "nights",
    months: "calendar months",
    due: "Stay amount",
    collect: "Amount shown to collect",
    revenue: "Stay revenue",
    unavailable: "These dates overlap a hold or a confirmed stay. Choose different dates.",
    accept: "I accept the non-contractual terms copy shown here",
    hold: "Temporary hold",
    confirm: "Confirm occupancy",
    held: "The unit is held temporarily.",
    confirmed: "Occupancy is confirmed. No money was collected.",
    pay: "Payment and invoice",
    message: "Message about this stay",
    dispute: "Open a dispute",
    login: "Sign in before placing a hold",
    lines: { base: "Stay rate", discount: "Discount", service: "Service fee", tax: "Tax", deposit: "Deposit" } as Record<string, string>,
    notAdopted: "not adopted",
    notRevenue: "not revenue",
    sample: "The operational figures in the requirements are a non-contractual test example. They are not charged on this stay.",
    mine: "My stays",
    emptyMine: "This account has no stays.",
  },
} as const;

export function StayScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const params = useSearchParams();
  const router = useRouter();
  const listing = params.get("listing") ?? "";
  const [arrival, setArrival] = useState(params.get("arrival") ?? "");
  const [departure, setDeparture] = useState(params.get("departure") ?? "");
  const [guests, setGuests] = useState(params.get("guests") ?? "1");
  const [quote, setQuote] = useState<Quote | null>(null);
  const [booking, setBooking] = useState<Booking | null>(null);
  const [mine, setMine] = useState<Booking[]>([]);
  const [accept, setAccept] = useState(false);
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [error, setError] = useState<unknown>(null);
  const [holdKey, setHoldKey] = useState("");
  const [unit, setUnit] = useState<{ title: string; neighborhood_name: string; price_minor: number; price_unit: string; transaction_type: string } | null>(null);
  const base = locale === "en" ? "/en/stay" : "/stay";
  const accountHref = locale === "en" ? "/en/account" : "/account";

  useEffect(() => {
    setHoldKey(crypto.randomUUID());
  }, []);

  useEffect(() => {
    if (!listing) {
      setUnit(null);
      return;
    }
    let stop = false;
    api<{ title: string; neighborhood_name: string; price_minor: number; price_unit: string; transaction_type: string }>(
      `/api/v1/public/listings/${listing}?locale=${locale}`,
    )
      .then((body) => {
        if (!stop) setUnit(body);
      })
      .catch(() => {
        if (!stop) setUnit(null);
      });
    return () => {
      stop = true;
    };
  }, [listing, locale]);

  useEffect(() => {
    setArrival(params.get("arrival") ?? "");
    setDeparture(params.get("departure") ?? "");
    setGuests(params.get("guests") ?? "1");
  }, [params]);

  useEffect(() => {
    const arrivalValue = params.get("arrival");
    const departureValue = params.get("departure");
    if (!listing || !arrivalValue || !departureValue) {
      setQuote(null);
      setState("idle");
      return;
    }
    let stop = false;
    setState("loading");
    const search = new URLSearchParams({
      listing_id: listing,
      arrival: arrivalValue,
      departure: departureValue,
      guests: params.get("guests") || "1",
      locale,
    });
    api<Quote>(`/api/v1/public/stays/quote?${search.toString()}`, { headers: { "Accept-Language": locale } })
      .then((body) => {
        if (stop) return;
        setQuote(body);
        setError(null);
        setState("ready");
      })
      .catch((caught) => {
        if (stop) return;
        setQuote(null);
        setError(caught);
        setState("error");
      });
    return () => {
      stop = true;
    };
  }, [listing, locale, params]);

  useEffect(() => {
    api<{ items: Booking[] }>("/api/v1/account/stays", { headers: { "Accept-Language": locale } })
      .then((body) => setMine(body.items ?? []))
      .catch(() => setMine([]));
  }, [locale, booking]);

  function push(nextArrival: string, nextDeparture: string, nextGuests: string) {
    const next = new URLSearchParams();
    if (listing) next.set("listing", listing);
    if (nextArrival) next.set("arrival", nextArrival);
    if (nextDeparture) next.set("departure", nextDeparture);
    if (nextGuests) next.set("guests", nextGuests);
    const query = next.toString();
    router.push(query ? `${base}?${query}` : base);
    setHoldKey(crypto.randomUUID());
    setBooking(null);
    setAccept(false);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    push(arrival, departure, guests);
  }

  async function hold() {
    if (!quote) return;
    setError(null);
    try {
      const body = await api<Booking>("/api/v1/account/stays", {
        method: "POST",
        headers: { "Accept-Language": locale, "Idempotency-Key": holdKey },
        body: JSON.stringify({
          listing_id: listing,
          arrival: quote.arrival,
          departure: quote.departure,
          guests: quote.guests,
          quoted_stay_due_minor: quote.stay_due_minor,
          policy_version: quote.policy.version,
          accept_policy: true,
        }),
      });
      setBooking(body);
    } catch (caught) {
      setError(caught);
    }
  }

  async function confirm() {
    if (!booking) return;
    setError(null);
    try {
      const body = await api<Booking>(`/api/v1/account/stays/${booking.id}/confirm`, {
        method: "POST",
        headers: { "Accept-Language": locale, "Idempotency-Key": `confirm-${booking.id}` },
        body: JSON.stringify({}),
      });
      setBooking(body);
    } catch (caught) {
      setError(caught);
    }
  }

  const path = `${base}${params.toString() ? `?${params.toString()}` : ""}`;
  const title = locale === "en" ? quote?.title_en : quote?.title_ar;
  const includes = locale === "en" ? quote?.includes_en : quote?.includes_ar;
  const policyText = locale === "en" ? quote?.policy.summary_en : quote?.policy.summary_ar;
  const signedOut = error instanceof ApiError && (error.status === 401 || error.status === 403);

  return (
    <PublicFrame locale={locale} path={path}>
      <section className="grid gap-4">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <p className="text-sm text-ink/80">{text.lead}</p>
        <p className="rounded-lg border border-line bg-olive-soft p-3 text-sm">{text.sample}</p>
        {!listing ? <p className="rounded-lg border border-dashed border-line bg-card p-4 text-sm">{text.missing}</p> : null}
        {listing ? (
          <article className="grid gap-2 overflow-hidden rounded-xl border border-line bg-card">
            <div className="flex aspect-[4/3] max-h-48 items-center justify-center bg-olive-soft text-sm text-muted">{text.noPhoto}</div>
            <div className="grid gap-1 p-4">
              <h2 className="font-semibold">{text.unit}</h2>
              <p>{unit?.title || listing}</p>
              {unit?.neighborhood_name ? <p className="text-sm text-muted">{unit.neighborhood_name}</p> : null}
              <p className="text-sm text-muted">{text.approximate}</p>
              {unit ? (
                <p>
                  {money(unit.price_minor, locale)} {unit.price_unit}
                </p>
              ) : null}
            </div>
          </article>
        ) : null}
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4 md:grid-cols-3" onSubmit={submit}>
          <Label>
            {text.arrival}
            <Input type="date" value={arrival} onChange={(event) => setArrival(event.target.value)} required />
          </Label>
          <Label>
            {text.departure}
            <Input type="date" value={departure} onChange={(event) => setDeparture(event.target.value)} required />
          </Label>
          <Label>
            {text.guests}
            <Input value={guests} inputMode="numeric" onChange={(event) => setGuests(event.target.value)} required />
          </Label>
          <div className="md:col-span-3">
            <Button type="submit">{text.price}</Button>
          </div>
        </form>
        {state === "loading" ? <p className="text-sm">{locale === "en" ? "Loading…" : "جارٍ التحميل…"}</p> : null}
        {state === "error" ? <Failure error={error} /> : null}
        {quote && state === "ready" ? (
          <div className="grid gap-3 rounded-lg border border-line bg-card p-4">
            <h2 className="font-semibold">{title}</h2>
            <p className="text-sm">
              {quote.departure}
              {quote.nights > 0 ? ` · ${quote.nights} ${text.nights}` : ""}
              {quote.months > 0 ? ` · ${quote.months} ${text.months}` : ""}
            </p>
            <ul className="grid gap-1 text-sm">
              {quote.lines.map((line) => (
                <li key={line.code}>
                  {text.lines[line.code] ?? line.code}: {money(line.minor, locale)}
                  {line.minor === 0 ? ` · ${text.notAdopted}` : ""}
                  {!line.counts_as_stay ? ` · ${text.notRevenue}` : ""}
                </li>
              ))}
            </ul>
            <p className="text-sm">
              {text.due}: {money(quote.stay_due_minor, locale)}
            </p>
            <p className="text-sm">
              {text.collect}: {money(quote.collect_minor, locale)} · {text.revenue}: {money(quote.revenue_minor, locale)}
            </p>
            <p className="text-sm text-ink/80">{includes}</p>
            <p className="text-sm">{policyText}</p>
            <p className="text-sm text-ink/70">
              {quote.quote_minutes} / {quote.hold_minutes}
            </p>
            {!quote.available ? <p className="text-sm">{text.unavailable}</p> : null}
            {booking?.status === "held" ? <p className="text-sm">{text.held}</p> : null}
            {booking?.status === "confirmed" ? (
              <p className="text-sm">
                {text.confirmed}{" "}
                <a className="underline" href={`${locale === "en" ? "/en/pay" : "/pay"}?stay=${booking.id}`}>
                  {text.pay}
                </a>{" "}
                <a className="underline" href={`${locale === "en" ? "/en/messages" : "/messages"}?subject_kind=booking&subject_id=${booking.id}`}>
                  {text.message}
                </a>{" "}
                <a className="underline" href={`${locale === "en" ? "/en/disputes" : "/disputes"}?subject_kind=booking&subject_id=${booking.id}`}>
                  {text.dispute}
                </a>
              </p>
            ) : null}
            {quote.available && !booking ? (
              <Label>
                <span className="flex items-center gap-2">
                  <input type="checkbox" checked={accept} onChange={(event) => setAccept(event.target.checked)} />
                  {text.accept}
                </span>
              </Label>
            ) : null}
            <div className="flex flex-wrap gap-2">
              {quote.available && !booking ? (
                <Button type="button" disabled={!accept || holdKey === ""} onClick={() => void hold()}>
                  {text.hold}
                </Button>
              ) : null}
              {booking?.status === "held" ? (
                <Button type="button" onClick={() => void confirm()}>
                  {text.confirm}
                </Button>
              ) : null}
            </div>
            {signedOut ? (
              <a className="text-sm underline" href={accountHref}>
                {text.login}
              </a>
            ) : null}
            {error && !signedOut ? <Failure error={error} /> : null}
          </div>
        ) : null}
        <section className="grid gap-2">
          <h2 className="text-lg font-semibold">{text.mine}</h2>
          {mine.length === 0 ? <p className="text-sm text-ink/70">{text.emptyMine}</p> : null}
          <ul className="grid gap-2">
            {mine.map((item) => (
              <li key={item.id} className="rounded-lg border border-line bg-card p-3 text-sm">
                <p>{locale === "en" ? item.title_en : item.title_ar}</p>
                <p>
                  {item.arrival} – {item.departure} · {item.status} · {money(item.stay_due_minor, locale)}
                  {item.status === "confirmed" ? (
                    <>
                      {" "}
                      <a className="underline" href={`${locale === "en" ? "/en/pay" : "/pay"}?stay=${item.id}`}>
                        {text.pay}
                      </a>{" "}
                      <a className="underline" href={`${locale === "en" ? "/en/messages" : "/messages"}?subject_kind=booking&subject_id=${item.id}`}>
                        {text.message}
                      </a>{" "}
                      <a className="underline" href={`${locale === "en" ? "/en/disputes" : "/disputes"}?subject_kind=booking&subject_id=${item.id}`}>
                        {text.dispute}
                      </a>
                    </>
                  ) : null}
                </p>
              </li>
            ))}
          </ul>
        </section>
      </section>
    </PublicFrame>
  );
}
