import {
  View,
  Text,
  ScrollView,
  Pressable,
  ActivityIndicator,
  SafeAreaView,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useListing } from "../../../../lib/hooks/useListing";
import { useHoldEstimate } from "../../../../lib/hooks/useDiscovery";
import { useAuthStore } from "../../../../lib/auth";
import PhotoGallery from "../../../../components/listing/PhotoGallery";
import HostInfoCard from "../../../../components/listing/HostInfoCard";
import HoldExplainer from "../../../../components/listing/HoldExplainer";
import AvailabilityCalendar from "../../../../components/listing/AvailabilityCalendar";

// Route params passed from feed / search / map preview cards.
type ListingParams = {
  id: string;
  hostName?: string;
  hostReputation?: string;
  thumbnailUrl?: string;
  driveTimeMin?: string;
};

function priceLabel(perHour?: number, perDay?: number): string {
  const parts: string[] = [];
  if (perDay != null) parts.push(`$${perDay}/day`);
  if (perHour != null) parts.push(`$${perHour}/hr`);
  return parts.join("  ·  ");
}

function driveLabel(raw?: string): string {
  const minutes = parseFloat(raw ?? "0");
  if (minutes <= 0) return "";
  if (minutes < 1) return "< 1 min drive";
  return `${Math.round(minutes)} min drive`;
}

export default function ListingDetailScreen() {
  const router = useRouter();
  const { id, hostName, hostReputation, thumbnailUrl, driveTimeMin } =
    useLocalSearchParams<ListingParams>();

  const user = useAuthStore((s) => s.user);
  const { data: listingData, isLoading, isError } = useListing(id ?? null);
  const { data: holdEstimate } = useHoldEstimate(id ?? null);

  const listing = listingData?.listing;
  const isHost = listing?.hostId === user?.id;

  const photos = thumbnailUrl ? [thumbnailUrl] : [];
  const displayHostName = hostName ?? "Host";
  const displayReputation = parseInt(hostReputation ?? "0", 10);
  const drive = driveLabel(driveTimeMin);

  if (isLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-white">
        <ActivityIndicator size="large" color="#0284c7" />
      </View>
    );
  }

  if (isError || !listing) {
    return (
      <View className="flex-1 items-center justify-center bg-white px-8">
        <Ionicons name="alert-circle-outline" size={48} color="#ef4444" />
        <Text className="text-lg font-semibold text-gray-800 text-center mt-4">
          Listing not found
        </Text>
        <Pressable
          onPress={() => router.back()}
          className="mt-6 px-6 py-3 bg-sky-600 rounded-2xl"
        >
          <Text className="text-white font-semibold">Go Back</Text>
        </Pressable>
      </View>
    );
  }

  const price = priceLabel(listing.pricePerHour, listing.pricePerDay);

  return (
    <SafeAreaView className="flex-1 bg-white">
      {/* Back button overlaid on the photo */}
      <View className="absolute top-12 left-4 z-10">
        <Pressable
          onPress={() => router.back()}
          className="w-9 h-9 bg-black/40 rounded-full items-center justify-center"
          hitSlop={8}
        >
          <Ionicons name="chevron-back" size={20} color="white" />
        </Pressable>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ paddingBottom: 120 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Photo gallery */}
        <PhotoGallery photos={photos} />

        {/* Main content */}
        <View className="p-4 gap-y-5">
          {/* Title + status */}
          <View>
            <Text className="text-2xl font-bold text-gray-900">
              {listing.title}
            </Text>
            {listing.status !== "ACTIVE" && (
              <View className="mt-1 self-start px-2 py-0.5 bg-amber-100 rounded-full">
                <Text className="text-xs font-medium text-amber-700">
                  {listing.status}
                </Text>
              </View>
            )}
          </View>

          {/* Price + drive time */}
          <View className="flex-row items-center gap-x-4">
            {price ? (
              <Text className="text-lg font-semibold text-sky-600">{price}</Text>
            ) : null}
            {drive ? (
              <View className="flex-row items-center gap-x-1">
                <Ionicons name="car-outline" size={15} color="#6b7280" />
                <Text className="text-sm text-gray-500">{drive}</Text>
              </View>
            ) : null}
          </View>

          {/* Description */}
          {listing.description ? (
            <View>
              <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-2">
                About this item
              </Text>
              <Text className="text-sm text-gray-700 leading-relaxed">
                {listing.description}
              </Text>
            </View>
          ) : null}

          {/* Host info */}
          <HostInfoCard
            hostName={displayHostName}
            reputationScore={displayReputation}
            memberSince={listing.createdAt}
          />

          {/* Hold explainer — shown only to renters, not the host */}
          {!isHost && holdEstimate ? (
            <HoldExplainer
              holdAmount={holdEstimate.holdAmount}
              itemValue={holdEstimate.itemValue}
              guaranteeGap={holdEstimate.guaranteeGap}
            />
          ) : null}

          {/* Availability */}
          <AvailabilityCalendar availability={listing.availability} />
        </View>
      </ScrollView>

      {/* Fixed bottom CTA */}
      <View className="absolute bottom-0 left-0 right-0 bg-white border-t border-gray-100 px-4 py-4">
        {isHost ? (
          <Pressable
            className="bg-gray-800 rounded-2xl py-4 items-center"
            onPress={() => {
              // Navigate to edit listing — wired in Phase 3+
            }}
          >
            <Text className="text-white font-semibold text-base">
              Edit Listing
            </Text>
          </Pressable>
        ) : (
          <Pressable
            className="bg-sky-600 rounded-2xl py-4 items-center"
            onPress={() =>
              router.push({
                pathname: "/listing/[id]/checkout" as never,
                params: { id },
              })
            }
          >
            <Text className="text-white font-semibold text-base">
              Rent Now
            </Text>
          </Pressable>
        )}
      </View>
    </SafeAreaView>
  );
}
