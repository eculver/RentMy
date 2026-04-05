import { ReactNode } from "react";
import { View, Text, Pressable, SafeAreaView } from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../lib/auth";

interface KYCGateProps {
  children: ReactNode;
}

/**
 * KYCGate wraps a screen or flow that requires identity verification.
 *
 * - VERIFIED: renders children normally.
 * - PENDING / not started: renders a prompt to verify identity.
 * - REJECTED: renders a rejection message with a retry button.
 * - ESCALATED: renders a "under review" message.
 */
export default function KYCGate({ children }: KYCGateProps) {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const identityStatus = user?.identityStatus;

  if (identityStatus === "VERIFIED") {
    return <>{children}</>;
  }

  if (identityStatus === "REJECTED") {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-1 items-center justify-center px-8">
          <Ionicons name="close-circle-outline" size={56} color="#ef4444" />
          <Text className="text-xl font-bold text-gray-900 text-center mt-4">
            Verification Failed
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-2">
            Your identity verification was not successful. Please retry with a
            valid government-issued ID.
          </Text>
          <Pressable
            className="mt-6 px-6 py-3 bg-sky-600 rounded-2xl"
            onPress={() => router.push("/(tabs)/(profile)/verify" as never)}
          >
            <Text className="text-white font-semibold">Retry Verification</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  if (identityStatus === "ESCALATED") {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-1 items-center justify-center px-8">
          <Ionicons name="hourglass-outline" size={56} color="#f59e0b" />
          <Text className="text-xl font-bold text-gray-900 text-center mt-4">
            Verification Under Review
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-2">
            Your identity is being reviewed by our team. You'll be able to
            book once approved — usually within 24 hours.
          </Text>
        </View>
      </SafeAreaView>
    );
  }

  // PENDING or no status — prompt the user to verify.
  return (
    <SafeAreaView className="flex-1 bg-white">
      <View className="flex-1 items-center justify-center px-8">
        <Ionicons name="shield-outline" size={56} color="#0284c7" />
        <Text className="text-xl font-bold text-gray-900 text-center mt-4">
          Identity Verification Required
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-2">
          Please verify your identity to rent items on RentMy.
        </Text>
        <Pressable
          className="mt-6 px-6 py-3 bg-sky-600 rounded-2xl"
          onPress={() => router.push("/(tabs)/(profile)/verify" as never)}
        >
          <Text className="text-white font-semibold">Verify Identity</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
