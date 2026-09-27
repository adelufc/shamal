"use client";

import { FormEvent, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import type { Locale } from "@/lib/types";

type Preview = { received: number; created: boolean; conversation_id?: string };
type Inbox = { id: string; subject_kind: string; subject_id: string; unread: number; received: number; delivery_status: string };
type Message = { id: string; kind: string; body: string; hidden: boolean; send_status: string; delivery_status: string; provider_adopted: boolean };

const copy = {
  ar: {
    title: "الرسائل",
    lead: "فتح هذه الشاشة لا يُحتسب رسالة مستلمة. لا تُرسل الرسالة إلى مزود خارجي، ولا تُعدّل بعد إرسالها. التصحيح رسالة لاحقة.",
    login: "ادخل إلى الحساب لإرسال رسالة عن إعلان أو زيارة أو حجز أو صفقة.",
    account: "الحساب",
    subject: "الموضوع",
    body: "الرسالة",
    send: "إرسال",
    received: "الرسائل المستلمة في هذا الموضوع",
    inbox: "صندوق الوارد",
    empty: "لا توجد رسائل مستلمة.",
    refresh: "جلب الرسائل الفائتة",
    report: "إبلاغ عن إساءة",
    reportHint: "الإبلاغ يذهب للمراجعة ولا يمنع الوصول إلى الحجز أو النزاع.",
    notSent: "لم تُسلّم خارج شمال",
    kinds: { listing: "إعلان", visit: "زيارة", booking: "حجز", deal: "صفقة", dispute: "نزاع" } as Record<string, string>,
  },
  en: {
    title: "Messages",
    lead: "Opening this screen does not count as a received message. A message is not delivered to an external provider, and it is not edited after it is sent. A correction is a later message.",
    login: "Sign in to message about a listing, visit, booking, or deal.",
    account: "Account",
    subject: "Subject",
    body: "Message",
    send: "Send",
    received: "Received messages on this subject",
    inbox: "Inbox",
    empty: "There are no received messages.",
    refresh: "Fetch missed messages",
    report: "Report abuse",
    reportHint: "A report goes to review and does not block the booking or a dispute.",
    notSent: "Not delivered outside Shamal",
    kinds: { listing: "Listing", visit: "Visit", booking: "Booking", deal: "Deal", dispute: "Dispute" } as Record<string, string>,
  },
} as const;

export function MessageScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const params = useSearchParams();
  const kind = params.get("subject_kind") ?? "";
  const subject = params.get("subject_id") ?? "";
  const [preview, setPreview] = useState<Preview | null>(null);
  const [inbox, setInbox] = useState<Inbox[]>([]);
  const [thread, setThread] = useState<Message[]>([]);
  const [open, setOpen] = useState("");
  const [body, setBody] = useState("");
  const [reason, setReason] = useState("");
  const [signedOut, setSignedOut] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const base = locale === "en" ? "/en/messages" : "/messages";
  const path = `${base}${kind && subject ? `?subject_kind=${kind}&subject_id=${subject}` : ""}`;

  async function loadInbox() {
    const list = await api<{ items: Inbox[] }>("/api/v1/account/messages", { headers: { "Accept-Language": locale } });
    setInbox(list.items ?? []);
  }

  useEffect(() => {
    let stop = false;
    const previewPath = kind && subject ? `/api/v1/account/messages/preview?subject_kind=${kind}&subject_id=${subject}` : "";
    Promise.all([
      api<{ items: Inbox[] }>("/api/v1/account/messages", { headers: { "Accept-Language": locale } }),
      previewPath ? api<Preview>(previewPath, { headers: { "Accept-Language": locale } }) : Promise.resolve(null),
    ])
      .then(([list, seen]) => {
        if (stop) return;
        setInbox(list.items ?? []);
        setPreview(seen);
        setSignedOut(false);
        setState("ready");
      })
      .catch((caught) => {
        if (stop) return;
        if (caught instanceof ApiError && (caught.status === 401 || caught.status === 403)) {
          setSignedOut(true);
          setState("ready");
          return;
        }
        setError(caught);
        setState("error");
      });
    return () => {
      stop = true;
    };
  }, [kind, subject, locale]);

  async function send(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const sent = await api<Message>(open ? `/api/v1/account/messages/${open}` : "/api/v1/account/messages", {
        method: "POST",
        headers: { "Accept-Language": locale, "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify(open ? { body } : { subject_kind: kind, subject_id: subject, body }),
      });
      setBody("");
      setOpen(sent.id ? open || "" : open);
      if (!open && "conversation_id" in sent) setOpen(String((sent as Message & { conversation_id?: string }).conversation_id ?? ""));
      const posted = sent as Message & { conversation_id?: string };
      if (posted.conversation_id) setOpen(posted.conversation_id);
      await loadInbox();
      if (posted.conversation_id || open) {
        const id = posted.conversation_id || open;
        const view = await api<{ items: Message[] }>(`/api/v1/account/messages/${id}`, { headers: { "Accept-Language": locale } });
        setThread(view.items ?? []);
      }
    } catch (caught) {
      setError(caught);
    }
  }

  async function show(id: string) {
    setError(null);
    setOpen(id);
    try {
      const view = await api<{ items: Message[] }>(`/api/v1/account/messages/${id}`, { headers: { "Accept-Language": locale } });
      setThread(view.items ?? []);
    } catch (caught) {
      setError(caught);
    }
  }

  async function refresh() {
    if (!open || thread.length === 0) return;
    const after = thread[thread.length - 1]?.id;
    const view = await api<{ items: Message[] }>(`/api/v1/account/messages/${open}?after=${after}`, { headers: { "Accept-Language": locale } });
    const incoming = view.items ?? [];
    setThread((current) => {
      const known = new Set(current.map((item) => item.id));
      return [...current, ...incoming.filter((item) => !known.has(item.id))];
    });
  }

  async function report() {
    if (!open || !reason.trim()) return;
    setError(null);
    try {
      await api(`/api/v1/account/messages/${open}/report`, {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ kind: "abuse", reason: reason.trim() }),
      });
      setReason("");
    } catch (caught) {
      setError(caught);
    }
  }

  return (
    <PublicFrame locale={locale} path={path}>
      <section className="grid gap-4">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <p className="text-sm text-ink/80">{text.lead}</p>
        {state === "loading" ? <p className="text-sm">{locale === "en" ? "Loading…" : "جارٍ التحميل…"}</p> : null}
        {signedOut ? (
          <p className="text-sm">
            {text.login}{" "}
            <a className="underline" href={locale === "en" ? "/en/account" : "/account"}>
              {text.account}
            </a>
          </p>
        ) : null}
        {preview ? (
          <p className="text-sm">
            {text.subject}: {text.kinds[kind] ?? kind} · {text.received}: {preview.received}
          </p>
        ) : null}
        {!signedOut && kind && subject ? (
          <form className="grid gap-2 rounded-lg border border-line bg-card p-4" onSubmit={(event) => void send(event)}>
            <Label>
              {text.body}
              <Input value={body} onChange={(event) => setBody(event.target.value)} required />
            </Label>
            <Button type="submit">{text.send}</Button>
          </form>
        ) : null}
        <section className="grid gap-2">
          <h2 className="text-lg font-semibold">{text.inbox}</h2>
          {inbox.length === 0 ? <p className="text-sm text-ink/70">{text.empty}</p> : null}
          {inbox.map((item) => (
            <button key={item.id} className="rounded-lg border border-line bg-card p-3 text-start text-sm" type="button" onClick={() => void show(item.id)}>
              {text.kinds[item.subject_kind] ?? item.subject_kind} · {item.received} · {item.unread} · {text.notSent}
            </button>
          ))}
        </section>
        {open ? (
          <section className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
            <Button type="button" onClick={() => void refresh()}>
              {text.refresh}
            </Button>
            {thread.map((item) => (
              <p key={item.id}>
                {item.hidden ? (locale === "en" ? "Hidden" : "مخفية") : item.body} · {item.delivery_status}
              </p>
            ))}
            <p>{text.reportHint}</p>
            <Label>
              {text.report}
              <Input value={reason} onChange={(event) => setReason(event.target.value)} />
            </Label>
            <Button type="button" onClick={() => void report()}>
              {text.report}
            </Button>
          </section>
        ) : null}
        {error ? <Failure error={error} /> : null}
        {state === "error" ? <Failure error={error} /> : null}
      </section>
    </PublicFrame>
  );
}
