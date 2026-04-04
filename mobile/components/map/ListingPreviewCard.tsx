import { View, Text, Image, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useRouter } from "expo-router";
import { RankedListing } from "../../lib/hooks/useDiscovery";

interface ListingPreviewCardProps {
  listing: RankedListing;
  onDismiss: () => void;
}

function driveLabel(minutes: number): string {
  if (minutes <= 0) return "";
  if (minutes < 1) return "< 1 min";
  return `${Math.round(minutes)} min drive`;
}

function priceLabel(listing: RankedListing): string {
  if (listing.pricePerDay != null) return `$${listing.pricePerDay}/day`;
  if (listing.pricePerHour != null) return `$${listing.pricePerHour}/hr`;
  return "";
}

// ListingPreviewCard is an absolute-positioned bottom card shown when a map
// marker is tapped. Tap the card to navigate to listing detail; tap X to dismiss.
export default function ListingPreviewCard({
  listing,
  onDismiss,
}: ListingPreviewCardProps) {
  const router = useRouter();
  const price = priceLabel(listing);
  const drive = driveLabel(listing.driveTimeMin);

  return (
    <View className="absolute bottom-6 left-4 right-4 bg-white rounded-2xl shadow-lg border border-gray-100 overflow-hidden flex-row">
      {/* Dismiss button — rendered above the main pressable */}
      <Pressable
        onPress={onDismiss}
        className="absolute top-2 right-2 z-10 p-1"
        hitSlop={8}
      >
        <Ionicons name="close-circle" size={20} color="#9ca3af" />
      </Pressable>

      {/* Main tappable area */}
      <Pressable
        onPress={() => router.push(`/listing/${listing.id}` as never)}
        className="flex-row flex-1"
      >
        {/* Thumbnail */}
        {listing.thumbnailUrl ? (
          <Image
            source={{ uri: listing.thumbnailUrl }}
            className="w-24 h-24"
            resizeMode="cover"
          />
        ) : (
          <View className="w-24 h-24 bg-gray-100 items-center justify-center">
            <Ionicons name="image-outline" size={24} color="#9ca3af" />
          </View>
        )}

        {/* Info */}
        <View className="flex-1 p-3 justify-center pr-8">
          <Text className="text-sm font-semibold text-gray-900" numberOfLines={2}>
            {listing.title}
          </Text>
          {price ? (
            <Text className="text-sm font-medium text-sky-600 mt-1">{price}</Text>
          ) : null}
          <View className="flex-row items-center mt-1 gap-x-3">
            {drive ? (
              <View className="flex-row items-center gap-x-1">
                <Ionicons name="car-outline" size={12} color="#6b7280" />
                <Text className="text-xs text-gray-500">{drive}</Text>
              </View>
            ) : null}
            <View className="flex-row items-center gap-x-1">
              <Ionicons name="star" size={11} color="#f59e0b" />
              <Text className="text-xs text-gray-600">
                {(listing.hostReputation / 200).toFixed(1)}
              </Text>
            </View>
          </View>
        </View>
      </Pressable>
    </View>
  );
}
