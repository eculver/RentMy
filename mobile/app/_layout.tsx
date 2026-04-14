import "../global.css";
import { useEffect } from "react";
import { Stack } from "expo-router";
import { QueryClientProvider } from "@tanstack/react-query";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import StripeProviderWrapper from "../components/providers/StripeProviderWrapper";
import { queryClient } from "../lib/query";
import { useAuthStore } from "../lib/auth";

export default function RootLayout() {
  const isLoading = useAuthStore((s) => s.isLoading);
  const loadToken = useAuthStore((s) => s.loadToken);

  useEffect(() => {
    loadToken();
  }, [loadToken]);

  if (isLoading) {
    return null;
  }

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <StripeProviderWrapper>
        <QueryClientProvider client={queryClient}>
          <Stack screenOptions={{ headerShown: false }}>
            <Stack.Screen name="(tabs)" />
            <Stack.Screen name="(auth)" />
          </Stack>
        </QueryClientProvider>
      </StripeProviderWrapper>
    </GestureHandlerRootView>
  );
}
