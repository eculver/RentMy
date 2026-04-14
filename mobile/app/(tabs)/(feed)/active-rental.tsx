/**
 * Active rental screen — shown when a rental is in progress (status ACTIVE).
 *
 * Displays a live countdown to scheduled_end, a "Navigate to return" button
 * that deep-links to the native Maps app, a "Start check-out" button, and a
 * late return warning banner when past the scheduled end time.
 */
import { useState, useEffect } from "react";
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
import * as Linking from "expo-linking";
import { Ionicons } from "@expo/vector-icons";
import { useBooking } from "../../../lib/hooks/useBooking";

type Params = { transactionId: string };

// ── Helpers ──────────────────────────────────────────────────────────────────

function formatCountdown(ms: number): string {
  if (ms <= 0) return "00:00:00";
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  return [h, m, s].map((v) => String(v).padStart(2, "0")).join(":");
}

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

export default function ActiveRentalScreen() {
  const router = useRouter();
  const { transactionId } = useLocalSearchParams<Params>();

  const { data, isLoading, error } = useBooking(transactionId ?? null);

  const [now, setNow] = useState(() => Date.now());

  // Tick every second for countdown
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

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
            Active rental
          </Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Text className="text-gray-500 text-center">
            Unable to load rental details.
          </Text>
        </View>
      </SafeAreaView>
    );
  }

  const { booking } = data;
  const endMs = new Date(booking.scheduledEnd).getTime();
  const remaining = endMs - now;
  const isLate = remaining < 0;

  const handleNavigate = () => {
    // Open the native Maps app; item pickup address is at the listing location.
    // Until the booking API returns listing coordinates, we open a generic Maps search.
    const url = `maps://maps.apple.com/?q=RentMy+pickup+${booking.listingId}`;
    void Linking.openURL(url).catch(() => {
      Alert.alert(
        "Maps unavailable",
        "Could not open the Maps app on this device.",
      );
    });
  };

  const handleReportIssue = () => {
    router.push({
      pathname: "/(tabs)/(rentals)/dispute" as never,
      params: { transactionId: booking.id },
    });
  };

  return (
    <View testID="screen-active-rental" className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Active rental
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ padding: 16, gap: 16 }}
      >
        {/* Late return warning banner */}
        {isLate && (
          <View className="bg-red-50 border border-red-200 rounded-2xl px-4 py-3 flex-row items-start gap-x-3">
            <Ionicons name="warning" size={20} color="#dc2626" />
            <View className="flex-1">
              <Text className="text-sm font-semibold text-red-800">
                Late return
              </Text>
              <Text className="text-xs text-red-600 mt-0.5">
                The scheduled return time has passed. Please return the item as
                soon as possible to avoid additional fees.
              </Text>
            </View>
          </View>
        )}

        {/* Countdown hero */}
        <View className="bg-gray-50 rounded-2xl px-4 py-6 items-center gap-y-2">
          <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
            {isLate ? "Time over by" : "Time remaining"}
          </Text>
          <Text
            testID="active-rental-countdown"
            className={`text-5xl font-bold font-mono tracking-wider ${
              isLate
                ? "text-red-600"
                : remaining < 30 * 60 * 1000
                ? "text-amber-600"
                : "text-gray-900"
            }`}
          >
            {formatCountdown(Math.abs(remaining))}
          </Text>
          <Text className="text-xs text-gray-500">
            Due {formatDate(booking.scheduledEnd)}
          </Text>
        </View>

        {/* Booking details */}
        <View className="bg-gray-50 rounded-2xl overflow-hidden">
          <View className="px-4 py-3 border-b border-gray-200">
            <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
              Rental details
            </Text>
          </View>
          <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
            <Text className="text-sm text-gray-600">Booking ID</Text>
            <Text className="text-xs font-mono text-gray-500">
              {booking.id.slice(-8).toUpperCase()}
            </Text>
          </View>
          <View className="px-4 py-3 border-b border-gray-100">
            <Text className="text-sm text-gray-600 mb-0.5">Started</Text>
            <Text className="text-sm font-medium text-gray-900">
              {booking.actualStart
                ? formatDate(booking.actualStart)
                : formatDate(booking.scheduledStart)}
            </Text>
          </View>
          <View className="px-4 py-3">
            <Text className="text-sm text-gray-600 mb-0.5">Due back</Text>
            <Text
              className={`text-sm font-medium ${
                isLate ? "text-red-600" : "text-gray-900"
              }`}
            >
              {formatDate(booking.scheduledEnd)}
            </Text>
          </View>
        </View>

        {/* Navigate to return */}
        <Pressable
          testID="btn-navigate-return"
          className="bg-sky-600 rounded-2xl py-4 items-center flex-row justify-center gap-x-2"
          onPress={handleNavigate}
        >
          <Ionicons name="navigate-outline" size={18} color="white" />
          <Text className="text-white font-semibold text-base">
            Navigate to return location
          </Text>
        </Pressable>

        {/* Start check-out */}
        <Pressable
          testID="btn-start-checkout"
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

        {/* Report issue */}
        <Pressable
          testID="btn-report-issue"
          className="border border-red-200 rounded-2xl py-3 items-center flex-row justify-center gap-x-2"
          onPress={handleReportIssue}
        >
          <Ionicons name="flag-outline" size={16} color="#dc2626" />
          <Text className="text-red-600 font-medium text-sm">
            Report an issue
          </Text>
        </Pressable>
      </ScrollView>
    </View>
  );
}
