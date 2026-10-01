"use client";

import { useRef, useState, type ReactNode } from "react";
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
  const [open, setOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLElement>(null);
  const prefix = locale === "en" ? "/en" : "";
  const links = [
    { href: homePath(locale), label: copy.home },
    { href: `${prefix}/search`, label: copy.search },
    { href: `${prefix}/request`, label: copy.request },
    { href: `${prefix}/account`, label: copy.account },
    { href: `${prefix}/favorites`, label: copy.favorites },
    { href: `${prefix}/stay`, label: copy.stay },
    { href: `${prefix}/messages`, label: copy.messages },
    { href: `${prefix}/disputes`, label: copy.disputes },
    { href: `${prefix}/notifications`, label: copy.notifications },
    { href: `${prefix}/owner`, label: copy.owner },
    { href: "/admin/login", label: copy.admin },
    { href: languageHref(locale, path), label: copy.language },
  ];

  function toggle(next: boolean) {
    setOpen(next);
    window.setTimeout(() => {
      if (next) menu.current?.querySelector("a")?.focus();
      else menuButton.current?.focus();
    }, 0);
  }

  return (
    <div dir={locale === "en" ? "ltr" : "rtl"} lang={locale} className="min-h-screen overflow-x-hidden">
      <DocumentLocale locale={locale} />
      <p className="bg-[#FFF6DE] px-4 py-2 text-center text-sm text-[#7B560D]">{copy.testEnv}</p>
      <header data-chrome="navy" className="border-b border-white/15 bg-ink text-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-3">
          <a href={homePath(locale)} className="text-lg font-semibold">
            {copy.brand}
          </a>
          <button
            ref={menuButton}
            type="button"
            className="inline-flex min-h-11 items-center rounded-lg border border-white/40 px-3 text-white md:hidden"
            aria-expanded={open}
            onClick={() => toggle(!open)}
          >
            {locale === "en" ? "Menu" : "القائمة"}
          </button>
          <nav className="hidden flex-wrap items-center gap-4 text-sm md:flex">
            {links.slice(0, 4).map((item) => (
              <a key={item.href + item.label} href={item.href}>
                {item.label}
              </a>
            ))}
            <a href={languageHref(locale, path)}>{copy.language}</a>
          </nav>
        </div>
        <nav ref={menu} className={`${open ? "grid" : "hidden"} gap-1 border-t border-white/15 px-4 py-3 text-sm md:hidden`}>
          {links.map((item) => (
            <a key={item.href + item.label} className="inline-flex min-h-11 items-center" href={item.href}>
              {item.label}
            </a>
          ))}
        </nav>
      </header>
      <main className="mx-auto grid max-w-6xl gap-6 px-4 py-8">{children}</main>
      <footer className="mx-auto grid max-w-6xl gap-2 px-4 pb-10 text-sm text-muted">
        <p>{copy.boundary}</p>
        <p>{copy.priceNote}</p>
      </footer>
    </div>
  );
}
