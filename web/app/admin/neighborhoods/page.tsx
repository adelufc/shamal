"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label, TextArea } from "@/components/ui/input";

type Neighborhood = {
  id: string;
  status: string;
  version: number;
  issues: string[];
  texts: { locale: string; name: string; slug: string }[];
  service_sale: boolean;
  sync_status?: string;
};

const statuses: Record<string, string> = {
  draft: "مسودة",
  in_review: "قيد المراجعة",
  changes_requested: "طُلب تعديل",
  published: "منشور",
  suspended: "موقوف",
};

function syncLabel(status: string) {
  if (status === "recorded") return "مسجّلة، ولم تكتمل بعد";
  if (status === "succeeded") return "اكتملت";
  if (status === "running") return "قيد التنفيذ";
  return status;
}

const issueText: Record<string, string> = {
  name_required: "الاسم العربي والإنجليزي مطلوبان",
  invalid_boundary: "الحد غير صالح",
  outside_city: "الحد خارج نطاق الرياض الاختباري",
  boundary_overlap: "الحد يتداخل مع حي منشور",
  duplicate_neighborhood: "الاسم يطابق حياً موجوداً",
  official_boundary_unapproved: "لا يُقبل حد رسمي قبل اعتماد المصدر",
  city_boundary_unapproved: "النشر موقوف حتى اعتماد حد الرياض",
};

export default function NeighborhoodsPage() {
  const me = useSession();
  const [items, setItems] = useState<Neighborhood[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [nameAR, setNameAR] = useState("");
  const [nameEN, setNameEN] = useState("");
  const [geojson, setGeojson] = useState("");
  const [sale, setSale] = useState(false);
  const [reason, setReason] = useState("");
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [pending, setPending] = useState(false);

  const can = (action: string) => (me?.actions.geography ?? []).includes(action);

  async function load() {
    setState("loading");
    try {
      const payload = await api<{ items: Neighborhood[] }>("/api/v1/geo/neighborhoods?page_size=100");
      setItems(payload.items);
      setState("ready");
    } catch (caught) {
      setError(caught);
      setState("error");
    }
  }

  useEffect(() => {
    if (me) void load();
  }, [me]);

  async function create(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await api("/api/v1/geo/neighborhoods", {
        method: "POST",
        body: JSON.stringify({
          name_ar: nameAR,
          name_en: nameEN,
          slug_ar: nameAR,
          slug_en: nameEN.trim().toLowerCase().replace(/\s+/g, "-"),
          description_ar: nameAR,
          description_en: nameEN,
          geojson,
          service_sale: sale,
          service_annual: false,
          service_monthly: false,
          service_daily: false,
          coverage: "served",
          show_in_directory: true,
          show_on_map: true,
          boundary_note_ar: "حد أدخله المحرر.",
          boundary_note_en: "Boundary entered by the editor.",
          boundary_is_official: false,
        }),
      });
      setNameAR("");
      setNameEN("");
      setGeojson("");
      setSale(false);
      await load();
    } catch (caught) {
      setError(caught);
    } finally {
      setPending(false);
    }
  }

  async function command(id: string, version: number, action: string) {
    setError(null);
    try {
      await api(`/api/v1/geo/neighborhoods/${id}/${action}`, {
        method: "POST",
        body: JSON.stringify({ version, reason }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">أحياء الرياض</h1>
      <p className="text-sm text-ink/75">إضافة حي لا تحتاج تغييراً في الكود. النشر يسجّل مهمة مزامنة، ولا يعلن نجاح المزامنة قبل اكتمالها.</p>
      {can("create") ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={create}>
          <div className="grid gap-3 sm:grid-cols-2">
            <Label>
              الاسم العربي
              <Input value={nameAR} onChange={(event) => setNameAR(event.target.value)} required />
            </Label>
            <Label>
              الاسم الإنجليزي
              <Input value={nameEN} onChange={(event) => setNameEN(event.target.value)} required />
            </Label>
          </div>
          <Label>
            الحد بصيغة GeoJSON
            <TextArea value={geojson} onChange={(event) => setGeojson(event.target.value)} placeholder='{"type":"Polygon","coordinates":[[[...]]]}' />
          </Label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={sale} onChange={(event) => setSale(event.target.checked)} />
            تفعيل البيع
          </label>
          <Button disabled={pending}>{pending ? "جارٍ الحفظ…" : "حفظ مسودة"}</Button>
        </form>
      ) : null}
      {can("approve") ? (
        <Label>
          سبب طلب التعديل أو الإيقاف
          <Input value={reason} onChange={(event) => setReason(event.target.value)} />
        </Label>
      ) : null}
      {error ? <Failure error={error} /> : null}
      {state === "loading" ? <p className="text-sm">جارٍ تحميل الأحياء…</p> : null}
      {state === "error" && !(error instanceof ApiError) ? <p className="text-sm text-warn">تعذر تحميل الأحياء.</p> : null}
      {state === "ready" && items.length === 0 ? <p className="rounded-lg border border-dashed border-line p-4 text-sm">لا توجد أحياء.</p> : null}
      <div className="flex flex-wrap gap-2 [&_input]:border-action [&_select]:border-action">
        <label className="grid gap-2 text-sm">
          بحث بالاسم
          <Input value={query} onChange={(event) => setQuery(event.target.value)} />
        </label>
        <label className="grid gap-2 text-sm">
          الحالة
          <select className="min-h-11 rounded-lg border border-action bg-card px-3" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="">الكل</option>
            {Object.entries(statuses).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <ul className="grid gap-3">
        {items
          .filter((item) => !statusFilter || item.status === statusFilter)
          .filter((item) => {
            const name = `${item.texts?.find((text) => text.locale === "ar")?.name ?? ""} ${item.texts?.find((text) => text.locale === "en")?.name ?? ""}`;
            return name.includes(query.trim());
          })
          .map((item) => {
          const ar = item.texts?.find((text) => text.locale === "ar")?.name;
          const en = item.texts?.find((text) => text.locale === "en")?.name;
          return (
            <li id={item.id} key={item.id} className="rounded-lg border border-line bg-card p-4">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <h2 className="font-semibold">{ar}</h2>
                <span className="text-sm">{statuses[item.status] ?? item.status}</span>
              </div>
              <p className="text-sm text-ink/70">{en}</p>
              {item.issues?.length ? <p className="mt-2 text-sm text-warn">ملاحظات الحفظ: {item.issues.map((code) => issueText[code] ?? code).join("، ")}</p> : null}
              {item.sync_status ? <p className="mt-1 text-sm">المزامنة: {syncLabel(item.sync_status)}</p> : null}
              <div className="mt-3 flex flex-wrap gap-2">
                {can("update") && (item.status === "draft" || item.status === "changes_requested") ? (
                  <Button variant="ghost" onClick={() => command(item.id, item.version, "submit")}>
                    إرسال للمراجعة
                  </Button>
                ) : null}
                {can("approve") && item.status === "in_review" ? (
                  <Button variant="ghost" onClick={() => command(item.id, item.version, "request-changes")}>
                    طلب تعديل
                  </Button>
                ) : null}
                {can("publish") && item.status === "in_review" ? (
                  <Button onClick={() => command(item.id, item.version, "publish")}>نشر</Button>
                ) : null}
                {can("archive") && item.status === "published" ? (
                  <Button variant="ghost" onClick={() => command(item.id, item.version, "suspend")}>
                    إيقاف
                  </Button>
                ) : null}
                {can("publish") && item.status === "suspended" ? (
                  <Button onClick={() => command(item.id, item.version, "reactivate")}>إعادة نشر</Button>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
