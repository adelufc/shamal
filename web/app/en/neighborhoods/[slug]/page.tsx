import { NeighborhoodBody } from "@/app/neighborhoods/[slug]/page";
import { PublicFrame } from "@/components/public-frame";
import { chrome } from "@/lib/chrome";
import { loadNeighborhood } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function EnglishNeighborhoodPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const copy = chrome.en;
  let payload;
  try {
    payload = await loadNeighborhood("en", slug);
  } catch {
    return (
      <PublicFrame locale="en" path={`/en/neighborhoods/${slug}`}>
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="en" path={`/en/neighborhoods/${slug}`}>
      <NeighborhoodBody locale="en" payload={payload} />
    </PublicFrame>
  );
}
