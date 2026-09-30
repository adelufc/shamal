import type { HomePayload, Locale, PublicContent } from "./types";

function origin() {
  return process.env.SHAMAL_API_ORIGIN ?? "http://127.0.0.1:4318";
}

export async function loadHome(locale: Locale): Promise<HomePayload> {
  const response = await fetch(`${origin()}/api/v1/public/home?locale=${locale}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("home");
  }
  return response.json();
}

export async function loadNeighborhood(locale: Locale, slug: string) {
  const response = await fetch(
    `${origin()}/api/v1/public/neighborhoods/${encodeURIComponent(slug)}?locale=${locale}`,
    { cache: "no-store" },
  );
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new Error("neighborhood");
  }
  return response.json() as Promise<{
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
  }>;
}

export async function loadContent(locale: Locale, slug: string): Promise<PublicContent | null> {
  const response = await fetch(
    `${origin()}/api/v1/public/content?locale=${locale}&slug=${encodeURIComponent(slug)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw new Error("content");
  }
  const body = (await response.json()) as { items: PublicContent[] };
  return body.items[0] ?? null;
}
