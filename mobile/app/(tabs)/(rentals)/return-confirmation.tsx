/**
 * Return confirmation screen — shown after check-out completes.
 *
 * Displays the transaction summary, photo diff status (pending or complete),
 * and hold release status. Links to the dispute filing screen if needed.
 */
import {
  View,
  Text,
  ScrollView,
  Pressable,
  ActivityIndicator,
  SafeAreaView,
  RefreshControl,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useBooking } from "../../../lib/hooks/useBooking";
import { useTransactionDisputes } from "../../../lib/hooks/useDispute";
import HoldStatusCard from "../../../components/rental/HoldStatusCard";

type Params = {
  transactionId: string;
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function formatDollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function ReturnConfirmationScreen() {
  const router = useRouter();
  const { transactionId } = useLocalSearchParams<Params>();

  const { data, isLoading, isRefetching, refetch, error } = useBooking(
    transactionId ?? null,
  );
  const { data: disputes } = useTransactionDisputes(transactionId ?? null);

  if (isLoading) {
    return (
      <SafeAreaView className="flex-1 bg-white items-center justify-center">
        <ActivityIndicator size="large" color="#0284c7" />
      </SafeAreaView>
    );
  }

  if (error || !data) {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <Ionicons name="chevron-back" size={24} color="#111827" />
          </Pressable>
          <Text className="text-lg font-semibold text-gray-900 ml-2">
            Return summary
          </Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Text className="text-gray-500 text-center">
            Unable to load return details. Please try again.
          </Text>
        </View>
      </SafeAreaView>
    );
  }

  const { booking } = data;
  const hasOpenDispute = (disputes ?? []).some(
    (d) => d.status !== "RESOLVED" && d.status !== "CLOSED",
  );
  const isCompleted = booking.status === "COMPLETED";

  // Build a placeholder hold allocation from the booking.
  // In a real implementation these would come from the transaction/hold API.
  // For now we show zero allocations until the backend returns them.
  const holdAllocation = {
    authorizedCents: 0,
    capturedLateCents: 0,
    capturedDamageCents: 0,
    damageReserveCents: 0,
    releasedCents: 0,
  };

  return (
    <View testID="screen-return-confirmation" className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Return summary
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ paddingVertical: 24, paddingHorizontal: 16, gap: 16 }}
        refreshControl={
          <RefreshControl
            refreshing={isRefetching}
            onRefresh={refetch}
            tintColor="#0284c7"
          />
        }
      >
        {/* Status hero */}
        <View className="items-center pb-2">
          <View
            className="w-16 h-16 rounded-full items-center justify-center mb-3"
            style={{ backgroundColor: isCompleted ? "#dcfce7" : "#fef9c3" }}
          >
            <Ionicons
              name={isCompleted ? "checkmark-circle" : "time-outline"}
              size={36}
              color={isCompleted ? "#16a34a" : "#ca8a04"}
            />
          </View>
          <Text testID="return-status-label" className="text-xl font-bold text-gray-900 text-center">
            {isCompleted ? "Return complete" : "Processing return"}
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-1 px-4 leading-relaxed">
            {isCompleted
              ? "The item has been returned. Check your hold status below."
              : "Your return is being processed. This usually takes a few minutes."}
          </Text>
        </View>

        {/* Transaction details */}
        <View className="bg-gray-50 rounded-2xl overflow-hidden">
          <View className="px-4 py-3 border-b border-gray-200">
            <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
              Transaction details
            </Text>
          </View>
          <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
            <Text className="text-sm text-gray-600">Booking ID</Text>
            <Text className="text-xs font-mono text-gray-500">
              {booking.id.slice(-8).toUpperCase()}
            </Text>
          </View>
          <View className="px-4 py-3 border-b border-gray-100">
            <Text className="text-sm text-gray-600 mb-0.5">Pickup</Text>
            <Text className="text-sm font-medium text-gray-900">
              {formatDate(booking.scheduledStart)}
            </Text>
          </View>
          <View className="px-4 py-3 border-b border-gray-100">
            <Text className="text-sm text-gray-600 mb-0.5">Returned</Text>
            <Text className="text-sm font-medium text-gray-900">
              {booking.actualEnd
                ? formatDate(booking.actualEnd)
                : "Pending confirmation"}
            </Text>
          </View>
          {booking.cancellationFee != null && booking.cancellationFee > 0 && (
            <View className="px-4 py-3">
              <Text className="text-sm text-gray-600 mb-0.5">Late fee</Text>
              <Text className="text-sm font-semibold text-amber-700">
                {formatDollars(booking.cancellationFee)}
              </Text>
            </View>
          )}
        </View>

        {/* Hold status */}
        <HoldStatusCard allocation={holdAllocation} />

        {/* Photo diff pending notice */}
        {isCompleted && !hasOpenDispute && (
          <View className="bg-sky-50 rounded-2xl px-4 py-3 flex-row items-start gap-x-3">
            <Ionicons name="camera-outline" size={18} color="#0284c7" />
            <View className="flex-1">
              <Text className="text-sm font-semibold text-sky-800">
                Photo comparison in progress
              </Text>
              <Text className="text-xs text-sky-600 mt-0.5">
                We're comparing your check-in and check-out photos. You'll be notified when the review is complete.
              </Text>
            </View>
          </View>
        )}

        {/* Open dispute banner */}
        {hasOpenDispute && (
          <View className="bg-red-50 rounded-2xl px-4 py-3 flex-row items-start gap-x-3">
            <Ionicons name="warning-outline" size={18} color="#dc2626" />
            <View className="flex-1">
              <Text className="text-sm font-semibold text-red-800">
                Dispute open
              </Text>
              <Text className="text-xs text-red-600 mt-0.5">
                A dispute has been filed for this rental. Track its progress below.
              </Text>
            </View>
          </View>
        )}

        {/* Action buttons */}
        <View className="gap-y-3">
          {hasOpenDispute && (
            <Pressable
              className="bg-red-600 rounded-2xl py-4 items-center flex-row justify-center gap-x-2"
              onPress={() =>
                router.push({
                  pathname: "/(tabs)/(rentals)/dispute-status" as never,
                  params: { transactionId: booking.id },
                })
              }
            >
              <Ionicons name="warning-outline" size={18} color="white" />
              <Text className="text-white font-semibold text-base">
                View dispute
              </Text>
            </Pressable>
          )}

          {!hasOpenDispute && isCompleted && (
            <>
              <Pressable
                testID="btn-rate-rental"
                className="bg-sky-600 rounded-2xl py-4 items-center flex-row justify-center gap-x-2"
                onPress={() =>
                  router.push({
                    pathname: "/(tabs)/(rentals)/rate" as never,
                    params: { transactionId: booking.id },
                  })
                }
              >
                <Ionicons name="star-outline" size={18} color="white" />
                <Text className="text-white font-semibold text-base">
                  Rate this rental
                </Text>
              </Pressable>

              <Pressable
                testID="btn-file-dispute"
                className="border border-red-200 rounded-2xl py-3 items-center flex-row justify-center gap-x-2"
                onPress={() =>
                  router.push({
                    pathname: "/(tabs)/(rentals)/dispute" as never,
                    params: { transactionId: booking.id },
                  })
                }
              >
                <Ionicons name="flag-outline" size={15} color="#dc2626" />
                <Text className="text-red-600 font-medium text-sm">
                  File a dispute
                </Text>
              </Pressable>
            </>
          )}

          <Pressable
            testID="btn-back-to-rentals"
            className="border border-gray-200 rounded-2xl py-4 items-center"
            onPress={() => router.back()}
          >
            <Text className="text-gray-700 font-semibold text-base">
              Back to rentals
            </Text>
          </Pressable>
        </View>
      </ScrollView>
    </View>
  );
}
