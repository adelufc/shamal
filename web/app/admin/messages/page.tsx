"use client";

import { useEffect, useState } from "react";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

type Item = { id: string; subject_kind: string; subject_id: string; client_name: string; assignee_id?: string; unread: number; received: number; recipient_locale: string; first_human_reply_at?: string };

const kinds: Record<string, string> = { listing: "إعلان", visit: "زيارة", booking: "حجز", deal: "صفقة", dispute: "نزاع" };
type Message = { id: string; kind: string; body: string; hidden: boolean; delivery_status: string };
type Canned = { id: string; body: string };

export default function AdminMessagesPage() {
  const me = useSession();
  const [items, setItems] = useState<Item[]>([]);
  const [thread, setThread] = useState<Message[]>([]);
  const [open, setOpen] = useState("");
  const [locale, setLocale] = useState("ar");
  const [canned, setCanned] = useState<Canned[]>([]);
  const [body, setBody] = useState("");
  const [note, setNote] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const can = (action: string) => (me?.actions.messages ?? []).includes(action);

  async function load() {
    const list = await api<{ items: Item[] }>("/api/v1/messages");
    setItems(list.items ?? []);
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

  async function show(item: Item) {
    setOpen(item.id);
    setLocale(item.recipient_locale);
    setError(null);
    try {
      const view = await api<{ items: Message[] }>(`/api/v1/messages/${item.id}`);
      setThread(view.items ?? []);
      const replies = await api<{ items: Canned[] }>(`/api/v1/messages/canned?locale=${item.recipient_locale}`);
      setCanned(replies.items ?? []);
    } catch (caught) {
      setError(caught);
    }
  }

  async function send(kind: string, extra: Record<string, string> = {}) {
    if (!open) return;
    setError(null);
    try {
      await api(`/api/v1/messages/${open}`, {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify({ kind, body: kind === "internal_note" ? note : body, ...extra }),
      });
      setBody("");
      setNote("");
      const view = await api<{ items: Message[] }>(`/api/v1/messages/${open}`);
      setThread(view.items ?? []);
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function hide(id: string) {
    setError(null);
    try {
      await api("/api/v1/messages/hide", { method: "POST", body: JSON.stringify({ message_id: id, reason }) });
      setReason("");
      if (open) {
        const view = await api<{ items: Message[] }>(`/api/v1/messages/${open}`);
        setThread(view.items ?? []);
      }
    } catch (caught) {
      setError(caught);
    }
  }

  async function assign() {
    if (!open || !me) return;
    setError(null);
    try {
      await api(`/api/v1/messages/${open}/assign`, { method: "POST", body: JSON.stringify({ assignee_id: me.id }) });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  if (!can("view")) return <p className="text-sm">لا توجد صلاحية لعرض الرسائل.</p>;
  if (state === "loading") return <p className="text-sm">جارٍ التحميل…</p>;
  if (state === "error" && items.length === 0) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">الرسائل</h1>
      <p className="text-sm text-ink/80">الرد الآلي والرد الجاهز لا يُحسبان أول رد بشري. الملاحظة الداخلية لا تصل إلى العميل. الحذف الإشرافي يخفي النص عن العميل ويبقي نسخة للتدقيق. لا مزود رسائل خارجي، لذلك حالة التسليم تبقى غير مرسلة.</p>
      {error ? <Failure error={error} /> : null}
      {items.length === 0 ? <p className="text-sm">لا توجد محادثات.</p> : null}
      {items.map((item) => (
        <button key={item.id} className="rounded-lg border border-line bg-card p-3 text-start text-sm" type="button" onClick={() => void show(item)}>
          {item.client_name || "عميل"} · {kinds[item.subject_kind] ?? item.subject_kind} · {item.subject_id.slice(0, 8)} · غير مقروء {item.unread}
          {item.assignee_id ? " · مسند" : " · غير مسند"}
          {item.first_human_reply_at ? " · تم أول رد بشري" : " · بلا رد"}
        </button>
      ))}
      {open ? (
        <article className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
          {thread.map((item) => (
            <p key={item.id}>
              {item.kind}: {item.hidden ? "مخفية" : item.body} · {item.delivery_status}
              {can("archive") && !item.hidden ? (
                <button className="mx-2 underline" type="button" onClick={() => void hide(item.id)}>
                  إخفاء
                </button>
              ) : null}
            </p>
          ))}
          {can("update") ? (
            <>
              <Label>
                رد
                <Input value={body} onChange={(event) => setBody(event.target.value)} />
              </Label>
              <Button type="button" onClick={() => void send("message")}>
                إرسال رد بشري
              </Button>
              <Label>
                ملاحظة داخلية
                <Input value={note} onChange={(event) => setNote(event.target.value)} />
              </Label>
              <Button type="button" onClick={() => void send("internal_note")}>
                حفظ الملاحظة
              </Button>
              {canned.map((item) => (
                <Button key={item.id} type="button" onClick={() => void send("canned", { canned_id: item.id })}>
                  رد جاهز ({locale}): {item.body}
                </Button>
              ))}
              <Button type="button" onClick={() => void assign()}>
                أسندها إلي
              </Button>
            </>
          ) : null}
          {can("archive") ? (
            <Label>
              سبب الإخفاء
              <Input value={reason} onChange={(event) => setReason(event.target.value)} />
            </Label>
          ) : null}
        </article>
      ) : null}
    </section>
  );
}
