import {
  FlatList,
  RefreshControl,
  View,
  Text,
  Pressable,
  ActivityIndicator,
} from "react-native";
import { useRouter } from "expo-router";
import { useCallback } from "react";
import { Ionicons } from "@expo/vector-icons";
import { useLocation } from "../../../lib/hooks/useLocation";
import { useFeed, RankedListing } from "../../../lib/hooks/useDiscovery";
import ListingFeedCard from "../../../components/listing/ListingFeedCard";

function SkeletonCard() {
  return (
    <View className="bg-white rounded-2xl overflow-hidden border border-gray-100 shadow-sm mb-3 mx-4">
      <View className="w-full h-44 bg-gray-200 animate-pulse" />
      <View className="p-3 gap-y-2">
        <View className="h-4 bg-gray-200 rounded w-3/4" />
        <View className="h-3 bg-gray-200 rounded w-1/3" />
        <View className="h-3 bg-gray-200 rounded w-1/2" />
      </View>
    </View>
  );
}

function EmptyState() {
  return (
    <View className="flex-1 items-center justify-center py-24 px-8">
      <Text className="text-5xl mb-4">📦</Text>
      <Text className="text-lg font-semibold text-gray-800 text-center">
        No listings nearby
      </Text>
      <Text className="text-sm text-gray-500 text-center mt-2">
        Try expanding your search radius or check back later.
      </Text>
    </View>
  );
}

export default function FeedScreen() {
  const router = useRouter();
  const { lat, lng, loading: locationLoading, error: locationError } = useLocation();

  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    isRefetching,
    refetch,
  } = useFeed(lat, lng);

  const listings: RankedListing[] = data?.pages.flatMap((p) => p.listings ?? []) ?? [];

  const onEndReached = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  if (locationLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-white">
        <ActivityIndicator size="large" color="#0284c7" />
        <Text className="text-gray-500 mt-3 text-sm">Getting your location…</Text>
      </View>
    );
  }

  if (locationError) {
    return (
      <View className="flex-1 items-center justify-center bg-white px-8">
        <Text className="text-lg font-semibold text-gray-800 text-center">
          Location unavailable
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-2">{locationError}</Text>
      </View>
    );
  }

  return (
    <View className="flex-1 bg-gray-50">
      {isLoading ? (
        <View className="pt-4">
          {[1, 2, 3].map((i) => (
            <SkeletonCard key={i} />
          ))}
        </View>
      ) : (
        <FlatList
          data={listings}
          keyExtractor={(item) => item.id}
          renderItem={({ item }) => (
            <View>
              <ListingFeedCard
                listing={item}
                onPress={() =>
                  router.push({
                    pathname: "/listing/[id]" as never,
                    params: {
                      id: item.id,
                      hostName: item.hostName,
                      hostReputation: String(item.hostReputation),
                      thumbnailUrl: item.thumbnailUrl ?? "",
                      driveTimeMin: String(item.driveTimeMin),
                    },
                  })
                }
              />
              {/* Rent Now shortcut — takes user directly to the booking request screen */}
              <Pressable
                className="mx-4 -mt-2 mb-3 bg-sky-600 rounded-xl py-2.5 flex-row items-center justify-center gap-x-1.5"
                onPress={() =>
                  router.push({
                    pathname: "/(tabs)/(feed)/booking-request" as never,
                    params: {
                      id: item.id,
                      title: item.title,
                      pricePerHour: item.pricePerHour != null ? String(item.pricePerHour) : "",
                      pricePerDay: item.pricePerDay != null ? String(item.pricePerDay) : "",
                      hostName: item.hostName,
                    },
                  })
                }
              >
                <Ionicons name="flash-outline" size={15} color="white" />
                <Text className="text-white text-sm font-semibold">Rent Now</Text>
              </Pressable>
            </View>
          )}
          ListEmptyComponent={<EmptyState />}
          ListFooterComponent={
            isFetchingNextPage ? (
              <ActivityIndicator size="small" color="#0284c7" className="py-4" />
            ) : null
          }
          contentContainerStyle={{ paddingTop: 12, paddingBottom: 24, flexGrow: 1 }}
          onEndReached={onEndReached}
          onEndReachedThreshold={0.4}
          refreshControl={
            <RefreshControl
              refreshing={isRefetching}
              onRefresh={refetch}
              tintColor="#0284c7"
            />
          }
        />
      )}
    </View>
  );
}
