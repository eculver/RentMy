import { View, Text, Image } from "react-native";
import { Ionicons } from "@expo/vector-icons";

export type DamageClassification =
  | "NO_DAMAGE"
  | "MINOR_DAMAGE"
  | "MAJOR_DAMAGE"
  | "INCONCLUSIVE";

interface PhotoPair {
  checkInUrl: string;
  checkOutUrl: string;
  classification: DamageClassification;
  confidence: number;
}

interface PhotoDiffResultProps {
  pairs: PhotoPair[];
  /** Overall classification across all pairs. */
  overallClassification: DamageClassification;
  overallConfidence: number;
}

const CLASSIFICATION_CONFIG: Record<
  DamageClassification,
  { label: string; color: string; bgColor: string; icon: string }
> = {
  NO_DAMAGE: {
    label: "No damage",
    color: "#16a34a",
    bgColor: "#f0fdf4",
    icon: "checkmark-circle-outline",
  },
  MINOR_DAMAGE: {
    label: "Minor damage",
    color: "#d97706",
    bgColor: "#fffbeb",
    icon: "warning-outline",
  },
  MAJOR_DAMAGE: {
    label: "Major damage",
    color: "#dc2626",
    bgColor: "#fef2f2",
    icon: "alert-circle-outline",
  },
  INCONCLUSIVE: {
    label: "Inconclusive",
    color: "#6b7280",
    bgColor: "#f9fafb",
    icon: "help-circle-outline",
  },
};

function ClassificationBadge({
  classification,
  confidence,
}: {
  classification: DamageClassification;
  confidence: number;
}) {
  const cfg = CLASSIFICATION_CONFIG[classification];
  return (
    <View
      className="flex-row items-center gap-x-1.5 px-3 py-1.5 rounded-full self-start"
      style={{ backgroundColor: cfg.bgColor }}
    >
      <Ionicons
        name={cfg.icon as React.ComponentProps<typeof Ionicons>["name"]}
        size={14}
        color={cfg.color}
      />
      <Text className="text-xs font-semibold" style={{ color: cfg.color }}>
        {cfg.label}
      </Text>
      <Text className="text-xs" style={{ color: cfg.color }}>
        {Math.round(confidence * 100)}%
      </Text>
    </View>
  );
}

/**
 * Displays side-by-side check-in / check-out photo pairs with damage
 * classification badges and a confidence indicator.
 */
export default function PhotoDiffResult({
  pairs,
  overallClassification,
  overallConfidence,
}: PhotoDiffResultProps) {
  return (
    <View className="bg-gray-50 rounded-2xl overflow-hidden">
      <View className="px-4 py-3 border-b border-gray-200 flex-row justify-between items-center">
        <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
          Photo comparison
        </Text>
        <ClassificationBadge
          classification={overallClassification}
          confidence={overallConfidence}
        />
      </View>

      {pairs.length === 0 ? (
        <View className="px-4 py-6 items-center">
          <Text className="text-sm text-gray-400">
            Photo comparison pending…
          </Text>
        </View>
      ) : (
        <View className="px-4 py-3 gap-y-4">
          {pairs.map((pair, i) => (
            <View key={i}>
              {/* Label row */}
              <View className="flex-row justify-between mb-2">
                <Text className="text-xs text-gray-500 w-1/2 text-center">
                  Check-in
                </Text>
                <Text className="text-xs text-gray-500 w-1/2 text-center">
                  Check-out
                </Text>
              </View>

              {/* Photo pair */}
              <View className="flex-row gap-x-2">
                <Image
                  source={{ uri: pair.checkInUrl }}
                  className="flex-1 h-28 rounded-xl bg-gray-200"
                  resizeMode="cover"
                />
                <Image
                  source={{ uri: pair.checkOutUrl }}
                  className="flex-1 h-28 rounded-xl bg-gray-200"
                  resizeMode="cover"
                />
              </View>

              {/* Per-pair badge */}
              <View className="mt-2 items-center">
                <ClassificationBadge
                  classification={pair.classification}
                  confidence={pair.confidence}
                />
              </View>
            </View>
          ))}
        </View>
      )}
    </View>
  );
}
