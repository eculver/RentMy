import { View, Text, Pressable, SafeAreaView } from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

type ConfirmationParams = {
  id: string;
  transactionId: string;
  holdAmount: string;
  rentalFee: string;
  totalImpact: string;
  scheduledStart: string;
  scheduledEnd: string;
};

function dollars(centsStr: string): string {
  const cents = parseInt(centsStr, 10);
  return isNaN(cents) ? "$0.00" : `$${(cents / 100).toFixed(2)}`;
}

function formatDate(isoStr: string): string {
  try {
    return new Date(isoStr).toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    });
  } catch {
    return isoStr;
  }
}

export default function ConfirmationScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<ConfirmationParams>();

  return (
    <SafeAreaView className="flex-1 bg-white">
      <View className="flex-1 items-center justify-center px-8">
        {/* Success icon */}
        <View className="w-20 h-20 bg-green-100 rounded-full items-center justify-center mb-6">
          <Ionicons name="checkmark-circle" size={48} color="#16a34a" />
        </View>

        <Text className="text-2xl font-bold text-gray-900 text-center">
          Booking confirmed!
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-2">
          Your rental has been booked. The host has been notified.
        </Text>

        {/* Booking summary card */}
        <View className="w-full mt-8 border border-gray-100 rounded-2xl overflow-hidden">
          <View className="bg-gray-50 px-4 py-3">
            <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
              Booking summary
            </Text>
          </View>

          {params.scheduledStart && params.scheduledEnd && (
            <View className="px-4 py-3 border-b border-gray-100">
              <Text className="text-xs text-gray-500 mb-1">Rental period</Text>
              <Text className="text-sm font-medium text-gray-900">
                {formatDate(params.scheduledStart)} →{" "}
                {formatDate(params.scheduledEnd)}
              </Text>
            </View>
          )}

          <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
            <Text className="text-sm text-gray-600">Rental fee</Text>
            <Text className="text-sm font-medium text-gray-900">
              {dollars(params.rentalFee)}
            </Text>
          </View>

          <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
            <Text className="text-sm text-gray-600">Hold (released on return)</Text>
            <Text className="text-sm font-medium text-gray-900">
              {dollars(params.holdAmount)}
            </Text>
          </View>

          <View className="px-4 py-3 flex-row justify-between bg-gray-50">
            <Text className="text-sm font-semibold text-gray-900">
              Total card impact
            </Text>
            <Text className="text-sm font-bold text-gray-900">
              {dollars(params.totalImpact)}
            </Text>
          </View>
        </View>

        {/* CTAs */}
        <View className="w-full mt-8 gap-y-3">
          <Pressable
            className="bg-sky-600 rounded-2xl py-4 items-center"
            onPress={() => {
              // Navigate to messages — Phase 3 will wire this to a specific thread
              router.replace("/(tabs)/(messages)");
            }}
          >
            <Text className="text-white font-semibold text-base">
              Message Host
            </Text>
          </Pressable>

          <Pressable
            className="border border-gray-200 rounded-2xl py-4 items-center"
            onPress={() => router.replace("/(tabs)/(feed)")}
          >
            <Text className="text-gray-700 font-semibold text-base">
              View My Bookings
            </Text>
          </Pressable>
        </View>
      </View>
    </SafeAreaView>
  );
}
