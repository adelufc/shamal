"use client";

import { useEffect, useState } from "react";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { money } from "@/lib/chrome";

type Item = {
  id: string;
  display_name: string;
  email: string;
  stage: string;
  client_stage: string;
  transaction_type: string;
  neighborhood_id: string;
  specs_ar: string;
  specs_en: string;
  timing_note: string;
  assignee_id: string;
  budget_minor?: number;
  version: number;
  close_reason?: string;
};

const stageLabel: Record<string, string> = {
  new: "جديد",
  contacted: "تم التواصل",
  interested: "مهتم",
  visit: "زيارة",
  follow_up: "متابعة",
  negotiation: "تفاوض",
  agreement: "اتفاق",
  converted: "محول إلى صفقة",
  unqualified: "غير مؤهل",
  closed: "مغلق",
};

const nextStages: Record<string, string[]> = {
  new: ["contacted", "unqualified", "closed"],
  contacted: ["interested", "unqualified", "closed"],
  interested: ["visit", "follow_up", "unqualified", "closed"],
  visit: ["follow_up", "negotiation", "unqualified", "closed"],
  follow_up: ["visit", "negotiation", "unqualified", "closed"],
  negotiation: ["agreement", "unqualified", "closed"],
  agreement: ["unqualified", "closed"],
};

export default function RequestsPage() {
  const me = useSession();
  const [items, setItems] = useState<Item[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const can = (action: string) => (me?.actions.clients ?? []).includes(action);
  const canDeal = (me?.actions.deals ?? []).includes("create");

  async function load() {
    const body = await api<{ items: Item[] }>("/api/v1/clients/requests?page_size=100");
    setItems(body.items);
    setState("ready");
  }

  useEffect(() => {
    if (!me) return;
    let stop = false;
    load()
      .catch((caught) => {
        if (stop) return;
        setError(caught);
        setState("error");
      });
    return () => {
      stop = true;
    };
  }, [me]);

  async function move(item: Item, to: string) {
    setError(null);
    try {
      await api(`/api/v1/clients/requests/${item.id}/move`, {
        method: "POST",
        body: JSON.stringify({ version: item.version, to, reason: reasons[item.id] ?? "" }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function convert(item: Item) {
    setError(null);
    try {
      await api(`/api/v1/clients/requests/${item.id}/convert`, {
        method: "POST",
        body: JSON.stringify({ version: item.version }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function assign(item: Item) {
    if (!me) return;
    setError(null);
    try {
      await api(`/api/v1/clients/requests/${item.id}/assign`, {
        method: "POST",
        body: JSON.stringify({ version: item.version, assignee_id: me.id }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض الطلبات.</p>;
  if (state === "loading") return <p className="text-sm">جارٍ التحميل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">طلبات العملاء</h1>
      <p className="text-sm text-ink/80">الطلب بلا مسؤول يبقى في صف الإدارة. غير المؤهل والمغلق يحتاجان سبباً. التحويل إلى صفقة يكون من مرحلة الاتفاق.</p>
      {error ? <Failure error={error} /> : null}
      {items.length === 0 ? <p className="text-sm">لا توجد طلبات.</p> : null}
      {items.map((item) => (
        <article key={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold">{item.display_name}</h2>
            <span>{stageLabel[item.stage] ?? item.stage}</span>
          </div>
          <p>{item.email}</p>
          <p>{item.assignee_id ? "مُسند" : "صف الإدارة"}</p>
          {item.timing_note ? <p>{item.timing_note}</p> : null}
          {item.budget_minor ? <p>{money(item.budget_minor, "ar")}</p> : null}
          {item.specs_ar ? <p>{item.specs_ar}</p> : null}
          {item.specs_en ? <p dir="ltr">{item.specs_en}</p> : null}
          {item.close_reason ? <p>{item.close_reason}</p> : null}
          {can("update") && (nextStages[item.stage] || item.stage === "agreement") ? (
            <div className="grid gap-2">
              <Input
                value={reasons[item.id] ?? ""}
                onChange={(event) => setReasons({ ...reasons, [item.id]: event.target.value })}
                placeholder="سبب الإغلاق أو عدم الأهلية"
              />
              <div className="flex flex-wrap gap-2">
                {(nextStages[item.stage] ?? []).map((stage) => (
                  <Button key={stage} type="button" variant="ghost" onClick={() => void move(item, stage)}>
                    {stageLabel[stage] ?? stage}
                  </Button>
                ))}
                {item.stage === "agreement" && canDeal ? (
                  <Button type="button" onClick={() => void convert(item)}>
                    تحويل إلى صفقة
                  </Button>
                ) : null}
                {!item.assignee_id ? (
                  <Button type="button" variant="ghost" onClick={() => void assign(item)}>
                    إسناد إلي
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
