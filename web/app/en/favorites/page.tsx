import { FavoritesScreen } from "@/components/favorites-screen";
import { PublicFrame } from "@/components/public-frame";

export default function EnglishFavoritesPage() {
  return (
    <PublicFrame locale="en" path="/en/favorites">
      <FavoritesScreen locale="en" />
    </PublicFrame>
  );
}
