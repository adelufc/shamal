"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label, TextArea } from "@/components/ui/input";
import { money } from "@/lib/chrome";
import type { HomePayload, Locale, PublicListing } from "@/lib/types";

type AccountRequest = {
  id: string;
  stage: string;
  client_stage: string;
  transaction_type: string;
  specs_ar: string;
  specs_en: string;
  timing_note: string;
  assignee_id: string;
  budget_minor?: number;
  currency: string;
};

type AccountVisit = {
  id: string;
  status: string;
  starts_at: string;
  ends_at: string;
  instructions_ar: string;
  instructions_en: string;
  version: number;
  result?: string;
  result_note?: string;
};

type AccountDeal = {
  id: string;
  status: string;
  amount_minor?: number;
  currency: string;
  loss_reason?: string;
};

type Account = {
  display_name: string;
  email: string;
  requests: AccountRequest[];
  visits: AccountVisit[];
  deals: AccountDeal[];
};

const copy = {
  ar: {
    title: "حسابي",
    lead: "حساب العميل لا يطلب رمز تحقق. حساب الفريق يطلبه من شاشة المكتب. لا توجد قناة إرسال معتمدة لاستعادة كلمة المرور، ولن يُكشف إن كان البريد مسجلاً.",
    showPassword: "إظهار كلمة المرور",
    recovery: "استعادة كلمة المرور غير مفعّلة. لا تُرسل رسالة، ولا يُؤكد وجود البريد.",
    register: "حساب جديد",
    login: "دخول",
    name: "الاسم",
    email: "البريد",
    password: "كلمة المرور",
    phone: "الهاتف",
    create: "إنشاء الحساب",
    enter: "دخول",
    requests: "طلباتي",
    visits: "زياراتي",
    deals: "صفقاتي",
    emptyRequests: "لا توجد طلبات بعد.",
    emptyVisits: "لا توجد زيارات بعد.",
    emptyDeals: "لا توجد صفقات بعد.",
    queue: "في صف الإدارة",
    assigned: "مُسند",
    book: "طلب زيارة لإعلان منشور",
    listing: "الإعلان",
    choose: "اختر إعلاناً",
    when: "بداية الموعد",
    instructionsAR: "تعليمات بالعربية",
    instructionsEN: "Instructions in English",
    bookSend: "طلب الزيارة",
    cancel: "إلغاء الزيارة",
    reason: "سبب الإلغاء",
    logout: "خروج",
    favorites: "المفضلة",
    owner: "أداء الملاك",
    noListings: "لا توجد إعلانات منشورة لطلب زيارة.",
    loading: "جارٍ التحميل…",
  },
  en: {
    title: "My account",
    lead: "A client account does not ask for a verification code. A staff account does, on the work desk. No recovery channel is adopted, and the page does not say whether an email exists.",
    showPassword: "Show password",
    recovery: "Password recovery is not enabled. No message is sent, and the email is not confirmed as present or absent.",
    register: "New account",
    login: "Sign in",
    name: "Name",
    email: "Email",
    password: "Password",
    phone: "Phone",
    create: "Create the account",
    enter: "Sign in",
    requests: "My requests",
    visits: "My visits",
    deals: "My deals",
    emptyRequests: "There are no requests yet.",
    emptyVisits: "There are no visits yet.",
    emptyDeals: "There are no deals yet.",
    queue: "In the admin queue",
    assigned: "Assigned",
    book: "Ask for a visit to a published listing",
    listing: "Listing",
    choose: "Choose a listing",
    when: "Start time",
    instructionsAR: "تعليمات بالعربية",
    instructionsEN: "Instructions in English",
    bookSend: "Request the visit",
    cancel: "Cancel the visit",
    reason: "Cancellation reason",
    logout: "Sign out",
    favorites: "Favorites",
    owner: "Owner performance",
    noListings: "There are no published listings to visit.",
    loading: "Loading…",
  },
} as const;

const journey = {
  ar: {
    received: "طلب مستلم",
    follow_up: "متابعة",
    visit: "زيارة",
    negotiation: "تفاوض",
    documents: "مستندات",
    agreement: "اتفاق",
    completed: "مكتمل",
    closed_without_agreement: "مغلق دون اتفاق",
  },
  en: {
    received: "Request received",
    follow_up: "Follow-up",
    visit: "Visit",
    negotiation: "Negotiation",
    documents: "Documents",
    agreement: "Agreement",
    completed: "Completed",
    closed_without_agreement: "Closed without an agreement",
  },
} as const;

const visitStatus = {
  ar: {
    new: "جديد",
    awaiting_broker: "بانتظار الوسيط",
    awaiting_owner: "بانتظار المالك",
    confirmed: "مؤكد",
    completed: "مكتمل",
    cancelled: "ملغى",
    no_show: "لم يحضر",
  },
  en: {
    new: "New",
    awaiting_broker: "Awaiting the broker",
    awaiting_owner: "Awaiting the owner",
    confirmed: "Confirmed",
    completed: "Completed",
    cancelled: "Cancelled",
    no_show: "No show",
  },
} as const;

function dealLabel(status: string, locale: Locale) {
  const labels = journey[locale];
  if (status === "closed_success") return labels.completed;
  if (status === "closed_unsuccessful" || status === "cancelled") return labels.closed_without_agreement;
  if (status === "awaiting_documents") return labels.documents;
  if (status === "preliminary_agreement" || status === "awaiting_external") return labels.agreement;
  if (status === "negotiation") return labels.negotiation;
  return labels.follow_up;
}

function listingTitle(item: PublicListing, locale: Locale) {
  const match = item.texts?.find((text) => text.locale === locale);
  if (match?.title) return match.title;
  const other = item.texts?.find((text) => text.title);
  return other ? `${other.title} (${other.locale})` : item.id;
}

export function AccountDesk({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const [account, setAccount] = useState<Account | null>(null);
  const [listings, setListings] = useState<PublicListing[]>([]);
  const [state, setState] = useState<"loading" | "guest" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [tab, setTab] = useState<"login" | "register">("login");
  const [showPassword, setShowPassword] = useState(false);
  const [phone, setPhone] = useState("");
  const [listing, setListing] = useState("");
  const [when, setWhen] = useState("");
  const [instructionsAR, setInstructionsAR] = useState("");
  const [instructionsEN, setInstructionsEN] = useState("");
  const [reasons, setReasons] = useState<Record<string, string>>({});

  async function load() {
    const [me, home] = await Promise.all([
      api<Account>("/api/v1/account"),
      api<HomePayload>(`/api/v1/public/home?locale=${locale}`),
    ]);
    setAccount(me);
    setListings(home.listings);
    setState("ready");
  }

  useEffect(() => {
    let stop = false;
    load().catch((caught) => {
      if (stop) return;
      if (caught instanceof ApiError && (caught.status === 401 || caught.status === 403)) {
        setAccount(null);
        setState("guest");
        return;
      }
      setError(caught);
      setState("error");
    });
    return () => {
      stop = true;
    };
    // The first load distinguishes a guest from a service failure.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [locale]);

  async function refresh() {
    try {
      await load();
      setError(null);
    } catch (caught) {
      setError(caught);
      setState("error");
    }
  }

  async function register(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify({ display_name: name, email, password, phone }),
      });
      await api("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password, context: "client" }),
      });
      await refresh();
    } catch (caught) {
      setError(caught);
    }
  }

  async function login(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password, context: "client" }),
      });
      await refresh();
    } catch (caught) {
      setError(caught);
    }
  }

  async function book(event: FormEvent) {
    event.preventDefault();
    setError(null);
    if (!listing || !when) return;
    try {
      await api("/api/v1/account/visits", {
        method: "POST",
        body: JSON.stringify({
          listing_id: listing,
          starts_at: new Date(when).toISOString(),
          instructions_ar: instructionsAR,
          instructions_en: instructionsEN,
        }),
      });
      setWhen("");
      setInstructionsAR("");
      setInstructionsEN("");
      await refresh();
    } catch (caught) {
      setError(caught);
    }
  }

  async function cancel(visit: AccountVisit) {
    setError(null);
    try {
      await api(`/api/v1/account/visits/${visit.id}/cancel`, {
        method: "POST",
        body: JSON.stringify({ version: visit.version, reason: reasons[visit.id] ?? "" }),
      });
      await refresh();
    } catch (caught) {
      setError(caught);
    }
  }

  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" });
    setAccount(null);
    setState("guest");
  }

  if (state === "loading") return <p className="text-sm">{text.loading}</p>;
  if (state === "error" && !account) return <Failure error={error} />;

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">{text.title}</h1>
      {state === "guest" ? (
        <>
          <p className="text-sm text-muted">{text.lead}</p>
          <div className="flex gap-2">
            <Button type="button" variant={tab === "login" ? "primary" : "ghost"} onClick={() => setTab("login")}>
              {text.login}
            </Button>
            <Button type="button" variant={tab === "register" ? "primary" : "ghost"} onClick={() => setTab("register")}>
              {text.register}
            </Button>
          </div>
          <p className="text-sm text-muted">{text.recovery}</p>
          <div className="grid gap-4">
            {tab === "register" ? (
            <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={(event) => void register(event)}>
              <h2 className="font-semibold">{text.register}</h2>
              <Label>
                {text.name}
                <Input value={name} onChange={(event) => setName(event.target.value)} />
              </Label>
              <Label>
                {text.email}
                <Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} />
              </Label>
              <Label>
                {text.password}
                <Input type={showPassword ? "text" : "password"} value={password} onChange={(event) => setPassword(event.target.value)} />
              </Label>
              <label className="flex min-h-11 items-center gap-2 text-sm">
                <input type="checkbox" checked={showPassword} onChange={(event) => setShowPassword(event.target.checked)} />
                {text.showPassword}
              </label>
              <Label>
                {text.phone}
                <Input value={phone} onChange={(event) => setPhone(event.target.value)} />
              </Label>
              <Button type="submit">{text.create}</Button>
            </form>
            ) : (
            <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={(event) => void login(event)}>
              <h2 className="font-semibold">{text.login}</h2>
              <Label>
                {text.email}
                <Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} />
              </Label>
              <Label>
                {text.password}
                <Input type={showPassword ? "text" : "password"} value={password} onChange={(event) => setPassword(event.target.value)} />
              </Label>
              <Button type="submit">{text.enter}</Button>
            </form>
            )}
          </div>
          {error ? <Failure error={error} /> : null}
        </>
      ) : null}
      {account ? (
        <>
          <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
            <p>
              {account.display_name} · {account.email}
            </p>
            <span className="flex gap-3">
              <a className="underline" href={locale === "en" ? "/en/favorites" : "/favorites"}>
                {text.favorites}
              </a>
              <a className="underline" href={locale === "en" ? "/en/owner" : "/owner"}>
                {text.owner}
              </a>
              <button className="underline" onClick={() => void logout()}>
                {text.logout}
              </button>
            </span>
          </div>
          {error ? <Failure error={error} /> : null}
          <section className="grid gap-2">
            <h2 className="text-lg font-semibold">{text.requests}</h2>
            {account.requests.length === 0 ? <p className="text-sm">{text.emptyRequests}</p> : null}
            {account.requests.map((item) => (
              <article key={item.id} className="grid gap-1 rounded-lg border border-line bg-card p-4 text-sm">
                <p className="font-semibold">{journey[locale][item.client_stage as keyof (typeof journey)["ar"]] ?? item.client_stage}</p>
                <p>{item.assignee_id ? text.assigned : text.queue}</p>
                {item.timing_note ? <p>{item.timing_note}</p> : null}
                {item.budget_minor ? <p>{money(item.budget_minor, locale)}</p> : null}
                {item.specs_ar ? <p>{item.specs_ar}</p> : null}
                {item.specs_en ? <p dir="ltr">{item.specs_en}</p> : null}
              </article>
            ))}
          </section>
          <section className="grid gap-2">
            <h2 className="text-lg font-semibold">{text.visits}</h2>
            <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={(event) => void book(event)}>
              <h3 className="font-semibold">{text.book}</h3>
              {listings.length === 0 ? <p className="text-sm">{text.noListings}</p> : null}
              <Label>
                {text.listing}
                <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={listing} onChange={(event) => setListing(event.target.value)}>
                  <option value="">{text.choose}</option>
                  {listings.map((item) => (
                    <option key={item.id} value={item.id}>
                      {listingTitle(item, locale)}
                    </option>
                  ))}
                </select>
              </Label>
              <Label>
                {text.when}
                <Input type="datetime-local" value={when} onChange={(event) => setWhen(event.target.value)} />
              </Label>
              <Label>
                {text.instructionsAR}
                <TextArea value={instructionsAR} onChange={(event) => setInstructionsAR(event.target.value)} />
              </Label>
              <Label>
                {text.instructionsEN}
                <TextArea dir="ltr" value={instructionsEN} onChange={(event) => setInstructionsEN(event.target.value)} />
              </Label>
              <Button type="submit" disabled={listings.length === 0}>
                {text.bookSend}
              </Button>
            </form>
            {account.visits.length === 0 ? <p className="text-sm">{text.emptyVisits}</p> : null}
            {account.visits.map((item) => (
              <article key={item.id} className="grid gap-2 rounded-lg border border-line bg-card p-4 text-sm">
                <p className="font-semibold">{visitStatus[locale][item.status as keyof (typeof visitStatus)["ar"]] ?? item.status}</p>
                <p>
                  {new Date(item.starts_at).toLocaleString(locale === "en" ? "en-GB" : "ar-SA", { timeZone: "Asia/Riyadh" })}
                </p>
                {item.instructions_ar ? <p>{item.instructions_ar}</p> : null}
                {item.instructions_en ? <p dir="ltr">{item.instructions_en}</p> : null}
                {item.status !== "completed" && item.status !== "cancelled" && item.status !== "no_show" ? (
                  <div className="flex flex-wrap items-center gap-2">
                    <Input
                      value={reasons[item.id] ?? ""}
                      onChange={(event) => setReasons({ ...reasons, [item.id]: event.target.value })}
                      placeholder={text.reason}
                    />
                    <Button type="button" variant="ghost" onClick={() => void cancel(item)}>
                      {text.cancel}
                    </Button>
                  </div>
                ) : null}
              </article>
            ))}
          </section>
          <section className="grid gap-2">
            <h2 className="text-lg font-semibold">{text.deals}</h2>
            {account.deals.length === 0 ? <p className="text-sm">{text.emptyDeals}</p> : null}
            {account.deals.map((item) => (
              <article key={item.id} className="grid gap-1 rounded-lg border border-line bg-card p-4 text-sm">
                <p className="font-semibold">{dealLabel(item.status, locale)}</p>
                {item.amount_minor ? <p>{money(item.amount_minor, locale)}</p> : null}
              </article>
            ))}
          </section>
        </>
      ) : null}
    </section>
  );
}
