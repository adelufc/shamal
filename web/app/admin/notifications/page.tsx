"use client";

import { useEffect, useState } from "react";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";

type Notice = {
  id: string;
  kind: string;
  title_ar: string;
  body_ar: string;
  delivery_status: string;
  provider_adopted: boolean;
};

const kinds: Record<string, string> = {
  listing_match: "إعلان مطابق",
  visit_change: "تغيّر زيارة",
  payment_review: "مراجعة دفعة",
};

export default function AdminNotificationsPage() {
  const me = useSession();
  const [items, setItems] = useState<Notice[]>([]);
  const [muted, setMuted] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

  async function load() {
    const inbox = await api<{ items: Notice[]; channel_muted: boolean }>("/api/v1/account/notifications");
    setItems(inbox.items ?? []);
    setMuted(inbox.channel_muted);
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

  async function muteAll() {
    setError(null);
    try {
      await api("/api/v1/account/notifications/mute", {
        method: "POST",
        body: JSON.stringify({ channel: "in_app", muted: !muted }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  if (state === "loading") return <p className="text-sm">جارٍ تحميل إشعارات فريق العمل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">إشعارات فريق العمل</h1>
      <p className="text-sm text-muted">هذه إشعارات حساب المكتب داخل لوحة الإدارة. لا بريد تسويقي ولا مزود رسائل معتمد.</p>
      {error ? <Failure error={error} /> : null}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">القائمة</h2>
        <Button type="button" variant="ghost" onClick={() => void muteAll()}>
          {muted ? "إعادة إشعارات التطبيق" : "كتم إشعارات التطبيق"}
        </Button>
      </div>
      {items.length === 0 ? <p className="text-sm">لا توجد إشعارات جديدة.</p> : null}
      <ul className="grid gap-2">
        {items.map((item) => (
          <li key={item.id} className="grid gap-1 rounded-lg border border-line bg-card p-4 text-sm">
            <p className="font-semibold">{item.title_ar}</p>
            <p>{item.body_ar}</p>
            <p>{kinds[item.kind] ?? item.kind} · {item.provider_adopted ? "أُرسل عبر مزود" : "لم يُرسل إلى مزود"}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
