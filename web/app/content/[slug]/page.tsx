import { ContentBody } from "@/components/content-body";
import { PublicFrame } from "@/components/public-frame";
import { chrome, contentPath } from "@/lib/chrome";
import { loadContent } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function ContentPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const copy = chrome.ar;
  let item;
  try {
    item = await loadContent("ar", slug);
  } catch {
    return (
      <PublicFrame locale="ar" path={contentPath("ar", slug)}>
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="ar" path={contentPath("ar", slug)}>
      <ContentBody locale="ar" item={item} />
    </PublicFrame>
  );
}
