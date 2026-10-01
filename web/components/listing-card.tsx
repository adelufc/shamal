import { chrome, listingPath, money, priceUnit } from "@/lib/chrome";
import type { Locale } from "@/lib/types";

export function ListingCard({
  locale,
  id,
  title,
  description,
  neighborhood,
  typeName,
  area,
  rooms,
  priceMinor,
  unit,
  transaction,
  verified,
}: {
  locale: Locale;
  id: string;
  title: string;
  description?: string;
  neighborhood?: string;
  typeName?: string;
  area?: number;
  rooms?: number | null;
  priceMinor: number;
  unit: string;
  transaction: string;
  verified?: boolean;
}) {
  const copy = chrome[locale];
  const href = listingPath(locale, id);
  const stay = transaction === "daily" || transaction === "monthly";
  const facts = [
    neighborhood,
    typeName,
    area && area > 0 ? `${area} ${copy.area}` : "",
    rooms && rooms > 0 ? `${rooms} ${copy.rooms}` : "",
  ].filter(Boolean);
  return (
    <article className="overflow-hidden rounded-xl border border-line bg-card">
      <div className="flex aspect-[4/3] items-center justify-center bg-olive-soft text-sm text-muted">{copy.noPhoto}</div>
      <div className="grid gap-2 p-4">
        <h3 className="text-lg font-semibold">
          <a href={href}>{title || copy.details}</a>
        </h3>
        <p className="text-base">
          {money(priceMinor, locale)} {priceUnit(locale, unit, transaction)}
        </p>
        {facts.length > 0 ? <p className="text-sm text-muted">{facts.join(" · ")}</p> : null}
        {description ? <p className="text-sm text-muted">{description}</p> : null}
        <p className="text-sm text-muted">{copy.approximate}</p>
        {verified ? <p className="text-sm text-mint">{copy.verifiedOwner}</p> : null}
        <div className="flex flex-wrap gap-3 text-sm">
          <a className="underline" href={href}>
            {copy.details}
          </a>
          {stay ? (
            <a className="underline" href={`${locale === "en" ? "/en/stay" : "/stay"}?listing=${id}`}>
              {copy.stay}
            </a>
          ) : null}
          <a className="underline" href={`${locale === "en" ? "/en/request" : "/request"}`}>
            {copy.request}
          </a>
        </div>
      </div>
    </article>
  );
}
