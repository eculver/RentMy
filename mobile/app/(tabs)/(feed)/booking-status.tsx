/**
 * Booking status screen — shows the live state of a booking and provides
 * contextual action buttons (accept/decline for hosts, cancel, navigate to
 * pickup, etc.). Real-time updates arrive via a Pusher subscription on the
 * `private-transaction-{id}` channel.
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
  RefreshControl,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../../lib/auth";
import { useBooking, type BookingStatus } from "../../../lib/hooks/useBooking";
import { api } from "../../../lib/api";
import IncomingRequest from "../../../components/booking/IncomingRequest";
import CancelConfirmation from "../../../components/booking/CancelConfirmation";

type BookingStatusParams = {
  transactionId: string;
};

// ── Status display config ────────────────────────────────────────────────────

const STATUS_ICON: Record<BookingStatus, string> = {
  REQUESTED: "time-outline",
  ACCEPTED: "checkmark-circle-outline",
  DECLINED: "close-circle-outline",
  AUTO_DECLINED: "timer-outline",
  ACTIVE: "play-circle-outline",
  COMPLETED: "trophy-outline",
  DISPUTED: "warning-outline",
  CANCELLED: "close-circle-outline",
};

const STATUS_ICON_COLOR: Record<BookingStatus, string> = {
  REQUESTED: "#b45309",
  ACCEPTED: "#0284c7",
  DECLINED: "#6b7280",
  AUTO_DECLINED: "#6b7280",
  ACTIVE: "#16a34a",
  COMPLETED: "#059669",
  DISPUTED: "#dc2626",
  CANCELLED: "#6b7280",
};

const STATUS_LABEL: Record<BookingStatus, string> = {
  REQUESTED: "Waiting for host",
  ACCEPTED: "Booking accepted",
  DECLINED: "Booking declined",
  AUTO_DECLINED: "Request expired",
  ACTIVE: "Rental in progress",
  COMPLETED: "Rental completed",
  DISPUTED: "Dispute open",
  CANCELLED: "Booking cancelled",
};

const STATUS_DESC: Record<BookingStatus, string> = {
  REQUESTED:
    "The host has been notified and has 2 hours to accept or decline your request.",
  ACCEPTED:
    "Your booking is confirmed. Head to the pickup location when you're ready.",
  DECLINED:
    "The host declined your request. Your payment method was not charged.",
  AUTO_DECLINED:
    "The host didn't respond in time. Your payment method was not charged.",
  ACTIVE: "Your rental is active. Return the item before the scheduled end time.",
  COMPLETED:
    "This rental is complete. Thanks for using RentMy!",
  DISPUTED: "A dispute has been opened. Our team will review this rental.",
  CANCELLED: "This booking was cancelled.",
};

// ── Helpers ──────────────────────────────────────────────────────────────────

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

// ── Component ────────────────────────────────────────────────────────────────

export default function BookingStatusScreen() {
  const router = useRouter();
  const { transactionId } = useLocalSearchParams<BookingStatusParams>();
  const user = useAuthStore((s) => s.user);

  const { data, isLoading, isRefetching, refetch, error } = useBooking(
    transactionId ?? null,
  );

  const [cancelModalVisible, setCancelModalVisible] = useState(false);

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
          <Text className="text-lg font-semibold text-gray-900 ml-2">Booking</Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Text className="text-gray-500 text-center">
            Unable to load booking. Please try again.
          </Text>
          <Pressable
            className="mt-4 px-6 py-3 bg-sky-600 rounded-xl"
            onPress={() => void refetch()}
          >
            <Text className="text-white font-semibold">Retry</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  const { booking } = data;
  const isHost = user?.id === booking.hostId;
  const isRenter = user?.id === booking.renterId;
  const isTerminal = ["COMPLETED", "DECLINED", "AUTO_DECLINED", "CANCELLED"].includes(
    booking.status,
  );

  const iconName = STATUS_ICON[booking.status] as React.ComponentProps<
    typeof Ionicons
  >["name"];
  const iconColor = STATUS_ICON_COLOR[booking.status];

  // ── Actions ───────────────────────────────────────────────────────────────

  const handleAccept = async () => {
    try {
      await api.post(`api/v1/bookings/${booking.id}/accept`);
      await refetch();
    } catch {
      Alert.alert("Error", "Failed to accept booking. Please try again.");
    }
  };

  const handleDecline = async () => {
    try {
      await api.post(`api/v1/bookings/${booking.id}/decline`);
      await refetch();
    } catch {
      Alert.alert("Error", "Failed to decline booking. Please try again.");
    }
  };

  const handleCancel = async () => {
    try {
      await api.post(`api/v1/bookings/${booking.id}/cancel`);
      setCancelModalVisible(false);
      await refetch();
    } catch {
      Alert.alert("Error", "Failed to cancel booking. Please try again.");
      setCancelModalVisible(false);
    }
  };

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <SafeAreaView className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">Booking</Text>
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
            style={{ backgroundColor: `${iconColor}18` }}
          >
            <Ionicons name={iconName} size={36} color={iconColor} />
          </View>
          <Text className="text-xl font-bold text-gray-900 text-center">
            {STATUS_LABEL[booking.status]}
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-1 px-4 leading-relaxed">
            {STATUS_DESC[booking.status]}
          </Text>
        </View>

        {/* Host receives IncomingRequest card while status is REQUESTED */}
        {isHost && booking.status === "REQUESTED" && (
          <IncomingRequest
            booking={booking}
            onAccept={handleAccept}
            onDecline={handleDecline}
          />
        )}

        {/* Booking details card */}
        <View className="bg-gray-50 rounded-2xl overflow-hidden">
          <View className="px-4 py-3 border-b border-gray-200">
            <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
              Booking details
            </Text>
          </View>

          <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between items-center">
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
            <Text className="text-sm text-gray-600 mb-0.5">Return</Text>
            <Text className="text-sm font-medium text-gray-900">
              {formatDate(booking.scheduledEnd)}
            </Text>
          </View>

          <View className="px-4 py-3 flex-row justify-between items-center">
            <Text className="text-sm text-gray-600">Status</Text>
            <Text className="text-sm font-semibold" style={{ color: iconColor }}>
              {booking.status}
            </Text>
          </View>
        </View>

        {/* Cancellation fee notice (when cancelled and fee was applied) */}
        {booking.status === "CANCELLED" &&
          booking.cancellationFee != null &&
          booking.cancellationFee > 0 && (
            <View className="bg-amber-50 rounded-xl px-4 py-3 flex-row items-start gap-x-2">
              <Ionicons name="information-circle-outline" size={16} color="#b45309" />
              <Text className="text-sm text-amber-800 flex-1">
                A cancellation fee of{" "}
                <Text className="font-semibold">
                  ${(booking.cancellationFee / 100).toFixed(2)}
                </Text>{" "}
                was applied.
              </Text>
            </View>
          )}

        {/* Action buttons */}
        {!isTerminal && (
          <View className="gap-y-3">
            {/* Renter: navigate to pickup when accepted */}
            {isRenter && booking.status === "ACCEPTED" && (
              <Pressable
                className="bg-sky-600 rounded-2xl py-4 items-center flex-row justify-center gap-x-2"
                onPress={() =>
                  router.push({
                    pathname: "/(tabs)/(feed)/check-in" as never,
                    params: { transactionId: booking.id },
                  })
                }
              >
                <Ionicons name="navigate-outline" size={18} color="white" />
                <Text className="text-white font-semibold text-base">
                  Start check-in
                </Text>
              </Pressable>
            )}

            {/* Active: navigate to return */}
            {booking.status === "ACTIVE" && (
              <Pressable
                className="bg-green-600 rounded-2xl py-4 items-center flex-row justify-center gap-x-2"
                onPress={() =>
                  router.push({
                    pathname: "/(tabs)/(feed)/check-out" as never,
                    params: { transactionId: booking.id },
                  })
                }
              >
                <Ionicons name="arrow-undo-outline" size={18} color="white" />
                <Text className="text-white font-semibold text-base">
                  Start check-out
                </Text>
              </Pressable>
            )}

            {/* Message the other party */}
            <Pressable
              className="border border-gray-200 rounded-2xl py-4 items-center flex-row justify-center gap-x-2"
              onPress={() =>
                router.push({
                  pathname: "/(tabs)/(messages)" as never,
                })
              }
            >
              <Ionicons name="chatbubble-outline" size={18} color="#374151" />
              <Text className="text-gray-700 font-semibold text-base">
                Message {isHost ? "renter" : "host"}
              </Text>
            </Pressable>

            {/* Cancel (available while REQUESTED or ACCEPTED) */}
            {(booking.status === "REQUESTED" || booking.status === "ACCEPTED") && (
              <Pressable
                className="py-3 items-center"
                onPress={() => setCancelModalVisible(true)}
              >
                <Text className="text-sm text-red-500 font-medium">
                  Cancel booking
                </Text>
              </Pressable>
            )}
          </View>
        )}

        {/* Terminal: go back to feed */}
        {isTerminal && (
          <Pressable
            className="border border-gray-200 rounded-2xl py-4 items-center"
            onPress={() => router.replace("/(tabs)/(feed)" as never)}
          >
            <Text className="text-gray-700 font-semibold text-base">
              Back to feed
            </Text>
          </Pressable>
        )}
      </ScrollView>

      <CancelConfirmation
        visible={cancelModalVisible}
        scheduledStart={booking.scheduledStart}
        onConfirm={handleCancel}
        onDismiss={() => setCancelModalVisible(false)}
      />
    </SafeAreaView>
  );
}
