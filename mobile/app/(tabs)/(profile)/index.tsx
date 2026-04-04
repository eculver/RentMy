import { View, Text, FlatList, Pressable, ActivityIndicator } from "react-native";
import { router } from "expo-router";
import { useAuthStore } from "../../../lib/auth";
import { useMyListings, Listing } from "../../../lib/hooks/useListings";
import ListingCard from "../../../components/listing/ListingCard";

export default function ProfileScreen() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  const { data, isLoading, isError } = useMyListings();
  const listings: Listing[] = data?.listings ?? [];

  return (
    <View className="flex-1 bg-white">
      {/* Header */}
      <View className="px-6 pt-14 pb-6 border-b border-gray-100">
        <View className="flex-row items-center justify-between">
          <View className="w-14 h-14 rounded-full bg-sky-100 items-center justify-center mr-4">
            <Text className="text-sky-700 text-xl font-bold">
              {user?.name ? user.name.charAt(0).toUpperCase() : "?"}
            </Text>
          </View>
          <View className="flex-1">
            <Text className="text-xl font-bold text-gray-900">{user?.name || "Profile"}</Text>
            <Text className="text-sm text-gray-400 mt-0.5">{user?.email}</Text>
          </View>
        </View>

        <Pressable
          className="mt-5 w-full bg-sky-600 py-3 rounded-xl items-center"
          onPress={() => router.push("/(tabs)/(profile)/create-listing")}
        >
          <Text className="text-white font-semibold">+ Create Listing</Text>
        </Pressable>
      </View>

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
          <Text className="text-sm text-gray-400 text-center mt-8">
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
          onPress={logout}
        >
          <Text className="text-red-500 font-medium">Sign Out</Text>
        </Pressable>
      </View>
    </View>
  );
}
