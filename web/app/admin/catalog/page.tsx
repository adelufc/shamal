"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api, useSession } from "@/components/admin-shell";
import { placeLabels } from "@/lib/place-label";
import { propertyTypeName } from "@/lib/property-types";
import { Button } from "@/components/ui/button";
import { Input, Label, TextArea } from "@/components/ui/input";

type Neighborhood = { id: string; status: string; texts: { locale: string; name: string }[] };
type PropertyType = { code: string; name_ar: string; requires_rooms: boolean };
type Listing = { id: string; status: string; version: number; issues: string[] };
type Staff = { id: string; display_name: string; email: string };
type PropertyRow = {
  id: string;
  neighborhood_id: string;
  branch_id?: string;
  owner_user_id?: string;
  delegation_ref?: string;
  address_ar: string;
  address_en: string;
  type_code: string;
  area_sqm: number;
  precise_lon?: number | null;
  precise_lat?: number | null;
  version?: number;
  created_at?: string;
};
type RelatedListing = {
  id: string;
  property_id: string;
  unit_id: string;
  status: string;
  broker_user_id: string;
  texts?: { locale: string; title?: string }[];
};
const placeStatus: Record<string, string> = { draft: "مسودة", published: "منشور", in_review: "قيد المراجعة", suspended: "موقوف" };
const listingStatus: Record<string, string> = { draft: "مسودة", in_review: "قيد المراجعة", approved: "معتمد", published: "منشور", archived: "مؤرشف" };

function recordedPerson(id: string | undefined, access: "unknown" | "ready" | "hidden", people: Staff[]) {
  if (!id) return "غير مسجل";
  if (access === "unknown") return "جارٍ التحقق من الأسماء…";
  if (access !== "ready") return "غير متاح لهذا الحساب";
  return people.find((person) => person.id === id)?.display_name || "غير متاح لهذا الحساب";
}

async function loadPropertyLinks() {
  const [published, drafts] = await Promise.all([
    api<{ items: { id: string }[] }>("/api/v1/catalog/listings?status=published"),
    api<{ items: { id: string }[] }>("/api/v1/catalog/listings?status=draft"),
  ]);
  const ids = [...published.items, ...drafts.items].map((item) => item.id);
  const details: RelatedListing[] = [];
  let cursor = 0;
  async function next() {
    while (cursor < ids.length) {
      const id = ids[cursor];
      cursor += 1;
      details.push(await api<RelatedListing>(`/api/v1/catalog/listings/${id}`));
    }
  }
  await Promise.all([next(), next(), next(), next(), next(), next()]);
  return details;
}

export default function CatalogPage() {
  const me = useSession();
  const [neighborhoods, setNeighborhoods] = useState<Neighborhood[]>([]);
  const [types, setTypes] = useState<PropertyType[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [listing, setListing] = useState<Listing | null>(null);
  const [pending, setPending] = useState(false);

  const [neighborhood, setNeighborhood] = useState("");
  const [typeCode, setTypeCode] = useState("");
  const [area, setArea] = useState("");
  const [rooms, setRooms] = useState("");
  const [addressAR, setAddressAR] = useState("");
  const [addressEN, setAddressEN] = useState("");
  const [lon, setLon] = useState("");
  const [lat, setLat] = useState("");
  const [unit, setUnit] = useState("");
  const [transaction, setTransaction] = useState("sale");
  const [price, setPrice] = useState("");
  const [license, setLicense] = useState("");
  const [titleAR, setTitleAR] = useState("");
  const [titleEN, setTitleEN] = useState("");
  const [descriptionAR, setDescriptionAR] = useState("");
  const [descriptionEN, setDescriptionEN] = useState("");
  const [owner, setOwner] = useState("");
  const [broker, setBroker] = useState("");
  const [staff, setStaff] = useState<Staff[]>([]);
  const [staffAccess, setStaffAccess] = useState<"unknown" | "ready" | "hidden">("unknown");
  const [related, setRelated] = useState<RelatedListing[]>([]);
  const [relatedState, setRelatedState] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [relatedError, setRelatedError] = useState<unknown>(null);
  const [step, setStep] = useState(1);
  const [properties, setProperties] = useState<PropertyRow[]>([]);
  const [propertyTotal, setPropertyTotal] = useState(0);
  const [propertyPage, setPropertyPage] = useState(1);
  const [propertyState, setPropertyState] = useState<"loading" | "ready" | "error" | "forbidden">("loading");
  const [propertyError, setPropertyError] = useState<unknown>(null);
  const [openProperty, setOpenProperty] = useState("");

  const can = (action: string) => (me?.actions.catalog ?? []).includes(action);

  useEffect(() => {
    if (!me) return;
    if (!(me.actions.catalog ?? []).includes("view")) {
      setPropertyState("forbidden");
      return;
    }
    let stop = false;
    setProperties([]);
    setPropertyState("loading");
    api<{ items: PropertyRow[]; total: number }>(`/api/v1/catalog/properties?page_size=25&page=${propertyPage}`)
      .then((body) => {
        if (stop) return;
        setProperties(body.items);
        setPropertyTotal(body.total);
        setPropertyState("ready");
      })
      .catch((caught) => {
        if (stop) return;
        setProperties([]);
        setPropertyError(caught);
        setPropertyState(caught instanceof ApiError && caught.status === 403 ? "forbidden" : "error");
      });
    return () => {
      stop = true;
    };
  }, [me, propertyPage]);

  useEffect(() => {
    if (!me) return;
    let stop = false;
    Promise.allSettled([
      api<{ items: Neighborhood[] }>("/api/v1/geo/neighborhoods?page_size=100"),
      api<{ items: PropertyType[] }>("/api/v1/catalog/property-types"),
      api<{ staff: Staff[] }>("/api/v1/org/structure"),
    ]).then((results) => {
      if (stop) return;
      const [places, kinds, structure] = results;
      if (places.status === "rejected" || kinds.status === "rejected") {
        setError(places.status === "rejected" ? places.reason : kinds.reason);
        setState("error");
        return;
      }
      setNeighborhoods(places.value.items);
      setTypes(kinds.value.items);
      if (structure.status === "fulfilled") {
        setStaff(structure.value.staff);
        setStaffAccess("ready");
      } else {
        setStaff([]);
        setStaffAccess("hidden");
      }
      setState("ready");
    });
    return () => {
      stop = true;
    };
  }, [me]);

  useEffect(() => {
    if (!me || !(me.actions.catalog ?? []).includes("view")) return;
    let stop = false;
    setRelatedState("loading");
    loadPropertyLinks()
      .then((items) => {
        if (stop) return;
        setRelated(items);
        setRelatedState("ready");
      })
      .catch((caught) => {
        if (stop) return;
        setRelated([]);
        setRelatedError(caught);
        setRelatedState("error");
      });
    return () => {
      stop = true;
    };
  }, [me]);

  async function create(event: FormEvent) {
    event.preventDefault();
    setError(null);
    const riyals = Number(price);
    const areaSqm = Number(area);
    if (!Number.isFinite(riyals) || riyals <= 0 || !Number.isFinite(areaSqm) || areaSqm <= 0) {
      setError(new Error("أدخل مساحة وسعر أكبر من صفر. السعر بالريال كما يدخله المنشئ."));
      return;
    }
    setPending(true);
    try {
      const property = await api<{ id: string }>("/api/v1/catalog/properties", {
        method: "POST",
        body: JSON.stringify({
          neighborhood_id: neighborhood,
          owner_user_id: owner,
          type_code: typeCode,
          area_sqm: areaSqm,
          address_ar: addressAR,
          address_en: addressEN,
          precise_lon: lon === "" ? null : Number(lon),
          precise_lat: lat === "" ? null : Number(lat),
        }),
      });
      const createdUnit = await api<{ id: string }>("/api/v1/catalog/units", {
        method: "POST",
        body: JSON.stringify({
          property_id: property.id,
          unit_number: unit,
          rooms: rooms === "" ? null : Number(rooms),
        }),
      });
      const created = await api<Listing>("/api/v1/catalog/listings", {
        method: "POST",
        body: JSON.stringify({
          property_id: property.id,
          unit_id: createdUnit.id,
          neighborhood_id: neighborhood,
          transaction_type: transaction,
          price_minor: Math.round(riyals * 100),
          currency: "SAR",
          license_ref: license,
          broker_user_id: broker,
          texts: [
            { locale: "ar", title: titleAR, description: descriptionAR },
            { locale: "en", title: titleEN, description: descriptionEN },
          ],
        }),
      });
      setListing(created);
    } catch (caught) {
      setError(caught);
    } finally {
      setPending(false);
    }
  }

  async function move(action: "submit" | "approve" | "publish") {
    if (!listing) return;
    setError(null);
    try {
      const updated = await api<Listing>(`/api/v1/catalog/listings/${listing.id}/${action}`, {
        method: "POST",
        body: JSON.stringify({ version: listing.version }),
      });
      setListing(updated);
    } catch (caught) {
      setError(caught);
    }
  }

  const selected = types.find((item) => item.code === typeCode);
  const createAllowed = can("create");
  const labeledPlaces = placeLabels(neighborhoods);
  const placeName = (id: string) => labeledPlaces.find((item) => item.id === id)?.label ?? "";

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">{createAllowed ? "عقار ووحدة وإعلان" : "العقارات"}</h1>
      {createAllowed ? (
        <>
          <p className="text-sm text-muted">العقار ثم الوحدة ثم الإعلان. لا يُحفظ سجل قبل الضغط الأخير. لا يُنشر إعلان في حي غير مؤهل.</p>
          <p className="text-sm">الخطوة {step} من 3</p>
        </>
      ) : (
        <p className="text-sm text-muted">عرض العقارات المتاحة لصلاحية العرض. إنشاء عقار جديد غير متاح لهذا الحساب.</p>
      )}
      {state === "loading" ? <p className="text-sm">جارٍ تحميل الأنواع والأحياء…</p> : null}
      {state === "error" ? <Failure error={error} /> : null}
      {state === "ready" && neighborhoods.length === 0 ? <p className="text-sm">لا توجد أحياء لربط العقار بها.</p> : null}
      {can("create") && state === "ready" ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={create}>
          <Label>
            الحي
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)} required>
              <option value="">اختر حياً</option>
              {neighborhoods.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.texts?.find((text) => text.locale === "ar")?.name} — {placeStatus[item.status] ?? item.status}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            النوع
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={typeCode} onChange={(event) => setTypeCode(event.target.value)} required>
              <option value="">اختر النوع</option>
              {types.map((item) => (
                <option key={item.code} value={item.code}>
                  {propertyTypeName("ar", item.code, item.name_ar)}
                </option>
              ))}
            </select>
          </Label>
          <div className="grid gap-3 sm:grid-cols-2">
            <Label>
              المساحة بالمتر
              <Input value={area} onChange={(event) => setArea(event.target.value)} inputMode="decimal" required />
            </Label>
            <Label>
              رقم الوحدة
              <Input value={unit} onChange={(event) => setUnit(event.target.value)} required />
            </Label>
          </div>
          {selected?.requires_rooms ? (
            <Label>
              عدد الغرف
              <Input value={rooms} onChange={(event) => setRooms(event.target.value)} inputMode="numeric" />
            </Label>
          ) : null}
          <div className="grid gap-3 sm:grid-cols-2">
            <Label>
              العنوان العربي
              <Input value={addressAR} onChange={(event) => setAddressAR(event.target.value)} />
            </Label>
            <Label>
              العنوان الإنجليزي
              <Input value={addressEN} onChange={(event) => setAddressEN(event.target.value)} />
            </Label>
          </div>
          <details className="rounded-lg bg-olive-soft p-3">
            <summary className="min-h-11 cursor-pointer">إحداثيات متقدمة، والموقع العام يبقى تقريبياً داخل الحي</summary>
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <Label>
                خط الطول
                <Input value={lon} onChange={(event) => setLon(event.target.value)} inputMode="decimal" />
              </Label>
              <Label>
                خط العرض
                <Input value={lat} onChange={(event) => setLat(event.target.value)} inputMode="decimal" />
              </Label>
            </div>
          </details>
          <Label>
            نوع التعامل
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={transaction} onChange={(event) => setTransaction(event.target.value)}>
              <option value="sale">بيع</option>
              <option value="annual">سنوي</option>
              <option value="monthly">شهري</option>
              <option value="daily">يومي</option>
            </select>
          </Label>
          <Label>
            السعر بالريال
            <Input value={price} onChange={(event) => setPrice(event.target.value)} inputMode="decimal" required />
          </Label>
          <Label>
            مرجع الترخيص أو الاستثناء
            <Input value={license} onChange={(event) => setLicense(event.target.value)} />
          </Label>
          <div className="grid gap-3 sm:grid-cols-2">
            <Label>
              المالك
              <select className="min-h-11 rounded-lg border border-line bg-card px-3" value={owner} onChange={(event) => setOwner(event.target.value)}>
                <option value="">بدون مالك محدد</option>
                {staff.map((person) => (
                  <option key={person.id} value={person.id}>
                    {person.display_name} · {person.email}
                  </option>
                ))}
              </select>
            </Label>
            <Label>
              الوسيط
              <select className="min-h-11 rounded-lg border border-line bg-card px-3" value={broker} onChange={(event) => setBroker(event.target.value)}>
                <option value="">بدون وسيط محدد</option>
                {staff.map((person) => (
                  <option key={person.id} value={person.id}>
                    {person.display_name} · {person.email}
                  </option>
                ))}
              </select>
            </Label>
          </div>
          {me ? (
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="ghost" onClick={() => setOwner(me.id)}>
                أنا المالك
              </Button>
              <Button type="button" variant="ghost" onClick={() => setBroker(me.id)}>
                أنا الوسيط
              </Button>
            </div>
          ) : null}
          <Label>
            العنوان العربي للإعلان
            <Input value={titleAR} onChange={(event) => setTitleAR(event.target.value)} required />
          </Label>
          <Label>
            الوصف العربي
            <TextArea value={descriptionAR} onChange={(event) => setDescriptionAR(event.target.value)} required />
          </Label>
          <Label>
            Listing title
            <Input value={titleEN} onChange={(event) => setTitleEN(event.target.value)} required />
          </Label>
          <Label>
            English description
            <TextArea value={descriptionEN} onChange={(event) => setDescriptionEN(event.target.value)} required />
          </Label>
          <div className="flex flex-wrap gap-2">
            {step > 1 ? (
              <Button type="button" variant="ghost" onClick={() => setStep(step - 1)}>
                السابق
              </Button>
            ) : null}
            {step < 3 ? (
              <Button type="button" onClick={() => setStep(step + 1)}>
                التالي
              </Button>
            ) : (
              <Button disabled={pending}>{pending ? "جارٍ حفظ المسودة" : "حفظ المسودة"}</Button>
            )}
          </div>
        </form>
      ) : null}
      {error && state !== "error" ? <Failure error={error} /> : null}
      {propertyState === "loading" ? <p className="text-sm">جارٍ تحميل العقارات…</p> : null}
      {propertyState === "forbidden" ? <p className="text-sm">لا توجد صلاحية لعرض العقارات.</p> : null}
      {propertyState === "error" ? <Failure error={propertyError} /> : null}
      {propertyState === "ready" && propertyTotal === 0 ? <p className="text-sm">لا توجد عقارات.</p> : null}
      {propertyState === "ready" && propertyTotal > 0 ? (
        <div className="grid gap-3">
          <h2 className="text-lg font-semibold">قائمة العقارات</h2>
          <p className="text-sm">{`عرض ${(propertyPage - 1) * 25 + 1}–${Math.min(propertyPage * 25, propertyTotal)} من ${propertyTotal}`}</p>
          {properties.map((item) => (
            <article key={item.id} data-property-id={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
              <h3 className="font-semibold">{item.address_ar || "بدون عنوان عربي"}</h3>
              <p>
                {propertyTypeName("ar", item.type_code)}
                {placeName(item.neighborhood_id) ? ` · ${placeName(item.neighborhood_id)}` : ""}
                {` · ${item.area_sqm} م²`}
              </p>
              <Button type="button" variant="ghost" onClick={() => setOpenProperty(openProperty === item.id ? "" : item.id)}>
                {openProperty === item.id ? "إخفاء التفاصيل" : "التفاصيل"}
              </Button>
              {openProperty === item.id ? (
                <dl className="grid gap-1">
                  <div>مرجع العقار: {item.id}</div>
                  <div>العنوان: {item.address_ar.trim() ? item.address_ar : "غير مسجل"}</div>
                  <div>النوع: {propertyTypeName("ar", item.type_code) || "غير مسجل"}</div>
                  <div>
                    الحي: {placeName(item.neighborhood_id) || "غير مسجل"}{" "}
                    <a className="underline" href={`/admin/neighborhoods#${item.neighborhood_id}`}>
                      سجل الحي
                    </a>
                  </div>
                  <div>المساحة: {item.area_sqm} م²</div>
                  <div>مرجع التفويض: {item.delegation_ref?.trim() ? item.delegation_ref : "غير مسجل"}</div>
                  <div>المالك: {recordedPerson(item.owner_user_id, staffAccess, staff)}</div>
                  {relatedState === "loading" || relatedState === "idle" ? <div>جارٍ تحميل الوحدات والإعلانات…</div> : null}
                  {relatedState === "error" ? <Failure error={relatedError} /> : null}
                  {relatedState === "ready" && related.filter((entry) => entry.property_id === item.id).length === 0 ? (
                    <>
                      <div>الوحدات: غير مسجل</div>
                      <div>الإعلانات: غير مسجل</div>
                    </>
                  ) : null}
                  {relatedState === "ready"
                    ? related
                        .filter((entry) => entry.property_id === item.id)
                        .map((entry) => {
                          const href = entry.status === "published" ? `/listings/${entry.id}` : `/admin/listings?listing=${entry.id}`;
                          const title = entry.texts?.find((text) => text.locale === "ar")?.title || entry.texts?.[0]?.title || entry.id;
                          return (
                            <div key={entry.id} className="grid gap-1">
                              <div>
                                الوحدة:{" "}
                                {entry.unit_id ? (
                                  <a className="underline" href={href}>
                                    {entry.unit_id}
                                  </a>
                                ) : (
                                  "غير مسجل"
                                )}
                              </div>
                              <div>
                                الإعلان:{" "}
                                <a className="underline" href={href}>
                                  {title}
                                </a>
                                {` · ${listingStatus[entry.status] ?? entry.status}`}
                              </div>
                              <div>الوسيط: {recordedPerson(entry.broker_user_id, staffAccess, staff)}</div>
                            </div>
                          );
                        })
                    : null}
                </dl>
              ) : null}
            </article>
          ))}
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="ghost" disabled={propertyPage <= 1} onClick={() => setPropertyPage(propertyPage - 1)}>
              السابق
            </Button>
            <Button type="button" variant="ghost" disabled={propertyPage * 25 >= propertyTotal} onClick={() => setPropertyPage(propertyPage + 1)}>
              التالي
            </Button>
          </div>
        </div>
      ) : null}
      {listing ? (
        <div className="rounded-lg border border-line bg-card p-4 text-sm">
          <p>حالة الإعلان: {listing.status}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            {can("update") && listing.status === "draft" ? <Button onClick={() => move("submit")}>إرسال للمراجعة</Button> : null}
            {can("approve") && listing.status === "in_review" ? <Button onClick={() => move("approve")}>اعتماد</Button> : null}
            {can("publish") && listing.status === "approved" ? <Button onClick={() => move("publish")}>نشر</Button> : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}
