import { Suspense } from "react";
import { StayScreen } from "@/components/stay-screen";

export default function StayPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">جارٍ التحميل…</p>}>
      <StayScreen locale="ar" />
    </Suspense>
  );
}
