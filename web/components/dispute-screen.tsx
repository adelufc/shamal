"use client";

import { FormEvent, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import type { Locale } from "@/lib/types";

type Dispute = {
  id: string;
  reference: string;
  status: string;
  type: string;
  description: string;
  priority: string;
  sla_adopted: boolean;
  contractual: boolean;
  financial_complete: boolean;
  money_moved: boolean;
  refund_success: boolean;
  execution_status: string;
  external_delivery: string;
  objection_days: number;
  policy_adopted: boolean;
  notes?: { id: string; visibility: string; body: string }[];
  evidence?: { id: string; sha256: string }[];
};

const types = ["entry_failure", "mismatch", "cleanliness", "unit_issue", "service", "compensation", "refund"] as const;

const copy = {
  ar: {
    title: "النزاعات",
    lead: "القضية مرتبطة بحجز أو دفعة أو خدمة. قرار التعويض ينشئ طلب استرداد ولا يحرّك المال. الملاحظات الداخلية لا تظهر هنا. مهلة الرد ونافذة الاعتراض ٧ أيام أهداف مقترحة غير تعاقدية.",
    login: "ادخل إلى الحساب لفتح قضية.",
    account: "الحساب",
    mine: "قضاياي",
    empty: "لا توجد قضايا.",
    kind: "الارتباط",
    kinds: { booking: "حجز", payment: "دفعة", service: "خدمة" } as Record<string, string>,
    type: "النوع",
    types: {
      entry_failure: "تعذر دخول",
      mismatch: "عدم مطابقة",
      cleanliness: "نظافة",
      unit_issue: "مشكلة وحدة",
      service: "خدمة",
      compensation: "تعويض",
      refund: "استرداد",
    } as Record<string, string>,
    description: "الوصف",
    amount: "المبلغ المطلوب بالريال، إن وُجد",
    send: "فتح القضية",
    evidence: "دليل",
    note: "رد ظاهر",
    review: "طلب مراجعة القرار",
    money: "لم يتحرك مال",
    refund: "نجاح الاسترداد",
    no: "لا",
    statuses: {
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
    } as Record<string, string>,
  },
  en: {
    title: "Disputes",
    lead: "A case is linked to a booking, a payment, or a service. A compensation decision creates a refund request and does not move money. Internal notes are not shown here. The response target and the 7-day objection window are proposed, not a contract.",
    login: "Sign in to open a case.",
    account: "Account",
    mine: "My cases",
    empty: "There are no cases.",
    kind: "Link",
    kinds: { booking: "Booking", payment: "Payment", service: "Service" } as Record<string, string>,
    type: "Type",
    types: {
      entry_failure: "Could not enter",
      mismatch: "Not as described",
      cleanliness: "Cleanliness",
      unit_issue: "Unit problem",
      service: "Service",
      compensation: "Compensation",
      refund: "Refund",
    } as Record<string, string>,
    description: "Description",
    amount: "Requested amount in riyals, if any",
    send: "Open the case",
    evidence: "Evidence",
    note: "Visible reply",
    review: "Ask for a review of the decision",
    money: "No money moved",
    refund: "Refund success",
    no: "No",
    statuses: {
      open: "Open",
      classified: "Classified",
      in_progress: "In progress",
      awaiting_client: "Waiting for the client",
      awaiting_landlord: "Waiting for the landlord",
      escalated: "Escalated",
      proposed: "Proposed resolution",
      resolved: "Resolved",
      closed: "Closed",
      reopened: "Reopened",
    } as Record<string, string>,
  },
} as const;

export function DisputeScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const params = useSearchParams();
  const [kind, setKind] = useState(params.get("subject_kind") || "booking");
  const [subject, setSubject] = useState(params.get("subject_id") ?? "");
  const [type, setType] = useState<(typeof types)[number]>("cleanliness");
  const [description, setDescription] = useState("");
  const [riyals, setRiyals] = useState("");
  const [items, setItems] = useState<Dispute[]>([]);
  const [current, setCurrent] = useState<Dispute | null>(null);
  const [note, setNote] = useState("");
  const [review, setReview] = useState("");
  const [signedOut, setSignedOut] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const base = locale === "en" ? "/en/disputes" : "/disputes";

  async function load(id?: string) {
    const list = await api<{ items: Dispute[] }>("/api/v1/account/disputes", { headers: { "Accept-Language": locale } });
    setItems(list.items ?? []);
    if (id) {
      setCurrent(await api<Dispute>(`/api/v1/account/disputes/${id}`, { headers: { "Accept-Language": locale } }));
    }
  }

  useEffect(() => {
    let stop = false;
    api<{ items: Dispute[] }>("/api/v1/account/disputes", { headers: { "Accept-Language": locale } })
      .then((body) => {
        if (stop) return;
        setItems(body.items ?? []);
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
  }, [locale]);

  async function open(event: FormEvent) {
    event.preventDefault();
    setError(null);
    const amount = riyals.trim() === "" ? undefined : Math.round(Number(riyals) * 100);
    try {
      const created = await api<Dispute>("/api/v1/account/disputes", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({
          subject_kind: kind,
          subject_id: subject,
          type,
          description,
          occurred_at: new Date().toISOString(),
          ...(amount === undefined || Number.isNaN(amount) ? {} : { requested_amount_minor: amount }),
        }),
      });
      setDescription("");
      await load(created.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function upload(file: File) {
    if (!current) return;
    const data = new FormData();
    data.set("file", file);
    setError(null);
    try {
      await api(`/api/v1/account/disputes/${current.id}/evidence`, { method: "POST", body: data, headers: { "Accept-Language": locale } });
      await load(current.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function sendNote(event: FormEvent) {
    event.preventDefault();
    if (!current) return;
    setError(null);
    try {
      await api(`/api/v1/account/disputes/${current.id}/notes`, {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ body: note }),
      });
      setNote("");
      await load(current.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function askReview(event: FormEvent) {
    event.preventDefault();
    if (!current) return;
    setError(null);
    try {
      const next = await api<Dispute>(`/api/v1/account/disputes/${current.id}/review`, {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ reason: review }),
      });
      setReview("");
      setCurrent(next);
      await load(next.id);
    } catch (caught) {
      setError(caught);
    }
  }

  return (
    <PublicFrame locale={locale} path={`${base}${subject ? `?subject_kind=${kind}&subject_id=${subject}` : ""}`}>
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
        {!signedOut && state === "ready" ? (
          <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={(event) => void open(event)}>
            <Label>
              {text.kind}
              <select className="w-full rounded-md border border-line bg-card px-2 py-2" value={kind} onChange={(event) => setKind(event.target.value)}>
                {Object.keys(text.kinds).map((item) => (
                  <option key={item} value={item}>
                    {text.kinds[item]}
                  </option>
                ))}
              </select>
            </Label>
            <Label>
              {text.kinds[kind] ?? kind}
              <Input value={subject} onChange={(event) => setSubject(event.target.value)} required />
            </Label>
            <Label>
              {text.type}
              <select className="w-full rounded-md border border-line bg-card px-2 py-2" value={type} onChange={(event) => setType(event.target.value as (typeof types)[number])}>
                {types.map((item) => (
                  <option key={item} value={item}>
                    {text.types[item]}
                  </option>
                ))}
              </select>
            </Label>
            <Label>
              {text.description}
              <Input value={description} onChange={(event) => setDescription(event.target.value)} required />
            </Label>
            <Label>
              {text.amount}
              <Input value={riyals} inputMode="decimal" onChange={(event) => setRiyals(event.target.value)} />
            </Label>
            <Button type="submit">{text.send}</Button>
          </form>
        ) : null}
        <section className="grid gap-2">
          <h2 className="text-lg font-semibold">{text.mine}</h2>
          {items.length === 0 && !signedOut ? <p className="text-sm text-ink/70">{text.empty}</p> : null}
          {items.map((item) => (
            <button key={item.id} className="rounded-lg border border-line bg-card p-3 text-start text-sm" type="button" onClick={() => void load(item.id)}>
              {item.reference} · {text.statuses[item.status] ?? item.status} · {text.types[item.type] ?? item.type}
            </button>
          ))}
        </section>
        {current ? (
          <article className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
            <h2 className="font-semibold">{current.reference}</h2>
            <p>{current.description}</p>
            <p>
              {text.statuses[current.status] ?? current.status} · {current.priority} · {text.money}: {current.money_moved ? "yes" : text.no} · {text.refund}: {current.refund_success ? "yes" : text.no}
            </p>
            <p>
              {current.execution_status} · {current.external_delivery} · {current.objection_days} · {current.policy_adopted ? "" : text.no} · {current.sla_adopted ? "" : text.no} · {current.contractual ? "" : text.no}
            </p>
            {(current.evidence ?? []).map((item) => (
              <p key={item.id}>{item.sha256}</p>
            ))}
            {(current.notes ?? []).map((item) => (
              <p key={item.id}>{item.body}</p>
            ))}
            <Label>
              {text.evidence}
              <Input
                type="file"
                accept="image/jpeg,image/png,image/webp,application/pdf"
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) void upload(file);
                }}
              />
            </Label>
            <form className="grid gap-2" onSubmit={(event) => void sendNote(event)}>
              <Label>
                {text.note}
                <Input value={note} onChange={(event) => setNote(event.target.value)} />
              </Label>
              <Button type="submit">{text.note}</Button>
            </form>
            <form className="grid gap-2" onSubmit={(event) => void askReview(event)}>
              <Label>
                {text.review}
                <Input value={review} onChange={(event) => setReview(event.target.value)} />
              </Label>
              <Button type="submit">{text.review}</Button>
            </form>
          </article>
        ) : null}
        {error ? <Failure error={error} /> : null}
      </section>
    </PublicFrame>
  );
}
