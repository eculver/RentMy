import { View, Text, Pressable, SafeAreaView } from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

// @stripe/stripe-identity-react-native is not supported on web. This stub
// prevents Metro from trying to bundle native-only imports for the web platform.
export default function VerifyScreen() {
  const router = useRouter();

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
        <Ionicons name="shield-checkmark-outline" size={72} color="#9ca3af" />
        <Text className="text-lg font-semibold text-gray-800 text-center mt-4">
          Identity verification is only available on mobile devices
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-2">
          Please open the app on iOS or Android to verify your identity.
        </Text>
      </View>
    </SafeAreaView>
  );
}
