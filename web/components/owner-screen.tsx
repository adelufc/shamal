"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import type { Locale } from "@/lib/types";

type Measure = {
  code: string;
  value: number | null;
  unit: string;
  definition_ar: string;
  definition_en: string;
};

type OwnerListing = {
  id: string;
  status: string;
  transaction_type: string;
  unit_number: string;
  title_ar: string;
  title_en: string;
};

type OwnerView = {
  generated_at: string;
  timezone: string;
  period_start: string;
  period_end: string;
  definition_ar: string;
  definition_en: string;
  exclusions_ar: string;
  exclusions_en: string;
  commission_included: boolean;
  measures: Measure[];
  listings: OwnerListing[];
};

const copy = {
  ar: {
    title: "أداء الملاك",
    lead: "عدادات وحدات هذا الحساب وإعلاناته وليالي الإشغال المؤكدة. النسبة غير معتمدة، ولا تظهر عمولة أو ضريبة أو تسوية أو إدارة إيراد.",
    from: "من",
    to: "إلى",
    run: "عرض",
    generated: "وقت التوليد",
    period: "نافذة الليالي بتوقيت الرياض",
    exclusions: "ما لا يُحسب",
    listings: "إعلانات هذا المالك",
    empty: "لا وحدات أو إعلانات مسجّلة لهذا الحساب.",
    untitled: "لا عنوان بهذه اللغة.",
    guest: "هذه الشاشة لحساب يملك وحدات.",
    account: "دخول الحساب",
    desk: "دخول المكتب",
    loading: "جارٍ التحميل…",
    rate: "غير معتمدة",
    names: {
      units: "الوحدات",
      listings_total: "الإعلانات",
      listings_published: "الإعلانات المنشورة",
      occupied_nights: "ليالي الإشغال",
      distinct_units: "وحدات مشغولة",
      occupancy_rate: "نسبة الإشغال",
    },
  },
  en: {
    title: "Owner performance",
    lead: "Counts of this account's units and listings, and confirmed occupied nights. A rate is not adopted. Commission, tax, settlement, and revenue management are not shown.",
    from: "From",
    to: "To",
    run: "Show",
    generated: "Generated at",
    period: "Night window in Riyadh time",
    exclusions: "Left out",
    listings: "This owner's listings",
    empty: "This account has no recorded units or listings.",
    untitled: "No title in this language.",
    guest: "This screen is for an account that owns units.",
    account: "Account sign-in",
    desk: "Work desk sign-in",
    loading: "Loading…",
    rate: "Not adopted",
    names: {
      units: "Units",
      listings_total: "Listings",
      listings_published: "Published listings",
      occupied_nights: "Occupied nights",
      distinct_units: "Occupied units",
      occupancy_rate: "Occupancy rate",
    },
  },
} as const;

export function OwnerScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [view, setView] = useState<OwnerView | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "guest" | "error">("loading");
  const [error, setError] = useState<unknown>(null);

  async function load(start = from, end = to) {
    const query = new URLSearchParams();
    if (start) query.set("from", start);
    if (end) query.set("to", end);
    const suffix = query.toString() ? `?${query.toString()}` : "";
    setView(await api<OwnerView>(`/api/v1/owner/performance${suffix}`));
  }

  useEffect(() => {
    let stop = false;
    load()
      .then(() => {
        if (!stop) setState("ready");
      })
      .catch((caught: unknown) => {
        if (stop) return;
        setError(caught);
        setState(caught instanceof ApiError && caught.status === 401 ? "guest" : "error");
      });
    return () => {
      stop = true;
    };
  }, [locale]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setState("loading");
    try {
      await load();
      setState("ready");
    } catch (caught) {
      setError(caught);
      setState(caught instanceof ApiError && caught.status === 401 ? "guest" : "error");
    }
  }

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">{text.title}</h1>
      <p className="text-sm text-ink/80">{text.lead}</p>
      {state === "guest" ? (
        <p className="text-sm">
          {text.guest}{" "}
          <a className="underline" href={locale === "en" ? "/en/account" : "/account"}>
            {text.account}
          </a>
          {" · "}
          <a className="underline" href="/admin/login">
            {text.desk}
          </a>
        </p>
      ) : null}
      {state !== "guest" ? (
        <form className="flex flex-wrap items-end gap-3" onSubmit={(event) => void submit(event)}>
          <Label>
            {text.from}
            <Input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
          </Label>
          <Label>
            {text.to}
            <Input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
          </Label>
          <Button type="submit">{text.run}</Button>
        </form>
      ) : null}
      {state === "loading" ? <p className="text-sm">{text.loading}</p> : null}
      {error && state !== "guest" ? <Failure error={error} /> : null}
      {view && state === "ready" ? (
        <>
          <p className="text-sm">{locale === "en" ? view.definition_en : view.definition_ar}</p>
          <dl className="grid gap-2 text-sm">
            <div>
              <dt className="text-ink/70">{text.generated}</dt>
              <dd>{view.generated_at}</dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.period}</dt>
              <dd>
                {view.period_start} – {view.period_end} ({view.timezone})
              </dd>
            </div>
            <div>
              <dt className="text-ink/70">{text.exclusions}</dt>
              <dd>{locale === "en" ? view.exclusions_en : view.exclusions_ar}</dd>
            </div>
          </dl>
          <ul className="grid gap-3 sm:grid-cols-2">
            {view.measures.map((item) => (
              <li key={item.code} className="rounded-lg border border-line bg-card p-4">
                <p className="text-sm text-ink/70">{text.names[item.code as keyof typeof text.names] ?? item.code}</p>
                <p className="text-2xl font-semibold">{item.value === null ? text.rate : item.value}</p>
                <p className="mt-1 text-sm">{locale === "en" ? item.definition_en : item.definition_ar}</p>
              </li>
            ))}
          </ul>
          <section className="grid gap-2">
            <h2 className="text-lg font-semibold">{text.listings}</h2>
            {view.listings.length === 0 ? <p className="text-sm">{text.empty}</p> : null}
            <ul className="grid gap-2">
              {view.listings.map((item) => {
                const title = locale === "en" ? item.title_en : item.title_ar;
                return (
                  <li key={item.id} className="rounded-lg border border-line bg-card p-4 text-sm">
                    <p className="font-semibold">{title || text.untitled}</p>
                    <p>
                      {item.unit_number} · {item.status} · {item.transaction_type}
                    </p>
                  </li>
                );
              })}
            </ul>
          </section>
        </>
      ) : null}
    </section>
  );
}
