/**
 * GPSStatus — displays real-time GPS proximity status during a handoff.
 *
 * Shows a green "Within range" or red "Too far away" indicator based on
 * whether GPS has been verified by the server, with an action button to
 * trigger or re-trigger verification.
 */
import { View, Text, Pressable, ActivityIndicator } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface GPSStatusProps {
  verified: boolean;
  verifying: boolean;
  error: string | null;
  hasLocation: boolean;
  onVerify: () => void;
}

export default function GPSStatus({
  verified,
  verifying,
  error,
  hasLocation,
  onVerify,
}: GPSStatusProps) {
  return (
    <View className="bg-gray-50 rounded-2xl px-4 py-4 gap-y-3">
      {/* Header row */}
      <View className="flex-row items-center gap-x-3">
        <View
          className={`w-10 h-10 rounded-full items-center justify-center ${
            verified ? "bg-green-100" : error ? "bg-red-100" : "bg-gray-200"
          }`}
        >
          {verifying ? (
            <ActivityIndicator size="small" color="#0284c7" />
          ) : verified ? (
            <Ionicons name="checkmark-circle" size={24} color="#16a34a" />
          ) : error ? (
            <Ionicons name="close-circle" size={24} color="#dc2626" />
          ) : (
            <Ionicons name="location-outline" size={24} color="#6b7280" />
          )}
        </View>

        <View className="flex-1">
          <Text className="text-sm font-semibold text-gray-900">
            {verified
              ? "Location verified"
              : verifying
              ? "Verifying location…"
              : "GPS proximity check"}
          </Text>
          <Text className="text-xs text-gray-500 mt-0.5">
            {verified
              ? "You are within range of the pickup location."
              : hasLocation
              ? "Tap below to verify you are within 100 m of the item."
              : "Acquiring GPS signal…"}
          </Text>
        </View>
      </View>

      {/* Error notice */}
      {error && !verified && (
        <View className="bg-red-50 rounded-xl px-3 py-2">
          <Text className="text-xs text-red-700">{error}</Text>
        </View>
      )}

      {/* Verify button */}
      {!verified && (
        <Pressable
          testID="btn-verify-location"
          onPress={onVerify}
          disabled={verifying || !hasLocation}
          className={`rounded-xl py-3 items-center ${
            verifying || !hasLocation ? "bg-gray-200" : "bg-sky-600"
          }`}
        >
          <Text
            className={`text-sm font-semibold ${
              verifying || !hasLocation ? "text-gray-400" : "text-white"
            }`}
          >
            {verifying ? "Verifying…" : "Verify my location"}
          </Text>
        </Pressable>
      )}
    </View>
  );
}
