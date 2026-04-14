import { View, Text, FlatList, Pressable, ActivityIndicator } from "react-native";
import { router } from "expo-router";
import { useAuthStore } from "../../../lib/auth";
import { useMyListings, Listing } from "../../../lib/hooks/useListings";
import { useUserRatingsSummary, BUBBLE_LABELS } from "../../../lib/hooks/useRatings";
import ListingCard from "../../../components/listing/ListingCard";
import RatingBubbles from "../../../components/rating/RatingBubbles";

export default function ProfileScreen() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  const { data, isLoading, isError } = useMyListings();
  const listings: Listing[] = data?.listings ?? [];

  const { data: summaryData } = useUserRatingsSummary(user?.id ?? null);
  const summary = summaryData?.summary ?? [];

  return (
    <View className="flex-1 bg-white" testID="screen-profile">
      {/* Header */}
      <View className="px-6 pt-14 pb-6 border-b border-gray-100">
        <View className="flex-row items-center justify-between">
          <View className="w-14 h-14 rounded-full bg-sky-100 items-center justify-center mr-4">
            <Text className="text-sky-700 text-xl font-bold">
              {user?.name ? user.name.charAt(0).toUpperCase() : "?"}
            </Text>
          </View>
          <View className="flex-1">
            <Text testID="profile-name" className="text-xl font-bold text-gray-900">{user?.name || "Profile"}</Text>
            <Text testID="profile-email" className="text-sm text-gray-400 mt-0.5">{user?.email}</Text>
          </View>
        </View>

        <Pressable
          testID="btn-create-listing-nav"
          className="mt-5 w-full bg-sky-600 py-3 rounded-xl items-center"
          onPress={() => router.push("/(tabs)/(profile)/create-listing")}
        >
          <Text className="text-white font-semibold">+ Create Listing</Text>
        </Pressable>

        <Pressable
          testID="btn-invite-friends"
          className="mt-3 w-full border border-sky-300 py-3 rounded-xl items-center"
          onPress={() => router.push("/(tabs)/(profile)/referrals")}
        >
          <Text className="text-sky-600 font-semibold">Invite Friends — Earn $20</Text>
        </Pressable>
      </View>

      {/* Rating bubbles section */}
      {summary.length > 0 && (
        <View className="px-6 pt-5 pb-4 border-b border-gray-100">
          <Text className="text-base font-semibold text-gray-900 mb-3">
            Reviews from renters
          </Text>
          <RatingBubbles
            availableBubbles={summary.map((s) => s.bubble)}
            selected={summary.map((s) => s.bubble)}
            onToggle={() => {}}
            readOnly
          />
          <View className="flex-row flex-wrap gap-x-4 mt-3">
            {summary.map((item) => (
              <Text key={item.bubble} className="text-xs text-gray-500">
                {BUBBLE_LABELS[item.bubble]} ({item.count})
              </Text>
            ))}
          </View>
        </View>
      )}

      {/* Listings section */}
      <View className="flex-1 px-6 pt-4">
        <Text className="text-base font-semibold text-gray-900 mb-3">My Listings</Text>

        {isLoading && (
          <View className="flex-1 items-center justify-center">
            <ActivityIndicator size="small" color="#0284c7" />
          </View>
        )}

        {isError && (
          <Text className="text-sm text-red-500 text-center mt-4">
            Failed to load listings. Pull down to retry.
          </Text>
        )}

        {!isLoading && !isError && listings.length === 0 && (
          <Text testID="profile-listings-empty" className="text-sm text-gray-400 text-center mt-8">
            No listings yet. Tap "Create Listing" to get started.
          </Text>
        )}

        {!isLoading && !isError && listings.length > 0 && (
          <FlatList
            data={listings}
            keyExtractor={(item) => item.id}
            renderItem={({ item }) => <ListingCard listing={item} />}
            showsVerticalScrollIndicator={false}
            contentContainerStyle={{ paddingBottom: 100 }}
          />
        )}
      </View>

      {/* Sign out */}
      <View className="px-6 pb-8">
        <Pressable
          className="w-full border border-red-300 py-3 rounded-xl items-center"
          testID="btn-sign-out"
          onPress={logout}
        >
          <Text className="text-red-500 font-medium">Sign Out</Text>
        </Pressable>
      </View>
    </View>
  );
}
