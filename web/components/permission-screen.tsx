"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import type { Locale, SessionUser } from "@/lib/types";

type Staff = { id: string; display_name: string; email: string };
type Node = { id: string; kind: string; name_ar: string; name_en: string; status: string };
type Rule = {
  id: string;
  domain: string;
  action: string;
  effect: string;
  layer: string;
  scope_kind: string;
  starts_at?: string | null;
  ends_at?: string | null;
  temporary: boolean;
};
type BreakItem = { node_id: string; domain: string; action: string; reason: string };
type Account = {
  user: { id: string; display_name: string; email: string };
  direct: Rule[];
  inherited: Rule[];
  temporary: Rule[];
  deny: Rule[];
  breaks: BreakItem[];
};
type Preview = { allowed: boolean; layer: string; code: string; deny_source?: string };

const domains = ["geography", "catalog", "organization", "permissions", "files", "identity", "clients", "visits", "deals", "finance", "messages", "disputes", "reports"] as const;
const actions = ["view", "create", "update", "approve", "publish", "archive", "delete_draft"] as const;
const scopes = ["company", "branch", "department", "team", "assigned", "own"] as const;

const copy = {
  ar: {
    title: "الصلاحيات",
    lead: "منح فردي فقط. الشاشة تعرض المباشر والموروث والمؤقت والمنع وقطع الوراثة، ثم الناتج لمعاينة إجراء على سجل.",
    search: "ابحث عن موظف",
    searchHint: "الاسم أو البريد",
    find: "بحث",
    person: "الموظف",
    loading: "جارٍ التحميل…",
    emptyPeople: "لا توجد نتيجة.",
    direct: "مباشر",
    inherited: "موروث",
    temporary: "مؤقت",
    deny: "منع",
    breakTitle: "قطع الوراثة",
    empty: "لا شيء في هذا القسم.",
    preview: "معاينة إجراء على سجل",
    domain: "المجال",
    action: "الإجراء",
    record: "السجل",
    anyRecord: "سجل الشركة",
    createdBy: "منشئ السجل",
    anyone: "شخص آخر",
    run: "اعرض الناتج",
    result: "الصلاحية الناتجة",
    allowed: "مسموح",
    refused: "غير مسموح",
    layer: "الطبقة",
    grant: "منح فردي",
    effect: "الأثر",
    allow: "سماح",
    denyEffect: "منع",
    scope: "النطاق",
    until: "ينتهي في",
    save: "حفظ المنح",
    breakForm: "قطع وراثة واحد",
    node: "الوحدة",
    reason: "السبب",
    saveBreak: "تسجيل القطع",
    onlyCreate: "المنح الفردي وقطع الوراثة متاحان لصلاحية إنشاء الصلاحيات. المعاينة لصلاحية العرض أو لحسابك.",
    structure: "الهيكل",
    login: "هذه الشاشة لموظف. معاينة موظف آخر تحتاج صلاحية عرض الصلاحيات.",
    desk: "دخول المكتب",
    layers: {
      direct: "مباشر",
      inherited: "موروث",
      explicit_deny: "منع صريح",
      default_deny: "منع افتراضي",
      exception: "استثناء",
      constraint: "قيد",
    },
    scopes: { company: "الشركة", branch: "الفرع", department: "الإدارة", team: "الفريق", assigned: "المسند إليه", own: "سجلّه" },
  },
  en: {
    title: "Permissions",
    lead: "One grant at a time. The screen shows direct, inherited, temporary, deny, and an inheritance break, then the resulting permission for an action on a record.",
    search: "Find an employee",
    searchHint: "Name or email",
    find: "Search",
    person: "Employee",
    loading: "Loading…",
    emptyPeople: "No match.",
    direct: "Direct",
    inherited: "Inherited",
    temporary: "Temporary",
    deny: "Deny",
    breakTitle: "Inheritance break",
    empty: "Nothing is in this section.",
    preview: "Preview an action on a record",
    domain: "Domain",
    action: "Action",
    record: "Record",
    anyRecord: "Company record",
    createdBy: "Record creator",
    anyone: "Someone else",
    run: "Show the result",
    result: "Resulting permission",
    allowed: "Allowed",
    refused: "Not allowed",
    layer: "Layer",
    grant: "Individual grant",
    effect: "Effect",
    allow: "Allow",
    denyEffect: "Deny",
    scope: "Scope",
    until: "Ends at",
    save: "Save the grant",
    breakForm: "One inheritance break",
    node: "Unit",
    reason: "Reason",
    saveBreak: "Record the break",
    onlyCreate: "An individual grant or inheritance break requires permission to create permissions. Preview is available for view permission, or for your own account.",
    structure: "Structure",
    login: "This screen is for a staff member. Previewing another person requires permission to view permissions.",
    desk: "Work desk sign-in",
    layers: {
      direct: "Direct",
      inherited: "Inherited",
      explicit_deny: "Explicit deny",
      default_deny: "Default deny",
      exception: "Exception",
      constraint: "Constraint",
    },
    scopes: { company: "Company", branch: "Branch", department: "Department", team: "Team", assigned: "Assigned", own: "Own record" },
  },
} as const;

export function PermissionScreen({ locale, framed = false }: { locale: Locale; framed?: boolean }) {
  const text = copy[locale];
  const [me, setMe] = useState<SessionUser | null>(null);
  const [query, setQuery] = useState("");
  const [people, setPeople] = useState<Staff[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [userID, setUserID] = useState("");
  const [account, setAccount] = useState<Account | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [domain, setDomain] = useState<(typeof domains)[number]>("organization");
  const [action, setAction] = useState<(typeof actions)[number]>("view");
  const [branch, setBranch] = useState("");
  const [createdBy, setCreatedBy] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [effect, setEffect] = useState("allow");
  const [scope, setScope] = useState<(typeof scopes)[number]>("company");
  const [ends, setEnds] = useState("");
  const [breakNode, setBreakNode] = useState("");
  const [breakReason, setBreakReason] = useState("");

  const canCreate = (me?.actions.permissions ?? []).includes("create");

  async function loadPeople(q: string) {
    const payload = await api<{ staff: Staff[]; nodes: Node[] }>(`/api/v1/org/structure?q=${encodeURIComponent(q)}`, {
      headers: { "Accept-Language": locale },
    });
    setPeople(payload.staff);
    setNodes(payload.nodes);
    return payload.staff;
  }

  async function loadAccount(id: string) {
    const payload = await api<Account>(`/api/v1/permissions/account?user_id=${encodeURIComponent(id)}`, {
      headers: { "Accept-Language": locale },
    });
    setAccount(payload);
  }

  useEffect(() => {
    let stop = false;
    api<SessionUser>("/api/v1/me")
      .then(async (session) => {
        if (stop) return;
        setMe(session);
        const staff = await loadPeople("");
        const id = staff.some((item) => item.id === session.id) ? session.id : staff[0]?.id ?? session.id;
        setUserID(id);
        await loadAccount(id);
      })
      .catch((caught: unknown) => {
        if (!stop) setError(caught);
      })
      .finally(() => {
        if (!stop) setLoading(false);
      });
    return () => {
      stop = true;
    };
  }, [locale]);

  async function search(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const staff = await loadPeople(query);
      if (staff[0]) {
        setUserID(staff[0].id);
        await loadAccount(staff[0].id);
      } else {
        setAccount(null);
      }
    } catch (caught) {
      setError(caught);
    }
  }

  async function choose(id: string) {
    setUserID(id);
    setPreview(null);
    setError(null);
    try {
      await loadAccount(id);
    } catch (caught) {
      setAccount(null);
      setError(caught);
    }
  }

  async function runPreview(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      setPreview(
        await api<Preview>("/api/v1/permissions/preview", {
          method: "POST",
          headers: { "Accept-Language": locale },
          body: JSON.stringify({
            user_id: userID,
            domain,
            action,
            resource: { branch_id: branch, created_by: createdBy },
          }),
        }),
      );
    } catch (caught) {
      setPreview(null);
      setError(caught);
    }
  }

  async function grant(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/permissions/grants", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({
          user_id: userID,
          domain,
          action,
          effect,
          scope_kind: scope,
          ends_at: ends ? new Date(ends).toISOString() : "",
        }),
      });
      setEnds("");
      await loadAccount(userID);
    } catch (caught) {
      setError(caught);
    }
  }

  async function cut(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/permissions/breaks", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ node_id: breakNode, domain, action, reason: breakReason }),
      });
      setBreakReason("");
      await loadAccount(userID);
    } catch (caught) {
      setError(caught);
    }
  }

  const nodeName = (node: Node) => (locale === "en" ? node.name_en : node.name_ar);

  const body = (
    <section className="grid gap-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <a className="text-sm underline" href={locale === "en" ? "/en/admin/organization" : "/admin/organization"}>
          {text.structure}
        </a>
      </div>
      <p className="text-sm text-ink/80">{text.lead}</p>
      {loading ? <p className="text-sm">{text.loading}</p> : null}
      {error ? (
        <div className="grid gap-2">
          <Failure error={error} />
          {error instanceof ApiError && (error.status === 401 || error.status === 403) ? (
            <a className="text-sm underline" href="/admin/login">
              {text.desk}
            </a>
          ) : null}
          <p className="text-sm text-ink/70">{text.login}</p>
        </div>
      ) : null}
      <form className="flex flex-wrap items-end gap-2" onSubmit={search}>
        <Label>
          {text.search}
          <Input value={query} placeholder={text.searchHint} onChange={(event) => setQuery(event.target.value)} />
        </Label>
        <Button type="submit">{text.find}</Button>
      </form>
      <Label>
        {text.person}
        <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={userID} onChange={(event) => void choose(event.target.value)}>
          {people.length === 0 ? <option value="">{text.emptyPeople}</option> : null}
          {people.map((item) => (
            <option key={item.id} value={item.id}>
              {item.display_name} · {item.email}
            </option>
          ))}
        </select>
      </Label>
      {account ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <RuleList title={text.direct} items={account.direct} empty={text.empty} />
          <RuleList title={text.inherited} items={account.inherited} empty={text.empty} />
          <RuleList title={text.temporary} items={account.temporary} empty={text.empty} />
          <RuleList title={text.deny} items={account.deny} empty={text.empty} />
          <section className="rounded-lg border border-line bg-card p-3 sm:col-span-2">
            <h2 className="font-semibold">{text.breakTitle}</h2>
            {account.breaks.length === 0 ? <p className="text-sm text-ink/70">{text.empty}</p> : null}
            <ul className="grid gap-1 text-sm">
              {account.breaks.map((item) => (
                <li key={`${item.node_id}-${item.domain}-${item.action}`}>
                  {item.domain} · {item.action} · {item.reason}
                </li>
              ))}
            </ul>
          </section>
        </div>
      ) : null}
      <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={runPreview}>
        <h2 className="font-semibold">{text.preview}</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          <Label>
            {text.domain}
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={domain} onChange={(event) => setDomain(event.target.value as (typeof domains)[number])}>
              {domains.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            {text.action}
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={action} onChange={(event) => setAction(event.target.value as (typeof actions)[number])}>
              {actions.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            {text.record}
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={branch} onChange={(event) => setBranch(event.target.value)}>
              <option value="">{text.anyRecord}</option>
              {nodes
                .filter((item) => item.kind === "branch" && item.status === "active")
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {nodeName(item)}
                  </option>
                ))}
            </select>
          </Label>
          <Label>
            {text.createdBy}
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={createdBy} onChange={(event) => setCreatedBy(event.target.value)}>
              <option value="">{text.anyone}</option>
              {people.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.display_name}
                </option>
              ))}
            </select>
          </Label>
        </div>
        <Button type="submit">{text.run}</Button>
        {preview ? (
          <div>
            <h3 className="font-semibold">{text.result}</h3>
            <p>
              {preview.allowed ? text.allowed : text.refused} · {text.layer}: {text.layers[preview.layer as keyof typeof text.layers] ?? preview.layer} · {preview.code}
            </p>
          </div>
        ) : null}
      </form>
      {canCreate ? (
        <>
          <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={grant}>
            <h2 className="font-semibold">{text.grant}</h2>
            <Label>
              {text.effect}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={effect} onChange={(event) => setEffect(event.target.value)}>
                <option value="allow">{text.allow}</option>
                <option value="deny">{text.denyEffect}</option>
              </select>
            </Label>
            <Label>
              {text.scope}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={scope} onChange={(event) => setScope(event.target.value as (typeof scopes)[number])}>
                {scopes.map((item) => (
                  <option key={item} value={item}>
                    {text.scopes[item]}
                  </option>
                ))}
              </select>
            </Label>
            <Label>
              {text.until}
              <Input type="datetime-local" value={ends} onChange={(event) => setEnds(event.target.value)} />
            </Label>
            <Button type="submit">{text.save}</Button>
          </form>
          <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={cut}>
            <h2 className="font-semibold">{text.breakForm}</h2>
            <Label>
              {text.node}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={breakNode} onChange={(event) => setBreakNode(event.target.value)} required>
                <option value="">{text.record}</option>
                {nodes
                  .filter((item) => item.status === "active")
                  .map((item) => (
                    <option key={item.id} value={item.id}>
                      {nodeName(item)}
                    </option>
                  ))}
              </select>
            </Label>
            <Label>
              {text.reason}
              <Input value={breakReason} onChange={(event) => setBreakReason(event.target.value)} required />
            </Label>
            <Button type="submit">{text.saveBreak}</Button>
          </form>
        </>
      ) : (
        <p className="text-sm text-ink/70">{text.onlyCreate}</p>
      )}
    </section>
  );

  if (!framed) return body;
  return (
    <PublicFrame locale={locale} path={locale === "en" ? "/en/admin/permissions" : "/admin/permissions"}>
      {body}
    </PublicFrame>
  );
}

function labelOf(value: string) {
  const names: Record<string, string> = {
    geography: "الأحياء",
    catalog: "العقارات",
    organization: "الهيكل",
    permissions: "الصلاحيات",
    files: "الملفات",
    identity: "الهوية",
    clients: "العملاء",
    visits: "الزيارات",
    deals: "الصفقات",
    finance: "المال",
    messages: "الرسائل",
    disputes: "النزاعات",
    reports: "التقارير",
    view: "عرض",
    create: "إنشاء",
    update: "تعديل",
    approve: "اعتماد",
    publish: "نشر",
    archive: "أرشفة",
    export: "تصدير",
    allow: "سماح",
    deny: "منع",
    company: "الشركة",
    branch: "الفرع",
    department: "الإدارة",
    team: "الفريق",
    own: "السجل",
    assigned: "المسند",
  };
  return names[value] ?? value;
}

function RuleList({ title, items, empty }: { title: string; items: Rule[]; empty: string }) {
  return (
    <section className="rounded-lg border border-line bg-card p-3">
      <h2 className="font-semibold">{title}</h2>
      {items.length === 0 ? <p className="text-sm text-ink/70">{empty}</p> : null}
      <ul className="grid gap-1 text-sm">
        {items.map((item) => (
          <li key={`${title}-${item.id}-${item.layer}`}>
            {labelOf(item.domain)} · {labelOf(item.action)} · {labelOf(item.effect)} · {labelOf(item.scope_kind)}
            {item.starts_at ? ` · البداية ${item.starts_at}` : ""}
            {item.ends_at ? ` · النهاية ${item.ends_at}${new Date(item.ends_at).getTime() < Date.now() ? " · منتهٍ" : " · سارٍ"}` : " · بلا نهاية"}
          </li>
        ))}
      </ul>
    </section>
  );
}
