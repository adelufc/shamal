"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { ApiFailure, SessionUser } from "@/lib/types";
import { verificationDigits } from "@/lib/verification-code";

export class ApiError extends Error {
  status: number;
  body: ApiFailure;

  constructor(status: number, body: ApiFailure) {
    super(body.message || "تعذر إكمال الطلب.");
    this.status = status;
    this.body = body;
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("X-Shamal-Request", "1");
  if (init?.body && !headers.has("Content-Type") && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(path, { ...init, headers, credentials: "include" });
  const body = (await response.json().catch(() => ({}))) as T & ApiFailure;
  if (!response.ok) {
    throw new ApiError(response.status, body);
  }
  return body;
}

const SessionContext = createContext<SessionUser | null>(null);

export function useSession() {
  return useContext(SessionContext);
}

export function AdminShell({ children }: { children: ReactNode }) {
  const path = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<SessionUser | null>(null);
  const [state, setState] = useState<"loading" | "guest" | "ready" | "error">("loading");
  const [code, setCode] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let stop = false;
    api<SessionUser>("/api/v1/me")
      .then((user) => {
        if (stop) return;
        setMe(user);
        setState("ready");
      })
      .catch((error: unknown) => {
        if (stop) return;
        setMe(null);
        setState(error instanceof ApiError ? "guest" : "error");
      });
    return () => {
      stop = true;
    };
  }, [path]);

  useEffect(() => {
    if (state === "guest" && path !== "/admin/login") router.replace("/admin/login");
    if (state === "ready" && path === "/admin/login") router.replace("/admin/neighborhoods");
  }, [state, path, router]);

  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" });
    setMe(null);
    setState("guest");
    router.replace("/admin/login");
  }

  async function stepUp() {
    setNotice("");
    try {
      const { digits, letters } = verificationDigits(code);
      if (letters || digits.length !== 6) {
        setNotice("أدخل الرمز المكوّن من 6 أرقام لهذه اللحظة. القيمة الطويلة في الدليل ليست رمز الدخول.");
        return;
      }
      await api("/api/v1/auth/step-up", { method: "POST", body: JSON.stringify({ code: digits }) });
      const user = await api<SessionUser>("/api/v1/me");
      setMe(user);
      setCode("");
      setNotice("التحقق الإضافي ساري لخمس دقائق.");
    } catch (error) {
      setNotice(error instanceof ApiError ? error.message : "تعذر التحقق.");
    }
  }

  if (path === "/admin/login") return <>{children}</>;
  if (state === "loading" || state === "guest") {
    return <p className="p-6 text-sm">{state === "loading" ? "جارٍ التحقق من الجلسة…" : "جارٍ التحويل إلى الدخول…"}</p>;
  }
  if (state === "error" || !me) {
    return (
      <p className="p-6 text-sm">
        تعذر التحقق من الجلسة.{" "}
        <button className="underline" onClick={() => location.reload()}>
          إعادة المحاولة
        </button>
      </p>
    );
  }

  const needsStep =
    (me.actions.geography ?? []).includes("publish") ||
    (me.actions.catalog ?? []).includes("approve") ||
    (me.actions.visits ?? []).includes("approve") ||
    (me.actions.deals ?? []).includes("approve") ||
    (me.actions.finance ?? []).includes("approve") ||
    (me.actions.disputes ?? []).includes("approve");

  return (
    <SessionContext.Provider value={me}>
      <div className="min-h-screen">
        <header className="border-b border-line bg-card">
          <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-4 py-4">
            <div>
              <a href="/" className="text-lg font-semibold">
                شمال العقاري
              </a>
              <p className="text-sm text-ink/70">{me.display_name}</p>
            </div>
            <nav className="flex flex-wrap items-center gap-4 text-sm">
              <a href="/admin/neighborhoods">الأحياء</a>
              <a href="/admin/organization">الهيكل</a>
              <a href="/admin/permissions">الصلاحيات</a>
              <a href="/admin/catalog">العقارات</a>
              <a href="/admin/requests">الطلبات</a>
              <a href="/admin/visits">الزيارات</a>
              <a href="/admin/deals">الصفقات</a>
              <a href="/admin/refunds">الاسترداد</a>
              <a href="/admin/messages">الرسائل</a>
              <a href="/admin/disputes">النزاعات</a>
              <a href="/admin/reports">التقارير</a>
              <button onClick={logout} className="underline">
                خروج
              </button>
            </nav>
          </div>
        </header>
        {needsStep ? (
          <div className="border-b border-line bg-olive-soft">
            <form
              className="mx-auto flex max-w-5xl flex-wrap items-center gap-2 px-4 py-3 text-sm"
              onSubmit={(event) => {
                event.preventDefault();
                void stepUp();
              }}
            >
              <span>{me.step_up ? "التحقق الإضافي ساري." : "النشر والاعتماد يحتاجان رمزاً إضافياً."}</span>
              <input
                value={code}
                onChange={(event) => {
                  const next = verificationDigits(event.target.value);
                  setCode(next.letters ? "" : next.digits);
                }}
                inputMode="numeric"
                autoComplete="off"
                className="w-32 rounded-md border border-line bg-card px-2 py-1"
                aria-label="رمز من 6 أرقام"
              />
              <button className="rounded-md bg-olive px-3 py-1 text-paper">تأكيد</button>
              {notice ? <span>{notice}</span> : null}
            </form>
          </div>
        ) : null}
        <main className="mx-auto grid max-w-5xl gap-6 px-4 py-8">{children}</main>
      </div>
    </SessionContext.Provider>
  );
}

export function Failure({ error }: { error: unknown }) {
  if (error instanceof ApiError) {
    const fields = Object.values(error.body.fields ?? {});
    return (
      <div className="rounded-md border border-line bg-card p-3 text-sm text-warn">
        <p>{error.message}</p>
        {fields.length > 0 ? (
          <ul className="mt-2 list-disc pr-5">
            {fields.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        ) : null}
      </div>
    );
  }
  if (error instanceof Error) return <p className="text-sm text-warn">{error.message}</p>;
  return <p className="text-sm text-warn">تعذر إكمال الطلب.</p>;
}
