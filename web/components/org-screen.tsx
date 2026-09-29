"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import type { Locale, SessionUser } from "@/lib/types";

type Node = {
  id: string;
  kind: string;
  parent_id: string;
  name_ar: string;
  name_en: string;
  status: string;
  version: number;
};

type Staff = { id: string; display_name: string; email: string; status: string };
type Membership = { id: string; user_id: string; node_id: string; is_primary: boolean; ends_at?: string | null };
type Role = { id: string; domain: string; action: string; effect: string; scope_kind: string; scope_node_id: string; grantee_id: string };

const copy = {
  ar: {
    title: "الهيكل",
    lead: "الشركة والفرع والإدارة والفريق، ثم الموظف وعضويته. الدور ونطاق الوصول يظهران كما سُجّلا، دون مقارنة بصرية.",
    loading: "جارٍ تحميل الهيكل…",
    empty: "لا توجد وحدات.",
    employees: "الموظفون",
    noEmployees: "لا موظف على هذه الوحدة.",
    roles: "الدور ونطاق الوصول",
    noRoles: "لا دور مسجّل على هذه الوحدة.",
    primary: "عضوية أساسية",
    active: "نشط",
    closed: "مغلق",
    close: "إغلاق",
    reason: "سبب الإغلاق",
    create: "وحدة جديدة",
    kind: "النوع",
    parent: "تتبع",
    pick: "اختر الوحدة الأم",
    nameAR: "الاسم العربي",
    nameEN: "الاسم الإنجليزي",
    save: "حفظ",
    membership: "عضوية واحدة",
    person: "الموظف",
    unit: "الوحدة",
    onlyCreate: "إنشاء الوحدات والعضويات متاح لصلاحية إنشاء الهيكل.",
    permissions: "شاشة الصلاحيات",
    login: "هذه الشاشة لموظف لديه صلاحية عرض الهيكل.",
    desk: "دخول المكتب",
    kinds: { company: "شركة", branch: "فرع", department: "إدارة", team: "فريق" },
    scopes: { own: "سجلّه", assigned: "المسند إليه", team: "الفريق", department: "الإدارة", branch: "الفرع", company: "الشركة" },
  },
  en: {
    title: "Structure",
    lead: "Company, branch, department, and team, then the employee and membership. A role and its access scope are shown as stored. This is not a visual comparison.",
    loading: "Loading the structure…",
    empty: "There are no units.",
    employees: "Employees",
    noEmployees: "No employee is on this unit.",
    roles: "Role and access scope",
    noRoles: "No role is recorded on this unit.",
    primary: "Primary membership",
    active: "Active",
    closed: "Closed",
    close: "Close",
    reason: "Reason for closing",
    create: "New unit",
    kind: "Kind",
    parent: "Parent",
    pick: "Choose the parent unit",
    nameAR: "Arabic name",
    nameEN: "English name",
    save: "Save",
    membership: "One membership",
    person: "Employee",
    unit: "Unit",
    onlyCreate: "Creating units and memberships requires the structure create permission.",
    permissions: "Permission screen",
    login: "This screen is for a staff member who can view the structure.",
    desk: "Work desk sign-in",
    kinds: { company: "Company", branch: "Branch", department: "Department", team: "Team" },
    scopes: { own: "Own record", assigned: "Assigned", team: "Team", department: "Department", branch: "Branch", company: "Company" },
  },
} as const;

export function OrgScreen({ locale, framed = false }: { locale: Locale; framed?: boolean }) {
  const text = copy[locale];
  const [me, setMe] = useState<SessionUser | null>(null);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [staff, setStaff] = useState<Staff[]>([]);
  const [memberships, setMemberships] = useState<Membership[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [nameAR, setNameAR] = useState("");
  const [nameEN, setNameEN] = useState("");
  const [parent, setParent] = useState("");
  const [kind, setKind] = useState("team");
  const [reason, setReason] = useState("");
  const [member, setMember] = useState("");
  const [memberNode, setMemberNode] = useState("");
  const [primary, setPrimary] = useState(false);

  const can = (action: string) => (me?.actions.organization ?? []).includes(action);

  async function load() {
    setState("loading");
    try {
      const [session, payload] = await Promise.all([
        api<SessionUser>("/api/v1/me"),
        api<{ nodes: Node[]; staff: Staff[]; memberships: Membership[]; roles: Role[] }>("/api/v1/org/structure", {
          headers: { "Accept-Language": locale },
        }),
      ]);
      setMe(session);
      setNodes(payload.nodes);
      setStaff(payload.staff);
      setMemberships(payload.memberships);
      setRoles(payload.roles);
      setState("ready");
    } catch (caught) {
      setError(caught);
      setState("error");
    }
  }

  useEffect(() => {
    void load();
  }, [locale]);

  async function create(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/org/nodes", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ kind, parent_id: parent, name_ar: nameAR, name_en: nameEN }),
      });
      setNameAR("");
      setNameEN("");
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function close(id: string, version: number) {
    setError(null);
    try {
      await api(`/api/v1/org/nodes/${id}/close`, {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ version, reason }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  async function addMembership(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/org/memberships", {
        method: "POST",
        headers: { "Accept-Language": locale },
        body: JSON.stringify({ user_id: member, node_id: memberNode, is_primary: primary }),
      });
      setPrimary(false);
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  const depth = (id: string): number => {
    const node = nodes.find((item) => item.id === id);
    if (!node?.parent_id) return 0;
    return 1 + depth(node.parent_id);
  };
  const ordered = (() => {
    const children = new Map<string, Node[]>();
    for (const node of nodes) {
      const key = node.parent_id || "";
      children.set(key, [...(children.get(key) ?? []), node]);
    }
    const list: Node[] = [];
    const seen = new Set<string>();
    const walk = (parent: string) => {
      const group = [...(children.get(parent) ?? [])].sort((a, b) => a.name_ar.localeCompare(b.name_ar, "ar"));
      for (const node of group) {
        if (seen.has(node.id)) continue;
        seen.add(node.id);
        list.push(node);
        walk(node.id);
      }
    };
    walk("");
    for (const node of nodes) {
      if (!seen.has(node.id)) {
        seen.add(node.id);
        list.push(node);
        walk(node.id);
      }
    }
    return list;
  })();
  const name = (node: Node) => (locale === "en" ? node.name_en : node.name_ar);
  const person = (id: string) => staff.find((item) => item.id === id);

  const body = (
    <section className="grid gap-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <a className="text-sm underline" href={locale === "en" ? "/en/admin/permissions" : "/admin/permissions"}>
          {text.permissions}
        </a>
      </div>
      <p className="text-sm text-ink/80">{text.lead}</p>
      {state === "loading" ? <p className="text-sm">{text.loading}</p> : null}
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
      {state === "ready" && nodes.length === 0 ? <p className="text-sm">{text.empty}</p> : null}
      <ul className="grid gap-2">
        {ordered.map((item) => {
          const people = memberships.filter((entry) => entry.node_id === item.id && !entry.ends_at);
          const attached = roles.filter((entry) => entry.grantee_id === item.id);
          return (
            <li key={item.id} className="rounded-lg border border-line bg-card p-3" style={{ marginInlineStart: depth(item.id) * 16 }}>
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <h2 className="font-semibold">{name(item)}</h2>
                <span className="text-sm">{text.kinds[item.kind as keyof typeof text.kinds] ?? item.kind}</span>
              </div>
              <p className="text-sm text-ink/70">{locale === "en" ? item.name_ar : item.name_en}</p>
              <p className="text-sm">{item.status === "active" ? text.active : text.closed}</p>
              <h3 className="mt-2 text-sm font-semibold">{text.employees}</h3>
              {people.length === 0 ? <p className="text-sm text-ink/70">{text.noEmployees}</p> : null}
              <ul className="grid gap-1 text-sm">
                {people.map((entry) => {
                  const employee = person(entry.user_id);
                  return (
                    <li key={entry.id}>
                      {employee?.display_name ?? entry.user_id}
                      {employee?.email ? ` · ${employee.email}` : ""}
                      {entry.is_primary ? ` · ${text.primary}` : ""}
                    </li>
                  );
                })}
              </ul>
              <h3 className="mt-2 text-sm font-semibold">{text.roles}</h3>
              {attached.length === 0 ? <p className="text-sm text-ink/70">{text.noRoles}</p> : null}
              <ul className="grid gap-1 text-sm">
                {attached.map((entry) => (
                  <li key={entry.id}>
                    {entry.domain} · {entry.action} · {entry.effect} · {text.scopes[entry.scope_kind as keyof typeof text.scopes] ?? entry.scope_kind}
                  </li>
                ))}
              </ul>
              {can("archive") && item.status === "active" && item.kind !== "company" ? (
                <Button variant="ghost" className="mt-2" onClick={() => close(item.id, item.version)}>
                  {text.close}
                </Button>
              ) : null}
            </li>
          );
        })}
      </ul>
      {can("archive") ? (
        <Label>
          {text.reason}
          <Input value={reason} onChange={(event) => setReason(event.target.value)} />
        </Label>
      ) : null}
      {can("create") ? (
        <>
          <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={create}>
            <h2 className="font-semibold">{text.create}</h2>
            <Label>
              {text.kind}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={kind} onChange={(event) => setKind(event.target.value)}>
                <option value="branch">{text.kinds.branch}</option>
                <option value="department">{text.kinds.department}</option>
                <option value="team">{text.kinds.team}</option>
              </select>
            </Label>
            <Label>
              {text.parent}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={parent} onChange={(event) => setParent(event.target.value)} required>
                <option value="">{text.pick}</option>
                {nodes
                  .filter((item) => item.status === "active")
                  .map((item) => (
                    <option key={item.id} value={item.id}>
                      {name(item)}
                    </option>
                  ))}
              </select>
            </Label>
            <Label>
              {text.nameAR}
              <Input value={nameAR} onChange={(event) => setNameAR(event.target.value)} required />
            </Label>
            <Label>
              {text.nameEN}
              <Input value={nameEN} onChange={(event) => setNameEN(event.target.value)} required />
            </Label>
            <Button>{text.save}</Button>
          </form>
          <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={addMembership}>
            <h2 className="font-semibold">{text.membership}</h2>
            <Label>
              {text.person}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={member} onChange={(event) => setMember(event.target.value)} required>
                <option value="">{text.pick}</option>
                {staff.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.display_name}
                  </option>
                ))}
              </select>
            </Label>
            <Label>
              {text.unit}
              <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={memberNode} onChange={(event) => setMemberNode(event.target.value)} required>
                <option value="">{text.pick}</option>
                {nodes
                  .filter((item) => item.status === "active")
                  .map((item) => (
                    <option key={item.id} value={item.id}>
                      {name(item)}
                    </option>
                  ))}
              </select>
            </Label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={primary} onChange={(event) => setPrimary(event.target.checked)} />
              {text.primary}
            </label>
            <Button>{text.save}</Button>
          </form>
        </>
      ) : (
        <p className="text-sm text-ink/70">{text.onlyCreate}</p>
      )}
    </section>
  );

  if (!framed) return body;
  return (
    <PublicFrame locale={locale} path={locale === "en" ? "/en/admin/organization" : "/admin/organization"}>
      {body}
    </PublicFrame>
  );
}
