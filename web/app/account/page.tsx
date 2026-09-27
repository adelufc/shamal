import { AccountDesk } from "@/components/account-desk";
import { PublicFrame } from "@/components/public-frame";

export default function AccountPage() {
  return (
    <PublicFrame locale="ar" path="/account">
      <AccountDesk locale="ar" />
    </PublicFrame>
  );
}
