import { View, Text } from "react-native";
import { Ionicons } from "@expo/vector-icons";

export interface CapturedPhoto {
  path: string;
  orientation: { roll: number; pitch: number; yaw: number };
}

interface AngleEnforcedCameraProps {
  captures: CapturedPhoto[];
  onCapture: (photo: CapturedPhoto) => void;
  onDone?: () => void;
  maxPhotos?: number;
}

// react-native-vision-camera is not supported on web. This stub prevents
// Metro from trying to bundle native-only imports for the web platform.
export default function AngleEnforcedCamera(
  _props: AngleEnforcedCameraProps,
) {
  return (
    <View className="flex-1 items-center justify-center bg-black px-6">
      <Ionicons name="camera-outline" size={64} color="#6b7280" />
      <Text className="text-white text-center mt-4 text-base">
        Camera is only available on mobile devices
      </Text>
    </View>
  );
}
