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

function countsOf(labels: string[]) {
  const counts = new Map<string, number>();
  for (const label of labels) counts.set(label, (counts.get(label) ?? 0) + 1);
  return counts;
}

function arabic(item: PlaceLabelSource) {
  return item.texts?.find((text) => text.locale === "ar");
}

function placeName(item: PlaceLabelSource) {
  const english = item.texts?.find((text) => text.locale === "en");
  return arabic(item)?.name || english?.name || "حي بلا اسم";
}

function placeStatus(item: PlaceLabelSource) {
  if (!item.status || !statusLabel[item.status]) return "";
  return statusLabel[item.status];
}

function adminDescription(item: PlaceLabelSource) {
  const description = arabic(item)?.description?.trim();
  if (description) return description.length > 42 ? `${description.slice(0, 42)}…` : description;
  return arabic(item)?.slug?.trim() ?? "";
}

export function placeLabels(items: PlaceLabelSource[]) {
  const withStatus = items.map((item) => {
    const name = placeName(item);
    const status = placeStatus(item);
    return status ? `${name} — ${status}` : name;
  });
  const statusCounts = countsOf(withStatus);
  const withDescription = items.map((item, index) => {
    if ((statusCounts.get(withStatus[index]) ?? 0) <= 1) return withStatus[index];
    const description = adminDescription(item);
    return description ? `${withStatus[index]} — ${description}` : withStatus[index];
  });
  const descriptionCounts = countsOf(withDescription);
  const marked = items.map((item, index) => {
    const label = withDescription[index];
    if ((descriptionCounts.get(label) ?? 0) <= 1) return { id: item.id, label };
    return { id: item.id, label: `${withStatus[index]} — ${item.id.slice(0, 8)}` };
  });
  const shortCounts = countsOf(marked.map((item) => item.label));
  return marked.map((item, index) => {
    if ((shortCounts.get(item.label) ?? 0) <= 1) return item;
    if ((descriptionCounts.get(withDescription[index]) ?? 0) <= 1) return item;
    return { id: item.id, label: `${withStatus[index]} — ${item.id}` };
  });
}
