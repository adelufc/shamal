import { chrome, money, priceUnit } from "@/lib/chrome";
import { propertyTypeName } from "@/lib/property-types";
import { loadListing } from "@/lib/public";

export async function ListingBody({ locale, id }: { locale: "ar" | "en"; id: string }) {
  const copy = chrome[locale];
  const searchHref = locale === "en" ? "/en/search" : "/search";
  let item;
  try {
    item = await loadListing(locale, id);
  } catch {
    return <p className="rounded-xl border border-line bg-card p-4">{copy.loadError}</p>;
  }
  if (!item) {
    return (
      <div className="grid gap-3 rounded-xl border border-dashed border-line bg-card p-4">
        <p>{copy.missingNeighborhood}</p>
        <a className="underline" href={searchHref}>
          {copy.backToSearch}
        </a>
      </div>
    );
  }
  const stay = item.transaction_type === "daily" || item.transaction_type === "monthly";
  return (
    <article className="grid gap-4">
      <div className="flex aspect-[4/3] items-center justify-center rounded-xl bg-olive-soft text-sm text-muted">{copy.noPhoto}</div>
      <h1 className="text-3xl font-semibold">{item.title || copy.details}</h1>
      <p className="text-lg">
        {money(item.price_minor, locale)} {priceUnit(locale, item.price_unit, item.transaction_type)}
      </p>
      <p className="text-sm text-muted">
        {[item.neighborhood_name, propertyTypeName(locale, item.type_code, item.type_name), item.area_sqm > 0 ? `${item.area_sqm} ${copy.area}` : "", item.rooms ? `${item.rooms} ${copy.rooms}` : ""]
          .filter(Boolean)
          .join(" · ")}
      </p>
      <p className="rounded-xl border border-line bg-olive-soft p-3 text-sm">{copy.approximate}</p>
      {item.description ? <p>{item.description}</p> : <p className="text-sm text-muted">{locale === "en" ? "No description in this language." : "لا يوجد وصف بهذه اللغة."}</p>}
      {item.owner_verified ? <p className="text-sm text-mint">{copy.verifiedOwner}</p> : null}
      <div className="flex flex-wrap gap-3">
        {stay ? (
          <a className="inline-flex min-h-11 items-center rounded-lg bg-action px-3 text-white" href={`${locale === "en" ? "/en/stay" : "/stay"}?listing=${item.id}`}>
            {copy.stay}
          </a>
        ) : null}
        <a className="inline-flex min-h-11 items-center rounded-lg border border-line bg-card px-3 text-ink" href={locale === "en" ? "/en/request" : "/request"}>
          {copy.request}
        </a>
        <a className="inline-flex min-h-11 items-center underline" href={`${searchHref}?neighborhood_id=${item.neighborhood_id}`}>
          {copy.mapLink}
        </a>
      </div>
    </article>
  );
}
