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
import { useHoldEstimate } from "../../../../../lib/hooks/useDiscovery";
import { useListing } from "../../../../../lib/hooks/useListing";
import { useAuthStore } from "../../../../../lib/auth";
import { useCheckoutStore } from "../../../../../lib/stores/checkoutStore";
import { api } from "../../../../../lib/api";
import DurationPicker from "../../../../../components/checkout/DurationPicker";
import CostBreakdown from "../../../../../components/checkout/CostBreakdown";
import PaymentMethodSelector from "../../../../../components/checkout/PaymentMethodSelector";

type CheckoutParams = {
  id: string;
};

interface BookingResponse {
  transactionId: string;
  holdAmount: number;
  rentalFee: number;
  platformFee: number;
  totalImpact: number;
}

// Estimate rental fee from listing hold estimate and duration.
// The backend computes exact fees on booking; this is a display estimate.
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
    const days = Math.ceil(hours / 24);
    return Math.round(pricePerDay * days * 100);
  }
  if (pricePerHour != null) {
    return Math.round(pricePerHour * hours * 100);
  }
  return 0;
}

export default function CheckoutScreen() {
  const router = useRouter();
  const { id } = useLocalSearchParams<CheckoutParams>();
  const user = useAuthStore((s) => s.user);

  const { data: holdEstimate } = useHoldEstimate(id ?? null);
  const { data: listingData } = useListing(id ?? null);
  const listing = listingData?.listing;

  const {
    scheduledStart,
    scheduledEnd,
    paymentMethodId,
    holdAmount,
    rentalFee,
    totalImpact,
    setSchedule,
    setPaymentMethod,
    setAmounts,
    reset,
  } = useCheckoutStore();

  const [isSubmitting, setIsSubmitting] = useState(false);

  // KYC gate — user.identityStatus is not yet in the Phase 2 User type;
  // the field will be populated in Phase 4. We stub the check here.
  const identityStatus = (user as unknown as { identityStatus?: string })
    ?.identityStatus;
  const isVerified = identityStatus == null || identityStatus === "VERIFIED";

  const handleScheduleChange = (start: Date, end: Date) => {
    setSchedule(start, end);
    const hold = holdEstimate?.holdAmount ?? 0;
    const fee = estimateRentalFee(
      listing?.pricePerHour,
      listing?.pricePerDay,
      start,
      end,
    );
    setAmounts(hold, fee);
  };

  const handlePaymentMethodSelected = (methodId: string) => {
    setPaymentMethod(methodId);
  };

  const canConfirm =
    isVerified &&
    scheduledStart != null &&
    scheduledEnd != null &&
    scheduledEnd.getTime() > scheduledStart.getTime() &&
    paymentMethodId != null &&
    !isSubmitting;

  const handleConfirm = async () => {
    if (!canConfirm || !id || !user) return;

    setIsSubmitting(true);
    try {
      const result = await api
        .post("api/v1/bookings", {
          json: {
            listingId: id,
            paymentMethodId,
            scheduledStart: scheduledStart!.toISOString(),
            scheduledEnd: scheduledEnd!.toISOString(),
          },
        })
        .json<BookingResponse>();

      reset();

      router.replace({
        pathname: "/listing/[id]/confirmation" as never,
        params: {
          id,
          transactionId: result.transactionId,
          holdAmount: String(result.holdAmount),
          rentalFee: String(result.rentalFee),
          totalImpact: String(result.totalImpact),
          scheduledStart: scheduledStart!.toISOString(),
          scheduledEnd: scheduledEnd!.toISOString(),
        },
      });
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Booking failed. Please try again.";
      Alert.alert("Booking failed", message);
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isVerified) {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <Ionicons name="chevron-back" size={24} color="#111827" />
          </Pressable>
          <Text className="text-lg font-semibold text-gray-900 ml-2">
            Checkout
          </Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Ionicons name="shield-outline" size={56} color="#f59e0b" />
          <Text className="text-xl font-bold text-gray-900 text-center mt-4">
            Identity verification required
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-2">
            Please verify your identity to rent items on RentMy.
          </Text>
          <Pressable className="mt-6 px-6 py-3 bg-sky-600 rounded-2xl">
            <Text className="text-white font-semibold">Verify Identity</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  const displayHold = holdEstimate?.holdAmount ?? holdAmount;
  const displayTotal = displayHold + rentalFee;

  return (
    <SafeAreaView className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Checkout
        </Text>
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
            else setSchedule(s, new Date(s.getTime() + 3600000));
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
          onPaymentMethodSelected={handlePaymentMethodSelected}
        />
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
              Confirm Booking
            </Text>
          )}
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
