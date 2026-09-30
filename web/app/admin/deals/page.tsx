"use client";

import { useEffect, useState } from "react";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money } from "@/lib/chrome";

type Item = {
  id: string;
  display_name: string;
  status: string;
  amount_minor?: number;
  currency: string;
  agreement_ref: string;
  version: number;
  history_count: number;
  stage_events?: { id: string; from_status: string; to_status: string; reason: string; created_at: string }[];
  is_test: boolean;
  listing_id: string;
  property_id: string;
  owner_user_id: string;
  broker_user_id: string;
};

const statusLabel: Record<string, string> = {
  draft: "مسودة",
  qualified: "مؤهلة",
  negotiation: "تفاوض",
  awaiting_documents: "بانتظار مستندات",
  preliminary_agreement: "اتفاق مبدئي",
  awaiting_external: "بانتظار تنفيذ خارجي",
  closed_success: "مغلقة ناجحة",
  closed_unsuccessful: "مغلقة غير ناجحة",
  cancelled: "ملغاة",
};

const forward: Record<string, string> = {
  draft: "qualified",
  qualified: "negotiation",
  negotiation: "awaiting_documents",
  awaiting_documents: "preliminary_agreement",
  preliminary_agreement: "awaiting_external",
};

const backward: Record<string, string> = {
  qualified: "draft",
  negotiation: "qualified",
  awaiting_documents: "negotiation",
  preliminary_agreement: "awaiting_documents",
  awaiting_external: "preliminary_agreement",
};

export default function DealsPage() {
  const me = useSession();
  const [items, setItems] = useState<Item[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [amounts, setAmounts] = useState<Record<string, string>>({});
  const [refs, setRefs] = useState<Record<string, string>>({});
  const [dates, setDates] = useState<Record<string, string>>({});
  const [parties, setParties] = useState<Record<string, { listing: string; property: string; owner: string; broker: string }>>({});
  const can = (action: string) => (me?.actions.deals ?? []).includes(action);

  async function load() {
    const body = await api<{ items: Item[] }>("/api/v1/deals?page_size=100");
    setItems(body.items);
    setState("ready");
  }

  useEffect(() => {
    if (!me) return;
    let stop = false;
    load().catch((caught) => {
      if (stop) return;
      setError(caught);
      setState("error");
    });
    return () => {
      stop = true;
    };
  }, [me]);

  function party(item: Item) {
    return parties[item.id] ?? { listing: item.listing_id, property: item.property_id, owner: item.owner_user_id, broker: item.broker_user_id };
  }

  function amountMinor(item: Item) {
    const raw = amounts[item.id]?.trim() ?? "";
    if (raw === "") return undefined;
    const riyals = Number(raw);
    if (!Number.isFinite(riyals) || riyals <= 0) return undefined;
    return Math.round(riyals * 100);
  }

  async function move(item: Item, to: string) {
    setError(null);
    const current = party(item);
    const minor = amountMinor(item);
    try {
      await api(`/api/v1/deals/${item.id}/move`, {
        method: "POST",
        body: JSON.stringify({
          version: item.version,
          to,
          reason: reasons[item.id] ?? "",
          listing_id: current.listing,
          property_id: current.property,
          owner_user_id: current.owner,
          broker_user_id: current.broker,
          currency: "SAR",
          ...(minor === undefined ? {} : { amount_minor: minor }),
        }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function requestClose(item: Item) {
    setError(null);
    const minor = amountMinor(item);
    if (!refs[item.id] || minor === undefined || !dates[item.id]) {
      setError(new Error("مرجع الاتفاق والمبلغ وتاريخ الإغلاق مطلوبة. المبلغ بالريال كما يدخله مقدم الطلب."));
      return;
    }
    try {
      await api(`/api/v1/deals/${item.id}/request-close`, {
        method: "POST",
        body: JSON.stringify({
          version: item.version,
          agreement_ref: refs[item.id],
          amount_minor: minor,
          currency: "SAR",
          close_date: new Date(dates[item.id]).toISOString(),
        }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function approve(item: Item) {
    setError(null);
    try {
      await api(`/api/v1/deals/${item.id}/approve-close`, {
        method: "POST",
        body: JSON.stringify({ version: item.version }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض الصفقات.</p>;
  if (state === "loading") return <p className="text-sm">جارٍ التحميل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">الصفقات</h1>
      <p className="text-sm text-ink/80">الإغلاق الناجح يحتاج مرجع اتفاق والأطراف والمبلغ وتاريخ الإغلاق، ثم اعتماد شخص غير من طلب الإغلاق. لا تُدخل عمولة أو ضريبة أو رسم إلغاء؛ قاعدتها غير معتمدة.</p>
      {error ? <Failure error={error} /> : null}
      {items.length === 0 ? <p className="text-sm">لا توجد صفقات.</p> : null}
      {items.map((item) => {
        const current = party(item);
        const terminal = item.status === "closed_success" || item.status === "closed_unsuccessful" || item.status === "cancelled";
        return (
          <article key={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="font-semibold">{item.display_name}</h2>
              <span>{statusLabel[item.status] ?? item.status}</span>
            </div>
            {item.amount_minor ? <p>{money(item.amount_minor, "ar")}</p> : null}
            {item.agreement_ref ? <p>مرجع الاتفاق: {item.agreement_ref}</p> : null}
            <p>سجل المراحل: {item.history_count}</p>
            {(item.stage_events ?? []).length > 0 ? (
              <ul className="grid gap-1">
                {item.stage_events?.map((event) => (
                  <li key={event.id}>
                    {(statusLabel[event.from_status] ?? event.from_status) || "بداية"} ← {statusLabel[event.to_status] ?? event.to_status}
                    {event.reason ? ` · ${event.reason}` : ""} · {new Date(event.created_at).toLocaleString("ar-SA", { timeZone: "Asia/Riyadh" })}
                  </li>
                ))}
              </ul>
            ) : null}
            {can("update") && !terminal ? (
              <div className="grid gap-2 md:grid-cols-2">
                <Label>
                  الإعلان
                  <Input value={current.listing} onChange={(event) => setParties({ ...parties, [item.id]: { ...current, listing: event.target.value } })} />
                </Label>
                <Label>
                  العقار
                  <Input value={current.property} onChange={(event) => setParties({ ...parties, [item.id]: { ...current, property: event.target.value } })} />
                </Label>
                <Label>
                  المالك
                  <Input value={current.owner} onChange={(event) => setParties({ ...parties, [item.id]: { ...current, owner: event.target.value } })} />
                </Label>
                <Label>
                  الوسيط
                  <Input value={current.broker} onChange={(event) => setParties({ ...parties, [item.id]: { ...current, broker: event.target.value } })} />
                </Label>
                <Label>
                  المبلغ بالريال
                  <Input value={amounts[item.id] ?? ""} onChange={(event) => setAmounts({ ...amounts, [item.id]: event.target.value })} inputMode="decimal" />
                </Label>
                <Label>
                  سبب الرجوع أو الإغلاق غير الناجح
                  <Input value={reasons[item.id] ?? ""} onChange={(event) => setReasons({ ...reasons, [item.id]: event.target.value })} />
                </Label>
                <div className="flex flex-wrap gap-2 md:col-span-2">
                  {forward[item.status] ? (
                    <Button type="button" onClick={() => void move(item, forward[item.status])}>
                      {statusLabel[forward[item.status]]}
                    </Button>
                  ) : null}
                  {backward[item.status] ? (
                    <Button type="button" variant="ghost" onClick={() => void move(item, backward[item.status])}>
                      رجوع: {statusLabel[backward[item.status]]}
                    </Button>
                  ) : null}
                  <Button type="button" variant="ghost" onClick={() => void move(item, "closed_unsuccessful")}>
                    إغلاق غير ناجح
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => void move(item, "cancelled")}>
                    إلغاء
                  </Button>
                </div>
              </div>
            ) : null}
            {can("update") && item.status === "awaiting_external" ? (
              <div className="grid gap-2 md:grid-cols-3">
                <Label>
                  مرجع الاتفاق
                  <Input value={refs[item.id] ?? item.agreement_ref} onChange={(event) => setRefs({ ...refs, [item.id]: event.target.value })} />
                </Label>
                <Label>
                  تاريخ الإغلاق
                  <Input type="date" value={dates[item.id] ?? ""} onChange={(event) => setDates({ ...dates, [item.id]: event.target.value })} />
                </Label>
                <div className="flex items-end">
                  <Button type="button" onClick={() => void requestClose(item)}>
                    طلب الإغلاق
                  </Button>
                </div>
              </div>
            ) : null}
            {can("approve") && item.status === "awaiting_external" && item.agreement_ref ? (
              <Button type="button" onClick={() => void approve(item)}>
                اعتماد الإغلاق
              </Button>
            ) : null}
          </article>
        );
      })}
    </section>
  );
}
