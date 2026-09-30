"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import type { Locale, SessionUser } from "@/lib/types";

type Place = { id: string; name: string };
type Search = { id: string; name: string; neighborhood_id: string; transaction_type: string; muted: boolean; channel: string };
type Notice = {
  id: string;
  kind: string;
  title_ar: string;
  title_en: string;
  body_ar: string;
  body_en: string;
  delivery_status: string;
  provider_adopted: boolean;
  channel: string;
};

const copy = {
  ar: {
    title: "الإشعارات",
    lead: "الإشعار يبقى داخل التطبيق. لا بريد تسويقي، ولا مزود رسائل معتمد، فحالة التسليم تبقى غير مُرسلة.",
    login: "ادخل إلى الحساب لحفظ بحث أو مراجعة الإشعارات.",
    account: "الحساب",
    staffNote: "حفظ البحث لحساب العميل. هذه الشاشة تعرض إشعارات حساب المكتب داخل التطبيق، ويمكن كتم القناة.",
    name: "اسم البحث",
    neighborhood: "الحي",
    transaction: "نوع المعاملة",
    any: "أي نوع",
    save: "حفظ البحث",
    searches: "بحوث محفوظة",
    emptySearches: "لا يوجد بحث محفوظ.",
    muteSearch: "كتم هذا البحث",
    inbox: "الإشعارات",
    empty: "لا توجد إشعارات جديدة",
    back: "العودة إلى لوحة العمل",
    mute: "كتم إشعارات التطبيق",
    unmute: "إعادة إشعارات التطبيق",
    notSent: "لم يُرسل إلى مزود",
    kinds: { listing_match: "إعلان مطابق", visit_change: "تغيّر زيارة", payment_review: "مراجعة دفعة" },
    transactions: { "": "أي نوع", sale: "بيع", annual: "سنوي", monthly: "شهري", daily: "يومي" },
  },
  en: {
    title: "Notices",
    lead: "A notice stays in the app. There is no marketing mail and no adopted message provider, so delivery stays not sent.",
    login: "Sign in to save a search or read notices.",
    account: "Account",
    staffNote: "Saving a search belongs to a client account. This screen shows this desk account's in-app notices, and the channel can be muted.",
    name: "Search name",
    neighborhood: "Neighborhood",
    transaction: "Transaction",
    any: "Any type",
    save: "Save search",
    searches: "Saved searches",
    emptySearches: "There is no saved search.",
    muteSearch: "Mute this search",
    inbox: "Notices",
    empty: "There are no new notices.",
    back: "Back to the work desk",
    mute: "Mute in-app notices",
    unmute: "Turn in-app notices back on",
    notSent: "Not sent to a provider",
    kinds: { listing_match: "Matching listing", visit_change: "Visit change", payment_review: "Payment review" },
    transactions: { "": "Any type", sale: "Sale", annual: "Annual", monthly: "Monthly", daily: "Daily" },
  },
} as const;

export function NotificationScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const [places, setPlaces] = useState<Place[]>([]);
  const [searches, setSearches] = useState<Search[]>([]);
  const [items, setItems] = useState<Notice[]>([]);
  const [muted, setMuted] = useState(false);
  const [name, setName] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [transaction, setTransaction] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [guest, setGuest] = useState(false);
  const [clientAccount, setClientAccount] = useState(false);

  async function reload(forClient: boolean) {
    const inbox = await api<{ items: Notice[]; channel_muted: boolean }>("/api/v1/account/notifications", {
      headers: { "Accept-Language": locale },
    });
    setItems(inbox.items);
    setMuted(inbox.channel_muted);
    if (!forClient) {
      setSearches([]);
      return;
    }
    const saved = await api<{ items: Search[] }>("/api/v1/account/searches", { headers: { "Accept-Language": locale } });
    setSearches(saved.items);
  }

  useEffect(() => {
    api<{ neighborhoods: Place[] }>(`/api/v1/public/home?locale=${locale}`)
      .then((home) => {
        setPlaces(home.neighborhoods);
        setNeighborhood(home.neighborhoods[0]?.id ?? "");
      })
      .catch(() => setPlaces([]));
    api<SessionUser>("/api/v1/me")
      .then(async (me) => {
        const forClient = me.context === "client";
        setClientAccount(forClient);
        await reload(forClient);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && (err.status === 401 || err.status === 403)) setGuest(true);
        else setError(err);
      })
      .finally(() => setLoading(false));
  }, [locale]);

  async function save(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/account/searches", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ name, neighborhood_id: neighborhood, transaction_type: transaction, locale, channel: "in_app" }),
      });
      setName("");
      await reload(true);
    } catch (err) {
      setError(err);
    }
  }

  async function muteOne(id: string) {
    setError(null);
    try {
      await api(`/api/v1/account/searches/${id}/mute`, { method: "POST", headers: { "Accept-Language": locale } });
      await reload(true);
    } catch (err) {
      setError(err);
    }
  }

  async function muteAll() {
    setError(null);
    try {
      await api("/api/v1/account/notifications/mute", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ channel: "in_app", muted: !muted }),
      });
      await reload(clientAccount);
    } catch (err) {
      setError(err);
    }
  }

  return (
    <PublicFrame locale={locale} path={locale === "en" ? "/en/notifications" : "/notifications"}>
      <section className="grid gap-4">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <p className="text-sm text-ink/80">{text.lead}</p>
        {guest ? (
          <p className="text-sm">
            {text.login}{" "}
            <a className="underline" href={locale === "en" ? "/en/account" : "/account"}>
              {text.account}
            </a>
          </p>
        ) : null}
        {error ? <Failure error={error} /> : null}
        {loading ? <p className="text-sm">{locale === "en" ? "Loading…" : "جارٍ التحميل…"}</p> : null}
        {guest ? null : (
          <>
            {clientAccount ? (
              <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={save}>
                <Label>
                  {text.name}
                  <Input value={name} onChange={(event) => setName(event.target.value)} required />
                </Label>
                <Label>
                  {text.neighborhood}
                  <select className="rounded-md border border-line bg-paper px-2 py-2" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)} required>
                    {places.map((place) => (
                      <option key={place.id} value={place.id}>
                        {place.name}
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
                <Button type="submit">{text.save}</Button>
              </form>
            ) : (
              <div className="grid gap-2 text-sm">
                <a className="underline" href="/admin">
                  {text.back}
                </a>
                <p className="text-muted">{text.staffNote}</p>
              </div>
            )}
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="font-semibold">{clientAccount ? text.searches : text.inbox}</h2>
              <Button type="button" variant="ghost" onClick={() => void muteAll()}>
                {muted ? text.unmute : text.mute}
              </Button>
            </div>
            {clientAccount ? (
              <>
                {searches.length === 0 ? <p className="text-sm">{text.emptySearches}</p> : null}
                <ul className="grid gap-2">
                  {searches.map((item) => (
                    <li key={item.id} className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-line bg-card p-3 text-sm">
                      <span>
                        {item.name}
                        {item.muted ? (locale === "en" ? " · muted" : " · مكتوم") : ""}
                      </span>
                      {item.muted ? null : (
                        <button className="underline" onClick={() => void muteOne(item.id)}>
                          {text.muteSearch}
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
                <h2 className="font-semibold">{text.inbox}</h2>
              </>
            ) : null}
            {items.length === 0 ? <p className="text-sm">{text.empty}</p> : null}
            <ul className="grid gap-2">
              {items.map((item) => (
                <li key={item.id} className="grid gap-1 rounded-md border border-line bg-card p-3">
                  <p className="text-sm text-ink/70">{text.kinds[item.kind as keyof typeof text.kinds] ?? item.kind}</p>
                  <p className="font-semibold">{locale === "en" ? item.title_en : item.title_ar}</p>
                  <p className="text-sm">{locale === "en" ? item.body_en : item.body_ar}</p>
                  <p className="text-sm text-ink/70">
                    {text.notSent} · {item.delivery_status}
                    {item.provider_adopted ? "" : locale === "en" ? " · provider not adopted" : " · المزود غير معتمد"}
                  </p>
                </li>
              ))}
            </ul>
          </>
        )}
      </section>
    </PublicFrame>
  );
}
