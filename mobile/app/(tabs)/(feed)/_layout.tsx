import { Stack } from "expo-router";

export default function FeedLayout() {
  return (
    <Stack
      screenOptions={{
        headerShown: false,
      }}
    >
      <Stack.Screen
        name="listing/[id]"
        options={{
          headerShown: true,
          headerTitle: "Listing",
          headerBackTitle: "Back",
        }}
      />
    </Stack>
  );
}
