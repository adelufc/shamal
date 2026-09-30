import { chrome, homePath } from "@/lib/chrome";
import { loadContent } from "@/lib/public";

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
