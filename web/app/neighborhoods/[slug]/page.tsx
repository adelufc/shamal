import { PublicFrame } from "@/components/public-frame";
import { chrome, homePath, money, priceUnit } from "@/lib/chrome";
import { loadNeighborhood } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function NeighborhoodPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const copy = chrome.ar;
  let payload;
  try {
    payload = await loadNeighborhood("ar", slug);
  } catch {
    return (
      <PublicFrame locale="ar" path={`/neighborhoods/${slug}`}>
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="ar" path={`/neighborhoods/${slug}`}>
      <NeighborhoodBody locale="ar" payload={payload} />
    </PublicFrame>
  );
}

export function NeighborhoodBody({
  locale,
  payload,
}: {
  locale: "ar" | "en";
  payload: Awaited<ReturnType<typeof loadNeighborhood>>;
}) {
  const copy = chrome[locale];
  if (!payload) {
    return <p className="rounded-lg border border-dashed border-line bg-card p-4">{copy.emptyNeighborhoods}</p>;
  }
  const text = payload.neighborhood.texts?.[0];
  const note = locale === "en" ? payload.neighborhood.boundary_note_en : payload.neighborhood.boundary_note_ar;
  return (
    <>
      <a href={homePath(locale)} className="text-sm">
        {copy.back}
      </a>
      <h1 className="text-3xl font-semibold">{text?.name}</h1>
      <p className="text-sm text-ink/80">{text?.description}</p>
      <p className="rounded-lg border border-line bg-olive-soft p-3 text-sm">{note || copy.boundary}</p>
      <a className="text-sm underline" href={`${locale === "en" ? "/en/search" : "/search"}?neighborhood_id=${payload.neighborhood.id}`}>
        {copy.mapLink}
      </a>
      <section className="grid gap-3">
        <h2 className="text-xl font-semibold">{copy.listings}</h2>
        {payload.listings.length === 0 ? (
          <p className="rounded-lg border border-dashed border-line bg-card p-4">{copy.emptyListings}</p>
        ) : (
          <ul className="grid gap-3">
            {payload.listings.map((item) => (
              <li key={item.id} className="rounded-lg border border-line bg-card p-4">
                <h3 className="font-semibold">{item.texts?.[0]?.title}</h3>
                <p className="mt-1 text-sm">{item.texts?.[0]?.description}</p>
                <p className="mt-2 text-sm">
                  {money(item.price_minor, locale)} {priceUnit(locale, item.price_unit, item.transaction_type)}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}
