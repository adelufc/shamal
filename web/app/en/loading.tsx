import { chrome } from "@/lib/chrome";

export default function Loading() {
  return <p className="px-4 py-8 text-sm">{chrome.en.loading}</p>;
}
