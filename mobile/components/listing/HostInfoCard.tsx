import { View, Text } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface HostInfoCardProps {
  hostName: string;
  reputationScore: number; // 0–1000 as per PRD §8
  memberSince?: string;    // ISO date string
}

function starRating(score: number): string {
  return (score / 200).toFixed(1);
}

function formatMemberSince(iso?: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  return date.toLocaleDateString("en-US", { month: "long", year: "numeric" });
}

export default function HostInfoCard({
  hostName,
  reputationScore,
  memberSince,
}: HostInfoCardProps) {
  const rating = starRating(reputationScore);
  const since = formatMemberSince(memberSince);
  const isVerified = reputationScore >= 500;

  return (
    <View className="bg-gray-50 rounded-2xl p-4">
      <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">
        Your Host
      </Text>

      <View className="flex-row items-center gap-x-3">
        {/* Avatar placeholder */}
        <View className="w-12 h-12 rounded-full bg-sky-100 items-center justify-center">
          <Text className="text-lg font-bold text-sky-600">
            {hostName.charAt(0).toUpperCase()}
          </Text>
        </View>

        <View className="flex-1">
          <View className="flex-row items-center gap-x-2">
            <Text className="text-base font-semibold text-gray-900">
              {hostName}
            </Text>
            {isVerified && (
              <Ionicons name="shield-checkmark" size={16} color="#0284c7" />
            )}
          </View>

          <View className="flex-row items-center gap-x-3 mt-1">
            <View className="flex-row items-center gap-x-1">
              <Ionicons name="star" size={13} color="#f59e0b" />
              <Text className="text-sm text-gray-600">{rating}</Text>
            </View>
            {since ? (
              <Text className="text-sm text-gray-400">Member since {since}</Text>
            ) : null}
          </View>
        </View>
      </View>
    </View>
  );
}
