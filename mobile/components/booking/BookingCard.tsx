import { View, Text, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import type { Booking, BookingStatus } from "../../lib/hooks/useBooking";

interface BookingCardProps {
  booking: Booking;
  /** Label for the other party (host or renter name). */
  otherPartyName?: string;
  /** Listing title. */
  listingTitle?: string;
  onPress?: () => void;
}

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

const STATUS_COLOR: Record<BookingStatus, string> = {
  REQUESTED: "bg-amber-100 text-amber-700",
  ACCEPTED: "bg-sky-100 text-sky-700",
  DECLINED: "bg-gray-100 text-gray-500",
  AUTO_DECLINED: "bg-gray-100 text-gray-500",
  ACTIVE: "bg-green-100 text-green-700",
  COMPLETED: "bg-emerald-100 text-emerald-700",
  DISPUTED: "bg-red-100 text-red-600",
  CANCELLED: "bg-gray-100 text-gray-500",
};

function formatDateRange(start: string, end: string): string {
  const s = new Date(start);
  const e = new Date(end);
  const opts: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  };
  return `${s.toLocaleDateString("en-US", opts)} → ${e.toLocaleDateString("en-US", opts)}`;
}

export default function BookingCard({
  booking,
  otherPartyName,
  listingTitle,
  onPress,
}: BookingCardProps) {
  const badgeClass = STATUS_COLOR[booking.status] ?? "bg-gray-100 text-gray-500";
  const label = STATUS_LABEL[booking.status] ?? booking.status;

  return (
    <Pressable
      onPress={onPress}
      className="bg-white rounded-2xl border border-gray-100 shadow-sm p-4 mb-3 mx-4"
    >
      {/* Header row: title + status badge */}
      <View className="flex-row items-start justify-between gap-x-2">
        <Text
          className="text-base font-semibold text-gray-900 flex-1"
          numberOfLines={1}
        >
          {listingTitle ?? "Rental booking"}
        </Text>
        <View className={`px-2 py-0.5 rounded-full ${badgeClass.split(" ")[0]}`}>
          <Text className={`text-xs font-medium ${badgeClass.split(" ")[1]}`}>
            {label}
          </Text>
        </View>
      </View>

      {/* Date range */}
      <View className="flex-row items-center mt-2 gap-x-1">
        <Ionicons name="calendar-outline" size={13} color="#6b7280" />
        <Text className="text-xs text-gray-500">
          {formatDateRange(booking.scheduledStart, booking.scheduledEnd)}
        </Text>
      </View>

      {/* Other party */}
      {otherPartyName && (
        <View className="flex-row items-center mt-1 gap-x-1">
          <Ionicons name="person-outline" size={13} color="#6b7280" />
          <Text className="text-xs text-gray-500">{otherPartyName}</Text>
        </View>
      )}

      {/* Chevron */}
      <View className="absolute right-4 top-1/2 -translate-y-2">
        <Ionicons name="chevron-forward" size={16} color="#d1d5db" />
      </View>
    </Pressable>
  );
}
