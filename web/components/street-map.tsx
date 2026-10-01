"use client";

import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import type { LayerGroup, Map as LeafletMap } from "leaflet";
import "leaflet/dist/leaflet.css";

export type MapBox = { west: number; south: number; east: number; north: number };
export type MapDot = { key: string; lon: number; lat: number; count: number; label: string; numbered: boolean };

export type StreetMapHandle = {
  zoomIn: () => void;
  zoomOut: () => void;
  bounds: () => MapBox | null;
  frame: (box: MapBox, place: boolean) => void;
};

type Props = {
  label: string;
  bounds: MapBox;
  rings: number[][][];
  dots: MapDot[];
  onDetail: (detail: boolean) => void;
};

const TILES = "https://tile.openstreetmap.org/{z}/{x}/{y}.png";

export const StreetMap = forwardRef<StreetMapHandle, Props>(function StreetMap({ label, bounds, rings, dots, onDetail }, ref) {
  const host = useRef<HTMLDivElement>(null);
  const mapRef = useRef<LeafletMap | null>(null);
  const groupRef = useRef<LayerGroup | null>(null);
  const leafletRef = useRef<typeof import("leaflet") | null>(null);
  const startZoom = useRef<number | null>(null);
  const frameRef = useRef<{ box: MapBox; place: boolean } | null>(null);
  const onDetailRef = useRef(onDetail);
  const ringsRef = useRef(rings);
  const dotsRef = useRef(dots);
  onDetailRef.current = onDetail;
  ringsRef.current = rings;
  dotsRef.current = dots;

  useImperativeHandle(ref, () => ({
    zoomIn: () => mapRef.current?.zoomIn(),
    zoomOut: () => mapRef.current?.zoomOut(),
    bounds: () => {
      const box = mapRef.current?.getBounds();
      if (!box) return null;
      return { west: box.getWest(), south: box.getSouth(), east: box.getEast(), north: box.getNorth() };
    },
    frame: (box, place) => {
      frameRef.current = { box, place };
      const map = mapRef.current;
      if (!map || !host.current || host.current.clientWidth === 0) return;
      showFrame(map, box, place, startZoom);
    },
  }));

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    let cancelled = false;
    let map: LeafletMap | null = null;
    let observer: ResizeObserver | null = null;

    (async () => {
      const loaded = await import("leaflet");
      const L = loaded.default ?? loaded;
      if (cancelled || !host.current) return;
      leafletRef.current = L;
      map = L.map(host.current, {
        zoomControl: false,
        attributionControl: true,
        minZoom: 9,
        maxZoom: 18,
      });
      if (cancelled) {
        map.remove();
        return;
      }
      mapRef.current = map;
      L.tileLayer(TILES, {
        maxZoom: 19,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
      }).addTo(map);
      const group = L.layerGroup().addTo(map);
      groupRef.current = group;
      paint(L, group, ringsRef.current, dotsRef.current);
      let fitted = false;
      const fit = () => {
        if (!map || !host.current || host.current.clientWidth === 0) return;
        map.invalidateSize();
        if (fitted) return;
        const chosen = frameRef.current;
        if (chosen) showFrame(map, chosen.box, chosen.place, startZoom);
        else {
          showFrame(map, bounds, false, startZoom);
        }
        fitted = true;
      };
      map.on("zoomend", () => {
        const base = startZoom.current;
        if (base == null || !map) return;
        onDetailRef.current(map.getZoom() > base + 0.25);
      });
      observer = new ResizeObserver(() => fit());
      observer.observe(host.current);
      fit();
    })();

    return () => {
      cancelled = true;
      observer?.disconnect();
      map?.remove();
      mapRef.current = null;
      groupRef.current = null;
      leafletRef.current = null;
    };
  }, [bounds.west, bounds.south, bounds.east, bounds.north]);

  useEffect(() => {
    const L = leafletRef.current;
    const group = groupRef.current;
    if (!L || !group) return;
    paint(L, group, rings, dots);
  }, [rings, dots]);

  return (
    <div className="relative h-80 w-full overflow-hidden rounded-lg border border-line">
      <div ref={host} dir="ltr" className="absolute inset-0" aria-label={label} />
    </div>
  );
});

function showFrame(map: LeafletMap, box: MapBox, place: boolean, startZoom: { current: number | null }) {
  map.fitBounds(
    [
      [box.south, box.west],
      [box.north, box.east],
    ],
    { padding: [28, 28], maxZoom: place ? 16 : 14, animate: false },
  );
  if (!place && map.getZoom() < 12) map.setZoom(12, { animate: false });
  if (startZoom.current == null) startZoom.current = map.getZoom();
}

function paint(L: typeof import("leaflet"), group: LayerGroup, rings: number[][][], dots: MapDot[]) {
  group.clearLayers();
  for (const ring of rings) {
    const latlngs = ring
      .filter((pair) => pair.length >= 2 && Number.isFinite(pair[0]) && Number.isFinite(pair[1]))
      .map(([lon, lat]) => L.latLng(lat, lon));
    if (latlngs.length < 3) continue;
    L.polygon(latlngs, {
      color: "#047857",
      weight: 1.5,
      opacity: 0.9,
      fill: true,
      fillColor: "#10b981",
      fillOpacity: 0.38,
      interactive: false,
    }).addTo(group);
  }
  for (const dot of dots) {
    if (!Number.isFinite(dot.lon) || !Number.isFinite(dot.lat)) continue;
    const label = dot.label ? `<span class="shamal-label">${escapeHtml(dot.label)}</span>` : "";
    const mark = dot.numbered ? `<span class="shamal-count">${dot.count}</span>` : `<span class="shamal-dot"></span>`;
    const icon = L.divIcon({
      className: "shamal-marker",
      html: `<div class="shamal-pin">${mark}${label}</div>`,
      iconSize: dot.numbered ? [28, 28] : [12, 12],
      iconAnchor: dot.numbered ? [14, 14] : [6, 6],
    });
    L.marker([dot.lat, dot.lon], { icon, interactive: false, keyboard: false }).addTo(group);
  }
}

function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (ch) => {
    if (ch === "&") return "&amp;";
    if (ch === "<") return "&lt;";
    if (ch === ">") return "&gt;";
    if (ch === '"') return "&quot;";
    return "&#39;";
  });
}
