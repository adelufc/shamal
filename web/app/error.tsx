"use client";

export default function RouteError({ error }: { error: Error & { digest?: string } }) {
  const trace = typeof error.digest === "string" && error.digest !== "" ? error.digest : "";
  return (
    <main dir="rtl" lang="ar" className="mx-auto grid max-w-xl gap-3 px-4 py-16">
      <h1 className="text-2xl font-semibold">تعذر إكمال الطلب</h1>
      <p>حدث خطأ. أعد المحاولة لاحقًا.</p>
      <p>رقم التتبع: {trace || "غير متاح"}</p>
    </main>
  );
}
