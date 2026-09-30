"use client";

import { useEffect, useState } from "react";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { money } from "@/lib/chrome";

type Refund = {
  id: string;
  stay_id: string;
  amount_minor: number;
  currency: string;
  reason: string;
  status: string;
  success: boolean;
  approvals: number;
};

const statusLabel: Record<string, string> = {
  under_review: "قيد المراجعة",
  approved: "موافق عليه",
  sent_to_provider: "أُرسل إلى المزود",
  successful: "ناجح",
  rejected: "مرفوض",
  failed_retryable: "فشل قابل لإعادة المحاولة",
  cancelled: "ملغى",
};

export default function RefundsPage() {
  const me = useSession();
  const [items, setItems] = useState<Refund[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const can = (action: string) => (me?.actions.finance ?? []).includes(action);

  async function load() {
    const body = await api<{ items: Refund[] }>("/api/v1/finance/refunds");
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

  async function approve(item: Refund) {
    setError(null);
    try {
      await api(`/api/v1/finance/refunds/${item.id}/approve`, { method: "POST", body: "{}" });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض طلبات الاسترداد.</p>;
  if (state === "loading") return <p className="text-sm">جارٍ التحميل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">طلبات الاسترداد</h1>
      <p className="text-sm text-ink/80">
        كل طلب يبقى مستقلاً عن الإقامة. حتى اعتماد حد مالي، يحتاج الطلب موافقتين من شخصين غير مقدمه. الموافقة ليست إرسالاً إلى مزود وليست نجاحاً، لأن بوابة الدفع غير معتمدة. لا تُدخل عمولة أو ضريبة أو رسم إلغاء.
      </p>
      {error ? <Failure error={error} /> : null}
      {items.length === 0 ? <p className="text-sm">لا توجد طلبات استرداد.</p> : null}
      {items.map((item) => (
        <article key={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold">{money(item.amount_minor, "ar")}</h2>
            <span>{statusLabel[item.status] ?? item.status}</span>
          </div>
          <p>{item.reason}</p>
          <p>طلب {item.id.slice(0, 8)} · حجز {item.stay_id.slice(0, 8)} · {item.currency}</p>
          <p>الموافقات: {item.approvals} · نجاح الاسترداد: {item.success ? "نعم" : "لا"}</p>
          {item.status === "approved" ? <p>اعتُمد الطلب ولم يكتمل رد المبلغ بعد</p> : null}
          {can("approve") && item.status === "under_review" ? (
            <Button type="button" onClick={() => void approve(item)}>
              موافقة
            </Button>
          ) : null}
        </article>
      ))}
    </section>
  );
}
