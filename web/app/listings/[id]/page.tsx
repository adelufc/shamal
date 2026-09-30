import { ListingBody } from "@/components/listing-body";
import { PublicFrame } from "@/components/public-frame";
import { listingPath } from "@/lib/chrome";

export const dynamic = "force-dynamic";

export default async function ListingPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <PublicFrame locale="ar" path={listingPath("ar", id)}>
      <ListingBody locale="ar" id={id} />
    </PublicFrame>
  );
}
