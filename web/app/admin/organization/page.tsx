"use client";

import { FormEvent, useEffect, useState } from "react";
import { Failure, api, useSession } from "@/components/admin-shell";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

type Node = {
  id: string;
  kind: string;
  parent_id: string;
  name_ar: string;
  name_en: string;
  status: string;
  version: number;
};

const kinds: Record<string, string> = {
  company: "شركة",
  branch: "فرع",
  department: "إدارة",
  team: "فريق",
};

export default function OrganizationPage() {
  const me = useSession();
  const [items, setItems] = useState<Node[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [nameAR, setNameAR] = useState("");
  const [nameEN, setNameEN] = useState("");
  const [parent, setParent] = useState("");
  const [kind, setKind] = useState("team");
  const [reason, setReason] = useState("");

  const can = (action: string) => (me?.actions.organization ?? []).includes(action);

  async function load() {
    setState("loading");
    try {
      const payload = await api<{ items: Node[] }>("/api/v1/org/nodes");
      setItems(payload.items);
      setState("ready");
    } catch (caught) {
      setError(caught);
      setState("error");
    }
  }

  useEffect(() => {
    if (me) void load();
  }, [me]);

  async function create(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api("/api/v1/org/nodes", {
        method: "POST",
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
        body: JSON.stringify({ version, reason }),
      });
      await load();
    } catch (caught) {
      setError(caught);
    }
  }

  const depth = (id: string): number => {
    const node = items.find((item) => item.id === id);
    if (!node?.parent_id) return 0;
    return 1 + depth(node.parent_id);
  };

  return (
    <section className="grid gap-4">
      <h1 className="text-2xl font-semibold">الهيكل</h1>
      {state === "loading" ? <p className="text-sm">جارٍ تحميل الهيكل…</p> : null}
      {error ? <Failure error={error} /> : null}
      {state === "ready" && items.length === 0 ? <p className="text-sm">لا توجد وحدات.</p> : null}
      <ul className="grid gap-2">
        {items.map((item) => (
          <li key={item.id} className="rounded-lg border border-line bg-card p-3" style={{ marginInlineStart: depth(item.id) * 16 }}>
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h2 className="font-semibold">{item.name_ar}</h2>
              <span className="text-sm">{kinds[item.kind] ?? item.kind}</span>
            </div>
            <p className="text-sm text-ink/70">{item.name_en}</p>
            <p className="text-sm">{item.status === "active" ? "نشط" : "مغلق"}</p>
            {can("archive") && item.status === "active" && item.kind !== "company" ? (
              <Button variant="ghost" className="mt-2" onClick={() => close(item.id, item.version)}>
                إغلاق
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
      {can("archive") ? (
        <Label>
          سبب الإغلاق
          <Input value={reason} onChange={(event) => setReason(event.target.value)} />
        </Label>
      ) : null}
      {can("create") ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={create}>
          <h2 className="font-semibold">وحدة جديدة</h2>
          <Label>
            النوع
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={kind} onChange={(event) => setKind(event.target.value)}>
              <option value="branch">فرع</option>
              <option value="department">إدارة</option>
              <option value="team">فريق</option>
            </select>
          </Label>
          <Label>
            تتبع
            <select className="rounded-md border border-line bg-card px-3 py-2 text-sm" value={parent} onChange={(event) => setParent(event.target.value)} required>
              <option value="">اختر الوحدة الأم</option>
              {items
                .filter((item) => item.status === "active")
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name_ar}
                  </option>
                ))}
            </select>
          </Label>
          <Label>
            الاسم العربي
            <Input value={nameAR} onChange={(event) => setNameAR(event.target.value)} required />
          </Label>
          <Label>
            الاسم الإنجليزي
            <Input value={nameEN} onChange={(event) => setNameEN(event.target.value)} required />
          </Label>
          <Button>حفظ</Button>
        </form>
      ) : (
        <p className="text-sm text-ink/70">إنشاء الوحدات متاح لصلاحية الهيكل فقط.</p>
      )}
    </section>
  );
}
