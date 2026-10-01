"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Failure, api } from "@/components/admin-shell";
import { PublicFrame } from "@/components/public-frame";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { ListingCard } from "@/components/listing-card";
import { money } from "@/lib/chrome";
import { propertyTypeName } from "@/lib/property-types";
import { StreetMap, type StreetMapHandle } from "@/components/street-map";
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
  rooms: number | null;
  type_code: string;
  owner_verified: boolean;
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
    lead: "حرّك الخريطة ثم اختر البحث في هذه المنطقة. السعر بالريال كما أدخله منشئ الإعلان.",
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
    empty: "لم نجد عقارات تطابق اختياراتك. جرّب إزالة بعض المرشحات",
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
    areaNote: "موقع تقريبي داخل الحي. العلامات عند مركز الحي، وليست موقع العقار.",
    mapScope: "تجمعات الخريطة تشمل كل النتائج المطابقة، والقائمة تعرض الصفحة الحالية.",
    extra: "مرشحات إضافية",
    range: "عرض",
    of: "من",
    to: "إلى",
    previous: "السابق",
    next: "التالي",
    list: "القائمة",
    map: "الخريطة",
    applied: "الفلاتر المستخدمة",
    page: "الصفحة",
  },
  en: {
    title: "Search and map",
    lead: "Move the map, then choose search this area. The price is the amount entered by the person who created the listing, in SAR.",
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
    empty: "No properties match these choices. Try removing a filter.",
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
    areaNote: "Approximate location inside the neighborhood. Markers sit on the neighborhood center, not the property.",
    mapScope: "Map groups cover every matching result. The list shows the current page.",
    extra: "More filters",
    range: "Showing",
    of: "of",
    to: "to",
    previous: "Previous",
    next: "Next",
    list: "List",
    map: "Map",
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
  const [pane, setPane] = useState<"list" | "map">("list");

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
      .then((body) => attachMarkerTypes(body, search))
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
    next.delete("page");
    push(next);
  }

  function goTo(page: number) {
    const next = new URLSearchParams(params.toString());
    if (page <= 1) next.delete("page");
    else next.set("page", String(page));
    push(next);
  }

  const applied = payload?.query;
  const path = `${base}${params.toString() ? `?${params.toString()}` : ""}`;

  return (
    <PublicFrame locale={locale} path={path}>
      <section className="grid gap-4">
        <h1 className="text-2xl font-semibold">{text.title}</h1>
        <p className="text-sm text-ink/80">{text.lead}</p>
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4 md:grid-cols-2 [&_input]:border-action [&_input]:accent-action [&_select]:border-action" onSubmit={submit}>
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
          <details className="grid gap-3 rounded-lg bg-olive-soft p-3 md:col-span-2">
            <summary className="min-h-11 cursor-pointer">{text.extra}</summary>
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
          </details>
          <div className="md:col-span-2">
            <Button type="submit">{text.apply}</Button>
          </div>
        </form>
        {state === "loading" ? <p className="text-sm">{text.loading}</p> : null}
        {state === "error" ? <Failure error={error} /> : null}
        {payload && state === "ready" ? (
          <>
            <p className="text-sm">
              {payload.total === 0
                ? `0 ${text.results}`
                : `${text.range} ${(payload.page - 1) * payload.page_size + 1} ${text.to} ${Math.min(payload.page * payload.page_size, payload.total)} ${text.of} ${payload.total}`}
            </p>
            <p className="text-sm text-muted">{text.mapScope}</p>
            <div className="flex gap-2 lg:hidden">
              <Button type="button" variant={pane === "list" ? "primary" : "ghost"} onClick={() => setPane("list")}>
                {text.list}
              </Button>
              <Button type="button" variant={pane === "map" ? "primary" : "ghost"} onClick={() => setPane("map")}>
                {text.map}
              </Button>
            </div>
            {payload.total === 0 ? (
              <div className="grid gap-2 rounded-lg border border-dashed border-line bg-card p-4 text-sm">
                <p>{text.empty}</p>
                <p>{text.applied}</p>
                <FilterSummary locale={locale} query={applied} />
                <div className="flex flex-wrap gap-3 text-action">
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
              <div className={pane === "map" ? "hidden lg:grid" : "grid gap-3"}>
              <ul className="grid gap-3">
                {payload.listings.map((item) => {
                  const place = payload.places.find((entry) => entry.id === item.neighborhood_id);
                  return (
                    <li key={item.id}>
                      <ListingCard
                        locale={locale}
                        id={item.id}
                        title={item.title}
                        description={item.description}
                        neighborhood={place?.name}
                        typeName={propertyTypeName(locale, item.type_code)}
                        area={item.area_sqm}
                        rooms={item.rooms}
                        priceMinor={item.price_minor}
                        unit={item.price_unit}
                        transaction={item.transaction_type}
                        verified={item.owner_verified}
                      />
                    </li>
                  );
                })}
              </ul>
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="ghost" disabled={payload.page <= 1} onClick={() => goTo(payload.page - 1)}>
                  {text.previous}
                </Button>
                <Button type="button" variant="ghost" disabled={payload.page * payload.page_size >= payload.total} onClick={() => goTo(payload.page + 1)}>
                  {text.next}
                </Button>
              </div>
              </div>
              <div className={pane === "list" ? "hidden lg:block" : "block"}>
              <ListingMap
                locale={locale}
                places={payload.places}
                focusId={neighborhood}
                markers={payload.markers}
                clusters={payload.clusters}
                forceClusters={payload.marker_mode === "clusters"}
                areaActive={Boolean(applied?.in_area)}
                onClearArea={() => without(["in_area", "west", "south", "east", "north"])}
                onSearchArea={(box) => {
                  const next = new URLSearchParams(params.toString());
                  next.delete("page");
                  next.set("in_area", "1");
                  next.set("west", box.west.toFixed(5));
                  next.set("south", box.south.toFixed(5));
                  next.set("east", box.east.toFixed(5));
                  next.set("north", box.north.toFixed(5));
                  push(next);
                }}
              />
              </div>
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
  focusId,
  markers,
  clusters,
  forceClusters,
  areaActive,
  onClearArea,
  onSearchArea,
}: {
  locale: Locale;
  places: Place[];
  focusId: string;
  markers: Marker[];
  clusters: Cluster[];
  forceClusters: boolean;
  areaActive: boolean;
  onClearArea: () => void;
  onSearchArea: (box: Box) => void;
}) {
  const text = copy[locale];
  const mapRef = useRef<StreetMapHandle>(null);
  const [detail, setDetail] = useState(false);
  const drawn = useMemo(() => {
    try {
      return shapes(places);
    } catch {
      return null;
    }
  }, [places]);
  const resultView = useMemo(() => resultBounds(places, markers, clusters), [places, markers, clusters]);
  useEffect(() => {
    if (!drawn) return;
    if (focusId) {
      const box = storedBounds(places, focusId);
      if (box) mapRef.current?.frame(box, true);
      return;
    }
    mapRef.current?.frame(resultView ?? drawn.bounds, false);
  }, [focusId, places, drawn, resultView]);
  const dots = useMemo(
    () => mapDots(markers, clusters, forceClusters, detail ? 1 : 0, locale),
    [markers, clusters, forceClusters, detail, locale],
  );
  if (!drawn) return <p className="rounded-lg border border-line bg-card p-4 text-sm">{text.mapUnavailable}</p>;
  return (
    <section className="grid gap-2">
      <StreetMap ref={mapRef} label={text.title} bounds={drawn.bounds} rings={drawn.rings} dots={dots} onDetail={setDetail} />
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="ghost" onClick={() => mapRef.current?.zoomIn()}>
          {text.zoomIn}
        </Button>
        <Button type="button" variant="ghost" onClick={() => mapRef.current?.zoomOut()}>
          {text.zoomOut}
        </Button>
        <Button
          type="button"
          onClick={() => {
            const box = mapRef.current?.bounds();
            if (box) onSearchArea(box);
          }}
        >
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

async function attachMarkerTypes(body: SearchPayload, search: URLSearchParams) {
  const types = new Map(body.listings.map((item) => [item.id, item.type_code]));
  const missing = body.markers.some((item) => !item.type_code && !types.has(item.id));
  if (missing && body.total > body.listings.length) {
    const pages = Math.min(Math.ceil(body.total / 100), 5);
    for (let page = 1; page <= pages; page += 1) {
      const wide = new URLSearchParams(search);
      wide.set("page", String(page));
      wide.set("page_size", "100");
      try {
        const all = await api<SearchPayload>(`/api/v1/public/search?${wide.toString()}`);
        for (const item of all.listings) types.set(item.id, item.type_code);
      } catch {
        break;
      }
    }
  }
  return {
    ...body,
    markers: body.markers.map((item) => ({ ...item, type_code: item.type_code || types.get(item.id) || "" })),
  };
}

function mapDots(markers: Marker[], clusters: Cluster[], forceClusters: boolean, zoom: number, locale: Locale) {
  const labelFor = (codes: string[]) => {
    const unique = [...new Set(codes.filter(Boolean))];
    return unique.length === 1 ? propertyTypeName(locale, unique[0]) : "";
  };
  if (forceClusters) {
    return clusters
      .filter((item) => Number.isFinite(item.lon) && Number.isFinite(item.lat))
      .map((item) => ({ key: item.neighborhood_id, lon: item.lon as number, lat: item.lat as number, count: item.count, label: "", numbered: true }));
  }
  if (zoom === 0) {
    const groups = new Map<string, { key: string; lon: number; lat: number; count: number; codes: string[] }>();
    for (const marker of markers) {
      const current = groups.get(marker.neighborhood_id) ?? {
        key: marker.neighborhood_id,
        lon: marker.lon,
        lat: marker.lat,
        count: 0,
        codes: [],
      };
      current.count += 1;
      current.codes.push(marker.type_code);
      groups.set(marker.neighborhood_id, current);
    }
    return [...groups.values()].map((item) => ({ key: item.key, lon: item.lon, lat: item.lat, count: item.count, label: labelFor(item.codes), numbered: true }));
  }
  return markers.map((item) => ({ key: item.id, lon: item.lon, lat: item.lat, count: 1, label: propertyTypeName(locale, item.type_code), numbered: false }));
}

function storedBounds(places: Place[], id: string): Box | null {
  const place = places.find((item) => item.id === id);
  if (!place?.geojson) return null;
  try {
    return geometryBounds(place.geojson);
  } catch {
    return null;
  }
}

function resultBounds(places: Place[], markers: Marker[], clusters: Cluster[]): Box | null {
  const ids = new Set<string>();
  for (const marker of markers) ids.add(marker.neighborhood_id);
  for (const cluster of clusters) ids.add(cluster.neighborhood_id);
  let box: Box | null = null;
  for (const id of ids) {
    const place = places.find((item) => item.id === id);
    const next = place?.geojson ? safeGeometryBounds(place.geojson) : null;
    box = unionBox(box, next);
  }
  return box;
}

function safeGeometryBounds(geojson: string): Box | null {
  try {
    return geometryBounds(geojson);
  } catch {
    return null;
  }
}

function geometryBounds(geojson: string): Box | null {
  const geometry = JSON.parse(geojson) as { type: string; coordinates: number[][][] | number[][][][] };
  const polygons = geometry.type === "Polygon" ? [geometry.coordinates as number[][][]] : geometry.type === "MultiPolygon" ? (geometry.coordinates as number[][][][]) : [];
  const bounds: Box = { west: 180, south: 90, east: -180, north: -90 };
  let found = false;
  for (const polygon of polygons) {
    const ring = polygon[0];
    if (!ring) continue;
    for (const pair of ring) {
      if (!Number.isFinite(pair[0]) || !Number.isFinite(pair[1])) continue;
      found = true;
      bounds.west = Math.min(bounds.west, pair[0]);
      bounds.east = Math.max(bounds.east, pair[0]);
      bounds.south = Math.min(bounds.south, pair[1]);
      bounds.north = Math.max(bounds.north, pair[1]);
    }
  }
  return found ? bounds : null;
}

function unionBox(left: Box | null, right: Box | null): Box | null {
  if (!left) return right;
  if (!right) return left;
  return {
    west: Math.min(left.west, right.west),
    east: Math.max(left.east, right.east),
    south: Math.min(left.south, right.south),
    north: Math.max(left.north, right.north),
  };
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
