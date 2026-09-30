import { NeighborhoodBody } from "@/components/neighborhood-body";
import { PublicFrame } from "@/components/public-frame";
import { loadNeighborhood } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function NeighborhoodPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const result = await loadNeighborhood("ar", slug);
  return (
    <PublicFrame locale="ar" path={`/neighborhoods/${slug}`}>
      <NeighborhoodBody locale="ar" result={result} />
    </PublicFrame>
  );
}
