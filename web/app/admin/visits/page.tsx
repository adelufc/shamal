"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { placeLabels } from "@/lib/place-label";
import { propertyTypeName } from "@/lib/property-types";

type Item = {
  id: string;
  display_name: string;
  status: string;
  starts_at: string;
  ends_at: string;
  instructions_ar: string;
  instructions_en: string;
  broker_user_id: string;
  host_user_id?: string;
  listing_id?: string;
  property_id?: string;
  unit_id?: string;
  result: string;
  result_note: string;
  overdue: boolean;
  version: number;
  cancel_reason?: string;
};

const statusLabel: Record<string, string> = {
  new: "جديد",
  awaiting_broker: "بانتظار الوسيط",
  awaiting_owner: "بانتظار المالك",
  confirmed: "مؤكد",
  completed: "مكتمل",
  cancelled: "ملغى",
  no_show: "لم يحضر",
};

const views = [
  { id: "", label: "الكل" },
  { id: "today", label: "اليوم" },
  { id: "overdue", label: "المتأخر" },
  { id: "unassigned", label: "غير المسند" },
  { id: "results", label: "النتائج" },
];

export default function VisitsPage() {
  const me = useSession();
  const [items, setItems] = useState<Item[]>([]);
  const initialView = useSearchParams().get("view") ?? "";
  const [view, setView] = useState(initialView);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [slots, setSlots] = useState<Record<string, string>>({});
  const [listings, setListings] = useState<Record<string, { title: string; neighborhoodId: string; typeName: string }>>({});
  const [people, setPeople] = useState<Record<string, string>>({});
  const [places, setPlaces] = useState<Record<string, string>>({});
  const can = (action: string) => (me?.actions.visits ?? []).includes(action);
  const canCatalog = (me?.actions.catalog ?? []).includes("view");

  async function load(next = view) {
    const body = await api<{ items: Item[] }>(`/api/v1/visits?page_size=100${next ? `&view=${next}` : ""}`);
    setItems(body.items);
    setState("ready");
  }

  useEffect(() => {
    if (!me) return;
    let stop = false;
    load(view).catch((caught) => {
      if (stop) return;
      setError(caught);
      setState("error");
    });
    return () => {
      stop = true;
    };
  }, [me, view]);

  useEffect(() => {
    if (!me || items.length === 0) return;
    let stop = false;
    api<{ staff?: { id: string; display_name: string }[] }>("/api/v1/org/structure")
      .then((body) => {
        if (stop) return;
        const next: Record<string, string> = {};
        for (const person of body.staff ?? []) next[person.id] = person.display_name;
        setPeople(next);
      })
      .catch(() => setPeople({}));
    api<{ items: { id: string; status?: string; texts?: { locale: string; name?: string; slug?: string; description?: string }[] }[] }>("/api/v1/geo/neighborhoods?page_size=100")
      .then((body) => {
        if (stop) return;
        const next: Record<string, string> = {};
        for (const place of placeLabels(body.items)) next[place.id] = place.label;
        setPlaces(next);
      })
      .catch(() => setPlaces({}));
    if (!canCatalog) return () => { stop = true; };
    const ids = [...new Set(items.map((item) => item.listing_id).filter((id): id is string => Boolean(id)))].slice(0, 40);
    Promise.all(
      ids.map(async (id) => {
        try {
          const listing = await api<{ texts?: { locale: string; title: string }[]; neighborhood_id?: string; type_code?: string }>(`/api/v1/catalog/listings/${id}`);
          return [id, {
            title: listing.texts?.find((text) => text.locale === "ar")?.title || listing.texts?.[0]?.title || "إعلان بلا عنوان",
            neighborhoodId: listing.neighborhood_id ?? "",
            typeName: propertyTypeName("ar", listing.type_code),
          }] as const;
        } catch {
          return [id, { title: "تعذر تحميل الإعلان", neighborhoodId: "", typeName: "" }] as const;
        }
      }),
    ).then((rows) => {
      if (stop) return;
      setListings(Object.fromEntries(rows));
    });
    return () => { stop = true; };
  }, [me, items, canCatalog]);

  async function command(item: Item, action: string, extra: Record<string, unknown> = {}) {
    setError(null);
    try {
      await api(`/api/v1/visits/${item.id}/${action}`, {
        method: "POST",
        body: JSON.stringify({ version: item.version, reason: reasons[item.id] ?? "", ...extra }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض الزيارات.</p>;
  if (state === "loading") return <p className="text-sm">جارٍ التحميل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">الزيارات</h1>
      <details className="text-sm text-ink/80">
        <summary>مساعدة</summary>
        <p>الوسيط يؤكد ثم المالك. التعارض يُفحص عند التأكيد. الإكمال قبل نهاية الموعد يحتاج تصحيحاً من المدير وسبباً. المدة المقترحة للاختبار 30 دقيقة والفاصل 15 دقيقة.</p>
      </details>
      <div className="flex flex-wrap gap-2">
        {views.map((item) => (
          <Button key={item.id || "all"} type="button" variant={view === item.id ? "primary" : "ghost"} onClick={() => setView(item.id)}>
            {item.label}
          </Button>
        ))}
      </div>
      {error ? <Failure error={error} /> : null}
      {items.length === 0 ? <p className="text-sm">لا توجد زيارات في هذا العرض.</p> : null}
      {items.map((item) => (
        <article key={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold">{item.display_name}</h2>
            <span>
              {statusLabel[item.status] ?? item.status}
              {item.overdue ? " · متأخر" : ""}
              {!item.broker_user_id ? " · غير مسند" : ""}
            </span>
          </div>
          <p>رقم الزيارة {item.id}</p>
          <p>العميل: {item.display_name}</p>
          <p>
            العقار: {item.listing_id ? listings[item.listing_id]?.title || (canCatalog ? "جارٍ تحميل الإعلان" : "عرض الإعلان غير متاح لهذه الصلاحية") : "لا إعلان مرتبط"}
            {item.listing_id && listings[item.listing_id]?.typeName ? ` · ${listings[item.listing_id].typeName}` : ""}
            {item.listing_id && places[listings[item.listing_id]?.neighborhoodId ?? ""] ? ` · ${places[listings[item.listing_id].neighborhoodId]}` : ""}
            {item.unit_id ? " · وحدة مرتبطة بالإعلان" : ""}
          </p>
          {item.listing_id && canCatalog ? (
            <p>
              <a className="underline" href={`/listings/${item.listing_id}`}>الإعلان</a>
              {listings[item.listing_id]?.neighborhoodId ? (
                <>
                  {" · "}
                  <a className="underline" href={`/admin/neighborhoods#${listings[item.listing_id].neighborhoodId}`}>سجل الحي</a>
                </>
              ) : null}
            </p>
          ) : null}
          <p>الوسيط: {item.broker_user_id ? people[item.broker_user_id] || "مسند، واسمه غير ظاهر في قائمة الموظفين" : "غير مسند"}</p>
          <p>متابعة المالك: {item.host_user_id ? people[item.host_user_id] || "مسند، واسمه غير ظاهر في قائمة الموظفين" : "غير محدد"}</p>
          <p>
            البداية {new Date(item.starts_at).toLocaleString("ar-SA", { timeZone: "Asia/Riyadh" })} · النهاية{" "}
            {new Date(item.ends_at).toLocaleString("ar-SA", { timeZone: "Asia/Riyadh" })} · توقيت الرياض
          </p>
          {item.instructions_ar ? <p>رسالة للعميل: {item.instructions_ar}</p> : null}
          {item.result ? (
            <p>
              النتيجة:{" "}
              {item.result === "interested" ? "مهتم" : item.result === "follow_up" ? "متابعة" : item.result === "negotiation" ? "تفاوض" : item.result}
            </p>
          ) : null}
          {item.result_note ? <p>ملاحظة داخلية يراها فريق العمل فقط: {item.result_note}</p> : null}
          {item.cancel_reason ? <p>{item.cancel_reason}</p> : null}
          {item.status !== "completed" && item.status !== "cancelled" && item.status !== "no_show" ? (
            <div className="grid gap-2">
              <label className="grid gap-2">
                سبب الإلغاء أو الرفض أو إعادة الجدولة
                <Input value={reasons[item.id] ?? ""} onChange={(event) => setReasons({ ...reasons, [item.id]: event.target.value })} />
              </label>
              <label className="grid gap-2">
                الموعد الجديد بتوقيت الرياض
                <Input type="datetime-local" value={slots[item.id] ?? ""} onChange={(event) => setSlots({ ...slots, [item.id]: event.target.value })} />
              </label>
              <div className="flex flex-wrap gap-2">
                {can("update") && item.status === "awaiting_broker" ? (
                  <Button type="button" onClick={() => void command(item, "broker-confirm")}>
                    تأكيد الوسيط
                  </Button>
                ) : null}
                {can("approve") && item.status === "awaiting_owner" ? (
                  <Button type="button" onClick={() => void command(item, "owner-confirm")}>
                    تأكيد المالك
                  </Button>
                ) : null}
                {can("approve") && item.status === "awaiting_owner" ? (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() =>
                      void command(item, "owner-reject", {
                        starts_at: slots[item.id] ? new Date(slots[item.id]).toISOString() : undefined,
                      })
                    }
                  >
                    رفض المالك
                  </Button>
                ) : null}
                {can("update") && item.status === "confirmed" ? (
                  <Button type="button" onClick={() => void command(item, "complete", { result: "interested" })}>
                    مكتمل: مهتم
                  </Button>
                ) : null}
                {can("archive") && item.status === "confirmed" ? (
                  <Button type="button" variant="ghost" onClick={() => void command(item, "complete", { result: "follow_up", correction: true })}>
                    تصحيح المدير
                  </Button>
                ) : null}
                {can("update") ? (
                  <Button type="button" variant="ghost" onClick={() => void command(item, "cancel")}>
                    إلغاء
                  </Button>
                ) : null}
                {can("update") && item.status === "confirmed" ? (
                  <Button type="button" variant="ghost" onClick={() => void command(item, "no-show")}>
                    لم يحضر
                  </Button>
                ) : null}
                {can("update") && (item.status === "confirmed" || item.status === "awaiting_owner" || item.status === "awaiting_broker") ? (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() =>
                      void command(item, "reschedule", {
                        starts_at: slots[item.id] ? new Date(slots[item.id]).toISOString() : "",
                      })
                    }
                  >
                    إعادة جدولة
                  </Button>
                ) : null}
              </div>
            </div>
          ) : null}
        </article>
      ))}
    </section>
  );
}
