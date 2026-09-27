import { Suspense } from "react";
import { DisputeScreen } from "@/components/dispute-screen";

export default function DisputesPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">جارٍ التحميل…</p>}>
      <DisputeScreen locale="ar" />
    </Suspense>
  );
}
