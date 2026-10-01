type PlaceText = { locale: string; name?: string; slug?: string; description?: string };

export type PlaceLabelSource = {
  id: string;
  status?: string;
  coverage?: string;
  texts?: PlaceText[];
};

const statusLabel: Record<string, string> = {
  draft: "مسودة",
  published: "منشور",
  in_review: "قيد المراجعة",
  suspended: "موقوف",
};

const coverageLabel: Record<string, string> = {
  served: "مخدوم",
  partial: "تغطية جزئية",
  none: "بلا تغطية",
};

function baseLabel(item: PlaceLabelSource) {
  const arabic = item.texts?.find((text) => text.locale === "ar");
  const english = item.texts?.find((text) => text.locale === "en");
  const name = arabic?.name || english?.name || "حي بلا اسم";
  const parts = [name];
  if (english?.name && english.name !== name) parts.push(english.name);
  if (item.status && statusLabel[item.status]) parts.push(statusLabel[item.status]);
  const description = arabic?.description?.trim();
  if (description) parts.push(description.length > 42 ? `${description.slice(0, 42)}…` : description);
  else if (arabic?.slug) parts.push(arabic.slug);
  if (item.coverage && coverageLabel[item.coverage]) parts.push(coverageLabel[item.coverage]);
  return parts.join(" — ");
}

export function placeLabels(items: PlaceLabelSource[]) {
  const bases = items.map(baseLabel);
  const counts = new Map<string, number>();
  for (const label of bases) counts.set(label, (counts.get(label) ?? 0) + 1);
  return items.map((item, index) => {
    const label = bases[index];
    const shared = (counts.get(label) ?? 0) > 1;
    return { id: item.id, label: shared ? `${label} — ${item.id.slice(0, 8)}` : label };
  });
}
