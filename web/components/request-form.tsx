"use client";

import { FormEvent, useEffect, useState } from "react";
import { Failure, api } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label, TextArea } from "@/components/ui/input";
import type { HomePayload, Locale } from "@/lib/types";

const copy = {
  ar: {
    title: "طلب عقار",
    lead: "صف نوع المعاملة والحي والموعد والمواصفات. الميزانية اختيارية وتُكتب بالريال كما تدخلها. لا تُضاف عمولة أو رسم.",
    transaction: "نوع المعاملة",
    sale: "بيع",
    annual: "إيجار سنوي",
    monthly: "إيجار شهري",
    daily: "إيجار يومي",
    budget: "الميزانية بالريال، إن وُجدت",
    neighborhood: "الحي",
    choose: "اختر حياً منشوراً",
    timing: "الموعد المناسب",
    specsAR: "المواصفات",
    specsEN: "Specifications in English",
    contact: "وسيلة التواصل",
    phone: "الهاتف",
    email: "البريد",
    either: "أي منهما",
    name: "الاسم",
    emailField: "البريد",
    phoneField: "الهاتف",
    send: "إرسال الطلب",
    sending: "جارٍ الإرسال…",
    sent: "وصل الطلب وهو في صف الإدارة إلى أن يُسند لموظف.",
    empty: "لا توجد أحياء منشورة يمكن الطلب منها.",
    budgetInvalid: "إذا أدخلت ميزانية، اكتب مبلغاً أكبر من صفر بالريال.",
    missing: "الاسم والبريد والحي ووسيلة التواصل مطلوبة.",
  },
  en: {
    title: "Property request",
    lead: "Describe the transaction, neighborhood, timing, and specifications. A budget is optional and is entered in SAR. No commission or fee is added.",
    transaction: "Transaction",
    sale: "Sale",
    annual: "Annual rent",
    monthly: "Monthly rent",
    daily: "Daily rent",
    budget: "Budget in SAR, if you have one",
    neighborhood: "Neighborhood",
    choose: "Choose a published neighborhood",
    timing: "Preferred timing",
    specsAR: "المواصفات",
    specsEN: "Specifications in English",
    contact: "Contact method",
    phone: "Phone",
    email: "Email",
    either: "Either",
    name: "Name",
    emailField: "Email",
    phoneField: "Phone",
    send: "Send the request",
    sending: "Sending…",
    sent: "The request is in the admin queue until a staff member is assigned.",
    empty: "There are no published neighborhoods to request from.",
    budgetInvalid: "If you enter a budget, use an amount greater than zero in SAR.",
    missing: "Name, email, neighborhood, and a contact method are required.",
  },
} as const;

export function RequestForm({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const [places, setPlaces] = useState<HomePayload["neighborhoods"]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [pending, setPending] = useState(false);
  const [done, setDone] = useState("");
  const [transaction, setTransaction] = useState("sale");
  const [budget, setBudget] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [timing, setTiming] = useState("");
  const [specsAR, setSpecsAR] = useState("");
  const [specsEN, setSpecsEN] = useState("");
  const [contact, setContact] = useState("either");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");

  useEffect(() => {
    let stop = false;
    api<HomePayload>(`/api/v1/public/home?locale=${locale}`)
      .then((home) => {
        if (stop) return;
        setPlaces(home.neighborhoods);
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
  }, [locale]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setDone("");
    if (!name.trim() || !email.trim() || !neighborhood || !contact) {
      setError(new Error(text.missing));
      return;
    }
    let budgetMinor: number | undefined;
    if (budget.trim() !== "") {
      const riyals = Number(budget);
      if (!Number.isFinite(riyals) || riyals <= 0) {
        setError(new Error(text.budgetInvalid));
        return;
      }
      budgetMinor = Math.round(riyals * 100);
    }
    setPending(true);
    try {
      const created = await api<{ id: string }>("/api/v1/public/requests", {
        method: "POST",
        body: JSON.stringify({
          display_name: name,
          email,
          phone,
          transaction_type: transaction,
          neighborhood_id: neighborhood,
          timing_note: timing,
          specs_ar: locale === "en" ? "" : specsAR,
          specs_en: locale === "en" ? specsEN : "",
          contact_method: contact,
          currency: "SAR",
          ...(budgetMinor === undefined ? {} : { budget_minor: budgetMinor }),
        }),
      });
      setDone(`${text.sent} ${created.id}`);
      setBudget("");
      setTiming("");
      setSpecsAR("");
      setSpecsEN("");
    } catch (caught) {
      setError(caught);
    } finally {
      setPending(false);
    }
  }

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">{text.title}</h1>
      <p className="text-sm text-ink/80">{text.lead}</p>
      {state === "loading" ? <p className="text-sm">{locale === "en" ? "Loading…" : "جارٍ التحميل…"}</p> : null}
      {state === "error" ? <Failure error={error} /> : null}
      {state === "ready" && places.length === 0 ? <p className="text-sm">{text.empty}</p> : null}
      {state === "ready" && places.length > 0 ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={(event) => void submit(event)}>
          <Label>
            {text.transaction}
            <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={transaction} onChange={(event) => setTransaction(event.target.value)}>
              <option value="sale">{text.sale}</option>
              <option value="annual">{text.annual}</option>
              <option value="monthly">{text.monthly}</option>
              <option value="daily">{text.daily}</option>
            </select>
          </Label>
          <Label>
            {text.budget}
            <Input value={budget} onChange={(event) => setBudget(event.target.value)} inputMode="decimal" placeholder="" />
          </Label>
          <Label>
            {text.neighborhood}
            <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)}>
              <option value="">{text.choose}</option>
              {places.map((place) => (
                <option key={place.id} value={place.id}>
                  {place.name}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            {text.timing}
            <Input value={timing} onChange={(event) => setTiming(event.target.value)} />
          </Label>
          <Label>
            {locale === "en" ? text.specsEN : text.specsAR}
            <TextArea
              dir={locale === "en" ? "ltr" : "rtl"}
              value={locale === "en" ? specsEN : specsAR}
              onChange={(event) => (locale === "en" ? setSpecsEN(event.target.value) : setSpecsAR(event.target.value))}
              required
            />
          </Label>
          <Label>
            {text.contact}
            <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={contact} onChange={(event) => setContact(event.target.value)}>
              <option value="phone">{text.phone}</option>
              <option value="email">{text.email}</option>
              <option value="either">{text.either}</option>
            </select>
          </Label>
          <Label>
            {text.name}
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </Label>
          <Label>
            {text.emailField}
            <Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} />
          </Label>
          <Label>
            {text.phoneField}
            <Input value={phone} onChange={(event) => setPhone(event.target.value)} />
          </Label>
          {error ? <Failure error={error} /> : null}
          {done ? <p className="text-sm">{done}</p> : null}
          <Button type="submit" disabled={pending}>
            {pending ? text.sending : text.send}
          </Button>
        </form>
      ) : null}
    </section>
  );
}
