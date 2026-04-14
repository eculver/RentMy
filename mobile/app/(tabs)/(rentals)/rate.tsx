/**
 * Rating prompt screen — shown after a rental completes.
 *
 * Displays a bubble selection grid appropriate for the current user's role
 * (renter rating host, or host rating renter). Submits via the ratings API.
 */
import { useState } from "react";
import {
  View,
  Text,
  ScrollView,
  Pressable,
  ActivityIndicator,
  Alert,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../../lib/auth";
import { useBooking } from "../../../lib/hooks/useBooking";
import {
  HOST_BUBBLES,
  RENTER_BUBBLES,
  BUBBLE_LABELS,
  type RatingBubble,
  useSubmitRating,
} from "../../../lib/hooks/useRatings";
import RatingBubbles from "../../../components/rating/RatingBubbles";

type Params = {
  transactionId: string;
};

export default function RateScreen() {
  const router = useRouter();
  const { transactionId } = useLocalSearchParams<Params>();
  const user = useAuthStore((s) => s.user);

  const { data, isLoading, error } = useBooking(transactionId ?? null);
  const [selected, setSelected] = useState<RatingBubble[]>([]);

  const { mutate, isPending, isSuccess } = useSubmitRating(transactionId ?? "");

  if (isLoading) {
    return (
      <View className="flex-1 bg-white items-center justify-center">
        <ActivityIndicator size="large" color="#0284c7" />
      </View>
    );
  }

  if (error || !data || !user) {
    return (
      <View className="flex-1 bg-white">
        <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <Ionicons name="chevron-back" size={24} color="#111827" />
          </Pressable>
          <Text className="text-lg font-semibold text-gray-900 ml-2">Rate</Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Text className="text-gray-500 text-center">
            Unable to load rental details.
          </Text>
        </View>
      </View>
    );
  }

  const { booking } = data;
  const isRenter = user.id === booking.renterId;
  const availableBubbles = isRenter ? RENTER_BUBBLES : HOST_BUBBLES;
  const counterpartyLabel = isRenter ? "the host" : "the renter";

  const toggleBubble = (bubble: RatingBubble) => {
    setSelected((prev) =>
      prev.includes(bubble) ? prev.filter((b) => b !== bubble) : [...prev, bubble],
    );
  };

  const handleSubmit = () => {
    if (selected.length === 0) {
      Alert.alert("Select at least one", "Please select at least one bubble.");
      return;
    }
    mutate(
      { bubbles: selected },
      {
        onSuccess: () => {
          setTimeout(() => {
            router.replace("/(tabs)/(rentals)" as never);
          }, 800);
        },
        onError: () => {
          Alert.alert("Error", "Failed to submit rating. Please try again.");
        },
      },
    );
  };

  return (
    <View testID="screen-rate" className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Rate your experience
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ paddingVertical: 24, paddingHorizontal: 16, gap: 16 }}
      >
        {/* Hero */}
        <View className="items-center">
          <View className="w-16 h-16 rounded-full bg-amber-50 items-center justify-center mb-3">
            <Ionicons name="star" size={36} color="#f59e0b" />
          </View>
          <Text className="text-xl font-bold text-gray-900 text-center">
            How was it?
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-1 px-4 leading-relaxed">
            Tap the bubbles that best describe your experience with{" "}
            {counterpartyLabel}.
          </Text>
        </View>

        {/* Bubble grid */}
        <View testID="rating-bubbles-container" className="bg-gray-50 rounded-2xl p-4">
          <RatingBubbles
            availableBubbles={availableBubbles}
            selected={selected}
            onToggle={toggleBubble}
          />
        </View>

        {/* Selected preview */}
        {selected.length > 0 && (
          <View className="flex-row flex-wrap gap-2">
            {selected.map((b) => (
              <View
                key={b}
                className="flex-row items-center gap-x-1 bg-sky-50 rounded-full px-3 py-1.5"
              >
                <Ionicons name="checkmark" size={12} color="#0284c7" />
                <Text className="text-xs font-medium text-sky-700">
                  {BUBBLE_LABELS[b]}
                </Text>
              </View>
            ))}
          </View>
        )}

        {isSuccess && (
          <View testID="rating-success-message" className="bg-green-50 rounded-2xl px-4 py-3 flex-row items-center gap-x-2">
            <Ionicons name="checkmark-circle" size={18} color="#16a34a" />
            <Text className="text-sm text-green-700 font-medium">
              Rating submitted!
            </Text>
          </View>
        )}

        {/* Submit */}
        <Pressable
          testID="btn-submit-rating"
          className={[
            "rounded-2xl py-4 items-center flex-row justify-center gap-x-2",
            selected.length > 0 && !isPending ? "bg-sky-600" : "bg-sky-300",
          ].join(" ")}
          onPress={handleSubmit}
          disabled={selected.length === 0 || isPending}
        >
          {isPending ? (
            <ActivityIndicator color="white" />
          ) : (
            <>
              <Ionicons name="star-outline" size={18} color="white" />
              <Text className="text-white font-semibold text-base">
                Submit rating
              </Text>
            </>
          )}
        </Pressable>

        <Pressable
          className="py-3 items-center"
          onPress={() => router.replace("/(tabs)/(rentals)" as never)}
        >
          <Text className="text-sm text-gray-500">Skip</Text>
        </Pressable>
      </ScrollView>
    </View>
  );
}
