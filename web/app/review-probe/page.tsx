export const dynamic = "force-dynamic";

export default async function ReviewProbe({ searchParams }: { searchParams: Promise<{ probe?: string }> }) {
  const query = await searchParams;
  if (query.probe === "1") {
    throw new Error("probe");
  }
  return (
    <main dir="rtl" className="mx-auto max-w-xl px-4 py-16">
      <p>هذه الصفحة لا تعرض خطأ إلا عند طلب الفحص.</p>
    </main>
  );
}
