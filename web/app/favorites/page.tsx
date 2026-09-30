import { FavoritesScreen } from "@/components/favorites-screen";
import { PublicFrame } from "@/components/public-frame";

export default function FavoritesPage() {
  return (
    <PublicFrame locale="ar" path="/favorites">
      <FavoritesScreen locale="ar" />
    </PublicFrame>
  );
}
