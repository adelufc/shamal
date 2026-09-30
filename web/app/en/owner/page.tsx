import { OwnerScreen } from "@/components/owner-screen";
import { PublicFrame } from "@/components/public-frame";

export default function EnglishOwnerPage() {
  return (
    <PublicFrame locale="en" path="/en/owner">
      <OwnerScreen locale="en" />
    </PublicFrame>
  );
}
