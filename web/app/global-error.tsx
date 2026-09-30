"use client";

export default function RootError({ error }: { error: Error & { digest?: string } }) {
  const trace = typeof error.digest === "string" && error.digest !== "" ? error.digest : "";
  return (
    <html lang="ar" dir="rtl">
      <body style={{ fontFamily: "Tahoma, sans-serif", margin: "2rem" }}>
        <main>
          <h1>تعذر إكمال الطلب</h1>
          <p>حدث خطأ. أعد المحاولة لاحقًا.</p>
          <p>رقم التتبع: {trace || "غير متاح"}</p>
        </main>
      </body>
    </html>
  );
}
