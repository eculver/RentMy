/**
 * Check-in screen — GPS + PIN + photos required for both host and renter.
 *
 * Renter flow: verify GPS → enter PIN from host → take ≥3 photos → complete.
 * Host flow:   verify GPS → show/send PIN to renter → take ≥3 photos → complete.
 *
 * Calls POST /api/v1/bookings/:id/check-in with captured mediaIds when complete.
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
import { useQueryClient } from "@tanstack/react-query";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../lib/auth";
import { useBooking } from "../../lib/hooks/useBooking";
import { useProximity } from "../../lib/hooks/useProximity";
import { api } from "../../lib/api";
import GPSStatus from "../handoff/GPSStatus";
import PINEntry from "../handoff/PINEntry";
import PINDisplay from "../handoff/PINDisplay";
import PhotoGrid from "../handoff/PhotoGrid";
import AngleEnforcedCamera from "../camera/AngleEnforcedCamera";

type Params = { transactionId: string };

const MIN_PHOTOS = 3;

export default function CheckInScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { transactionId } = useLocalSearchParams<Params>();
  const user = useAuthStore((s) => s.user);

  const { data: bookingData, isLoading } = useBooking(transactionId ?? null);
  const isRenter =
    bookingData !== undefined && user?.id === bookingData.booking.renterId;

  const proximity = useProximity(transactionId ?? "", "CHECK_IN", isRenter);

  const [showCamera, setShowCamera] = useState(false);
  const [completing, setCompleting] = useState(false);

  const handleComplete = async () => {
    if (!proximity.canComplete) return;
    setCompleting(true);
    try {
      await api.post(`api/v1/bookings/${transactionId}/check-in`, {
        json: { mediaIds: [] }, // photo upload handled via MediaService in a future task
      });
      await queryClient.invalidateQueries({ queryKey: ["bookings", transactionId] });
      router.replace({
        pathname: "/(tabs)/(feed)/booking-status" as never,
        params: { transactionId },
      });
    } catch {
      Alert.alert("Check-in failed", "Could not complete check-in. Please try again.");
    } finally {
      setCompleting(false);
    }
  };

  if (isLoading || !bookingData) {
    return (
      <SafeAreaView className="flex-1 bg-white items-center justify-center">
        <ActivityIndicator size="large" color="#0284c7" />
      </SafeAreaView>
    );
  }

  // Full-screen camera mode
  if (showCamera) {
    return (
      <View className="flex-1 bg-black">
        <AngleEnforcedCamera
          captures={proximity.photos}
          onCapture={proximity.addPhoto}
          onDone={() => setShowCamera(false)}
          maxPhotos={6}
        />
        <Pressable
          className="absolute top-12 left-4 bg-black/60 rounded-full w-10 h-10 items-center justify-center"
          onPress={() => setShowCamera(false)}
        >
          <Ionicons name="chevron-back" size={22} color="white" />
        </Pressable>
      </View>
    );
  }

  return (
    <View testID="screen-check-in" className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Check-in
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ padding: 16, gap: 16 }}
      >
        {/* Role badge */}
        <View className="flex-row items-center gap-x-2 bg-sky-50 rounded-xl px-3 py-2">
          <Ionicons name="person-outline" size={16} color="#0284c7" />
          <Text className="text-xs font-medium text-sky-700">
            {isRenter ? "You are the renter" : "You are the host"}
          </Text>
        </View>

        {/* Step 1: GPS */}
        <View className="gap-y-2">
          <View className="flex-row items-center gap-x-2">
            <View
              className={`w-6 h-6 rounded-full items-center justify-center ${
                proximity.gpsVerified ? "bg-green-100" : "bg-gray-200"
              }`}
            >
              <Text
                className={`text-xs font-bold ${
                  proximity.gpsVerified ? "text-green-700" : "text-gray-500"
                }`}
              >
                1
              </Text>
            </View>
            <Text className="text-sm font-semibold text-gray-700">
              Verify your location
            </Text>
          </View>
          <GPSStatus
            verified={proximity.gpsVerified}
            verifying={proximity.gpsVerifying}
            error={proximity.gpsError}
            hasLocation={
              proximity.currentLat !== null && proximity.currentLng !== null
            }
            onVerify={proximity.verifyGPS}
          />
        </View>

        {/* Step 2: PIN (host shows; renter enters) */}
        <View className="gap-y-2">
          <View className="flex-row items-center gap-x-2">
            <View
              className={`w-6 h-6 rounded-full items-center justify-center ${
                (!isRenter || proximity.pinVerified) ? "bg-green-100" : "bg-gray-200"
              }`}
            >
              <Text
                className={`text-xs font-bold ${
                  (!isRenter || proximity.pinVerified)
                    ? "text-green-700"
                    : "text-gray-500"
                }`}
              >
                2
              </Text>
            </View>
            <Text className="text-sm font-semibold text-gray-700">
              {isRenter ? "Enter check-in PIN" : "Share PIN with renter"}
            </Text>
          </View>
          {isRenter ? (
            <PINEntry
              verified={proximity.pinVerified}
              verifying={proximity.pinVerifying}
              error={proximity.pinError}
              onSubmit={proximity.verifyPIN}
            />
          ) : (
            <PINDisplay transactionId={transactionId ?? ""} />
          )}
        </View>

        {/* Step 3: Photos */}
        <View className="gap-y-2">
          <View className="flex-row items-center gap-x-2">
            <View
              className={`w-6 h-6 rounded-full items-center justify-center ${
                proximity.photos.length >= MIN_PHOTOS
                  ? "bg-green-100"
                  : "bg-gray-200"
              }`}
            >
              <Text
                className={`text-xs font-bold ${
                  proximity.photos.length >= MIN_PHOTOS
                    ? "text-green-700"
                    : "text-gray-500"
                }`}
              >
                3
              </Text>
            </View>
            <Text className="text-sm font-semibold text-gray-700">
              Photograph the item
            </Text>
          </View>

          <PhotoGrid
            photos={proximity.photos}
            minPhotos={MIN_PHOTOS}
            onRemove={proximity.removePhoto}
          />

          <Pressable
            testID="btn-open-camera"
            className="border border-gray-200 rounded-2xl py-3 items-center flex-row justify-center gap-x-2"
            onPress={() => setShowCamera(true)}
          >
            <Ionicons name="camera-outline" size={18} color="#374151" />
            <Text className="text-gray-700 font-semibold text-sm">
              {proximity.photos.length === 0
                ? "Open camera"
                : "Take more photos"}
            </Text>
          </Pressable>
        </View>
      </ScrollView>

      {/* Footer CTA */}
      <View className="px-4 py-4 border-t border-gray-100">
        <Pressable
          testID="btn-complete-checkin"
          onPress={handleComplete}
          disabled={!proximity.canComplete || completing}
          className={`rounded-2xl py-4 items-center flex-row justify-center gap-x-2 ${
            proximity.canComplete && !completing
              ? "bg-green-600"
              : "bg-gray-200"
          }`}
        >
          {completing ? (
            <ActivityIndicator size="small" color="white" />
          ) : (
            <Ionicons
              name="checkmark-circle-outline"
              size={20}
              color={proximity.canComplete ? "white" : "#9ca3af"}
            />
          )}
          <Text
            className={`font-semibold text-base ${
              proximity.canComplete && !completing
                ? "text-white"
                : "text-gray-400"
            }`}
          >
            {completing ? "Completing check-in…" : "Complete check-in"}
          </Text>
        </Pressable>

        {!proximity.canComplete && (
          <Text className="text-xs text-center text-gray-400 mt-2">
            {isRenter
              ? `Complete GPS + PIN + ${MIN_PHOTOS} photos to continue`
              : `Complete GPS + ${MIN_PHOTOS} photos to continue`}
          </Text>
        )}
      </View>
    </View>
  );
}
