import { Suspense } from "react";
import { SearchScreen } from "@/components/search-screen";

export default function EnglishSearchPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">Loading…</p>}>
      <SearchScreen locale="en" />
    </Suspense>
  );
}
