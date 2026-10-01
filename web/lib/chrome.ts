import type { Locale } from "./types";

export const chrome = {
  ar: {
    brand: "شمال العقاري",
    cityLine: "الرياض",
    language: "English",
    admin: "لوحة العمل",
    home: "الأحياء",
    request: "عرض - طلب عقار",
    account: "حسابي",
    search: "بحث وخريطة",
    favorites: "المفضلة",
    owner: "أداء الملاك",
    stay: "إقامة",
    messages: "الرسائل",
    disputes: "النزاعات",
    notifications: "الإشعارات",
    mapLink: "بحث وخريطة هذا الحي",
    countsNeighborhoods: "أحياء منشورة",
    countsListings: "إعلانات منشورة",
    services: "الخدمات المفعّلة",
    sale: "بيع",
    annual: "إيجار سنوي",
    monthly: "إيجار شهري",
    daily: "إيجار يومي",
    listings: "الإعلانات المنشورة",
    emptyListings: "لا توجد إعلانات في هذا الحي حاليًا",
    emptyNeighborhoods: "لا توجد أحياء منشورة.",
    missingNeighborhood: "هذا الرابط لا يطابق حياً منشوراً.",
    unpublishedNeighborhood: "هذا الحي غير منشور.",
    backToSearch: "الرجوع إلى البحث",
    details: "عرض التفاصيل",
    noPhoto: "لا توجد صورة",
    approximate: "موقع تقريبي داخل الحي",
    verifiedOwner: "المالك موثّق",
    testEnv: "بيئة اختبار",
    rooms: "غرف",
    area: "م²",
    boundary: "حدود الأحياء المعروضة تقريبية لغرض الاختبار، وليست حداً رسمياً معتمداً.",
    priceNote: "السعر يدخله منشئ الإعلان بالريال. المنصة لا تفترض عمولة أو رسوماً.",
    loadError: "تعذر تحميل البيانات الآن. حاول مرة أخرى",
    retry: "إعادة المحاولة",
    loading: "جارٍ التحميل…",
    listingCount: "إعلان منشور",
    back: "العودة إلى الأحياء",
    noMap: "بحث وخريطة هذا الحي",
  },
  en: {
    brand: "Shamal Real Estate",
    cityLine: "Riyadh",
    language: "العربية",
    admin: "Work desk",
    home: "Neighborhoods",
    request: "Property request",
    account: "My account",
    search: "Search and map",
    favorites: "Favorites",
    owner: "Owner performance",
    stay: "Stay",
    messages: "Messages",
    disputes: "Disputes",
    notifications: "Notices",
    mapLink: "Search and map for this neighborhood",
    countsNeighborhoods: "Published neighborhoods",
    countsListings: "Published listings",
    services: "Enabled services",
    sale: "Sale",
    annual: "Annual rent",
    monthly: "Monthly rent",
    daily: "Daily rent",
    listings: "Published listings",
    emptyListings: "There are no listings in this neighborhood right now.",
    emptyNeighborhoods: "There are no published neighborhoods.",
    missingNeighborhood: "This link does not match a published neighborhood.",
    unpublishedNeighborhood: "This neighborhood is not published.",
    backToSearch: "Back to search",
    details: "View details",
    noPhoto: "No photo",
    approximate: "Approximate location inside the neighborhood",
    verifiedOwner: "Verified owner",
    testEnv: "Test environment",
    rooms: "rooms",
    area: "m²",
    boundary: "The neighborhood boundaries shown here are approximate test masks, not an adopted official boundary.",
    priceNote: "The price is entered by the person who created the listing, in SAR. The platform does not assume a commission or a fee.",
    loadError: "The data could not be loaded just now. Try again.",
    retry: "Try again",
    loading: "Loading…",
    listingCount: "published listings",
    back: "Back to neighborhoods",
    noMap: "Search and map for this neighborhood",
  },
} as const;

export function otherLocale(locale: Locale): Locale {
  return locale === "ar" ? "en" : "ar";
}

export function languageHref(locale: Locale, path: string): string {
  if (locale === "ar") {
    return path === "/" ? "/en" : `/en${path}`;
  }
  const stripped = path.replace(/^\/en/, "") || "/";
  return stripped;
}

export function homePath(locale: Locale): string {
  return locale === "en" ? "/en" : "/";
}

export function neighborhoodPath(locale: Locale, slug: string): string {
  const encoded = encodeURIComponent(slug);
  return locale === "en" ? `/en/neighborhoods/${encoded}` : `/neighborhoods/${encoded}`;
}

export function listingPath(locale: Locale, id: string): string {
  return locale === "en" ? `/en/listings/${id}` : `/listings/${id}`;
}

export function contentPath(locale: Locale, slug: string): string {
  const encoded = encodeURIComponent(slug);
  return locale === "en" ? `/en/content/${encoded}` : `/content/${encoded}`;
}

export function money(minor: number, locale: Locale): string {
  return new Intl.NumberFormat(locale === "en" ? "en-SA" : "ar-SA", {
    style: "currency",
    currency: "SAR",
  }).format(minor / 100);
}

export function priceUnit(locale: Locale, unit: string, transaction: string): string {
  const kind =
    transaction === "sale" || unit === "total"
      ? "total"
      : transaction === "annual" || unit === "year" || unit === "yearly"
        ? "year"
        : transaction === "monthly" || unit === "month" || unit === "monthly"
          ? "month"
          : transaction === "daily" || unit === "night" || unit === "nightly"
            ? "night"
            : "";
  if (locale === "en") {
    if (kind === "total") return "total";
    if (kind === "year") return "per year";
    if (kind === "month") return "per month";
    if (kind === "night") return "per night";
    return unit;
  }
  if (kind === "total") return "الإجمالي";
  if (kind === "year") return "في السنة";
  if (kind === "month") return "في الشهر";
  if (kind === "night") return "في الليلة";
  return unit;
}
