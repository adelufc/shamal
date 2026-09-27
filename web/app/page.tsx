import { PublicFrame } from "@/components/public-frame";
import { chrome, neighborhoodPath } from "@/lib/chrome";
import { loadHome } from "@/lib/public";
import { HomeView } from "@/components/home-view";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const copy = chrome.ar;
  let data;
  try {
    data = await loadHome("ar");
  } catch {
    return (
      <PublicFrame locale="ar" path="/">
        <p className="rounded-lg border border-line bg-card p-4">{copy.loadError}</p>
      </PublicFrame>
    );
  }
  return (
    <PublicFrame locale="ar" path="/">
      <HomeView locale="ar" data={data} neighborhoodPath={(slug) => neighborhoodPath("ar", slug)} />
    </PublicFrame>
  );
}
