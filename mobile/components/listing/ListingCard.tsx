import { View, Text, Image, Pressable } from "react-native";
import { Listing } from "../../lib/hooks/useListings";
import Badge from "../ui/Badge";

interface ListingCardProps {
  listing: Listing;
  onPress?: () => void;
}

const statusVariant: Record<Listing["status"], "info" | "success" | "warning" | "error"> = {
  PENDING: "warning",
  ACTIVE: "success",
  FLAGGED: "error",
  SUSPENDED: "error",
};

export default function ListingCard({ listing, onPress }: ListingCardProps) {
  const price = listing.pricePerDay != null
    ? `$${listing.pricePerDay}/day`
    : listing.pricePerHour != null
    ? `$${listing.pricePerHour}/hr`
    : null;

  return (
    <Pressable
      className="flex-row bg-white rounded-2xl overflow-hidden border border-gray-100 shadow-sm mb-3"
      onPress={onPress}
    >
      {listing.thumbnailUrl ? (
        <Image
          source={{ uri: listing.thumbnailUrl }}
          className="w-20 h-20"
          resizeMode="cover"
        />
      ) : (
        <View className="w-20 h-20 bg-gray-100 items-center justify-center">
          <Text className="text-gray-400 text-xs">No photo</Text>
        </View>
      )}

      <View className="flex-1 px-3 py-2 justify-between">
        <Text className="text-sm font-semibold text-gray-900" numberOfLines={1}>
          {listing.title}
        </Text>
        {price && (
          <Text className="text-sm text-sky-600 font-medium">{price}</Text>
        )}
        <Badge label={listing.status} variant={statusVariant[listing.status]} />
      </View>
    </Pressable>
  );
}
