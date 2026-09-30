import type { HomePayload, Locale, PublicContent } from "./types";

function origin() {
  return process.env.SHAMAL_API_ORIGIN ?? "http://127.0.0.1:4318";
}

function safeDecode(value: string) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export async function loadHome(locale: Locale): Promise<HomePayload> {
  const response = await fetch(`${origin()}/api/v1/public/home?locale=${locale}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("home");
  }
  return response.json();
}

export type NeighborhoodPayload = {
  neighborhood: {
    id: string;
    status: string;
    boundary_is_official: boolean;
    boundary_note_ar: string;
    boundary_note_en: string;
    service_sale: boolean;
    service_annual: boolean;
    service_monthly: boolean;
    service_daily: boolean;
    texts: { locale: string; name: string; slug: string; description: string }[];
  };
  listings: HomePayload["listings"];
};

export type NeighborhoodResult =
  | { state: "ready"; payload: NeighborhoodPayload }
  | { state: "missing" }
  | { state: "unpublished" }
  | { state: "error" };

export async function loadNeighborhood(locale: Locale, slug: string): Promise<NeighborhoodResult> {
  const decoded = safeDecode(slug);
  try {
    const response = await fetch(
      `${origin()}/api/v1/public/neighborhoods/${encodeURIComponent(decoded)}?locale=${locale}`,
      { cache: "no-store" },
    );
    if (response.status === 404) {
      const body = (await response.json().catch(() => ({}))) as { code?: string };
      return body.code === "unpublished" ? { state: "unpublished" } : { state: "missing" };
    }
    if (!response.ok) return { state: "error" };
    return { state: "ready", payload: (await response.json()) as NeighborhoodPayload };
  } catch {
    return { state: "error" };
  }
}

export type PublicListingDetail = {
  id: string;
  neighborhood_id: string;
  neighborhood_name: string;
  transaction_type: string;
  price_minor: number;
  price_unit: string;
  currency: string;
  title: string;
  description: string;
  area_sqm: number;
  rooms: number | null;
  type_code: string;
  type_name: string;
  owner_verified: boolean;
  approximate_location: boolean;
};

export async function loadListing(locale: Locale, id: string): Promise<PublicListingDetail | null> {
  const response = await fetch(`${origin()}/api/v1/public/listings/${encodeURIComponent(id)}?locale=${locale}`, {
    cache: "no-store",
  });
  if (response.status === 404) return null;
  if (!response.ok) throw new Error("listing");
  return response.json();
}

export async function loadContent(locale: Locale, slug: string): Promise<PublicContent | null> {
  const response = await fetch(
    `${origin()}/api/v1/public/content?locale=${locale}&slug=${encodeURIComponent(safeDecode(slug))}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw new Error("content");
  }
  const body = (await response.json()) as { items: PublicContent[] };
  return body.items[0] ?? null;
}
