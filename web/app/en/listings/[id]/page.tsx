import { ListingBody } from "@/components/listing-body";
import { PublicFrame } from "@/components/public-frame";
import { listingPath } from "@/lib/chrome";

export const dynamic = "force-dynamic";

export default async function EnglishListingPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <PublicFrame locale="en" path={listingPath("en", id)}>
      <ListingBody locale="en" id={id} />
    </PublicFrame>
  );
}
