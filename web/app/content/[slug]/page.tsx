import { PublicFrame } from "@/components/public-frame";
import { chrome, contentPath, homePath } from "@/lib/chrome";
import { loadContent } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function ContentPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const copy = chrome.ar;
  let item;
  try {
    item = await loadContent("ar", slug);
  } catch {
    return (
      <PublicFrame locale="ar" path={contentPath("ar", slug)}>
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="ar" path={contentPath("ar", slug)}>
      <ContentBody locale="ar" item={item} />
    </PublicFrame>
  );
}

export function ContentBody({
  locale,
  item,
}: {
  locale: "ar" | "en";
  item: Awaited<ReturnType<typeof loadContent>>;
}) {
  const copy = chrome[locale];
  if (!item) {
    return (
      <>
        <a href={homePath(locale)} className="text-sm">
          {copy.back}
        </a>
        <p className="rounded-lg border border-dashed border-line bg-card p-4">
          {locale === "en" ? "This page is not published." : "هذه الصفحة غير منشورة."}
        </p>
      </>
    );
  }
  return (
    <>
      <a href={homePath(locale)} className="text-sm">
        {copy.back}
      </a>
      <h1 className="text-3xl font-semibold">{item.title}</h1>
      <p className="whitespace-pre-wrap text-sm text-ink/80">{item.body}</p>
      {item.neighborhood_id ? (
        <a className="text-sm underline" href={`${locale === "en" ? "/en/search" : "/search"}?neighborhood_id=${item.neighborhood_id}`}>
          {locale === "en" ? "Neighborhood record" : "سجل الحي"}
        </a>
      ) : null}
    </>
  );
}
