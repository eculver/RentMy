import { Stack } from "expo-router";

export default function ProfileLayout() {
  return (
    <Stack>
      <Stack.Screen name="index" options={{ headerShown: false }} />
      <Stack.Screen name="create-listing" options={{ title: "Create Listing" }} />
      <Stack.Screen name="verify" options={{ headerShown: false }} />
    </Stack>
  );
}
