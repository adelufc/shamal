import { Suspense } from "react";
import { SearchScreen } from "@/components/search-screen";

export default function SearchPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">جارٍ التحميل…</p>}>
      <SearchScreen locale="ar" />
    </Suspense>
  );
}
