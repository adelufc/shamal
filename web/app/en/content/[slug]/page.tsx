import { ContentBody } from "@/components/content-body";
import { PublicFrame } from "@/components/public-frame";
import { chrome, contentPath } from "@/lib/chrome";
import { loadContent } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function EnglishContentPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const copy = chrome.en;
  let item;
  try {
    item = await loadContent("en", slug);
  } catch {
    return (
      <PublicFrame locale="en" path={contentPath("en", slug)}>
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="en" path={contentPath("en", slug)}>
      <ContentBody locale="en" item={item} />
    </PublicFrame>
  );
}
