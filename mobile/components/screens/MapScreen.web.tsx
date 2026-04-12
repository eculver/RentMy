import { View, Text } from "react-native";

// react-native-maps is not supported on web. This stub renders a placeholder
// instead of the native MapView, preventing Metro from trying to bundle
// native-only imports when building for the web platform.
export default function MapScreen() {
  return (
    <View className="flex-1 items-center justify-center bg-white px-8">
      <Text className="text-lg font-semibold text-gray-800 text-center">
        Map view is only available on mobile devices
      </Text>
      <Text className="text-sm text-gray-500 text-center mt-2">
        Please open the app on iOS or Android to use the map.
      </Text>
    </View>
  );
}
