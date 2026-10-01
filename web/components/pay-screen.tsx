"use client";

import { FormEvent, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money } from "@/lib/chrome";
import type { Locale } from "@/lib/types";

type Stay = {
  id: string;
  status: string;
  arrival: string;
  departure: string;
  stay_due_minor: number;
  title_ar: string;
  title_en: string;
};

type Attempt = {
  id: string;
  method: string;
  provider: string;
  gateway_adopted: boolean;
  amount_minor: number;
  currency: string;
  status: string;
  decisive: boolean;
  success: boolean;
  receipt_id?: string;
  invoice_id?: string;
  label_ar: string;
  label_en: string;
  operational_default: boolean;
  contractual: boolean;
  beneficiary_adopted: boolean;
  proof_confirms: boolean;
  provider_query: string;
};

type InvoiceLine = {
  code: string;
  quantity: number;
  unit_minor: number;
  amount_minor: number;
  counts_as_stay: boolean;
};

type Invoice = {
  id: string;
  serial: number;
  total_minor: number;
  currency: string;
  seller_adopted: boolean;
  seller_note_ar: string;
  seller_note_en: string;
  lines: InvoiceLine[];
};

type Refund = {
  id: string;
  amount_minor: number;
  currency: string;
  reason: string;
  status: string;
  success: boolean;
  approvals: number;
};

const methods = ["mada", "visa", "mastercard", "bank_transfer"] as const;

const copy = {
  ar: {
    title: "الدفع والفاتورة",
    lead: "لا توجد بوابة دفع معتمدة. المحاولة تُفتح على محوّل اختباري وتبقى قيد التحقق حتى يصل إشعار من مزود. الرجوع إلى هذه الصفحة لا يُحصّل المبلغ، والشاشة لا تعرض نجاحاً من عندها.",
    login: "ادخل إلى الحساب لفتح محاولة دفع على إقامة مؤكدة.",
    account: "الحساب",
    missing: "اختر إقامة مؤكدة.",
    notConfirmed: "هذه الإقامة غير مؤكدة، ولا تُفتح لها محاولة دفع.",
    due: "مستحق الإقامة",
    method: "طريقة الدفع",
    methods: { mada: "مدى", visa: "فيزا", mastercard: "ماستركارد", bank_transfer: "تحويل بنكي" } as Record<string, string>,
    open: "فتح محاولة",
    noCard: "لا يُطلب رقم البطاقة ولا رمز الأمان هنا، ولا يُحفظان.",
    transfer: "مهلة ٢٤ ساعة وشرط ٤٨ ساعة قبل الوصول افتراض تشغيلي غير تعاقدي. لا يوجد حساب مستفيد معتمد، لذلك لا يُعرض اسم ولا رقم حساب.",
    proof: "ملاحظة إثبات التحويل",
    proofSend: "حفظ الملاحظة",
    proofHint: "حفظ الملاحظة لا يثبت وصول المال.",
    attempts: "المحاولات",
    emptyAttempts: "لا توجد محاولة بعد.",
    collected: "محصّل وفق إشعار المزود",
    notCollected: "غير محصّل",
    decisive: "جواب حاسم",
    openDecision: "قرار البوابة ما زال مفتوحاً",
    invoice: "الفاتورة",
    noInvoice: "تُصدر الفاتورة مع أول محاولة، ولا تُعد إيصالاً.",
    serial: "رقم تسلسلي",
    seller: "البائع غير معتمد",
    lines: { base: "سعر المدة", discount: "خصم", service: "رسم خدمة", tax: "ضريبة", deposit: "تأمين" } as Record<string, string>,
    notAdopted: "غير معتمد",
    notRevenue: "ليس إيراداً",
    total: "الإجمالي",
    correct: "بيان تصحيح",
    correctSend: "إصدار بيان",
    correctHint: "البيان لا يغيّر إجمالي الفاتورة الصادرة.",
    refund: "طلب استرداد",
    refundAmount: "المبلغ بالريال",
    refundReason: "السبب",
    refundSend: "إرسال الطلب",
    refundHint: "لا يُسترد مبلغ قبل التحصيل. الطلب يحتاج موافقتين، والموافقة ليست نجاحاً.",
    refunds: "طلبات الاسترداد",
    emptyRefunds: "لا توجد طلبات استرداد.",
    cancel: "إلغاء الطلب",
    successFlag: "نجاح الاسترداد",
    no: "لا",
    yes: "نعم",
    statuses: {
      under_review: "قيد المراجعة",
      approved: "موافق عليه",
      sent_to_provider: "أُرسل إلى المزود",
      successful: "ناجح",
      rejected: "مرفوض",
      failed_retryable: "فشل قابل لإعادة المحاولة",
      cancelled: "ملغى",
      pending_review: "قيد التحقق",
      mismatch_review: "قيد مراجعة فرق المبلغ",
      collected: "محصّل",
    } as Record<string, string>,
  },
  en: {
    title: "Payment and invoice",
    lead: "No payment gateway is adopted. An attempt opens on an explicit test adapter and stays pending verification until a provider notice arrives. Returning to this page does not collect money, and this screen does not invent a success.",
    login: "Sign in to open a payment attempt on a confirmed stay.",
    account: "Account",
    missing: "Choose a confirmed stay.",
    notConfirmed: "This stay is not confirmed, so no payment attempt can be opened.",
    due: "Stay amount due",
    method: "Payment method",
    methods: { mada: "mada", visa: "Visa", mastercard: "Mastercard", bank_transfer: "Bank transfer" } as Record<string, string>,
    open: "Open attempt",
    noCard: "A card number and security code are not asked for here, and they are not stored.",
    transfer: "The 24-hour window and the 48-hour lead before arrival are operational defaults, not a contract. No beneficiary account is adopted, so no name or account number is shown.",
    proof: "Transfer note",
    proofSend: "Save the note",
    proofHint: "Saving a note does not prove the money arrived.",
    attempts: "Attempts",
    emptyAttempts: "There is no attempt yet.",
    collected: "Collected from a provider notice",
    notCollected: "Not collected",
    decisive: "Decisive answer",
    openDecision: "The gateway decision is still open",
    invoice: "Invoice",
    noInvoice: "The invoice is issued with the first attempt. It is not a receipt.",
    serial: "Serial",
    seller: "Seller is not adopted",
    lines: { base: "Stay price", discount: "Discount", service: "Service fee", tax: "Tax", deposit: "Deposit" } as Record<string, string>,
    notAdopted: "Not adopted",
    notRevenue: "Not revenue",
    total: "Total",
    correct: "Correction notice",
    correctSend: "Issue notice",
    correctHint: "The notice does not change the issued invoice total.",
    refund: "Refund request",
    refundAmount: "Amount in riyals",
    refundReason: "Reason",
    refundSend: "Send request",
    refundHint: "Nothing is refunded before collection. A request needs two approvals, and approval is not success.",
    refunds: "Refund requests",
    emptyRefunds: "There are no refund requests.",
    cancel: "Cancel request",
    successFlag: "Refund success",
    no: "No",
    yes: "Yes",
    statuses: {
      under_review: "Under review",
      approved: "Approved",
      sent_to_provider: "Sent to the provider",
      successful: "Successful",
      rejected: "Rejected",
      failed_retryable: "Failed, retryable",
      cancelled: "Cancelled",
      pending_review: "Pending verification",
      mismatch_review: "Amount difference in review",
      collected: "Collected",
    } as Record<string, string>,
  },
} as const;

export function PayScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const params = useSearchParams();
  const stayID = params.get("stay") ?? "";
  const [stays, setStays] = useState<Stay[]>([]);
  const [signedOut, setSignedOut] = useState(false);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [method, setMethod] = useState<(typeof methods)[number]>("mada");
  const [attempts, setAttempts] = useState<Attempt[]>([]);
  const [invoice, setInvoice] = useState<Invoice | null>(null);
  const [refunds, setRefunds] = useState<Refund[]>([]);
  const [proof, setProof] = useState("");
  const [reason, setReason] = useState("");
  const [refundRiyals, setRefundRiyals] = useState("");
  const [notice, setNotice] = useState("");
  const base = locale === "en" ? "/en/pay" : "/pay";
  const accountHref = locale === "en" ? "/en/account" : "/account";
  const stay = stays.find((item) => item.id === stayID) ?? null;
  const path = `${base}${stayID ? `?stay=${stayID}` : ""}`;

  async function loadMoney(id: string) {
    const payments = await api<{ items: Attempt[] }>(`/api/v1/account/stays/${id}/payments`, {
      headers: { "Accept-Language": locale },
    });
    setAttempts(payments.items ?? []);
    const refundList = await api<{ items: Refund[] }>(`/api/v1/account/stays/${id}/refunds`, {
      headers: { "Accept-Language": locale },
    });
    setRefunds(refundList.items ?? []);
    if ((payments.items ?? []).some((item) => item.invoice_id)) {
      const issued = await api<Invoice>(`/api/v1/account/stays/${id}/invoice`, {
        headers: { "Accept-Language": locale },
      });
      setInvoice(issued);
    } else {
      setInvoice(null);
    }
  }

  useEffect(() => {
    let stop = false;
    api<{ items: Stay[] }>("/api/v1/account/stays", { headers: { "Accept-Language": locale } })
      .then((body) => {
        if (stop) return;
        setStays(body.items ?? []);
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

  useEffect(() => {
    if (!stay || stay.status !== "confirmed") return;
    let stop = false;
    loadMoney(stay.id).catch((caught) => {
      if (stop) return;
      setError(caught);
    });
    return () => {
      stop = true;
    };
  }, [stay, locale]);

  async function openAttempt(event: FormEvent) {
    event.preventDefault();
    if (!stay) return;
    setError(null);
    try {
      await api<Attempt>(`/api/v1/account/stays/${stay.id}/payments`, {
        method: "POST",
        headers: { "Accept-Language": locale, "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify({ method, amount_minor: stay.stay_due_minor }),
      });
      await loadMoney(stay.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function saveProof(event: FormEvent, attemptID: string) {
    event.preventDefault();
    if (!stay) return;
    setError(null);
    try {
      await api(`/api/v1/account/payments/${attemptID}/proof`, {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ note: proof }),
      });
      setProof("");
      await loadMoney(stay.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function sendRefund(event: FormEvent) {
    event.preventDefault();
    if (!stay) return;
    const riyals = Number(refundRiyals);
    if (!Number.isFinite(riyals) || riyals <= 0 || !reason.trim()) {
      setError(new Error(locale === "en" ? "Enter a positive amount and a reason." : "أدخل مبلغاً موجباً وسبباً."));
      return;
    }
    setError(null);
    try {
      await api(`/api/v1/account/stays/${stay.id}/refunds`, {
        method: "POST",
        headers: { "Accept-Language": locale, "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify({ amount_minor: Math.round(riyals * 100), reason: reason.trim() }),
      });
      setReason("");
      setRefundRiyals("");
      await loadMoney(stay.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function cancelRefund(id: string) {
    if (!stay) return;
    setError(null);
    try {
      await api(`/api/v1/account/refunds/${id}/cancel`, { method: "POST", headers: { "Accept-Language": locale }, body: "{}" });
      await loadMoney(stay.id);
    } catch (caught) {
      setError(caught);
    }
  }

  async function correct(event: FormEvent) {
    event.preventDefault();
    if (!stay || !notice.trim()) return;
    setError(null);
    try {
      await api(`/api/v1/account/stays/${stay.id}/invoice/corrections`, {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ reason: notice.trim() }),
      });
      setNotice("");
      await loadMoney(stay.id);
    } catch (caught) {
      setError(caught);
    }
  }

  const label = (item: Attempt) => (locale === "en" ? item.label_en : item.label_ar);
  const statusName = (value: string) => text.statuses[value] ?? value;

  return (
    <PublicFrame locale={locale} path={path}>
      <section className="grid gap-4">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <p className="text-sm text-ink/80">{text.lead}</p>
        <p className="text-sm">{text.noCard}</p>
        {state === "loading" ? <p className="text-sm">{locale === "en" ? "Loading…" : "جارٍ التحميل…"}</p> : null}
        {state === "error" ? <Failure error={error} /> : null}
        {signedOut ? (
          <p className="text-sm">
            {text.login}{" "}
            <a className="underline" href={accountHref}>
              {text.account}
            </a>
          </p>
        ) : null}
        {!signedOut && state === "ready" && !stayID ? (
          <div className="grid gap-2">
            <p className="text-sm">{text.missing}</p>
            {stays
              .filter((item) => item.status === "confirmed")
              .map((item) => (
                <a key={item.id} className="rounded-lg border border-line bg-card p-3 text-sm underline" href={`${base}?stay=${item.id}`}>
                  {locale === "en" ? item.title_en : item.title_ar} · {item.arrival} – {item.departure} · {money(item.stay_due_minor, locale)}
                </a>
              ))}
          </div>
        ) : null}
        {stay && stay.status !== "confirmed" ? <p className="text-sm">{text.notConfirmed}</p> : null}
        {stay && stay.status === "confirmed" ? (
          <div className="grid gap-4">
            <p className="text-sm">
              {locale === "en" ? stay.title_en : stay.title_ar} · {stay.arrival} – {stay.departure}
            </p>
            <p className="text-sm">
              {text.due}: {money(stay.stay_due_minor, locale)}
            </p>
            <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={openAttempt}>
              <p className="text-sm font-semibold">{text.method}</p>
              <div className="flex flex-wrap gap-3 text-sm">
                {methods.map((item) => (
                  <label key={item} className="flex items-center gap-2">
                    <input type="radio" name="method" checked={method === item} onChange={() => setMethod(item)} />
                    {text.methods[item]}
                  </label>
                ))}
              </div>
              {method === "bank_transfer" ? <p className="text-sm text-ink/80">{text.transfer}</p> : null}
              <Button type="submit">{text.open}</Button>
            </form>
            <section className="grid gap-2">
              <h2 className="text-lg font-semibold">{text.attempts}</h2>
              {attempts.length === 0 ? <p className="text-sm text-ink/70">{text.emptyAttempts}</p> : null}
              {attempts.map((item) => (
                <article key={item.id} className="grid gap-1 rounded-lg border border-line bg-card p-3 text-sm">
                  <p>
                    {text.methods[item.method] ?? item.method} · {item.status === "collected" || item.status === "successful" ? <span className="text-mint">{statusName(item.status)}</span> : statusName(item.status)} · {label(item)}
                  </p>
                  <p>
                    {money(item.amount_minor, locale)} · {item.provider} · {text.openDecision}
                  </p>
                  <p>
                    {item.success ? <span className="text-mint">{text.collected}</span> : text.notCollected} · {text.decisive}: {item.decisive ? text.yes : text.no}
                  </p>
                  {item.method === "bank_transfer" ? (
                    <form className="mt-2 grid gap-2" onSubmit={(event) => void saveProof(event, item.id)}>
                      <p>{text.proofHint}</p>
                      <Label>
                        {text.proof}
                        <Input value={proof} onChange={(event) => setProof(event.target.value)} />
                      </Label>
                      <Button type="submit">{text.proofSend}</Button>
                    </form>
                  ) : null}
                </article>
              ))}
            </section>
            <section className="grid gap-2 rounded-lg border border-line bg-card p-4">
              <h2 className="text-lg font-semibold">{text.invoice}</h2>
              {!invoice ? <p className="text-sm">{text.noInvoice}</p> : null}
              {invoice ? (
                <>
                  <p className="text-sm">
                    {text.serial}: {invoice.serial} · {text.seller}
                  </p>
                  <p className="text-sm">{locale === "en" ? invoice.seller_note_en : invoice.seller_note_ar}</p>
                  <ul className="grid gap-1 text-sm">
                    {invoice.lines.map((line) => (
                      <li key={line.code}>
                        {text.lines[line.code] ?? line.code}: {money(line.amount_minor, locale)}
                        {line.amount_minor === 0 ? ` · ${text.notAdopted}` : ""}
                        {!line.counts_as_stay ? ` · ${text.notRevenue}` : ""}
                      </li>
                    ))}
                  </ul>
                  <p className="text-sm">
                    {text.total}: {money(invoice.total_minor, locale)}
                  </p>
                  <form className="grid gap-2" onSubmit={(event) => void correct(event)}>
                    <p className="text-sm">{text.correctHint}</p>
                    <Label>
                      {text.correct}
                      <Input value={notice} onChange={(event) => setNotice(event.target.value)} />
                    </Label>
                    <Button type="submit">{text.correctSend}</Button>
                  </form>
                </>
              ) : null}
            </section>
            <section className="grid gap-2 rounded-lg border border-line bg-card p-4">
              <h2 className="text-lg font-semibold">{text.refund}</h2>
              <p className="text-sm text-ink/80">{text.refundHint}</p>
              <form className="grid gap-2 md:grid-cols-2" onSubmit={(event) => void sendRefund(event)}>
                <Label>
                  {text.refundAmount}
                  <Input value={refundRiyals} inputMode="decimal" onChange={(event) => setRefundRiyals(event.target.value)} />
                </Label>
                <Label>
                  {text.refundReason}
                  <Input value={reason} onChange={(event) => setReason(event.target.value)} />
                </Label>
                <div className="md:col-span-2">
                  <Button type="submit">{text.refundSend}</Button>
                </div>
              </form>
              <h3 className="text-sm font-semibold">{text.refunds}</h3>
              {refunds.length === 0 ? <p className="text-sm text-ink/70">{text.emptyRefunds}</p> : null}
              {refunds.map((item) => (
                <article key={item.id} className="grid gap-1 text-sm">
                  <p>
                    {money(item.amount_minor, locale)} · {item.status === "successful" || item.status === "collected" ? <span className="text-mint">{statusName(item.status)}</span> : statusName(item.status)} · {text.successFlag}: {item.success ? <span className="text-mint">{text.yes}</span> : text.no}
                  </p>
                  <p>{item.reason}</p>
                  {item.status === "under_review" ? (
                    <Button type="button" onClick={() => void cancelRefund(item.id)}>
                      {text.cancel}
                    </Button>
                  ) : null}
                </article>
              ))}
            </section>
            {error ? <Failure error={error} /> : null}
          </div>
        ) : null}
      </section>
    </PublicFrame>
  );
}
