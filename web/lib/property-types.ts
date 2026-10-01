import type { Locale } from "@/lib/types";

const stored = {
  apartment: { ar: "شقة", en: "Apartment" },
  chalet: { ar: "شاليه", en: "Chalet" },
  floor: { ar: "دور", en: "Floor" },
  land: { ar: "أرض", en: "Land" },
  small_unit: { ar: "وحدة صغيرة", en: "Small unit" },
  townhouse: { ar: "منزل متلاصق", en: "Townhouse" },
  villa: { ar: "فيلا", en: "Villa" },
} as const;

export function propertyTypeName(locale: Locale, code: string | undefined, storedName?: string) {
  const trimmed = storedName?.trim() ?? "";
  if (trimmed && trimmed !== code) return trimmed;
  if (!code) return "";
  const row = stored[code as keyof typeof stored];
  if (!row) return "";
  return locale === "en" ? row.en : row.ar;
}
