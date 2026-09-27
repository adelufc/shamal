import { Suspense } from "react";
import { MessageScreen } from "@/components/message-screen";

export default function EnglishMessagesPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">Loading…</p>}>
      <MessageScreen locale="en" />
    </Suspense>
  );
}
