import { useState, useEffect, useRef } from "react";
import { View, Text, Pressable, ActivityIndicator } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import type { Booking } from "../../lib/hooks/useBooking";

interface IncomingRequestProps {
  booking: Booking;
  renterName?: string;
  renterReputation?: number;
  /** Auto-decline timeout in seconds from booking creation. Default: 7200 (2 hours). */
  autoDeclineSeconds?: number;
  onAccept: () => Promise<void>;
  onDecline: () => Promise<void>;
}

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

function formatCountdown(seconds: number): string {
  if (seconds <= 0) return "Expired";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export default function IncomingRequest({
  booking,
  renterName,
  renterReputation,
  autoDeclineSeconds = 7200,
  onAccept,
  onDecline,
}: IncomingRequestProps) {
  const [accepting, setAccepting] = useState(false);
  const [declining, setDeclining] = useState(false);

  // Countdown timer
  const createdMs = new Date(booking.createdAt).getTime();
  const expiresMs = createdMs + autoDeclineSeconds * 1000;

  const [remaining, setRemaining] = useState(() =>
    Math.max(0, Math.round((expiresMs - Date.now()) / 1000)),
  );

  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    intervalRef.current = setInterval(() => {
      const secs = Math.max(0, Math.round((expiresMs - Date.now()) / 1000));
      setRemaining(secs);
      if (secs === 0 && intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    }, 1000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [expiresMs]);

  const handleAccept = async () => {
    setAccepting(true);
    try {
      await onAccept();
    } finally {
      setAccepting(false);
    }
  };

  const handleDecline = async () => {
    setDeclining(true);
    try {
      await onDecline();
    } finally {
      setDeclining(false);
    }
  };

  const busy = accepting || declining;

  return (
    <View className="bg-white rounded-2xl border border-gray-100 shadow-sm p-4 mb-3 mx-4">
      {/* Header */}
      <View className="flex-row items-center justify-between mb-3">
        <Text className="text-base font-semibold text-gray-900">
          New rental request
        </Text>
        <View
          className={`flex-row items-center gap-x-1 px-2 py-0.5 rounded-full ${
            remaining > 0 ? "bg-amber-50" : "bg-gray-100"
          }`}
        >
          <Ionicons
            name="time-outline"
            size={12}
            color={remaining > 0 ? "#b45309" : "#9ca3af"}
          />
          <Text
            className={`text-xs font-medium ${
              remaining > 0 ? "text-amber-700" : "text-gray-500"
            }`}
          >
            {formatCountdown(remaining)}
          </Text>
        </View>
      </View>

      {/* Renter info */}
      {renterName && (
        <View className="flex-row items-center gap-x-2 mb-3">
          <View className="w-8 h-8 rounded-full bg-sky-100 items-center justify-center">
            <Ionicons name="person" size={16} color="#0284c7" />
          </View>
          <View>
            <Text className="text-sm font-medium text-gray-900">{renterName}</Text>
            {renterReputation != null && (
              <View className="flex-row items-center gap-x-1">
                <Ionicons name="star" size={11} color="#f59e0b" />
                <Text className="text-xs text-gray-500">
                  {(renterReputation / 200).toFixed(1)} reputation
                </Text>
              </View>
            )}
          </View>
        </View>
      )}

      {/* Date range */}
      <View className="flex-row items-center gap-x-1 mb-4">
        <Ionicons name="calendar-outline" size={14} color="#6b7280" />
        <Text className="text-sm text-gray-600">
          {formatDateRange(booking.scheduledStart, booking.scheduledEnd)}
        </Text>
      </View>

      {/* Action buttons */}
      <View className="flex-row gap-x-3">
        <Pressable
          onPress={handleDecline}
          disabled={busy}
          className="flex-1 border border-gray-200 rounded-xl py-3 items-center"
        >
          {declining ? (
            <ActivityIndicator size="small" color="#6b7280" />
          ) : (
            <Text className="text-sm font-semibold text-gray-600">Decline</Text>
          )}
        </Pressable>
        <Pressable
          onPress={handleAccept}
          disabled={busy}
          className={`flex-1 rounded-xl py-3 items-center ${busy ? "bg-gray-200" : "bg-sky-600"}`}
        >
          {accepting ? (
            <ActivityIndicator size="small" color="white" />
          ) : (
            <Text
              className={`text-sm font-semibold ${busy ? "text-gray-400" : "text-white"}`}
            >
              Accept
            </Text>
          )}
        </Pressable>
      </View>
    </View>
  );
}
