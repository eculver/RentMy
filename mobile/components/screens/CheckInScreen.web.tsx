import { View, Text, SafeAreaView } from "react-native";

// This screen uses react-native-vision-camera which is not supported on web.
export default function CheckInScreen() {
  return (
    <SafeAreaView className="flex-1 bg-white">
      <View className="flex-1 items-center justify-center px-8">
        <Text className="text-lg font-semibold text-gray-800 text-center">
          Check-in is only available on mobile devices
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-2">
          Please open the app on iOS or Android to check in.
        </Text>
      </View>
    </SafeAreaView>
  );
}
