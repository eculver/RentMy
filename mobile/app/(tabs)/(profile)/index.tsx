import { View, Text, Pressable } from "react-native";
import { router } from "expo-router";
import { useAuthStore } from "../../../lib/auth";

export default function ProfileScreen() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  return (
    <View className="flex-1 bg-white px-6 pt-12">
      <Text className="text-2xl font-bold">{user?.name || "Profile"}</Text>
      <Text className="text-gray-400 mt-1">{user?.email}</Text>

      <Pressable
        className="mt-8 w-full bg-sky-600 py-3 rounded-xl items-center"
        onPress={() => router.push("/(tabs)/(profile)/create-listing")}
      >
        <Text className="text-white font-semibold">+ Create Listing</Text>
      </Pressable>

      <Pressable
        className="mt-4 w-full border border-red-500 py-3 rounded-xl items-center"
        onPress={logout}
      >
        <Text className="text-red-500 font-medium">Sign Out</Text>
      </Pressable>
    </View>
  );
}
