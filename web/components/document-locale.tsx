"use client";

import { useEffect } from "react";
import type { Locale } from "@/lib/types";

export function DocumentLocale({ locale }: { locale: Locale }) {
  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dir = locale === "en" ? "ltr" : "rtl";
  }, [locale]);
  return null;
}
