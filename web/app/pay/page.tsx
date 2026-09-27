import { Suspense } from "react";
import { PayScreen } from "@/components/pay-screen";

export default function PayPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">جارٍ التحميل…</p>}>
      <PayScreen locale="ar" />
    </Suspense>
  );
}
