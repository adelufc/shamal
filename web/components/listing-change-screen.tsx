"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money } from "@/lib/chrome";
import type { Locale, SessionUser } from "@/lib/types";

type Listing = {
  id: string;
  status: string;
  version: number;
  transaction_type: string;
  price_minor: number;
  neighborhood_id: string;
  license_ref: string;
  owner_user_id: string;
  title_ar: string;
  title_en: string;
  pending_changes: { price_minor?: number; neighborhood_id?: string; license_ref?: string; owner_user_id?: string } | null;
};
type Place = { id: string; status: string; texts: { locale: string; name: string }[] };
type Staff = { id: string; display_name: string; email: string };

const copy = {
  ar: {
    title: "تعديل إعلان منشور",
    lead: "السعر والحي والترخيص والمالك بعد النشر يُحفظان معلّقين. السعر الظاهر للجمهور لا يتبدل من هذه الشاشة.",
    listing: "الإعلان",
    empty: "لا يوجد إعلان منشور.",
    price: "السعر الجديد بالريال",
    neighborhood: "الحي",
    license: "الترخيص",
    owner: "المالك",
    save: "حفظ التغيير المعلّق",
    current: "المنشور الآن",
    pending: "طلب التغيير المعلّق",
    none: "لا تغيير معلّق.",
    waiting: "هذا التغيير بانتظار المراجعة. لا يزال السعر المنشور كما هو",
    replace: "الحفظ يستبدل طلب التغيير المعلّق ولا يغيّر السعر المنشور.",
    publishedPrice: "السعر المنشور",
    pendingPrice: "السعر في الطلب المعلّق",
    onlyUpdate: "تسجيل التغيير لمن لديه صلاحية تعديل العقار.",
    login: "هذه الشاشة لموظف يستطيع عرض العقار.",
    desk: "دخول المكتب",
    loading: "جارٍ التحميل…",
    english: "English",
  },
  en: {
    title: "Edit a published listing",
    lead: "Price, neighborhood, license, and owner after publication are stored as a pending change. The price the public sees does not change from this screen.",
    listing: "Listing",
    empty: "There is no published listing.",
    price: "New price in SAR",
    neighborhood: "Neighborhood",
    license: "License",
    owner: "Owner",
    save: "Save the pending change",
    current: "Published now",
    pending: "Pending change request",
    none: "There is no pending change.",
    waiting: "This change is waiting for review. The published price is unchanged.",
    replace: "Saving replaces the pending request and does not change the published price.",
    publishedPrice: "Published price",
    pendingPrice: "Price in the pending request",
    onlyUpdate: "Recording a change needs catalog update permission.",
    login: "This screen is for a staff member who can view the catalog.",
    desk: "Work desk sign-in",
    loading: "Loading…",
    english: "العربية",
  },
} as const;

export function ListingChangeScreen({ locale, framed = false }: { locale: Locale; framed?: boolean }) {
  const text = copy[locale];
  const [me, setMe] = useState<SessionUser | null>(null);
  const [items, setItems] = useState<Listing[]>([]);
  const [places, setPlaces] = useState<Place[]>([]);
  const [staff, setStaff] = useState<Staff[]>([]);
  const [id, setId] = useState("");
  const [price, setPrice] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [license, setLicense] = useState("");
  const [owner, setOwner] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);

  const current = items.find((item) => item.id === id) ?? null;
  const canUpdate = (me?.actions.catalog ?? []).includes("update");

  async function load() {
    const [session, listings, geo, structure] = await Promise.all([
      api<SessionUser>("/api/v1/me"),
      api<{ items: Listing[] }>("/api/v1/catalog/listings?status=published"),
      api<{ items: Place[] }>("/api/v1/geo/neighborhoods?page_size=100"),
      api<{ staff: Staff[] }>("/api/v1/org/structure"),
    ]);
    setMe(session);
    setItems(listings.items);
    setPlaces(geo.items);
    setStaff(structure.staff);
    setId((value) => value || listings.items[0]?.id || "");
  }

  useEffect(() => {
    load()
      .catch((caught: unknown) => setError(caught))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!current) return;
    const pending = current.pending_changes;
    setPrice(String((pending?.price_minor ?? current.price_minor) / 100));
    setNeighborhood(pending?.neighborhood_id || current.neighborhood_id);
    setLicense(pending?.license_ref || current.license_ref);
    setOwner(pending?.owner_user_id || current.owner_user_id);
  }, [current?.id, current?.version]);

  async function save(event: FormEvent) {
    event.preventDefault();
    if (!current) return;
    setError(null);
    const riyals = Number(price);
    if (!Number.isFinite(riyals) || riyals <= 0) {
      setError(new Error(locale === "en" ? "Enter a price greater than zero." : "أدخل سعراً أكبر من صفر."));
      return;
    }
    try {
      const updated = await api<Listing & { pending_changes?: Listing["pending_changes"]; price_minor: number; version: number }>(
        `/api/v1/catalog/listings/${current.id}/pending-change`,
        {
          method: "POST",
          body: JSON.stringify({
            version: current.version,
            price_minor: Math.round(riyals * 100),
            neighborhood_id: neighborhood,
            license_ref: license,
            owner_user_id: owner,
          }),
        },
      );
      setItems((list) => list.map((item) => (item.id === current.id ? { ...item, version: updated.version, pending_changes: updated.pending_changes ?? item.pending_changes } : item)));
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  const placeName = (place: Place) => place.texts.find((item) => item.locale === locale)?.name ?? place.texts[0]?.name ?? place.id;
  const title = (item: Listing) => (locale === "en" ? item.title_en : item.title_ar) || item.id;

  const body = (
    <section className="grid gap-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        {framed ? null : (
          <a className="text-sm underline" href="/en/admin/listings">
            {text.english}
          </a>
        )}
      </div>
      <p className="text-sm text-ink/80">{text.lead}</p>
      {loading ? <p className="text-sm">{text.loading}</p> : null}
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
      {!loading && items.length === 0 ? <p className="text-sm">{text.empty}</p> : null}
      {items.length > 0 ? (
        <Label>
          {text.listing}
          <select className="rounded-md border border-line bg-paper px-2 py-2" value={id} onChange={(event) => setId(event.target.value)}>
            {items.map((item) => (
              <option key={item.id} value={item.id}>
                {title(item)}
              </option>
            ))}
          </select>
        </Label>
      ) : null}
      {current ? (
        <article className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          <h2 className="font-semibold">{text.current}</h2>
          <p>
            {text.publishedPrice}: {money(current.price_minor, locale)}
          </p>
          <h2 className="font-semibold">{text.pending}</h2>
          {current.pending_changes?.price_minor ? (
            <>
              <p>
                {text.pendingPrice}: {money(current.pending_changes.price_minor, locale)}
              </p>
              <p>{text.waiting}</p>
              <p>{text.replace}</p>
            </>
          ) : (
            <p>{text.none}</p>
          )}
        </article>
      ) : null}
      {canUpdate && current ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={save}>
          <Label>
            {text.price}
            <Input value={price} onChange={(event) => setPrice(event.target.value)} inputMode="decimal" required />
          </Label>
          <Label>
            {text.neighborhood}
            <select className="rounded-md border border-line bg-paper px-2 py-2" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)} required>
              {places
                .filter((place) => place.status === "published" || place.id === neighborhood)
                .map((place) => (
                  <option key={place.id} value={place.id}>
                    {placeName(place)}
                  </option>
                ))}
            </select>
          </Label>
          <Label>
            {text.license}
            <Input value={license} onChange={(event) => setLicense(event.target.value)} required />
          </Label>
          <Label>
            {text.owner}
            <select className="rounded-md border border-line bg-paper px-2 py-2" value={owner} onChange={(event) => setOwner(event.target.value)} required>
              {staff.map((person) => (
                <option key={person.id} value={person.id}>
                  {person.display_name}
                </option>
              ))}
            </select>
          </Label>
          <Button type="submit">{text.save}</Button>
        </form>
      ) : (
        <p className="text-sm text-ink/70">{text.onlyUpdate}</p>
      )}
    </section>
  );

  if (!framed) return body;
  return <PublicFrame locale={locale} path="/en/admin/listings">{body}</PublicFrame>;
}
