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
import { File, Paths } from "expo-file-system";

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

// Minimal valid 4x4 white JPEG — used by the __DEV__ camera bypass to create a
// real image file that travels through the normal media-upload pipeline.
const TINY_JPEG_BASE64 =
  "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkI" +
  "CQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/wAALCAAEAAQBAREA/8QAFAABAAAAAAAA" +
  "AAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AVN//2Q==";

/**
 * Camera component with gyroscope-based angle enforcement.
 *
 * - Shows a circular orientation indicator (green = new angle, orange = too close to existing photo).
 * - Soft-blocks shutter when current angle is <30° from any existing capture (warns but still allows).
 * - Stores roll/pitch/yaw orientation metadata with each captured photo.
 */
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

  // __DEV__ camera bypass — shown in simulator (no camera device) so Maestro can
  // drive the create-listing flow without hardware camera access. The generated
  // test photo is a real JPEG file that goes through the normal upload pipeline.
  if (__DEV__ && !device) {
    const addTestPhoto = () => {
      const file = new File(Paths.cache, `test-photo-${captures.length}.jpg`);
      // Decode base64 → Uint8Array to avoid the native base64 write path
      // which throws UnableToWriteBase64DataException in the iOS simulator.
      const raw = globalThis.atob(TINY_JPEG_BASE64);
      const bytes = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
      file.write(bytes);
      onCapture({
        path: file.uri,
        orientation: { roll: 0, pitch: 0, yaw: captures.length * 45 },
      });
    };

    return (
      <View
        testID="camera-dev-bypass"
        className="flex-1 bg-black items-center justify-center px-6"
      >
        <Text className="text-white text-lg font-semibold mb-2">
          Dev Camera Bypass
        </Text>
        <Text className="text-gray-400 text-sm mb-8 text-center">
          No camera in simulator — tap below to add a test photo
        </Text>

        <Text className="text-white text-sm mb-4">
          {captures.length}/{maxPhotos} photos
        </Text>

        {!isFull && (
          <Pressable
            testID="btn-use-test-photo"
            onPress={addTestPhoto}
            className="bg-sky-600 px-8 py-4 rounded-xl mb-6"
          >
            <Text className="text-white font-semibold text-base">
              Use test photo
            </Text>
          </Pressable>
        )}

        {onDone && captures.length > 0 && (
          <Pressable
            testID="btn-continue-photos"
            onPress={onDone}
            className="bg-green-600 px-8 py-4 rounded-xl"
          >
            <Text className="text-white font-semibold text-base">
              Continue — {captures.length} photo{captures.length !== 1 ? "s" : ""}
            </Text>
          </Pressable>
        )}
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
