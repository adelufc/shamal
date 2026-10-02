import { ListingCard } from "@/components/listing-card";
import { chrome, homePath } from "@/lib/chrome";
import { propertyTypeName } from "@/lib/property-types";
import { type NeighborhoodResult } from "@/lib/public";

export function NeighborhoodBody({ locale, result }: { locale: "ar" | "en"; result: NeighborhoodResult }) {
  const copy = chrome[locale];
  const searchHref = locale === "en" ? "/en/search" : "/search";
  if (result.state === "error") {
    return <p className="rounded-xl border border-line bg-card p-4">{copy.loadError}</p>;
  }
  if (result.state === "missing") {
    return (
      <div className="grid gap-3 rounded-xl border border-dashed border-line bg-card p-4">
        <p>{copy.missingNeighborhood}</p>
        <a className="underline" href={searchHref}>
          {copy.backToSearch}
        </a>
      </div>
    );
  }
  if (result.state === "unpublished") {
    return (
      <div className="grid gap-3 rounded-xl border border-dashed border-line bg-card p-4">
        <p>{copy.unpublishedNeighborhood}</p>
        <a className="underline" href={searchHref}>
          {copy.backToSearch}
        </a>
      </div>
    );
  }
  const payload = result.payload;
  const text = payload.neighborhood.texts?.[0];
  const note = locale === "en" ? payload.neighborhood.boundary_note_en : payload.neighborhood.boundary_note_ar;
  const services = [
    payload.neighborhood.service_sale ? copy.sale : "",
    payload.neighborhood.service_annual ? copy.annual : "",
    payload.neighborhood.service_monthly ? copy.monthly : "",
    payload.neighborhood.service_daily ? copy.daily : "",
  ].filter(Boolean);
  return (
    <>
      <a href={homePath(locale)} className="text-sm">
        {copy.back}
      </a>
      <h1 className="text-3xl font-semibold">{text?.name}</h1>
      <p className="text-sm text-muted">{text?.description}</p>
      <p className="rounded-xl border border-line bg-olive-soft p-3 text-sm">{note || copy.boundary}</p>
      {services.length > 0 ? (
        <p className="text-sm">
          {copy.services}: {services.join(" · ")}
        </p>
      ) : null}
      <a className="text-sm underline" href={`${searchHref}?neighborhood_id=${payload.neighborhood.id}`}>
        {copy.mapLink}
      </a>
      <section className="grid gap-3">
        <h2 className="text-xl font-semibold">{copy.listings}</h2>
        {payload.listings.length === 0 ? (
          <p className="rounded-xl border border-dashed border-line bg-card p-4">{copy.emptyListings}</p>
        ) : (
          <ul className="grid gap-3">
            {payload.listings.map((item) => (
              <li key={item.id}>
                <ListingCard
                  locale={locale}
                  id={item.id}
                  title={item.texts?.[0]?.title ?? ""}
                  description={item.texts?.[0]?.description}
                  neighborhood={text?.name}
                  typeName={propertyTypeName(locale, item.type_code)}
                  area={item.area_sqm}
                  rooms={item.rooms}
                  priceMinor={item.price_minor}
                  unit={item.price_unit}
                  transaction={item.transaction_type}
                />
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}
