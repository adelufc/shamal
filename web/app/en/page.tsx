import { HomeView } from "@/components/home-view";
import { PublicFrame } from "@/components/public-frame";
import { chrome, neighborhoodPath } from "@/lib/chrome";
import { loadHome } from "@/lib/public";

export const dynamic = "force-dynamic";

export default async function EnglishHomePage() {
  const copy = chrome.en;
  let data;
  try {
    data = await loadHome("en");
  } catch {
    return (
      <PublicFrame locale="en" path="/en">
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="en" path="/en">
      <HomeView locale="en" data={data} neighborhoodPath={(slug) => neighborhoodPath("en", slug)} />
    </PublicFrame>
  );
}
