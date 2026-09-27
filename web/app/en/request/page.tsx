import { PublicFrame } from "@/components/public-frame";
import { RequestForm } from "@/components/request-form";

export default function EnglishRequestPage() {
  return (
    <PublicFrame locale="en" path="/en/request">
      <RequestForm locale="en" />
    </PublicFrame>
  );
}
