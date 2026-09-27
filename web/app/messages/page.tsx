import { Suspense } from "react";
import { MessageScreen } from "@/components/message-screen";

export default function MessagesPage() {
  return (
    <Suspense fallback={<p className="p-6 text-sm">جارٍ التحميل…</p>}>
      <MessageScreen locale="ar" />
    </Suspense>
  );
}
