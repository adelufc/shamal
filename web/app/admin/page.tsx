"use client";

import { useEffect, useState } from "react";
import { api, useSession } from "@/components/admin-shell";

const tasks = [
  { href: "/admin/requests?unassigned=1", label: "طلبات غير مسندة", domain: "clients", action: "view", key: "requests" },
  { href: "/admin/visits?view=today", label: "زيارات اليوم", domain: "visits", action: "view", key: "visits" },
  { href: "/admin/listings?pending=1", label: "تغييرات معلّقة", domain: "catalog", action: "view", key: "pending" },
  { href: "/admin/messages?waiting=1", label: "رسائل بانتظار الرد", domain: "messages", action: "view", key: "messages" },
] as const;

export default function AdminHome() {
  const me = useSession();
  const [counts, setCounts] = useState<Record<string, number | null>>({});
  const allowed = tasks.filter((item) => (me?.actions[item.domain] ?? []).includes(item.action));

  useEffect(() => {
    if (!me) return;
    let stop = false;
    async function read() {
      const next: Record<string, number | null> = {};
      if ((me.actions.clients ?? []).includes("view")) {
        try {
          const body = await api<{ total: number }>("/api/v1/clients/requests?unassigned=1&page_size=1");
          next.requests = body.total;
        } catch {
          next.requests = null;
        }
      }
      if ((me.actions.visits ?? []).includes("view")) {
        try {
          const body = await api<{ total: number }>("/api/v1/visits?view=today&page_size=1");
          next.visits = body.total;
        } catch {
          next.visits = null;
        }
      }
      if ((me.actions.catalog ?? []).includes("view")) {
        try {
          const body = await api<{ items: { pending_changes: unknown }[] }>("/api/v1/catalog/listings");
          next.pending = body.items.filter((item) => item.pending_changes).length;
        } catch {
          next.pending = null;
        }
      }
      if ((me.actions.messages ?? []).includes("view")) {
        try {
          const body = await api<{ items: { first_human_reply_at?: string }[] }>("/api/v1/messages");
          next.messages = (body.items ?? []).filter((item) => !item.first_human_reply_at).length;
        } catch {
          next.messages = null;
        }
      }
      if (!stop) setCounts(next);
    }
    void read();
    return () => {
      stop = true;
    };
  }, [me]);

  return (
    <section className="grid gap-4">
      <h1 className="text-3xl font-semibold">ملخص العمل</h1>
      <p className="max-w-2xl text-sm text-muted">{me?.display_name} · الأرقام للحسابات التي يستطيع هذا الحساب فتحها.</p>
      {allowed.length === 0 ? <p className="text-sm">لا توجد مهمة ظاهرة لهذا الحساب.</p> : null}
      <ul className="grid gap-3 sm:grid-cols-2">
        {allowed.map((item) => (
          <li key={item.href}>
            <a className="block rounded-xl border border-line bg-card p-4" href={item.href}>
              <span className="block text-2xl font-semibold">{counts[item.key] == null ? "…" : counts[item.key]}</span>
              <span className="font-semibold">{item.label}</span>
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}
