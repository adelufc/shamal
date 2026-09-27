import { Suspense } from "react";
import { PayScreen } from "@/components/pay-screen";

export default function EnglishPayPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">Loading…</p>}>
      <PayScreen locale="en" />
    </Suspense>
  );
}
