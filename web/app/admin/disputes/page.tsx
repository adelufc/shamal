"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money } from "@/lib/chrome";

type Dispute = {
  id: string;
  reference: string;
  status: string;
  type: string;
  description: string;
  priority: string;
  financial_complete: boolean;
  money_moved: boolean;
  refund_success: boolean;
  stay_id?: string;
  execution_status: string;
  hold_amount_minor?: number;
  response_due_at: string;
  assignee_id?: string;
  subject_kind?: string;
  subject_id?: string;
};

const statusLabel: Record<string, string> = {
  open: "مفتوحة",
  classified: "مصنفة",
  in_progress: "قيد المعالجة",
  awaiting_client: "بانتظار العميل",
  awaiting_landlord: "بانتظار المؤجر",
  escalated: "مصعدة",
  proposed: "حل مقترح",
  resolved: "محلولة",
  closed: "مغلقة",
  reopened: "أعيد فتحها",
};

export default function AdminDisputesPage() {
  const me = useSession();
  const [items, setItems] = useState<Dispute[]>([]);
  const [reason, setReason] = useState<Record<string, string>>({});
  const [amount, setAmount] = useState<Record<string, string>>({});
  const [note, setNote] = useState<Record<string, string>>({});
  const [error, setError] = useState<unknown>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [q, setQ] = useState("");
  const [statusFilter, setStatusFilter] = useState(useSearchParams().get("status") ?? "");
  const [page, setPage] = useState(1);
  const [people, setPeople] = useState<Record<string, string>>({});
  const [lastAction, setLastAction] = useState<Record<string, string>>({});
  const can = (action: string) => (me?.actions.disputes ?? []).includes(action);

  async function load() {
    const body = await api<{ items: Dispute[] }>("/api/v1/disputes");
    setItems(body.items ?? []);
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

  useEffect(() => {
    if (!me) return;
    api<{ staff?: { id: string; display_name: string }[] }>("/api/v1/org/structure")
      .then((body) => {
        const next: Record<string, string> = {};
        for (const person of body.staff ?? []) next[person.id] = person.display_name;
        setPeople(next);
      })
      .catch(() => setPeople({}));
  }, [me]);

  async function context(item: Dispute) {
    setError(null);
    try {
      const body = await api<{ events?: { kind: string; to_status?: string; created_at: string }[] }>(`/api/v1/disputes/${item.id}`);
      const last = body.events?.[body.events.length - 1];
      setLastAction({
        ...lastAction,
        [item.id]: last
          ? `${last.kind}${last.to_status ? ` → ${statusLabel[last.to_status] ?? last.to_status}` : ""} · ${new Date(last.created_at).toLocaleString("ar-SA", { timeZone: "Asia/Riyadh" })}`
          : "لا إجراء مسجّل",
      });
    } catch (caught) {
      setError(caught);
    }
  }

  async function move(item: Dispute, to: string, extra: Record<string, unknown> = {}) {
    setError(null);
    try {
      await api(`/api/v1/disputes/${item.id}/move`, {
        method: "POST",
        body: JSON.stringify({ to, reason: reason[item.id] ?? "", ...extra }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function notice(item: Dispute) {
    setError(null);
    try {
      await api(`/api/v1/disputes/${item.id}/notice`, { method: "POST", body: JSON.stringify({ body: note[item.id] ?? "" }) });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function internal(item: Dispute) {
    setError(null);
    try {
      await api(`/api/v1/disputes/${item.id}/notes`, {
        method: "POST",
        body: JSON.stringify({ visibility: "internal", body: note[item.id] ?? "" }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  const filtered = items.filter((item) => {
    if (statusFilter && item.status !== statusFilter) return false;
    const haystack = `${item.reference} ${item.description}`.toLowerCase();
    return haystack.includes(q.trim().toLowerCase());
  });
  const visible = filtered.slice((page - 1) * 25, page * 25);

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض النزاعات.</p>;
  if (state === "loading") return <p className="text-sm">جارٍ التحميل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">النزاعات</h1>
      <details className="text-sm text-ink/80">
        <summary>مساعدة</summary>
        <p>
          الحالات: مفتوحة، مصنفة، قيد المعالجة، بانتظار العميل، بانتظار المؤجر، مصعدة، حل مقترح، محلولة، مغلقة، أعيد فتحها. الانتقال غير المدرج يُرفض. تعويض معتمد ينشئ طلب استرداد بحالة مراجعة ولا يغيّر الدفتر. القضية لا تُغلق مالياً من هنا. الإشعار يُحفظ داخل القضية ولا يُرسل عبر مزود. لا تُدخل عمولة أو ضريبة أو رسم إلغاء. اسم العميل ليس في قائمة القضايا، فيظهر وصف القضية ومرجع الحجز.
        </p>
      </details>
      <div className="flex flex-wrap items-end gap-2 [&_input]:border-action [&_select]:border-action">
        <label className="grid gap-1 text-sm">
          بحث بالمرجع أو الوصف
          <Input value={q} onChange={(event) => { setQ(event.target.value); setPage(1); }} />
        </label>
        <label className="grid gap-1 text-sm">
          الحالة
          <select className="rounded-md border border-line bg-card px-3 py-2" value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value); setPage(1); }}>
            <option value="">الكل</option>
            {Object.entries(statusLabel).map(([value, label]) => (
              <option key={value} value={value}>{label}</option>
            ))}
          </select>
        </label>
      </div>
      {error ? <Failure error={error} /> : null}
      {filtered.length === 0 ? <p className="text-sm">لا توجد قضايا.</p> : null}
      <p className="text-sm">{filtered.length === 0 ? "عرض 0 من 0" : `عرض ${(page - 1) * 25 + 1}–${Math.min(page * 25, filtered.length)} من ${filtered.length}`}</p>
      {visible.map((item) => (
        <article key={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold">{item.reference}</h2>
            <span>{statusLabel[item.status] ?? item.status}</span>
          </div>
          <p>{item.description}</p>
          <p>
            {(
              {
                entry_failure: "تعذر دخول",
                mismatch: "عدم تطابق",
                cleanliness: "نظافة",
                unit_issue: "حالة الوحدة",
                service: "خدمة",
                compensation: "تعويض",
                refund: "استرداد",
              } as Record<string, string>
            )[item.type] ?? item.type}{" "}
            · {item.priority === "critical" ? "حرج" : item.priority === "normal" ? "عادي" : item.priority}
            {item.stay_id ? ` · مرجع الحجز ${item.stay_id}` : " · بلا حجز مرتبط"}
            {` · المسؤول: ${item.assignee_id ? people[item.assignee_id] || "مسند بلا اسم ظاهر" : "غير مسند"}`}
            · تحرك المال: {item.money_moved ? "نعم" : "لا"} · اكتمال مالي: {item.financial_complete ? "نعم" : "لا"} · نجاح الاسترداد: {item.refund_success ? "نعم" : "لا"}
          </p>
          {item.hold_amount_minor ? <p>مبلغ مجمد في سجل التسوية فقط: {money(item.hold_amount_minor, "ar")}</p> : null}
          <p>{lastAction[item.id] ? `آخر إجراء: ${lastAction[item.id]}` : "آخر إجراء يظهر عند الطلب."}</p>
          <Button type="button" variant="ghost" onClick={() => void context(item)}>سياق القضية</Button>
          {item.stay_id ? <p>لا شاشة حجز للموظف، والمرجع أعلاه هو سجل الحجز.</p> : null}
          <Label>
            السبب
            <Input value={reason[item.id] ?? ""} onChange={(event) => setReason({ ...reason, [item.id]: event.target.value })} />
          </Label>
          {can("update") && item.status === "open" ? (
            <Button type="button" onClick={() => void move(item, "classified")}>
              تصنيف
            </Button>
          ) : null}
          {can("update") && item.status === "classified" ? (
            <Button type="button" onClick={() => void move(item, "in_progress")}>
              بدء المعالجة
            </Button>
          ) : null}
          {can("update") && (item.status === "in_progress" || item.status === "escalated") ? (
            <>
              <Label>
                مبلغ التعويض بالريال
                <Input value={amount[item.id] ?? ""} onChange={(event) => setAmount({ ...amount, [item.id]: event.target.value })} />
              </Label>
              <Button
                type="button"
                onClick={() =>
                  void move(item, "proposed", {
                    kind: "compensation",
                    amount_minor: Math.round(Number(amount[item.id] || "0") * 100),
                    actions: "طلب استرداد فقط",
                  })
                }
              >
                اقتراح تعويض
              </Button>
            </>
          ) : null}
          {can("approve") && item.status === "proposed" ? (
            <Button type="button" onClick={() => void move(item, "resolved")}>
              اعتماد القرار
            </Button>
          ) : null}
          {can("update") ? (
            <>
              <Label>
                إشعار أو ملاحظة
                <Input value={note[item.id] ?? ""} onChange={(event) => setNote({ ...note, [item.id]: event.target.value })} />
              </Label>
              <div className="flex flex-wrap gap-2">
                <Button type="button" onClick={() => void notice(item)}>
                  إشعار الطرف داخل القضية
                </Button>
                <Button type="button" onClick={() => void internal(item)}>
                  ملاحظة داخلية
                </Button>
              </div>
            </>
          ) : null}
          {can("approve") && item.status === "resolved" ? (
            <Button type="button" onClick={() => void move(item, "closed")}>
              إغلاق
            </Button>
          ) : null}
        </article>
      ))}
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>السابق</Button>
        <Button type="button" variant="ghost" disabled={page * 25 >= filtered.length} onClick={() => setPage(page + 1)}>التالي</Button>
      </div>
    </section>
  );
}
