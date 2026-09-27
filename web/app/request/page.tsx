import { PublicFrame } from "@/components/public-frame";
import { RequestForm } from "@/components/request-form";

export default function RequestPage() {
  return (
    <PublicFrame locale="ar" path="/request">
      <RequestForm locale="ar" />
    </PublicFrame>
  );
}
