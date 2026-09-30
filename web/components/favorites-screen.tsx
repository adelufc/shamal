"use client";

import { useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { money, priceUnit } from "@/lib/chrome";
import type { HomePayload, Locale, PublicListing } from "@/lib/types";

type Favorite = {
  listing_id: string;
  status: string;
  transaction_type: string;
  price_minor: number;
  price_unit: string;
  currency: string;
  title: string;
};

const copy = {
  ar: {
    title: "المفضلة",
    lead: "حفظ إعلان منشور في حساب العميل. هذا ليس البحث المحفوظ الذي يُستخدم للإشعار.",
    saved: "المحفوظة",
    empty: "لا توجد إعلانات في المفضلة.",
    available: "إعلانات منشورة يمكن حفظها",
    none: "لا توجد إعلانات منشورة لإضافتها.",
    add: "أضف إلى المفضلة",
    remove: "إزالة",
    untitled: "لا عنوان بهذه اللغة.",
    kept: "محفوظة",
    guest: "المفضلة لحساب عميل مسجّل.",
    desk: "دخول الحساب",
    staff: "حساب المكتب لا يحفظ المفضلة. ادخل بحساب عميل.",
    loading: "جارٍ التحميل…",
  },
  en: {
    title: "Favorites",
    lead: "Save a published listing on the client account. This is not the saved search used for notices.",
    saved: "Saved",
    empty: "There are no favorites.",
    available: "Published listings you can save",
    none: "There are no published listings to add.",
    add: "Save",
    remove: "Remove",
    untitled: "No title in this language.",
    kept: "Saved",
    guest: "Favorites belong to a signed-in client account.",
    desk: "Account sign-in",
    staff: "A work-desk account does not keep favorites. Sign in with a client account.",
    loading: "Loading…",
  },
} as const;

export function FavoritesScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const [items, setItems] = useState<Favorite[]>([]);
  const [listings, setListings] = useState<PublicListing[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "guest" | "staff" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const accountHref = locale === "en" ? "/en/account" : "/account";

  async function load() {
    const [saved, home] = await Promise.all([
      api<{ items: Favorite[] }>(`/api/v1/account/favorites?locale=${locale}`),
      api<HomePayload>(`/api/v1/public/home?locale=${locale}`),
    ]);
    setItems(saved.items);
    setListings(home.listings);
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
        if (caught instanceof ApiError && caught.status === 401) setState("guest");
        else if (caught instanceof ApiError && caught.status === 403) setState("staff");
        else setState("error");
      });
    return () => {
      stop = true;
    };
  }, [locale]);

  async function add(listingID: string) {
    setError(null);
    try {
      await api(`/api/v1/account/favorites?locale=${locale}`, {
        method: "POST",
        body: JSON.stringify({ listing_id: listingID }),
      });
      await load();
      setState("ready");
    } catch (caught) {
      setError(caught);
    }
  }

  async function remove(listingID: string) {
    setError(null);
    try {
      await api(`/api/v1/account/favorites/${listingID}?locale=${locale}`, { method: "DELETE" });
      await load();
      setState("ready");
    } catch (caught) {
      setError(caught);
    }
  }

  const savedIDs = new Set(items.map((item) => item.listing_id));
  const titleOf = (item: PublicListing) => item.texts?.find((row) => row.locale === locale)?.title ?? "";

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">{text.title}</h1>
      <p className="text-sm text-ink/80">{text.lead}</p>
      {state === "loading" ? <p className="text-sm">{text.loading}</p> : null}
      {state === "guest" ? (
        <p className="text-sm">
          {text.guest}{" "}
          <a className="underline" href={accountHref}>
            {text.desk}
          </a>
        </p>
      ) : null}
      {state === "staff" ? (
        <p className="text-sm">
          {text.staff}{" "}
          <a className="underline" href={accountHref}>
            {text.desk}
          </a>
        </p>
      ) : null}
      {error && state !== "guest" && state !== "staff" ? <Failure error={error} /> : null}
      {state === "ready" ? (
        <>
          <section className="grid gap-2">
            <h2 className="text-lg font-semibold">{text.saved}</h2>
            {items.length === 0 ? <p className="text-sm">{text.empty}</p> : null}
            <ul className="grid gap-3">
              {items.map((item) => (
                <li key={item.listing_id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
                  <h3 className="font-semibold">{item.title || text.untitled}</h3>
                  <p>
                    {money(item.price_minor, locale)} {priceUnit(locale, item.price_unit, item.transaction_type)}
                  </p>
                  <div>
                    <Button type="button" onClick={() => void remove(item.listing_id)}>
                      {text.remove}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          </section>
          <section className="grid gap-2">
            <h2 className="text-lg font-semibold">{text.available}</h2>
            {listings.length === 0 ? <p className="text-sm">{text.none}</p> : null}
            <ul className="grid gap-3">
              {listings.map((item) => (
                <li key={item.id} className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-line bg-card p-4 text-sm">
                  <span className="font-semibold">{titleOf(item) || text.untitled}</span>
                  {savedIDs.has(item.id) ? (
                    <span>{text.kept}</span>
                  ) : (
                    <Button type="button" onClick={() => void add(item.id)}>
                      {text.add}
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          </section>
        </>
      ) : null}
    </section>
  );
}
