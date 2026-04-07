import { View, Text } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface PaymentMethodSelectorProps {
  onPaymentMethodSelected: (paymentMethodId: string) => void;
  selectedPaymentMethodId: string | null;
}

// @stripe/stripe-react-native is not supported on web. This stub prevents
// Metro from trying to bundle native-only imports for the web platform.
export default function PaymentMethodSelector(
  _props: PaymentMethodSelectorProps,
) {
  return (
    <View className="gap-y-3">
      <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
        Payment method
      </Text>
      <View className="flex-row items-center gap-x-3 border border-gray-200 rounded-xl px-4 py-4">
        <Ionicons name="card-outline" size={20} color="#9ca3af" />
        <Text className="text-sm text-gray-500">
          Payment is only available on mobile devices
        </Text>
      </View>
    </View>
  );
}
