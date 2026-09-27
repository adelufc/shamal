import { Suspense } from "react";
import { DisputeScreen } from "@/components/dispute-screen";

export default function EnglishDisputesPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">Loading…</p>}>
      <DisputeScreen locale="en" />
    </Suspense>
  );
}
