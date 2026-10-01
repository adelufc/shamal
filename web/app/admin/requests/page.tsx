"use client";

import { FormEvent, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
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
  const [open, setOpen] = useState("");
  const [total, setTotal] = useState(0);
  const params = useSearchParams();
  const [q, setQ] = useState(params.get("q") ?? "");
  const [appliedQ, setAppliedQ] = useState(params.get("q") ?? "");
  const [unassigned, setUnassigned] = useState(params.get("unassigned") === "1");
  const [page, setPage] = useState(Number(params.get("page") || "1") || 1);
  const [refresh, setRefresh] = useState(0);
  const can = (action: string) => (me?.actions.clients ?? []).includes(action);
  const canDeal = (me?.actions.deals ?? []).includes("create");

  useEffect(() => {
    if (!me) return;
    let stop = false;
    const query = new URLSearchParams({ page_size: "25", page: String(page) });
    if (appliedQ.trim()) query.set("q", appliedQ.trim());
    if (unassigned) query.set("unassigned", "1");
    setItems([]);
    setState("loading");
    api<{ items: Item[]; total: number }>(`/api/v1/clients/requests?${query}`)
      .then((body) => {
        if (stop) return;
        setItems(body.items);
        setTotal(body.total);
        setState("ready");
      })
      .catch((caught) => {
        if (stop) return;
        setError(caught);
        setState("error");
      });
    return () => {
      stop = true;
    };
  }, [me, page, unassigned, appliedQ, refresh]);

  async function move(item: Item, to: string) {
    setError(null);
    try {
      await api(`/api/v1/clients/requests/${item.id}/move`, {
        method: "POST",
        body: JSON.stringify({ version: item.version, to, reason: reasons[item.id] ?? "" }),
      });
      setRefresh((value) => value + 1);
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
      setRefresh((value) => value + 1);
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
      setRefresh((value) => value + 1);
    } catch (caught) {
      setError(caught);
    }
  }

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض الطلبات.</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">طلبات العملاء</h1>
      <p className="text-sm text-muted">الطلب بلا مسؤول يبقى في صف الإدارة. فتح التفاصيل لا يغيّر المرحلة. غير المؤهل والمغلق يحتاجان سبباً.</p>
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          setAppliedQ(q);
          setPage(1);
        }}
      >
        <label className="grid gap-2 text-sm">
          بحث بالاسم أو البريد أو الرقم
          <Input value={q} onChange={(event) => setQ(event.target.value)} />
        </label>
        <label className="flex min-h-11 items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={unassigned}
            onChange={(event) => {
              setUnassigned(event.target.checked);
              setPage(1);
            }}
          />
          غير المسند
        </label>
        <Button type="submit">بحث</Button>
      </form>
      <p className="text-sm">
        {state === "loading" && items.length === 0
          ? "جارٍ التحميل…"
          : total === 0
            ? "عرض 0 من 0"
            : `عرض ${(page - 1) * 25 + 1}–${Math.min(page * 25, total)} من ${total}`}
      </p>
      {error ? <Failure error={error} /> : null}
      {items.length === 0 ? <p className="text-sm">لا توجد طلبات.</p> : null}
      {items.map((item) => (
        <article key={item.id} data-request-id={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold">{item.display_name}</h2>
            <span>{stageLabel[item.stage] ?? item.stage}</span>
          </div>
          <p>{item.email}</p>
          <p>{item.assignee_id ? "مُسند" : "غير مسند، ويبقى في صف الإدارة"}</p>
          <p>
            {item.transaction_type === "sale"
              ? "بيع"
              : item.transaction_type === "annual"
                ? "سنوي"
                : item.transaction_type === "monthly"
                  ? "شهري"
                  : item.transaction_type === "daily"
                    ? "يومي"
                    : item.transaction_type}
          </p>
          <Button type="button" variant="ghost" onClick={() => setOpen(open === item.id ? "" : item.id)}>
            {open === item.id ? "إخفاء التفاصيل" : "التفاصيل"}
          </Button>
          {open === item.id ? (
            <>
          {item.timing_note ? <p>{item.timing_note}</p> : null}
          {item.budget_minor ? <p>{money(item.budget_minor, "ar")}</p> : null}
          {item.specs_ar ? <p>{item.specs_ar}</p> : null}
          {item.specs_en ? <p dir="ltr">{item.specs_en}</p> : null}
          {item.close_reason ? <p>{item.close_reason}</p> : null}
          {open === item.id && can("update") && (nextStages[item.stage] || item.stage === "agreement") ? (
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
            </>
          ) : null}
        </article>
      ))}
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>
          السابق
        </Button>
        <Button type="button" variant="ghost" disabled={page * 25 >= total} onClick={() => setPage(page + 1)}>
          التالي
        </Button>
      </div>
    </section>
  );
}
