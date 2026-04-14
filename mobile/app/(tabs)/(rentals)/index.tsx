/**
 * Rentals tab — shows all of the current user's bookings grouped by state.
 *
 * Active rentals show a "Return" action that navigates to the active-rental
 * management screen. Completed rentals show post-rental state: a rate prompt
 * badge if the user hasn't yet rated, and hold release status.
 */
import {
  View,
  Text,
  FlatList,
  Pressable,
  ActivityIndicator,
  RefreshControl,
  SafeAreaView,
} from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../../lib/auth";
import { useRenterBookings, useHostBookings } from "../../../lib/hooks/useBookings";
import { useTransactionRatings } from "../../../lib/hooks/useRatings";
import type { Booking, BookingStatus } from "../../../lib/hooks/useBooking";
import RatingPrompt from "../../../components/rating/RatingPrompt";
import { useState } from "react";

// ── Types ─────────────────────────────────────────────────────────────────────

interface RentalRowProps {
  booking: Booking;
  currentUserId: string;
  onPress: () => void;
  onRate?: () => void;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

const STATUS_COLOR: Record<BookingStatus, string> = {
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
  REQUESTED: "Pending",
  ACCEPTED: "Accepted",
  DECLINED: "Declined",
  AUTO_DECLINED: "Expired",
  ACTIVE: "Active",
  COMPLETED: "Completed",
  DISPUTED: "Disputed",
  CANCELLED: "Cancelled",
};

// ── RentalRow ─────────────────────────────────────────────────────────────────

function RentalRow({ booking, currentUserId, onPress, onRate }: RentalRowProps) {
  const isHost = currentUserId === booking.hostId;
  const color = STATUS_COLOR[booking.status];

  return (
    <Pressable
      testID={`rental-row-${booking.id.slice(-8).toLowerCase()}`}
      className="bg-white rounded-2xl mx-4 mb-3 overflow-hidden border border-gray-100 shadow-sm"
      onPress={onPress}
    >
      <View className="px-4 py-3 flex-row items-center justify-between border-b border-gray-50">
        <View className="flex-row items-center gap-x-2">
          <View
            className="w-2 h-2 rounded-full"
            style={{ backgroundColor: color }}
          />
          <Text testID={`status-${booking.status.toLowerCase()}`} className="text-xs font-semibold" style={{ color }}>
            {STATUS_LABEL[booking.status]}
          </Text>
        </View>
        <Text className="text-xs font-mono text-gray-400">
          {booking.id.slice(-8).toUpperCase()}
        </Text>
      </View>

      <View className="px-4 py-3">
        <Text className="text-sm text-gray-500 mb-0.5">
          {isHost ? "Renter booking" : "Your rental"}
        </Text>
        <Text className="text-xs text-gray-400">
          {formatDate(booking.scheduledStart)} → {formatDate(booking.scheduledEnd)}
        </Text>

        {/* Action row */}
        <View className="flex-row gap-x-2 mt-3">
          {booking.status === "ACTIVE" && (
            <Pressable
              className="flex-1 bg-green-600 rounded-xl py-2.5 items-center flex-row justify-center gap-x-1.5"
              onPress={(e) => {
                e.stopPropagation();
                onPress();
              }}
            >
              <Ionicons name="arrow-undo-outline" size={14} color="white" />
              <Text className="text-white text-xs font-semibold">Return</Text>
            </Pressable>
          )}

          {booking.status === "COMPLETED" && onRate && (
            <Pressable
              className="flex-row items-center gap-x-1 px-3 py-2 bg-amber-50 rounded-xl"
              onPress={(e) => {
                e.stopPropagation();
                onRate();
              }}
            >
              <Ionicons name="star-outline" size={13} color="#b45309" />
              <Text className="text-amber-700 text-xs font-semibold">Rate</Text>
            </Pressable>
          )}

          {booking.status === "DISPUTED" && (
            <Pressable
              testID="btn-view-dispute"
              className="flex-row items-center gap-x-1 px-3 py-2 bg-red-50 rounded-xl"
              onPress={(e) => {
                e.stopPropagation();
                onPress();
              }}
            >
              <Ionicons name="warning-outline" size={13} color="#dc2626" />
              <Text className="text-red-700 text-xs font-semibold">Dispute open</Text>
            </Pressable>
          )}
        </View>
      </View>
    </Pressable>
  );
}

// ── RatingAwareRow ─────────────────────────────────────────────────────────────

function RatingAwareRow({
  booking,
  currentUserId,
  onPress,
  onOpenRatingPrompt,
}: {
  booking: Booking;
  currentUserId: string;
  onPress: () => void;
  onOpenRatingPrompt: (transactionId: string) => void;
}) {
  const { data: ratings } = useTransactionRatings(
    booking.status === "COMPLETED" ? booking.id : null,
  );
  const hasRated = (ratings ?? []).some((r) => r.fromUserId === currentUserId);

  return (
    <RentalRow
      booking={booking}
      currentUserId={currentUserId}
      onPress={onPress}
      onRate={
        booking.status === "COMPLETED" && !hasRated
          ? () => onOpenRatingPrompt(booking.id)
          : undefined
      }
    />
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

export default function RentalsScreen() {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);

  const {
    data: renterData,
    isLoading: renterLoading,
    isRefetching: renterRefetching,
    refetch: refetchRenter,
  } = useRenterBookings();

  const {
    data: hostData,
    isLoading: hostLoading,
    isRefetching: hostRefetching,
    refetch: refetchHost,
  } = useHostBookings();

  const [ratingTransactionId, setRatingTransactionId] = useState<string | null>(null);

  const isLoading = renterLoading || hostLoading;
  const isRefetching = renterRefetching || hostRefetching;

  const handleRefresh = () => {
    void refetchRenter();
    void refetchHost();
  };

  // Merge and deduplicate bookings from both perspectives
  const allBookings: Booking[] = [
    ...(renterData?.bookings ?? []),
    ...(hostData?.bookings ?? []),
  ].filter(
    (b, i, arr) => arr.findIndex((x) => x.id === b.id) === i,
  );

  // Sort: active first, then by scheduled start descending
  const sorted = [...allBookings].sort((a, b) => {
    const order: Record<BookingStatus, number> = {
      ACTIVE: 0,
      ACCEPTED: 1,
      REQUESTED: 2,
      DISPUTED: 3,
      COMPLETED: 4,
      DECLINED: 5,
      AUTO_DECLINED: 5,
      CANCELLED: 5,
    };
    const diff = order[a.status] - order[b.status];
    if (diff !== 0) return diff;
    return new Date(b.scheduledStart).getTime() - new Date(a.scheduledStart).getTime();
  });

  function handleBookingPress(booking: Booking) {
    if (booking.status === "ACTIVE") {
      router.push({
        pathname: "/(tabs)/(feed)/active-rental" as never,
        params: { transactionId: booking.id },
      });
    } else if (booking.status === "COMPLETED") {
      router.push({
        pathname: "/(tabs)/(rentals)/return-confirmation" as never,
        params: { transactionId: booking.id },
      });
    } else if (booking.status === "DISPUTED") {
      router.push({
        pathname: "/(tabs)/(rentals)/dispute-status" as never,
        params: { transactionId: booking.id },
      });
    } else {
      router.push({
        pathname: "/(tabs)/(rentals)/booking-status" as never,
        params: { transactionId: booking.id },
      });
    }
  }

  if (isLoading) {
    return (
      <SafeAreaView className="flex-1 bg-gray-50 items-center justify-center">
        <ActivityIndicator size="large" color="#0284c7" />
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView testID="screen-rentals" className="flex-1 bg-gray-50">
      <View className="px-4 pt-4 pb-3 border-b border-gray-100 bg-white">
        <Text className="text-xl font-bold text-gray-900">My rentals</Text>
      </View>

      <FlatList
        data={sorted}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <RatingAwareRow
            booking={item}
            currentUserId={user?.id ?? ""}
            onPress={() => handleBookingPress(item)}
            onOpenRatingPrompt={(id) => setRatingTransactionId(id)}
          />
        )}
        ListEmptyComponent={
          <View className="flex-1 items-center justify-center py-24 px-8">
            <Text className="text-5xl mb-4">🤝</Text>
            <Text className="text-lg font-semibold text-gray-800 text-center">
              No rentals yet
            </Text>
            <Text className="text-sm text-gray-500 text-center mt-2">
              Your bookings as renter and host will appear here.
            </Text>
          </View>
        }
        contentContainerStyle={{ paddingTop: 12, paddingBottom: 24, flexGrow: 1 }}
        refreshControl={
          <RefreshControl
            refreshing={isRefetching}
            onRefresh={handleRefresh}
            tintColor="#0284c7"
          />
        }
      />

      {ratingTransactionId && user && (
        <RatingPrompt
          transactionId={ratingTransactionId}
          currentUserId={user.id}
          renterId={
            renterData?.bookings.find((b) => b.id === ratingTransactionId)
              ?.renterId ??
            hostData?.bookings.find((b) => b.id === ratingTransactionId)
              ?.renterId ??
            ""
          }
          visible={true}
          onDismiss={() => setRatingTransactionId(null)}
        />
      )}
    </SafeAreaView>
  );
}
