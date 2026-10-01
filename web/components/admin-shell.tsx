"use client";

import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
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
  const [checkedPath, setCheckedPath] = useState<string | null>(null);
  const [code, setCode] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let stop = false;
    setCheckedPath(null);
    setState("loading");
    api<SessionUser>("/api/v1/me")
      .then((user) => {
        if (stop) return;
        setMe(user);
        setState("ready");
        setCheckedPath(path);
      })
      .catch((error: unknown) => {
        if (stop) return;
        setMe(null);
        setState(error instanceof ApiError ? "guest" : "error");
        setCheckedPath(path);
      });
    return () => {
      stop = true;
    };
  }, [path]);

  useEffect(() => {
    if (checkedPath !== path) return;
    if (state === "guest" && path !== "/admin/login") router.replace("/admin/login");
    if (state === "ready" && path === "/admin/login") router.replace("/admin/neighborhoods");
  }, [state, path, checkedPath, router]);

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
        setNotice("أدخل رمز التحقق المكون من 6 أرقام من تطبيق المصادقة");
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

  const groups = [
    {
      title: "المحتوى والعقارات",
      links: [
        ["/admin", "ملخص العمل"],
        ["/admin/neighborhoods", "الأحياء"],
        ["/admin/catalog", "العقارات"],
        ["/admin/content", "المحتوى"],
        ["/admin/listings", "تعديل الإعلان"],
      ],
    },
    {
      title: "العملاء والعمليات",
      links: [
        ["/admin/requests", "الطلبات"],
        ["/admin/visits", "الزيارات"],
        ["/admin/deals", "الصفقات"],
        ["/admin/messages", "الرسائل"],
        ["/admin/disputes", "النزاعات"],
      ],
    },
    {
      title: "المال والتقارير",
      links: [
        ["/admin/refunds", "الاسترداد"],
        ["/admin/reports", "التقارير"],
        ["/owner", "أداء الملاك"],
      ],
    },
    {
      title: "إدارة النظام",
      links: [
        ["/admin/organization", "الهيكل"],
        ["/admin/permissions", "الصلاحيات"],
        ["/admin/notifications", "الإشعارات"],
      ],
    },
  ] as const;

  return (
    <SessionContext.Provider value={me}>
      <AdminFrame path={path} name={me.display_name} groups={groups} onLogout={() => void logout()}>
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
              {me.step_up ? null : (
                <>
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
                </>
              )}
              {notice ? <span>{notice}</span> : null}
            </form>
          </div>
        ) : null}
        <main className="min-w-0 flex-1 px-4 py-6">{children}</main>
      </AdminFrame>
    </SessionContext.Provider>
  );
}

function AdminFrame({
  path,
  name,
  groups,
  onLogout,
  children,
}: {
  path: string;
  name: string;
  groups: readonly { title: string; links: readonly (readonly [string, string])[] }[];
  onLogout: () => void;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLElement>(null);
  function toggle(next: boolean) {
    setOpen(next);
    window.setTimeout(() => {
      if (next) menu.current?.querySelector("a")?.focus();
      else button.current?.focus();
    }, 0);
  }
  const current = groups.flatMap((group) => [...group.links]).find(([href]) => path === href || (href !== "/admin" && path.startsWith(href)));
  return (
    <div className="min-h-screen overflow-x-hidden lg:grid lg:grid-cols-[16rem_1fr]">
      <p className="bg-[#FFF6DE] px-4 py-2 text-center text-sm text-[#7B560D] lg:col-span-2">بيئة اختبار</p>
      <aside className="border-b border-line bg-card lg:border-b-0 lg:border-e">
        <div className="flex items-center justify-between gap-3 px-4 py-3">
          <div>
            <a href="/admin" className="text-lg font-semibold">
              شمال العقاري
            </a>
            <p className="text-sm text-muted">{name}</p>
            <p className="text-sm">{current?.[1] ?? "لوحة العمل"}</p>
          </div>
          <button
            ref={button}
            type="button"
            className="inline-flex min-h-11 items-center rounded-lg border border-[#7B877D] px-3 lg:hidden"
            aria-expanded={open}
            onClick={() => toggle(!open)}
          >
            القائمة
          </button>
        </div>
        <nav ref={menu} className={`${open ? "grid" : "hidden"} gap-4 px-4 pb-4 lg:grid`}>
          {groups.map((group) => (
            <div key={group.title} className="grid gap-1">
              <p className="text-sm font-semibold">{group.title}</p>
              {group.links.map(([href, label]) => {
                const active = path === href || (href !== "/admin" && path.startsWith(href));
                return (
                  <a key={href} href={href} className={`inline-flex min-h-11 items-center rounded-lg px-2 ${active ? "bg-olive-soft font-semibold" : ""}`} aria-current={active ? "page" : undefined}>
                    {label}
                  </a>
                );
              })}
            </div>
          ))}
          <button type="button" className="inline-flex min-h-11 items-center px-2 text-start underline" onClick={onLogout}>
            خروج
          </button>
        </nav>
      </aside>
      <div className="min-w-0">{children}</div>
    </div>
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
