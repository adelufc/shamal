import { Suspense } from "react";
import { StayScreen } from "@/components/stay-screen";

export default function EnglishStayPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">Loading…</p>}>
      <StayScreen locale="en" />
    </Suspense>
  );
}
