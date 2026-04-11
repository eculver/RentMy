import {
  View,
  Text,
  Pressable,
  TextInput,
} from "react-native";
import { forwardRef, useCallback, useState } from "react";
import BottomSheet, {
  BottomSheetView,
  BottomSheetBackdrop,
  BottomSheetBackdropProps,
} from "@gorhom/bottom-sheet";
import { SearchFilters } from "../../lib/hooks/useDiscovery";

interface FilterSheetProps {
  filters: SearchFilters;
  onApply: (filters: SearchFilters) => void;
  onReset: () => void;
}

const DRIVE_TIME_OPTIONS = [5, 15, 30, 60] as const;

const FilterSheet = forwardRef<BottomSheet, FilterSheetProps>(
  ({ filters, onApply, onReset }, ref) => {
    const [maxDriveMin, setMaxDriveMin] = useState<number | undefined>(
      filters.maxDriveMin,
    );
    const [minPrice, setMinPrice] = useState<string>(
      filters.minPrice != null ? String(filters.minPrice) : "",
    );
    const [maxPrice, setMaxPrice] = useState<string>(
      filters.maxPrice != null ? String(filters.maxPrice) : "",
    );

    const handleApply = useCallback(() => {
      const applied: SearchFilters = {};
      if (maxDriveMin != null) applied.maxDriveMin = maxDriveMin;
      const min = parseFloat(minPrice);
      const max = parseFloat(maxPrice);
      if (!isNaN(min) && min > 0) applied.minPrice = min;
      if (!isNaN(max) && max > 0) applied.maxPrice = max;
      onApply(applied);
    }, [maxDriveMin, minPrice, maxPrice, onApply]);

    const handleReset = useCallback(() => {
      setMaxDriveMin(undefined);
      setMinPrice("");
      setMaxPrice("");
      onReset();
    }, [onReset]);

    const renderBackdrop = useCallback(
      (props: BottomSheetBackdropProps) => (
        <BottomSheetBackdrop
          {...props}
          appearsOnIndex={0}
          disappearsOnIndex={-1}
        />
      ),
      [],
    );

    return (
      <BottomSheet
        ref={ref}
        index={-1}
        snapPoints={["55%"]}
        enablePanDownToClose
        backdropComponent={renderBackdrop}
      >
        <BottomSheetView className="px-5 pb-8">
          {/* Header */}
          <View className="flex-row justify-between items-center mb-6">
            <Text className="text-lg font-semibold text-gray-900">Filters</Text>
            <Pressable onPress={handleReset}>
              <Text className="text-sm text-sky-600 font-medium">Reset all</Text>
            </Pressable>
          </View>

          {/* Drive time */}
          <Text className="text-sm font-medium text-gray-700 mb-3">
            Max drive time
          </Text>
          <View className="flex-row gap-x-2 mb-6">
            {DRIVE_TIME_OPTIONS.map((min) => {
              const selected = maxDriveMin === min;
              return (
                <Pressable
                  key={min}
                  onPress={() => setMaxDriveMin(selected ? undefined : min)}
                  className={`flex-1 py-2 rounded-full border items-center ${
                    selected
                      ? "bg-sky-600 border-sky-600"
                      : "bg-white border-gray-200"
                  }`}
                >
                  <Text
                    className={`text-xs font-medium ${
                      selected ? "text-white" : "text-gray-700"
                    }`}
                  >
                    {min} min
                  </Text>
                </Pressable>
              );
            })}
          </View>

          {/* Price range */}
          <Text className="text-sm font-medium text-gray-700 mb-3">
            Price range ($/day)
          </Text>
          <View className="flex-row gap-x-3 mb-8">
            <View className="flex-1">
              <Text className="text-xs text-gray-500 mb-1">Min</Text>
              <TextInput
                value={minPrice}
                onChangeText={setMinPrice}
                placeholder="0"
                keyboardType="decimal-pad"
                className="border border-gray-200 rounded-xl px-3 py-2 text-sm text-gray-900 bg-white"
              />
            </View>
            <View className="flex-1">
              <Text className="text-xs text-gray-500 mb-1">Max</Text>
              <TextInput
                value={maxPrice}
                onChangeText={setMaxPrice}
                placeholder="Any"
                keyboardType="decimal-pad"
                className="border border-gray-200 rounded-xl px-3 py-2 text-sm text-gray-900 bg-white"
              />
            </View>
          </View>

          {/* Apply */}
          <Pressable
            onPress={handleApply}
            className="bg-sky-600 rounded-2xl py-3 items-center"
          >
            <Text className="text-white font-semibold text-base">
              Apply filters
            </Text>
          </Pressable>
        </BottomSheetView>
      </BottomSheet>
    );
  },
);

FilterSheet.displayName = "FilterSheet";
export default FilterSheet;
