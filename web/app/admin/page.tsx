export default function AdminHome() {
  return (
    <section className="grid gap-3">
      <h1 className="text-2xl font-semibold">مكتب العمل</h1>
      <p className="text-sm text-ink/80">إدارة الأحياء والهيكل والإعلانات، ثم طلبات العملاء والزيارات والصفقات. الشاشة تعرض ما تسمح به صلاحيات الحساب فقط.</p>
      <ul className="grid gap-2 text-sm sm:grid-cols-3">
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/neighborhoods" className="font-semibold">
            الأحياء
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/organization" className="font-semibold">
            الهيكل
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/catalog" className="font-semibold">
            العقارات والإعلانات
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/requests" className="font-semibold">
            طلبات العملاء
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/visits" className="font-semibold">
            الزيارات
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/deals" className="font-semibold">
            الصفقات
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/refunds" className="font-semibold">
            طلبات الاسترداد
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/messages" className="font-semibold">
            الرسائل
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/disputes" className="font-semibold">
            النزاعات
          </a>
        </li>
        <li className="rounded-lg border border-line bg-card p-4">
          <a href="/admin/reports" className="font-semibold">
            التقارير
          </a>
        </li>
      </ul>
    </section>
  );
}
