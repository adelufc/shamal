import { AccountDesk } from "@/components/account-desk";
import { PublicFrame } from "@/components/public-frame";

export default function EnglishAccountPage() {
  return (
    <PublicFrame locale="en" path="/en/account">
      <AccountDesk locale="en" />
    </PublicFrame>
  );
}
