import { useRef, useState } from "react";
import {
  View,
  Text,
  Pressable,
  StyleSheet,
  ActivityIndicator,
} from "react-native";
import { Camera, useCameraDevice, useCameraPermission } from "react-native-vision-camera";
import { useGyroscope, angularDistance, type Orientation } from "../../lib/hooks/useGyroscope";

// Sentinel path used in E2E mode — signals that no real file upload should occur.
export const E2E_FIXTURE_PATH = "e2e://fixture-photo.jpg";

export interface CapturedPhoto {
  path: string;
  orientation: Orientation;
}

interface AngleEnforcedCameraProps {
  captures: CapturedPhoto[];
  onCapture: (photo: CapturedPhoto) => void;
  onDone?: () => void;
  maxPhotos?: number;
}

const MIN_ANGLE_DEG = 30;

/**
 * Camera component with gyroscope-based angle enforcement.
 *
 * - Shows a circular orientation indicator (green = new angle, orange = too close to existing photo).
 * - Soft-blocks shutter when current angle is <30° from any existing capture (warns but still allows).
 * - Stores roll/pitch/yaw orientation metadata with each captured photo.
 */
const IS_E2E = process.env.EXPO_PUBLIC_E2E_MODE === "true";

export default function AngleEnforcedCamera({
  captures,
  onCapture,
  onDone,
  maxPhotos = 6,
}: AngleEnforcedCameraProps) {
  const { hasPermission, requestPermission } = useCameraPermission();
  const device = useCameraDevice("back");
  const cameraRef = useRef<Camera>(null);
  const orientation = useGyroscope(100);
  const [isCapturing, setIsCapturing] = useState(false);

  // E2E bypass: skip native camera, provide fixture captures via button press.
  // "Use Fixture Photo" remains visible until maxPhotos is reached so tests
  // that require multiple photos (e.g. check-in MIN_PHOTOS=3) can add them all
  // in a single camera-open session before tapping Continue.
  if (IS_E2E) {
    const fixtureOrientation: Orientation = { roll: 0, pitch: 0, yaw: 0 };
    return (
      <View testID="camera-e2e-bypass" className="flex-1 bg-black items-center justify-center px-8">
        <Text className="text-white text-base font-medium mb-6 text-center">
          E2E Mode — Camera bypassed
        </Text>
        <Text className="text-white text-sm mb-6 text-center opacity-60">
          {captures.length}/{maxPhotos} photo{maxPhotos !== 1 ? "s" : ""}
        </Text>
        {captures.length < maxPhotos && (
          <Pressable
            testID="btn-e2e-use-fixture"
            onPress={() => onCapture({ path: E2E_FIXTURE_PATH, orientation: fixtureOrientation })}
            className="bg-sky-600 px-8 py-3 rounded-xl mb-4"
          >
            <Text className="text-white font-semibold text-base">Use Fixture Photo</Text>
          </Pressable>
        )}
        {captures.length > 0 && onDone && (
          <Pressable
            testID="btn-e2e-continue"
            onPress={onDone}
            className="bg-green-600 px-8 py-3 rounded-xl"
          >
            <Text className="text-white font-semibold text-base">
              Continue — {captures.length} photo{captures.length !== 1 ? "s" : ""}
            </Text>
          </Pressable>
        )}
      </View>
    );
  }

  const isFull = captures.length >= maxPhotos;

  // Check if the current angle is too close to any existing capture
  const tooClose =
    captures.length > 0 &&
    captures.some(
      (cap) => angularDistance(orientation, cap.orientation) < MIN_ANGLE_DEG
    );

  const handleCapture = async () => {
    if (!cameraRef.current || isCapturing || isFull) return;

    setIsCapturing(true);
    try {
      const photo = await cameraRef.current.takePhoto({ flash: "off" });
      onCapture({
        path: photo.path,
        orientation: { ...orientation },
      });
    } catch {
      // Silent — user can retry
    } finally {
      setIsCapturing(false);
    }
  };

  if (!hasPermission) {
    return (
      <View className="flex-1 items-center justify-center bg-black px-6">
        <Text className="text-white text-center mb-6 text-base">
          Camera access is required to photograph your item
        </Text>
        <Pressable
          onPress={requestPermission}
          className="bg-sky-600 px-8 py-3 rounded-xl"
        >
          <Text className="text-white font-semibold text-base">
            Grant Permission
          </Text>
        </Pressable>
      </View>
    );
  }

  if (!device) {
    return (
      <View className="flex-1 items-center justify-center bg-black">
        <Text className="text-white">No camera available on this device</Text>
      </View>
    );
  }

  const indicatorColor = captures.length === 0 || !tooClose ? "bg-green-500" : "bg-orange-500";
  const indicatorLabel =
    captures.length === 0
      ? "Take your first photo"
      : !tooClose
      ? "New angle — ready to capture"
      : "Rotate device ≥30° for variety";

  return (
    <View className="flex-1 bg-black">
      <Camera
        ref={cameraRef}
        style={StyleSheet.absoluteFill}
        device={device}
        isActive
        photo
      />

      {/* Angle indicator */}
      <View className="absolute top-4 left-0 right-0 items-center px-4">
        <View className={`px-4 py-2 rounded-full ${indicatorColor}`}>
          <Text className="text-white text-sm font-medium text-center">
            {indicatorLabel}
          </Text>
        </View>
      </View>

      {/* Photo count badge */}
      <View className="absolute top-4 right-4">
        <View className="bg-black/60 px-3 py-1 rounded-full">
          <Text className="text-white text-sm font-medium">
            {captures.length}/{maxPhotos}
          </Text>
        </View>
      </View>

      {/* Continue button — appears after first capture */}
      {onDone && captures.length > 0 && (
        <View className="absolute bottom-36 left-0 right-0 items-center">
          <Pressable
            onPress={onDone}
            className="bg-sky-600 px-8 py-3 rounded-xl"
          >
            <Text className="text-white font-semibold text-base">
              Continue — {captures.length} photo{captures.length !== 1 ? "s" : ""}
            </Text>
          </Pressable>
        </View>
      )}

      {/* Shutter button */}
      <View className="absolute bottom-10 left-0 right-0 items-center">
        <Pressable
          onPress={handleCapture}
          disabled={isCapturing || isFull}
          style={[
            styles.shutter,
            { borderColor: tooClose && captures.length > 0 ? "#f97316" : "white" },
            (isCapturing || isFull) && styles.shutterDisabled,
          ]}
        >
          {isCapturing ? (
            <ActivityIndicator color="gray" />
          ) : isFull ? (
            <Text className="text-gray-400 text-xs">Full</Text>
          ) : null}
        </Pressable>
        {tooClose && captures.length > 0 && (
          <Text className="text-orange-400 text-xs mt-2">
            Rotate for a different angle
          </Text>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  shutter: {
    width: 72,
    height: 72,
    borderRadius: 36,
    borderWidth: 4,
    backgroundColor: "white",
    alignItems: "center",
    justifyContent: "center",
  },
  shutterDisabled: {
    opacity: 0.5,
  },
});
