import { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ActivityIndicator,
  SafeAreaView,
  Image,
} from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useStripeIdentity } from "@stripe/stripe-identity-react-native";
import {
  useStartVerification,
  useVerificationStatus,
} from "../../lib/hooks/useVerification";
import { useAuthStore } from "../../lib/auth";

type ScreenState =
  | "idle"
  | "starting"
  | "processing"
  | "verified"
  | "rejected"
  | "error";

// Placeholder brand logo — replace with actual asset when available.
// eslint-disable-next-line @typescript-eslint/no-require-imports
const BRAND_LOGO = Image.resolveAssetSource(require("../../assets/images/icon.png"));

export default function VerifyScreen() {
  const router = useRouter();
  const setIdentityStatus = useAuthStore((s) => s.setIdentityStatus);

  const [screenState, setScreenState] = useState<ScreenState>("idle");
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [ephemeralKeySecret, setEphemeralKeySecret] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string>("");

  const startMutation = useStartVerification();

  // Poll verification status only while the sheet has been submitted.
  const { data: statusData } = useVerificationStatus(screenState === "processing");

  const fetchOptions = useCallback(async () => {
    return {
      sessionId: sessionId ?? "",
      ephemeralKeySecret: ephemeralKeySecret ?? "",
      brandLogo: BRAND_LOGO,
    };
  }, [sessionId, ephemeralKeySecret]);

  const {
    status: stripeStatus,
    present,
    loading: stripeLoading,
  } = useStripeIdentity(fetchOptions);

  // React to Stripe Identity sheet dismissal.
  useEffect(() => {
    if (!stripeStatus) return;

    if (stripeStatus === "FlowCompleted") {
      setScreenState("processing");
    } else if (stripeStatus === "FlowCanceled") {
      setScreenState("idle");
    } else if (stripeStatus === "FlowFailed") {
      setScreenState("error");
      setErrorMessage("Verification failed. Please try again.");
    }
  }, [stripeStatus]);

  // React to backend status updates while polling.
  useEffect(() => {
    if (!statusData) return;

    if (statusData.status === "VERIFIED") {
      setIdentityStatus("VERIFIED");
      setScreenState("verified");
    } else if (statusData.status === "REJECTED") {
      setIdentityStatus("REJECTED");
      setScreenState("rejected");
    } else if (statusData.status === "ESCALATED") {
      setScreenState("error");
      setErrorMessage(
        "Your identity verification is under review. We'll notify you once complete.",
      );
    }
  }, [statusData, setIdentityStatus]);

  const handleStart = async () => {
    setScreenState("starting");
    try {
      const result = await startMutation.mutateAsync(undefined);
      if (!result.ephemeralKeySecret) {
        // Idempotent return — session already started, just poll status
        setScreenState("processing");
        return;
      }
      setSessionId(result.sessionId);
      setEphemeralKeySecret(result.ephemeralKeySecret);
      // present() is triggered in the effect below once session state settles
    } catch {
      setScreenState("error");
      setErrorMessage("Could not start verification. Please try again.");
    }
  };

  // Launch Stripe sheet once session credentials are ready.
  useEffect(() => {
    if (sessionId && ephemeralKeySecret && screenState === "starting" && !stripeLoading) {
      void present();
    }
  }, [sessionId, ephemeralKeySecret, screenState, stripeLoading, present]);

  const handleRetry = () => {
    setSessionId(null);
    setEphemeralKeySecret(null);
    setErrorMessage("");
    setScreenState("idle");
  };

  if (screenState === "verified") {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-1 items-center justify-center px-8">
          <Ionicons name="checkmark-circle" size={72} color="#22c55e" />
          <Text className="text-2xl font-bold text-gray-900 text-center mt-4">
            Identity Verified
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-2">
            You're all set to rent on RentMy.
          </Text>
          <Pressable
            className="mt-8 px-8 py-4 bg-sky-600 rounded-2xl"
            onPress={() => router.back()}
          >
            <Text className="text-white font-semibold text-base">Continue</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  if (screenState === "rejected") {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <Ionicons name="chevron-back" size={24} color="#111827" />
          </Pressable>
          <Text className="text-lg font-semibold text-gray-900 ml-2">
            Verify Identity
          </Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Ionicons name="close-circle" size={72} color="#ef4444" />
          <Text className="text-2xl font-bold text-gray-900 text-center mt-4">
            Verification Failed
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-2">
            We could not verify your identity. Please ensure your document is
            valid and the photos are clear.
          </Text>
          <Pressable
            className="mt-8 px-8 py-4 bg-sky-600 rounded-2xl"
            onPress={handleRetry}
          >
            <Text className="text-white font-semibold text-base">
              Try Again
            </Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  if (screenState === "error") {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <Ionicons name="chevron-back" size={24} color="#111827" />
          </Pressable>
          <Text className="text-lg font-semibold text-gray-900 ml-2">
            Verify Identity
          </Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Ionicons name="warning" size={72} color="#f59e0b" />
          <Text className="text-xl font-bold text-gray-900 text-center mt-4">
            Something went wrong
          </Text>
          <Text className="text-sm text-gray-500 text-center mt-2">
            {errorMessage}
          </Text>
          <Pressable
            className="mt-8 px-8 py-4 bg-sky-600 rounded-2xl"
            onPress={handleRetry}
          >
            <Text className="text-white font-semibold text-base">
              Try Again
            </Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  const isLoading = screenState === "starting" || screenState === "processing";

  return (
    <SafeAreaView className="flex-1 bg-white">
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Verify Identity
        </Text>
      </View>

      <View className="flex-1 items-center justify-center px-8">
        <Ionicons name="shield-checkmark-outline" size={72} color="#0284c7" />
        <Text className="text-2xl font-bold text-gray-900 text-center mt-4">
          Identity Verification
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-3 leading-6">
          RentMy requires all renters to verify their identity before booking.
          This helps keep the community safe and builds trust between members.
        </Text>

        <View className="mt-8 w-full gap-4">
          <View className="flex-row items-center gap-3">
            <Ionicons name="document-text-outline" size={20} color="#0284c7" />
            <Text className="text-sm text-gray-700 flex-1">
              Government-issued ID (passport, driver's license)
            </Text>
          </View>
          <View className="flex-row items-center gap-3">
            <Ionicons name="camera-outline" size={20} color="#0284c7" />
            <Text className="text-sm text-gray-700 flex-1">
              Selfie photo to match your ID
            </Text>
          </View>
          <View className="flex-row items-center gap-3">
            <Ionicons name="lock-closed-outline" size={20} color="#0284c7" />
            <Text className="text-sm text-gray-700 flex-1">
              Secured by Stripe Identity — your data is encrypted
            </Text>
          </View>
        </View>

        <Pressable
          className={`mt-10 w-full py-4 rounded-2xl items-center ${
            isLoading ? "bg-sky-300" : "bg-sky-600"
          }`}
          onPress={handleStart}
          disabled={isLoading}
        >
          {isLoading ? (
            <View className="flex-row items-center gap-2">
              <ActivityIndicator color="white" size="small" />
              <Text className="text-white font-semibold text-base">
                {screenState === "processing" ? "Verifying…" : "Starting…"}
              </Text>
            </View>
          ) : (
            <Text className="text-white font-semibold text-base">
              Start Verification
            </Text>
          )}
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
