/**
 * Booking request screen — quick entry point from the feed listing card.
 * Receives listing params via router, lets the user pick a duration and
 * payment method, then calls POST /api/v1/bookings.
 *
 * After a successful booking, the user is taken to the booking-status screen
 * for real-time tracking.
 */
import { useState } from "react";
import {
  View,
  Text,
  ScrollView,
  Pressable,
  ActivityIndicator,
  Alert,
  SafeAreaView,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useHoldEstimate } from "../../../lib/hooks/useDiscovery";
import { useAuthStore } from "../../../lib/auth";
import { useCheckoutStore } from "../../../lib/stores/checkoutStore";
import { api } from "../../../lib/api";
import DurationPicker from "../../../components/checkout/DurationPicker";
import CostBreakdown from "../../../components/checkout/CostBreakdown";
import PaymentMethodSelector from "../../../components/checkout/PaymentMethodSelector";

type BookingRequestParams = {
  id: string;
  title?: string;
  pricePerHour?: string;
  pricePerDay?: string;
  hostName?: string;
};

interface BookingResponse {
  transactionId: string;
  holdAmount: number;
  rentalFee: number;
  platformFee: number;
  totalImpact: number;
}

function estimateRentalFee(
  pricePerHour: number | undefined,
  pricePerDay: number | undefined,
  start: Date | null,
  end: Date | null,
): number {
  if (!start || !end) return 0;
  const ms = end.getTime() - start.getTime();
  if (ms <= 0) return 0;
  const hours = ms / (1000 * 60 * 60);
  if (pricePerDay != null && hours >= 24) {
    return Math.round(pricePerDay * Math.ceil(hours / 24) * 100);
  }
  if (pricePerHour != null) {
    return Math.round(pricePerHour * hours * 100);
  }
  return 0;
}

export default function BookingRequestScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<BookingRequestParams>();
  const user = useAuthStore((s) => s.user);

  const listingId = params.id;
  const title = params.title ?? "Rental";
  const pricePerHour = params.pricePerHour ? Number(params.pricePerHour) : undefined;
  const pricePerDay = params.pricePerDay ? Number(params.pricePerDay) : undefined;

  const { data: holdEstimate } = useHoldEstimate(listingId ?? null);

  const {
    scheduledStart,
    scheduledEnd,
    paymentMethodId,
    holdAmount,
    rentalFee,
    setSchedule,
    setPaymentMethod,
    setAmounts,
    reset,
  } = useCheckoutStore();

  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleScheduleChange = (start: Date, end: Date) => {
    setSchedule(start, end);
    const hold = holdEstimate?.holdAmount ?? 0;
    const fee = estimateRentalFee(pricePerHour, pricePerDay, start, end);
    setAmounts(hold, fee);
  };

  const canConfirm =
    scheduledStart != null &&
    scheduledEnd != null &&
    scheduledEnd.getTime() > scheduledStart.getTime() &&
    paymentMethodId != null &&
    !isSubmitting;

  const handleConfirm = async () => {
    if (!canConfirm || !listingId || !user) return;

    setIsSubmitting(true);
    try {
      const result = await api
        .post("api/v1/bookings", {
          json: {
            listingId,
            paymentMethodId,
            scheduledStart: scheduledStart!.toISOString(),
            scheduledEnd: scheduledEnd!.toISOString(),
          },
        })
        .json<BookingResponse>();

      reset();

      router.replace({
        pathname: "/(tabs)/(feed)/booking-status" as never,
        params: { transactionId: result.transactionId },
      });
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Booking failed. Please try again.";
      Alert.alert("Booking failed", message);
    } finally {
      setIsSubmitting(false);
    }
  };

  const displayHold = holdEstimate?.holdAmount ?? holdAmount;

  return (
    <SafeAreaView className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <View className="ml-2 flex-1">
          <Text className="text-lg font-semibold text-gray-900" numberOfLines={1}>
            Request to rent
          </Text>
          <Text className="text-xs text-gray-500" numberOfLines={1}>
            {title}
          </Text>
        </View>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ padding: 16, gap: 24, paddingBottom: 120 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Duration picker */}
        <DurationPicker
          start={scheduledStart}
          end={scheduledEnd}
          onChangeStart={(s) => {
            if (scheduledEnd) handleScheduleChange(s, scheduledEnd);
            else setSchedule(s, new Date(s.getTime() + 3_600_000));
          }}
          onChangeEnd={(e) => {
            if (scheduledStart) handleScheduleChange(scheduledStart, e);
          }}
        />

        {/* Cost breakdown — shown once dates are selected */}
        {scheduledStart && scheduledEnd && (
          <CostBreakdown
            rentalFee={rentalFee}
            holdAmount={displayHold}
            totalImpact={displayHold + rentalFee}
          />
        )}

        {/* Payment method */}
        <PaymentMethodSelector
          selectedPaymentMethodId={paymentMethodId}
          onPaymentMethodSelected={setPaymentMethod}
        />

        {/* How it works note */}
        <View className="bg-sky-50 rounded-xl px-4 py-3">
          <Text className="text-xs font-semibold text-sky-800 mb-1">
            How it works
          </Text>
          <Text className="text-xs text-sky-700 leading-relaxed">
            Your request is sent to the host. They have 2 hours to accept or
            decline. Nothing is charged until the host accepts.
          </Text>
        </View>
      </ScrollView>

      {/* Fixed confirm CTA */}
      <View className="absolute bottom-0 left-0 right-0 bg-white border-t border-gray-100 px-4 py-4">
        {scheduledStart && scheduledEnd && (
          <Text className="text-center text-sm text-gray-500 mb-2">
            Total card impact:{" "}
            <Text className="font-semibold text-gray-900">
              ${((displayHold + rentalFee) / 100).toFixed(2)}
            </Text>
          </Text>
        )}
        <Pressable
          onPress={handleConfirm}
          disabled={!canConfirm}
          className={`rounded-2xl py-4 items-center ${
            canConfirm ? "bg-sky-600" : "bg-gray-200"
          }`}
        >
          {isSubmitting ? (
            <ActivityIndicator color="white" />
          ) : (
            <Text
              className={`font-semibold text-base ${
                canConfirm ? "text-white" : "text-gray-400"
              }`}
            >
              Send Request
            </Text>
          )}
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
