"use client";

import { useSession } from "@/components/admin-shell";

const tasks = [
  { href: "/admin/content", label: "مسودات المحتوى", domain: "catalog", action: "view" },
  { href: "/admin/listings", label: "تغييرات الإعلانات المعلّقة", domain: "catalog", action: "view" },
  { href: "/admin/requests?unassigned=1", label: "طلبات غير مسندة", domain: "clients", action: "view" },
  { href: "/admin/visits?view=today", label: "زيارات اليوم", domain: "visits", action: "view" },
  { href: "/admin/messages", label: "رسائل بانتظار الرد", domain: "messages", action: "view" },
  { href: "/admin/reports", label: "التقارير", domain: "reports", action: "view" },
] as const;

export default function AdminHome() {
  const me = useSession();
  const allowed = tasks.filter((item) => (me?.actions[item.domain] ?? []).includes(item.action));
  return (
    <section className="grid gap-4">
      <h1 className="text-3xl font-semibold">ملخص العمل</h1>
      <p className="max-w-2xl text-sm text-muted">{me?.display_name} · تظهر هنا المهام التي يستطيع هذا الحساب فتحها.</p>
      {allowed.length === 0 ? <p className="text-sm">لا توجد مهمة ظاهرة لهذا الحساب.</p> : null}
      <ul className="grid gap-3 sm:grid-cols-2">
        {allowed.map((item) => (
          <li key={item.href}>
            <a className="block rounded-xl border border-line bg-card p-4 font-semibold" href={item.href}>
              {item.label}
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}
