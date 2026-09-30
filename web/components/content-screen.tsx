"use client";

import { FormEvent, useEffect, useState } from "react";
import { ApiError, Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label, TextArea } from "@/components/ui/input";
import type { Locale, SessionUser } from "@/lib/types";

type Text = { locale: string; title: string; body: string };
type Page = {
  id: string;
  kind: string;
  slug: string;
  neighborhood_id: string;
  status: string;
  version: number;
  texts: Text[];
};
type Place = { id: string; status: string; texts: { locale: string; name: string; slug: string }[] };

const kinds = ["home", "featured", "offer", "article", "faq", "policy", "service"] as const;

const copy = {
  ar: {
    title: "المحتوى",
    lead: "العربية والإنجليزية نصان منفصلان. النشر بعد المراجعة. نص الحي يبقى في سجل الحي ولا تُحفظ نسخة ثانية هنا. العرض نص تحريري ولا يغيّر سعر إعلان.",
    kind: "النوع",
    slug: "الرابط",
    neighborhood: "ربط بحي موجود",
    none: "بلا حي",
    titleAR: "العنوان العربي",
    bodyAR: "النص العربي",
    titleEN: "العنوان الإنجليزي",
    bodyEN: "النص الإنجليزي",
    save: "حفظ مسودة",
    submit: "إرسال للمراجعة",
    publish: "نشر",
    preview: "معاينة",
    empty: "لا توجد صفحات بعد.",
    places: "محتوى الأحياء",
    placesLead: "الأسماء والأوصاف المعروضة للحي تأتي من سجل الحي نفسه.",
    openPlace: "فتح سجل الحي",
    onlyCreate: "إنشاء المسودة لمن لديه صلاحية إنشاء العقار. النشر لمن لديه صلاحية النشر.",
    login: "هذه الشاشة لموظف يستطيع عرض العقار.",
    desk: "دخول المكتب",
    loading: "جارٍ التحميل…",
    english: "English",
    kinds: {
      home: "الرئيسية",
      featured: "مختار",
      offer: "عرض تحريري",
      article: "مقال",
      faq: "سؤال شائع",
      policy: "سياسة",
      service: "صفحة خدمة",
    },
    status: { draft: "مسودة", in_review: "قيد المراجعة", published: "منشور" },
  },
  en: {
    title: "Content",
    lead: "Arabic and English are stored as separate text. Publishing follows review. A neighborhood’s own wording stays on the neighborhood record and is not copied here. An offer is editorial text and does not change a listing price.",
    kind: "Kind",
    slug: "Address",
    neighborhood: "Link an existing neighborhood",
    none: "No neighborhood",
    titleAR: "Arabic title",
    bodyAR: "Arabic text",
    titleEN: "English title",
    bodyEN: "English text",
    save: "Save draft",
    submit: "Send for review",
    publish: "Publish",
    preview: "Preview",
    empty: "There are no pages yet.",
    places: "Neighborhood content",
    placesLead: "The names and descriptions shown for a neighborhood come from that neighborhood record.",
    openPlace: "Open the neighborhood record",
    onlyCreate: "Creating a draft needs catalog create permission. Publishing needs publish permission.",
    login: "This screen is for a staff member who can view the catalog.",
    desk: "Work desk sign-in",
    loading: "Loading…",
    english: "العربية",
    kinds: {
      home: "Home",
      featured: "Featured",
      offer: "Editorial offer",
      article: "Article",
      faq: "FAQ",
      policy: "Policy",
      service: "Service page",
    },
    status: { draft: "Draft", in_review: "In review", published: "Published" },
  },
} as const;

export function ContentScreen({ locale, framed = false }: { locale: Locale; framed?: boolean }) {
  const text = copy[locale];
  const [me, setMe] = useState<SessionUser | null>(null);
  const [pages, setPages] = useState<Page[]>([]);
  const [places, setPlaces] = useState<Place[]>([]);
  const [selected, setSelected] = useState<Page | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [kind, setKind] = useState<(typeof kinds)[number]>("article");
  const [slug, setSlug] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [titleAR, setTitleAR] = useState("");
  const [bodyAR, setBodyAR] = useState("");
  const [titleEN, setTitleEN] = useState("");
  const [bodyEN, setBodyEN] = useState("");

  const can = (action: string) => (me?.actions.catalog ?? []).includes(action);

  async function load() {
    const [session, list, geo] = await Promise.all([
      api<SessionUser>("/api/v1/me"),
      api<{ items: Page[] }>("/api/v1/content/pages"),
      api<{ items: Place[] }>("/api/v1/geo/neighborhoods?page_size=100"),
    ]);
    setMe(session);
    setPages(list.items);
    setPlaces(geo.items);
    setSelected((current) => list.items.find((item) => item.id === current?.id) ?? list.items[0] ?? null);
  }

  useEffect(() => {
    load()
      .catch((caught: unknown) => setError(caught))
      .finally(() => setLoading(false));
  }, []);

  async function create(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const created = await api<Page>("/api/v1/content/pages", {
        method: "POST",
        body: JSON.stringify({
          kind,
          slug,
          neighborhood_id: neighborhood,
          title_ar: titleAR,
          body_ar: bodyAR,
          title_en: titleEN,
          body_en: bodyEN,
        }),
      });
      setSlug("");
      setTitleAR("");
      setBodyAR("");
      setTitleEN("");
      setBodyEN("");
      await load();
      setSelected(created);
    } catch (caught) {
      setError(caught);
    }
  }

  async function move(page: Page, path: "submit" | "publish") {
    setError(null);
    try {
      const next = await api<Page>(`/api/v1/content/pages/${page.id}/${path}`, {
        method: "POST",
        body: JSON.stringify({ version: page.version }),
      });
      await load();
      setSelected(next);
    } catch (caught) {
      setError(caught);
    }
  }

  const placeName = (place: Place) => place.texts.find((item) => item.locale === locale)?.name ?? place.texts[0]?.name ?? place.id;
  const arabic = selected?.texts.find((item) => item.locale === "ar");
  const english = selected?.texts.find((item) => item.locale === "en");

  const body = (
    <section className="grid gap-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        {framed ? null : (
          <a className="text-sm underline" href="/en/admin/content">
            {text.english}
          </a>
        )}
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
      <section className="grid gap-2 rounded-lg border border-line bg-card p-4">
        <h2 className="font-semibold">{text.places}</h2>
        <p className="text-sm text-ink/70">{text.placesLead}</p>
        <ul className="grid gap-1 text-sm">
          {places.map((place) => (
            <li key={place.id}>
              {placeName(place)} · {place.status}{" "}
              <a className="underline" href="/admin/neighborhoods">
                {text.openPlace}
              </a>
            </li>
          ))}
        </ul>
      </section>
      {pages.length === 0 && !loading ? <p className="text-sm">{text.empty}</p> : null}
      <ul className="grid gap-2">
        {pages.map((page) => (
          <li key={page.id}>
            <button className="w-full rounded-lg border border-line bg-card p-3 text-start text-sm" onClick={() => setSelected(page)}>
              <span className="font-semibold">{text.kinds[page.kind as keyof typeof text.kinds] ?? page.kind}</span>
              {" · "}
              {page.slug}
              {" · "}
              {text.status[page.status as keyof typeof text.status] ?? page.status}
            </button>
          </li>
        ))}
      </ul>
      {selected ? (
        <article className="grid gap-3 rounded-lg border border-line bg-card p-4">
          <h2 className="font-semibold">{text.preview}</h2>
          <p className="text-sm text-ink/70">{text.status[selected.status as keyof typeof text.status] ?? selected.status}</p>
          <div dir="rtl">
            <h3 className="font-semibold">{arabic?.title}</h3>
            <p className="text-sm">{arabic?.body}</p>
          </div>
          <div dir="ltr">
            <h3 className="font-semibold">{english?.title}</h3>
            <p className="text-sm">{english?.body}</p>
          </div>
          {selected.neighborhood_id ? <p className="text-sm">{selected.neighborhood_id}</p> : null}
          <div className="flex flex-wrap gap-2">
            {can("update") && selected.status === "draft" ? (
              <Button type="button" onClick={() => void move(selected, "submit")}>
                {text.submit}
              </Button>
            ) : null}
            {can("publish") && selected.status === "in_review" ? (
              <Button type="button" onClick={() => void move(selected, "publish")}>
                {text.publish}
              </Button>
            ) : null}
          </div>
        </article>
      ) : null}
      {can("create") ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={create}>
          <Label>
            {text.kind}
            <select className="rounded-md border border-line bg-paper px-2 py-2" value={kind} onChange={(event) => setKind(event.target.value as (typeof kinds)[number])}>
              {kinds.map((item) => (
                <option key={item} value={item}>
                  {text.kinds[item]}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            {text.slug}
            <Input value={slug} onChange={(event) => setSlug(event.target.value)} required />
          </Label>
          <Label>
            {text.neighborhood}
            <select className="rounded-md border border-line bg-paper px-2 py-2" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)}>
              <option value="">{text.none}</option>
              {places.map((place) => (
                <option key={place.id} value={place.id}>
                  {placeName(place)}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            {text.titleAR}
            <Input value={titleAR} onChange={(event) => setTitleAR(event.target.value)} required />
          </Label>
          <Label>
            {text.bodyAR}
            <TextArea value={bodyAR} onChange={(event) => setBodyAR(event.target.value)} required />
          </Label>
          <Label>
            {text.titleEN}
            <Input value={titleEN} onChange={(event) => setTitleEN(event.target.value)} required />
          </Label>
          <Label>
            {text.bodyEN}
            <TextArea value={bodyEN} onChange={(event) => setBodyEN(event.target.value)} required />
          </Label>
          <Button type="submit">{text.save}</Button>
        </form>
      ) : (
        <p className="text-sm text-ink/70">{text.onlyCreate}</p>
      )}
    </section>
  );

  if (!framed) return body;
  return <PublicFrame locale={locale} path="/en/admin/content">{body}</PublicFrame>;
}
