import type { ReactNode } from "react";
import type { Locale } from "@/lib/types";
import { chrome, homePath, languageHref } from "@/lib/chrome";
import { DocumentLocale } from "./document-locale";

export function PublicFrame({
  locale,
  path,
  children,
}: {
  locale: Locale;
  path: string;
  children: ReactNode;
}) {
  const copy = chrome[locale];
  return (
    <div dir={locale === "en" ? "ltr" : "rtl"} lang={locale} className="min-h-screen">
      <DocumentLocale locale={locale} />
      <header className="border-b border-line bg-card">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-4 py-4">
          <a href={homePath(locale)} className="text-lg font-semibold">
            {copy.brand}
          </a>
          <nav className="flex items-center gap-4 text-sm">
            <a href={homePath(locale)}>{copy.home}</a>
            <a href={locale === "en" ? "/en/search" : "/search"}>{copy.search}</a>
            <a href={locale === "en" ? "/en/favorites" : "/favorites"}>{copy.favorites}</a>
            <a href={locale === "en" ? "/en/stay" : "/stay"}>{copy.stay}</a>
            <a href={locale === "en" ? "/en/messages" : "/messages"}>{copy.messages}</a>
            <a href={locale === "en" ? "/en/disputes" : "/disputes"}>{copy.disputes}</a>
            <a href={locale === "en" ? "/en/notifications" : "/notifications"}>{copy.notifications}</a>
            <a href={locale === "en" ? "/en/request" : "/request"}>{copy.request}</a>
            <a href={locale === "en" ? "/en/account" : "/account"}>{copy.account}</a>
            <a href="/admin/login">{copy.admin}</a>
            <a href={languageHref(locale, path)}>{copy.language}</a>
          </nav>
        </div>
      </header>
      <main className="mx-auto grid max-w-5xl gap-6 px-4 py-8">{children}</main>
      <footer className="mx-auto grid max-w-5xl gap-2 px-4 pb-10 text-sm text-ink/70">
        <p>{copy.boundary}</p>
        <p>{copy.priceNote}</p>
      </footer>
    </div>
  );
}
