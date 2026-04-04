import { View, Text, Image, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { RankedListing } from "../../lib/hooks/useDiscovery";

interface ListingFeedCardProps {
  listing: RankedListing;
  onPress?: () => void;
}

function driveLabel(minutes: number): string {
  if (minutes <= 0) return "";
  if (minutes < 1) return "< 1 min drive";
  return `${Math.round(minutes)} min drive`;
}

function priceLabel(listing: RankedListing): string {
  if (listing.pricePerDay != null) return `$${listing.pricePerDay}/day`;
  if (listing.pricePerHour != null) return `$${listing.pricePerHour}/hr`;
  return "";
}

function reputationLabel(score: number): string {
  // Reputation is stored as 0–1000 (PRD §8)
  return (score / 200).toFixed(1);
}

export default function ListingFeedCard({ listing, onPress }: ListingFeedCardProps) {
  const price = priceLabel(listing);
  const drive = driveLabel(listing.driveTimeMin);

  return (
    <Pressable
      className="bg-white rounded-2xl overflow-hidden border border-gray-100 shadow-sm mb-3 mx-4"
      onPress={onPress}
    >
      {/* Thumbnail */}
      {listing.thumbnailUrl ? (
        <Image
          source={{ uri: listing.thumbnailUrl }}
          className="w-full h-44"
          resizeMode="cover"
        />
      ) : (
        <View className="w-full h-44 bg-gray-100 items-center justify-center">
          <Ionicons name="image-outline" size={32} color="#9ca3af" />
        </View>
      )}

      <View className="p-3">
        {/* Title */}
        <Text className="text-base font-semibold text-gray-900" numberOfLines={1}>
          {listing.title}
        </Text>

        {/* Price + drive time */}
        <View className="flex-row items-center mt-1 gap-x-3">
          {price ? (
            <Text className="text-sm font-medium text-sky-600">{price}</Text>
          ) : null}
          {drive ? (
            <View className="flex-row items-center gap-x-1">
              <Ionicons name="car-outline" size={13} color="#6b7280" />
              <Text className="text-xs text-gray-500">{drive}</Text>
            </View>
          ) : null}
        </View>

        {/* Trust signals */}
        <View className="flex-row items-center mt-2 gap-x-3">
          <View className="flex-row items-center gap-x-1">
            <Ionicons name="star" size={12} color="#f59e0b" />
            <Text className="text-xs text-gray-600">{reputationLabel(listing.hostReputation)}</Text>
          </View>
          <Text className="text-xs text-gray-500">{listing.hostName}</Text>
          {listing.hostReputation >= 500 && (
            <View className="flex-row items-center gap-x-1">
              <Ionicons name="shield-checkmark-outline" size={12} color="#0284c7" />
              <Text className="text-xs text-sky-600">Verified host</Text>
            </View>
          )}
        </View>
      </View>
    </Pressable>
  );
}
