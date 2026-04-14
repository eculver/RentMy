import { useState, useEffect } from "react";
import { View, Text, Pressable, ActivityIndicator, Alert } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useStripe } from "@stripe/stripe-react-native";
import { api } from "../../lib/api";

interface SetupResponse {
  customerId: string;
  clientSecret: string;
}

interface PaymentMethodSelectorProps {
  onPaymentMethodSelected: (paymentMethodId: string) => void;
  selectedPaymentMethodId: string | null;
}

export default function PaymentMethodSelector({
  onPaymentMethodSelected,
  selectedPaymentMethodId,
}: PaymentMethodSelectorProps) {
  const { initPaymentSheet, presentPaymentSheet } = useStripe();
  const [isLoading, setIsLoading] = useState(false);

  // __DEV__ bypass: auto-select a stub payment method on the simulator.
  // Maestro cannot interact with the native Stripe payment sheet, so in
  // development builds we skip it and pass a placeholder payment method ID.
  // The backend's stub payment adapter accepts any payment method ID.
  useEffect(() => {
    if (__DEV__ && !selectedPaymentMethodId) {
      onPaymentMethodSelected("pm_stub_dev");
    }
  }, []);

  const openPaymentSheet = async () => {
    setIsLoading(true);
    try {
      // Fetch setup intent client secret from backend
      const setup = await api
        .post("api/v1/payments/setup")
        .json<SetupResponse>();

      const { error: initError } = await initPaymentSheet({
        merchantDisplayName: "RentMy",
        customerId: setup.customerId,
        customerEphemeralKeySecret: setup.clientSecret,
        setupIntentClientSecret: setup.clientSecret,
        allowsDelayedPaymentMethods: false,
      });

      if (initError) {
        Alert.alert("Payment setup failed", initError.message);
        return;
      }

      const { error: presentError, paymentOption } = await presentPaymentSheet();
      if (presentError) {
        if (presentError.code !== "Canceled") {
          Alert.alert("Payment setup failed", presentError.message);
        }
        return;
      }

      // Payment sheet does not return a paymentMethodId directly in setup mode;
      // the method is attached to the customer on the backend. Use a placeholder
      // that signals "method on file" — the backend resolves the saved method.
      const methodId = paymentOption?.label ?? "saved_method";
      onPaymentMethodSelected(methodId);
    } catch {
      Alert.alert("Error", "Could not set up payment method. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  if (selectedPaymentMethodId) {
    return (
      <View className="gap-y-3">
        <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
          Payment method
        </Text>
        <View testID="payment-method-selected" className="flex-row items-center justify-between border border-gray-200 rounded-xl px-4 py-3">
          <View className="flex-row items-center gap-x-3">
            <Ionicons name="card-outline" size={20} color="#374151" />
            <Text className="text-sm font-medium text-gray-800">
              {selectedPaymentMethodId === "saved_method"
                ? "Saved payment method"
                : selectedPaymentMethodId}
            </Text>
          </View>
          <Pressable onPress={openPaymentSheet}>
            <Text className="text-sm text-sky-600 font-medium">Change</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  return (
    <View className="gap-y-3">
      <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
        Payment method
      </Text>
      <Pressable
        onPress={openPaymentSheet}
        disabled={isLoading}
        className="flex-row items-center justify-between border border-dashed border-gray-300 rounded-xl px-4 py-4"
      >
        <View className="flex-row items-center gap-x-3">
          <Ionicons name="add-circle-outline" size={20} color="#0284c7" />
          <Text className="text-sm font-medium text-sky-600">
            Add payment method
          </Text>
        </View>
        {isLoading && <ActivityIndicator size="small" color="#0284c7" />}
      </Pressable>
    </View>
  );
}
