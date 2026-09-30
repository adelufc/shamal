"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { money, priceUnit } from "@/lib/chrome";
import type { Locale } from "@/lib/types";

type Place = {
  id: string;
  name: string;
  slug: string;
  lon: number;
  lat: number;
  geojson: string;
  boundary_is_official: boolean;
};

type Hit = {
  id: string;
  neighborhood_id: string;
  transaction_type: string;
  price_minor: number;
  price_unit: string;
  currency: string;
  title: string;
  description: string;
  area_sqm: number;
};

type Marker = Hit & { lon: number; lat: number };
type Cluster = {
  neighborhood_id: string;
  count: number;
  price_min_minor: number;
  price_max_minor: number;
  lon?: number;
  lat?: number;
};

type SearchPayload = {
  locale: Locale;
  query: {
    q: string;
    neighborhood_id: string;
    transaction: string;
    price_min_minor: number | null;
    price_max_minor: number | null;
    sort: string;
    in_area: boolean;
    west: number | null;
    south: number | null;
    east: number | null;
    north: number | null;
    bedrooms: number | null;
    amenities: string;
    verified: boolean;
  };
  listings: Hit[];
  markers: Marker[];
  clusters: Cluster[];
  marker_mode: "markers" | "clusters";
  total: number;
  page: number;
  page_size: number;
  places: Place[];
};

const copy = {
  ar: {
    title: "بحث وخريطة",
    lead: "القائمة والخريطة تستخدمان الطلب نفسه. تحريك الخريطة لا يغيّر النتائج إلا بعد «البحث في هذه المنطقة». السعر بالريال كما أدخله منشئ الإعلان.",
    query: "الاسم أو الحي",
    neighborhood: "الحي",
    anyNeighborhood: "كل الأحياء المنشورة",
    transaction: "نوع المعاملة",
    anyTransaction: "كل الأنواع",
    sale: "بيع",
    annual: "إيجار سنوي",
    monthly: "إيجار شهري",
    daily: "إيجار يومي",
    priceMin: "السعر من، بالريال",
    priceMax: "السعر إلى، بالريال",
    bedrooms: "غرف النوم، حداً أدنى",
    amenities: "المرافق",
    verified: "المالك موثّق",
    furnished: "مفروش",
    air_conditioning: "تكييف",
    internet: "إنترنت",
    parking: "موقف",
    bedding: "مفارش",
    self_entry: "دخول ذاتي",
    widenRooms: "إزالة غرف النوم",
    widenAmenities: "إزالة المرافق",
    widenVerified: "إزالة التوثيق",
    sort: "الترتيب",
    newest: "الأحدث نشراً",
    priceAsc: "السعر من الأقل",
    priceDesc: "السعر من الأعلى",
    areaAsc: "المساحة من الأصغر",
    areaDesc: "المساحة من الأكبر",
    apply: "بحث",
    stay: "تسعير الإقامة وتثبيتها",
    results: "نتيجة",
    empty: "لا توجد إعلانات منشورة تطابق هذه الفلاتر.",
    widenPrice: "إزالة حد السعر",
    widenNeighborhood: "إزالة الحي",
    widenText: "إزالة الاسم",
    widenArea: "إزالة حدود الخريطة",
    request: "طلب عقار",
    searchHere: "البحث في هذه المنطقة",
    clearArea: "إظهار كل المنطقة",
    zoomIn: "تقريب",
    zoomOut: "تبعيد",
    mapUnavailable: "تعذّر رسم الخريطة. القائمة والتصفية ما زالتا متاحتين.",
    loading: "جارٍ التحميل…",
    areaNote: "العلامات عند مركز الحي المنشور، وليست موقع العقار الدقيق. الحدود تقريبية للاختبار.",
    applied: "الفلاتر المستخدمة",
    page: "الصفحة",
  },
  en: {
    title: "Search and map",
    lead: "The list and the map use the same query. Moving the map does not change the results until you choose “Search this area”. The price is the amount entered by the person who created the listing, in SAR.",
    query: "Name or neighborhood",
    neighborhood: "Neighborhood",
    anyNeighborhood: "All published neighborhoods",
    transaction: "Transaction",
    anyTransaction: "All types",
    sale: "Sale",
    annual: "Annual rent",
    monthly: "Monthly rent",
    daily: "Daily rent",
    priceMin: "Price from, SAR",
    priceMax: "Price to, SAR",
    bedrooms: "Bedrooms, at least",
    amenities: "Amenities",
    verified: "Verified owner",
    furnished: "Furnished",
    air_conditioning: "Air conditioning",
    internet: "Internet",
    parking: "Parking",
    bedding: "Bedding",
    self_entry: "Self entry",
    widenRooms: "Remove bedrooms",
    widenAmenities: "Remove amenities",
    widenVerified: "Remove verification",
    sort: "Order",
    newest: "Newest published",
    priceAsc: "Price, low to high",
    priceDesc: "Price, high to low",
    areaAsc: "Area, small to large",
    areaDesc: "Area, large to small",
    apply: "Search",
    stay: "Price and hold this stay",
    results: "results",
    empty: "No published listings match these filters.",
    widenPrice: "Remove the price bound",
    widenNeighborhood: "Remove the neighborhood",
    widenText: "Remove the name",
    widenArea: "Remove the map bounds",
    request: "Request a property",
    searchHere: "Search this area",
    clearArea: "Show the whole area",
    zoomIn: "Zoom in",
    zoomOut: "Zoom out",
    mapUnavailable: "The map could not be drawn. The list and the filters are still available.",
    loading: "Loading…",
    areaNote: "Markers sit on the published neighborhood center, not the precise property location. The boundaries are approximate test masks.",
    applied: "Filters in this query",
    page: "Page",
  },
} as const;

type Box = { west: number; south: number; east: number; north: number };

const amenityKeys = ["furnished", "air_conditioning", "internet", "parking", "bedding", "self_entry"] as const;

export function SearchScreen({ locale }: { locale: Locale }) {
  const text = copy[locale];
  const params = useSearchParams();
  const router = useRouter();
  const [payload, setPayload] = useState<SearchPayload | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<unknown>(null);
  const [q, setQ] = useState(params.get("q") ?? "");
  const [neighborhood, setNeighborhood] = useState(params.get("neighborhood_id") ?? "");
  const [transaction, setTransaction] = useState(params.get("transaction") ?? "");
  const [priceMin, setPriceMin] = useState(riyalsFromMinor(params.get("price_min_minor")));
  const [priceMax, setPriceMax] = useState(riyalsFromMinor(params.get("price_max_minor")));
  const [sort, setSort] = useState(params.get("sort") ?? "newest");
  const [bedrooms, setBedrooms] = useState(params.get("bedrooms") ?? "");
  const [picked, setPicked] = useState(params.get("amenities") ?? "");
  const [verified, setVerified] = useState(params.get("verified") === "1");

  const base = locale === "en" ? "/en/search" : "/search";
  const requestHref = locale === "en" ? "/en/request" : "/request";

  useEffect(() => {
    let stop = false;
    setQ(params.get("q") ?? "");
    setNeighborhood(params.get("neighborhood_id") ?? "");
    setTransaction(params.get("transaction") ?? "");
    setPriceMin(riyalsFromMinor(params.get("price_min_minor")));
    setPriceMax(riyalsFromMinor(params.get("price_max_minor")));
    setSort(params.get("sort") ?? "newest");
    setBedrooms(params.get("bedrooms") ?? "");
    setPicked(params.get("amenities") ?? "");
    setVerified(params.get("verified") === "1");
    const search = new URLSearchParams(params.toString());
    search.set("locale", locale);
    setState("loading");
    api<SearchPayload>(`/api/v1/public/search?${search.toString()}`)
      .then((body) => {
        if (stop) return;
        setPayload(body);
        setError(null);
        setState("ready");
      })
      .catch((caught) => {
        if (stop) return;
        setError(caught);
        setState("error");
      });
    return () => {
      stop = true;
    };
  }, [locale, params]);

  function push(next: URLSearchParams) {
    const query = next.toString();
    router.push(query ? `${base}?${query}` : base);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    const next = new URLSearchParams();
    if (q.trim()) next.set("q", q.trim());
    if (neighborhood) next.set("neighborhood_id", neighborhood);
    if (transaction) next.set("transaction", transaction);
    const min = toMinor(priceMin);
    const max = toMinor(priceMax);
    if (min) next.set("price_min_minor", min);
    if (max) next.set("price_max_minor", max);
    if (sort && sort !== "newest") next.set("sort", sort);
    if (bedrooms.trim()) next.set("bedrooms", bedrooms.trim());
    if (picked.trim()) next.set("amenities", picked.trim());
    if (verified) next.set("verified", "1");
    if (params.get("in_area") === "1") {
      next.set("in_area", "1");
      for (const key of ["west", "south", "east", "north"]) {
        const value = params.get(key);
        if (value) next.set(key, value);
      }
    }
    push(next);
  }

  function without(keys: string[]) {
    const next = new URLSearchParams(params.toString());
    for (const key of keys) next.delete(key);
    push(next);
  }

  const applied = payload?.query;
  const path = `${base}${params.toString() ? `?${params.toString()}` : ""}`;

  return (
    <PublicFrame locale={locale} path={path}>
      <section className="grid gap-4">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <p className="text-sm text-ink/80">{text.lead}</p>
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4 md:grid-cols-2" onSubmit={submit}>
          <Label>
            {text.query}
            <Input value={q} onChange={(event) => setQ(event.target.value)} />
          </Label>
          <Label>
            {text.neighborhood}
            <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={neighborhood} onChange={(event) => setNeighborhood(event.target.value)}>
              <option value="">{text.anyNeighborhood}</option>
              {(payload?.places ?? []).map((place) => (
                <option key={place.id} value={place.id}>
                  {place.name}
                </option>
              ))}
            </select>
          </Label>
          <Label>
            {text.transaction}
            <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={transaction} onChange={(event) => setTransaction(event.target.value)}>
              <option value="">{text.anyTransaction}</option>
              <option value="sale">{text.sale}</option>
              <option value="annual">{text.annual}</option>
              <option value="monthly">{text.monthly}</option>
              <option value="daily">{text.daily}</option>
            </select>
          </Label>
          <Label>
            {text.sort}
            <select className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm" value={sort} onChange={(event) => setSort(event.target.value)}>
              <option value="newest">{text.newest}</option>
              <option value="price_asc">{text.priceAsc}</option>
              <option value="price_desc">{text.priceDesc}</option>
              <option value="area_asc">{text.areaAsc}</option>
              <option value="area_desc">{text.areaDesc}</option>
            </select>
          </Label>
          <Label>
            {text.priceMin}
            <Input value={priceMin} onChange={(event) => setPriceMin(event.target.value)} inputMode="decimal" />
          </Label>
          <Label>
            {text.priceMax}
            <Input value={priceMax} onChange={(event) => setPriceMax(event.target.value)} inputMode="decimal" />
          </Label>
          <Label>
            {text.bedrooms}
            <Input value={bedrooms} onChange={(event) => setBedrooms(event.target.value)} inputMode="numeric" />
          </Label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={verified} onChange={(event) => setVerified(event.target.checked)} />
            {text.verified}
          </label>
          <fieldset className="grid gap-2 md:col-span-2">
            <legend className="text-sm">{text.amenities}</legend>
            <div className="flex flex-wrap gap-3 text-sm">
              {amenityKeys.map((key) => {
                const selected = picked.split(",").map((item) => item.trim()).filter(Boolean);
                const on = selected.includes(key);
                return (
                  <label key={key} className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={(event) => {
                        const next = new Set(selected);
                        if (event.target.checked) next.add(key);
                        else next.delete(key);
                        setPicked(amenityKeys.filter((item) => next.has(item)).join(","));
                      }}
                    />
                    {text[key]}
                  </label>
                );
              })}
            </div>
          </fieldset>
          <div className="md:col-span-2">
            <Button type="submit">{text.apply}</Button>
          </div>
        </form>
        {state === "loading" ? <p className="text-sm">{text.loading}</p> : null}
        {state === "error" ? <Failure error={error} /> : null}
        {payload && state === "ready" ? (
          <>
            <p className="text-sm">
              {payload.total} {text.results}
              {payload.page > 1 ? ` · ${text.page} ${payload.page}` : ""}
            </p>
            {payload.total === 0 ? (
              <div className="grid gap-2 rounded-lg border border-dashed border-line bg-card p-4 text-sm">
                <p>{text.empty}</p>
                <p>{text.applied}</p>
                <FilterSummary locale={locale} query={applied} />
                <div className="flex flex-wrap gap-3">
                  {applied?.price_min_minor || applied?.price_max_minor ? (
                    <button className="underline" onClick={() => without(["price_min_minor", "price_max_minor"])}>
                      {text.widenPrice}
                    </button>
                  ) : null}
                  {applied?.neighborhood_id ? (
                    <button className="underline" onClick={() => without(["neighborhood_id"])}>
                      {text.widenNeighborhood}
                    </button>
                  ) : null}
                  {applied?.q ? (
                    <button className="underline" onClick={() => without(["q"])}>
                      {text.widenText}
                    </button>
                  ) : null}
                  {applied?.bedrooms ? (
                    <button className="underline" onClick={() => without(["bedrooms"])}>
                      {text.widenRooms}
                    </button>
                  ) : null}
                  {applied?.amenities ? (
                    <button className="underline" onClick={() => without(["amenities"])}>
                      {text.widenAmenities}
                    </button>
                  ) : null}
                  {applied?.verified ? (
                    <button className="underline" onClick={() => without(["verified"])}>
                      {text.widenVerified}
                    </button>
                  ) : null}
                  {applied?.in_area ? (
                    <button className="underline" onClick={() => without(["in_area", "west", "south", "east", "north"])}>
                      {text.widenArea}
                    </button>
                  ) : null}
                  <a className="underline" href={requestHref}>
                    {text.request}
                  </a>
                </div>
              </div>
            ) : null}
            <div className="grid gap-4 lg:grid-cols-2">
              <ul className="grid gap-3">
                {payload.listings.map((item) => (
                  <li key={item.id} className="rounded-lg border border-line bg-card p-4">
                    <h2 className="font-semibold">{item.title}</h2>
                    <p className="mt-1 text-sm text-ink/80">{item.description}</p>
                    <p className="mt-2 text-sm">
                      {money(item.price_minor, locale)} {priceUnit(locale, item.price_unit, item.transaction_type)}
                    </p>
                    {item.transaction_type === "daily" || item.transaction_type === "monthly" ? (
                      <a className="mt-2 inline-block text-sm underline" href={`${locale === "en" ? "/en/stay" : "/stay"}?listing=${item.id}`}>
                        {text.stay}
                      </a>
                    ) : null}
                  </li>
                ))}
              </ul>
              <ListingMap
                locale={locale}
                places={payload.places}
                markers={payload.markers}
                clusters={payload.clusters}
                forceClusters={payload.marker_mode === "clusters"}
                areaActive={Boolean(applied?.in_area)}
                onClearArea={() => without(["in_area", "west", "south", "east", "north"])}
                onSearchArea={(box) => {
                  const next = new URLSearchParams(params.toString());
                  next.set("in_area", "1");
                  next.set("west", box.west.toFixed(5));
                  next.set("south", box.south.toFixed(5));
                  next.set("east", box.east.toFixed(5));
                  next.set("north", box.north.toFixed(5));
                  push(next);
                }}
              />
            </div>
          </>
        ) : null}
      </section>
    </PublicFrame>
  );
}

function FilterSummary({ locale, query }: { locale: Locale; query: SearchPayload["query"] | undefined }) {
  if (!query) return null;
  const parts = [query.q, query.transaction, query.neighborhood_id].filter(Boolean);
  if (query.price_min_minor) parts.push(money(query.price_min_minor, locale));
  if (query.price_max_minor) parts.push(money(query.price_max_minor, locale));
  if (query.bedrooms) parts.push(String(query.bedrooms));
  if (query.amenities) parts.push(query.amenities);
  if (query.verified) parts.push(locale === "en" ? "Verified" : "موثّق");
  if (query.in_area) parts.push(locale === "en" ? "Map area" : "نطاق الخريطة");
  return <p>{parts.join(" · ") || (locale === "en" ? "No extra filters." : "بلا فلاتر إضافية.")}</p>;
}

function ListingMap({
  locale,
  places,
  markers,
  clusters,
  forceClusters,
  areaActive,
  onClearArea,
  onSearchArea,
}: {
  locale: Locale;
  places: Place[];
  markers: Marker[];
  clusters: Cluster[];
  forceClusters: boolean;
  areaActive: boolean;
  onClearArea: () => void;
  onSearchArea: (box: Box) => void;
}) {
  const text = copy[locale];
  const [zoom, setZoom] = useState(0);
  const drawn = useMemo(() => {
    try {
      return shapes(places);
    } catch {
      return null;
    }
  }, [places]);
  const dots = useMemo(() => mapDots(markers, clusters, forceClusters, zoom), [markers, clusters, forceClusters, zoom]);
  if (!drawn) return <p className="rounded-lg border border-line bg-card p-4 text-sm">{text.mapUnavailable}</p>;
  const view = zoomBox(drawn.bounds, zoom);
  const clustered = forceClusters || zoom === 0;
  return (
    <section className="grid gap-2">
      <svg viewBox="0 0 640 420" role="img" className="h-80 w-full rounded-lg border border-line bg-olive-soft" aria-label={text.title}>
        {drawn.rings.map((ring, index) => (
          <polygon key={index} points={ring.map(([lon, lat]) => point(lon, lat, view)).join(" ")} fill="#fffcf8" stroke="#3f4f3a" strokeWidth="1" />
        ))}
        {dots.map((item) => {
          const [x, y] = point(item.lon, item.lat, view).split(",");
          if (clustered) {
            return (
              <g key={item.key}>
                <circle cx={x} cy={y} r="14" fill="#3f4f3a" />
                <text x={x} y={y} textAnchor="middle" dy="4" fill="#fffcf8" fontSize="12">
                  {item.count}
                </text>
              </g>
            );
          }
          return <circle key={item.key} cx={x} cy={y} r="5" fill="#8a4b2f" />;
        })}
      </svg>
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="ghost" onClick={() => setZoom((value) => Math.min(value + 1, 4))}>
          {text.zoomIn}
        </Button>
        <Button type="button" variant="ghost" onClick={() => setZoom((value) => Math.max(value - 1, 0))}>
          {text.zoomOut}
        </Button>
        <Button type="button" onClick={() => onSearchArea(view)}>
          {text.searchHere}
        </Button>
        {areaActive ? (
          <Button type="button" variant="ghost" onClick={onClearArea}>
            {text.clearArea}
          </Button>
        ) : null}
      </div>
      <p className="text-sm text-ink/70">{text.areaNote}</p>
    </section>
  );
}

function mapDots(markers: Marker[], clusters: Cluster[], forceClusters: boolean, zoom: number) {
  if (forceClusters) {
    return clusters
      .filter((item) => Number.isFinite(item.lon) && Number.isFinite(item.lat))
      .map((item) => ({ key: item.neighborhood_id, lon: item.lon as number, lat: item.lat as number, count: item.count }));
  }
  if (zoom === 0) {
    const groups = new Map<string, { key: string; lon: number; lat: number; count: number }>();
    for (const marker of markers) {
      const current = groups.get(marker.neighborhood_id) ?? {
        key: marker.neighborhood_id,
        lon: marker.lon,
        lat: marker.lat,
        count: 0,
      };
      current.count += 1;
      groups.set(marker.neighborhood_id, current);
    }
    return [...groups.values()];
  }
  return markers.map((item) => ({ key: item.id, lon: item.lon, lat: item.lat, count: 1 }));
}

function shapes(places: Place[]) {
  const rings: number[][][] = [];
  const bounds: Box = { west: 180, south: 90, east: -180, north: -90 };
  for (const place of places) {
    if (!place.geojson) continue;
    const geometry = JSON.parse(place.geojson) as { type: string; coordinates: number[][][] | number[][][][] };
    const polygons = geometry.type === "Polygon" ? [geometry.coordinates as number[][][]] : (geometry.coordinates as number[][][][]);
    for (const polygon of polygons) {
      const ring = polygon[0];
      if (!ring) continue;
      rings.push(ring);
      for (const pair of ring) {
        bounds.west = Math.min(bounds.west, pair[0]);
        bounds.east = Math.max(bounds.east, pair[0]);
        bounds.south = Math.min(bounds.south, pair[1]);
        bounds.north = Math.max(bounds.north, pair[1]);
      }
    }
  }
  if (rings.length === 0) return null;
  const padX = (bounds.east - bounds.west) * 0.08 || 0.01;
  const padY = (bounds.north - bounds.south) * 0.08 || 0.01;
  bounds.west -= padX;
  bounds.east += padX;
  bounds.south -= padY;
  bounds.north += padY;
  return { rings, bounds };
}

function zoomBox(bounds: Box, zoom: number): Box {
  const scale = 2 ** zoom;
  const width = (bounds.east - bounds.west) / scale;
  const height = (bounds.north - bounds.south) / scale;
  const cx = (bounds.east + bounds.west) / 2;
  const cy = (bounds.north + bounds.south) / 2;
  return { west: cx - width / 2, east: cx + width / 2, south: cy - height / 2, north: cy + height / 2 };
}

function point(lon: number, lat: number, box: Box) {
  const x = ((lon - box.west) / (box.east - box.west)) * 640;
  const y = ((box.north - lat) / (box.north - box.south)) * 420;
  return `${x.toFixed(1)},${y.toFixed(1)}`;
}

function riyalsFromMinor(raw: string | null) {
  if (!raw) return "";
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) return "";
  return String(value / 100);
}

function toMinor(raw: string) {
  if (raw.trim() === "") return "";
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) return "";
  return String(Math.round(value * 100));
}
