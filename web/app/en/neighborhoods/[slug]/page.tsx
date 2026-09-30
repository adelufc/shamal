import { NeighborhoodBody } from "@/components/neighborhood-body";
import { PublicFrame } from "@/components/public-frame";
import { loadNeighborhood } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function EnglishNeighborhoodPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const result = await loadNeighborhood("en", slug);
  return (
    <PublicFrame locale="en" path={`/en/neighborhoods/${slug}`}>
      <NeighborhoodBody locale="en" result={result} />
    </PublicFrame>
  );
}
