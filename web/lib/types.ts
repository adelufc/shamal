export type Locale = "ar" | "en";

export type NeighborhoodCard = {
  id: string;
  name: string;
  slug: string;
  listing_count: number;
  service_sale: boolean;
  service_annual: boolean;
  service_monthly: boolean;
  service_daily: boolean;
  boundary_is_official: boolean;
};

export type PublicListing = {
  id: string;
  neighborhood_id: string;
  transaction_type: string;
  price_minor: number;
  price_unit: string;
  currency: string;
  texts?: { locale: string; title: string; description: string }[];
  precise_lon?: number | null;
  precise_lat?: number | null;
};

export type PublicContent = {
  id: string;
  kind: string;
  slug: string;
  neighborhood_id: string;
  locale: string;
  title: string;
  body: string;
};

export type HomePayload = {
  locale: Locale;
  city: { name: string; currency: string };
  neighborhoods: NeighborhoodCard[];
  listings: PublicListing[];
  counts: { published_neighborhoods: number; published_listings: number };
  content?: PublicContent[];
};

export type ApiFailure = {
  code?: string;
  message?: string;
  fields?: Record<string, string>;
};

export type SessionUser = {
  id: string;
  display_name: string;
  email: string;
  company_id: string;
  context: string;
  step_up: boolean;
  actions: Record<string, string[]>;
};
