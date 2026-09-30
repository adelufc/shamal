import { ListingCard } from "@/components/listing-card";
import { chrome, contentPath } from "@/lib/chrome";
import type { HomePayload, Locale, PublicContent } from "@/lib/types";

export function HomeView({
  locale,
  data,
  neighborhoodPath,
}: {
  locale: Locale;
  data: HomePayload;
  neighborhoodPath: (slug: string) => string;
}) {
  const copy = chrome[locale];
  return (
    <>
      <section className="grid gap-2">
        <p className="text-sm text-ink/70">{copy.cityLine}</p>
        <h1 className="text-3xl font-semibold">{copy.brand}</h1>
        <a href={locale === "en" ? "/en/search" : "/search"} className="text-sm underline">
          {copy.search}
        </a>
        <dl className="mt-2 flex flex-wrap gap-3 text-sm">
          <div className="rounded-lg border border-line bg-card px-3 py-2">
            <dt>{copy.countsNeighborhoods}</dt>
            <dd className="text-xl font-semibold">{data.counts.published_neighborhoods}</dd>
          </div>
          <div className="rounded-lg border border-line bg-card px-3 py-2">
            <dt>{copy.countsListings}</dt>
            <dd className="text-xl font-semibold">{data.counts.published_listings}</dd>
          </div>
        </dl>
      </section>
      {data.neighborhoods.length === 0 ? (
        <p className="rounded-lg border border-dashed border-line bg-card p-4">{copy.emptyNeighborhoods}</p>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2">
          {data.neighborhoods.map((item) => (
            <li key={item.id} className="rounded-lg border border-line bg-card p-4">
              <a href={neighborhoodPath(item.slug)} className="text-lg font-semibold">
                {item.name}
              </a>
              <p className="mt-1 text-sm text-ink/70">
                {item.listing_count} {copy.listingCount}
              </p>
              <ServiceLine locale={locale} item={item} />
            </li>
          ))}
        </ul>
      )}
      <ContentBlocks locale={locale} items={data.content ?? []} />
      <section className="grid gap-3">
        <h2 className="text-xl font-semibold">{copy.listings}</h2>
        {data.listings.length === 0 ? (
          <p className="rounded-lg border border-dashed border-line bg-card p-4">{copy.emptyListings}</p>
        ) : (
          <ul className="grid gap-3">
            {data.listings.map((item) => {
              const text = item.texts?.[0];
              const place = data.neighborhoods.find((entry) => entry.id === item.neighborhood_id);
              return (
                <li key={item.id}>
                  <ListingCard
                    locale={locale}
                    id={item.id}
                    title={text?.title ?? ""}
                    description={text?.description}
                    neighborhood={place?.name}
                    typeName={item.type_code}
                    area={item.area_sqm}
                    rooms={item.rooms}
                    priceMinor={item.price_minor}
                    unit={item.price_unit}
                    transaction={item.transaction_type}
                  />
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </>
  );
}

function ContentBlocks({ locale, items }: { locale: Locale; items: PublicContent[] }) {
  if (items.length === 0) return null;
  const heading = locale === "en" ? "Published pages" : "صفحات منشورة";
  const place = locale === "en" ? "Neighborhood record" : "سجل الحي";
  return (
    <section className="grid gap-3">
      <h2 className="text-xl font-semibold">{heading}</h2>
      <ul className="grid gap-3">
        {items.map((item) => {
          const pageHref = contentPath(locale, item.slug);
          const linked = Boolean(item.neighborhood_id);
          return (
            <li key={item.id} className="rounded-lg border border-line bg-card p-4">
              <a href={pageHref} className="font-semibold">
                {item.title}
              </a>
              <p className="mt-1 text-sm text-ink/80">{item.body}</p>
              {linked ? (
                <a className="mt-2 inline-block text-sm underline" href={`${locale === "en" ? "/en/search" : "/search"}?neighborhood_id=${item.neighborhood_id}`}>
                  {place}
                </a>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function ServiceLine({
  locale,
  item,
}: {
  locale: Locale;
  item: HomePayload["neighborhoods"][number];
}) {
  const copy = chrome[locale];
  const enabled = [
    item.service_sale ? copy.sale : null,
    item.service_annual ? copy.annual : null,
    item.service_monthly ? copy.monthly : null,
    item.service_daily ? copy.daily : null,
  ].filter(Boolean);
  return (
    <p className="mt-3 text-sm">
      {copy.services}: {enabled.join(locale === "en" ? ", " : "، ")}
    </p>
  );
}
