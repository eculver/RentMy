import {
  FlatList,
  View,
  Text,
  TextInput,
  Pressable,
  ActivityIndicator,
} from "react-native";
import { useRef, useCallback } from "react";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useDebouncedCallback } from "use-debounce";
import BottomSheet from "@gorhom/bottom-sheet";
import { useLocation } from "../../../lib/hooks/useLocation";
import { useSearch, RankedListing } from "../../../lib/hooks/useDiscovery";
import { useSearchStore } from "../../../lib/stores/searchStore";
import ListingFeedCard from "../../../components/listing/ListingFeedCard";
import FilterSheet from "../../../components/search/FilterSheet";
import type { SearchFilters } from "../../../lib/hooks/useDiscovery";

function EmptyIdle() {
  return (
    <View className="flex-1 items-center justify-center py-24 px-8">
      <Text className="text-5xl mb-4">🔍</Text>
      <Text className="text-lg font-semibold text-gray-800 text-center">
        Search for anything nearby
      </Text>
      <Text className="text-sm text-gray-500 text-center mt-2">
        Find items to rent from people around you.
      </Text>
    </View>
  );
}

function EmptyResults({ query }: { query: string }) {
  return (
    <View className="flex-1 items-center justify-center py-24 px-8">
      <Text className="text-5xl mb-4">😕</Text>
      <Text className="text-lg font-semibold text-gray-800 text-center">
        No results for "{query}"
      </Text>
      <Text className="text-sm text-gray-500 text-center mt-2">
        Try a different search or adjust your filters.
      </Text>
    </View>
  );
}

function activeFilterCount(filters: SearchFilters): number {
  return [filters.maxDriveMin, filters.minPrice, filters.maxPrice].filter(
    (v) => v != null,
  ).length;
}

export default function SearchScreen() {
  const router = useRouter();
  const { lat, lng } = useLocation();
  const { query, filters, setQuery, setFilters, resetFilters } = useSearchStore();
  const filterSheetRef = useRef<BottomSheet>(null);

  const debouncedSetQuery = useDebouncedCallback((value: string) => {
    setQuery(value);
  }, 300);

  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
  } = useSearch(query, lat, lng, filters);

  const listings: RankedListing[] = data?.pages.flatMap((p) => p.listings ?? []) ?? [];

  const onEndReached = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const openFilters = useCallback(() => {
    filterSheetRef.current?.expand();
  }, []);

  const handleApplyFilters = useCallback(
    (applied: SearchFilters) => {
      setFilters(applied);
      filterSheetRef.current?.close();
    },
    [setFilters],
  );

  const handleResetFilters = useCallback(() => {
    resetFilters();
    filterSheetRef.current?.close();
  }, [resetFilters]);

  const filterCount = activeFilterCount(filters);

  return (
    <View className="flex-1 bg-gray-50">
      {/* Search bar */}
      <View className="bg-white border-b border-gray-100 px-4 pt-14 pb-3">
        <View className="flex-row items-center gap-x-2">
          <View className="flex-1 flex-row items-center bg-gray-100 rounded-2xl px-3 py-2 gap-x-2">
            <Ionicons name="search-outline" size={18} color="#9ca3af" />
            <TextInput
              onChangeText={debouncedSetQuery}
              placeholder="Search listings…"
              placeholderTextColor="#9ca3af"
              returnKeyType="search"
              autoCapitalize="none"
              autoCorrect={false}
              className="flex-1 text-sm text-gray-900"
            />
          </View>
          <Pressable
            onPress={openFilters}
            className={`flex-row items-center gap-x-1 px-3 py-2 rounded-2xl border ${
              filterCount > 0
                ? "bg-sky-50 border-sky-200"
                : "bg-white border-gray-200"
            }`}
          >
            <Ionicons
              name="options-outline"
              size={18}
              color={filterCount > 0 ? "#0284c7" : "#6b7280"}
            />
            {filterCount > 0 && (
              <Text className="text-xs font-semibold text-sky-600">
                {filterCount}
              </Text>
            )}
          </Pressable>
        </View>
      </View>

      {/* Results */}
      {query.length === 0 ? (
        <EmptyIdle />
      ) : isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator size="large" color="#0284c7" />
        </View>
      ) : (
        <FlatList
          data={listings}
          keyExtractor={(item) => item.id}
          renderItem={({ item }) => (
            <ListingFeedCard
              listing={item}
              onPress={() => router.push(`/listing/${item.id}` as never)}
            />
          )}
          ListEmptyComponent={<EmptyResults query={query} />}
          ListFooterComponent={
            isFetchingNextPage ? (
              <ActivityIndicator size="small" color="#0284c7" className="py-4" />
            ) : null
          }
          contentContainerStyle={{ paddingTop: 12, paddingBottom: 24, flexGrow: 1 }}
          onEndReached={onEndReached}
          onEndReachedThreshold={0.4}
        />
      )}

      {/* Filter bottom sheet */}
      <FilterSheet
        ref={filterSheetRef}
        filters={filters}
        onApply={handleApplyFilters}
        onReset={handleResetFilters}
      />
    </View>
  );
}
