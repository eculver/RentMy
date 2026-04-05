/**
 * PhotoGrid — displays a grid of captured handoff photos with angle-diversity
 * badges. Used in check-in and check-out screens to confirm enough photos have
 * been taken from distinct angles.
 */
import { View, Text, Image, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import type { CapturedPhoto } from "../camera/AngleEnforcedCamera";

interface PhotoGridProps {
  photos: CapturedPhoto[];
  minPhotos: number;
  onRemove?: (index: number) => void;
}

export default function PhotoGrid({
  photos,
  minPhotos,
  onRemove,
}: PhotoGridProps) {
  const remaining = Math.max(0, minPhotos - photos.length);

  return (
    <View className="gap-y-3">
      {/* Status header */}
      <View className="flex-row items-center justify-between">
        <Text className="text-sm font-semibold text-gray-700">
          Photos ({photos.length}/{minPhotos} minimum)
        </Text>
        {photos.length >= minPhotos ? (
          <View className="flex-row items-center gap-x-1">
            <Ionicons name="checkmark-circle" size={16} color="#16a34a" />
            <Text className="text-xs font-medium text-green-700">
              Requirement met
            </Text>
          </View>
        ) : (
          <Text className="text-xs text-amber-600">
            {remaining} more needed
          </Text>
        )}
      </View>

      {/* Photo grid — 3 columns */}
      {photos.length > 0 && (
        <View className="flex-row flex-wrap gap-2">
          {photos.map((photo, index) => (
            <View
              key={index}
              className="relative"
              style={{ width: "30%" }}
            >
              <Image
                source={{ uri: `file://${photo.path}` }}
                className="w-full rounded-xl bg-gray-200"
                style={{ aspectRatio: 1 }}
                resizeMode="cover"
              />
              {/* Angle badge */}
              <View className="absolute bottom-1 left-1 bg-black/60 rounded-full px-1.5 py-0.5">
                <Text className="text-white text-[10px] font-medium">
                  #{index + 1}
                </Text>
              </View>
              {/* Remove button */}
              {onRemove && (
                <Pressable
                  className="absolute top-1 right-1 bg-black/60 rounded-full w-5 h-5 items-center justify-center"
                  onPress={() => onRemove(index)}
                  hitSlop={4}
                >
                  <Ionicons name="close" size={12} color="white" />
                </Pressable>
              )}
            </View>
          ))}
        </View>
      )}

      {/* Empty state placeholder slots */}
      {photos.length === 0 && (
        <View className="flex-row flex-wrap gap-2">
          {Array.from({ length: minPhotos }).map((_, i) => (
            <View
              key={i}
              className="rounded-xl bg-gray-100 border-2 border-dashed border-gray-300 items-center justify-center"
              style={{ width: "30%", aspectRatio: 1 }}
            >
              <Ionicons name="camera-outline" size={20} color="#9ca3af" />
            </View>
          ))}
        </View>
      )}
    </View>
  );
}
