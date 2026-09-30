import { OwnerScreen } from "@/components/owner-screen";
import { PublicFrame } from "@/components/public-frame";

export default function OwnerPage() {
  return (
    <PublicFrame locale="ar" path="/owner">
      <OwnerScreen locale="ar" />
    </PublicFrame>
  );
}
