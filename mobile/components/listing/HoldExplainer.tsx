import { View, Text } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface HoldExplainerProps {
  holdAmount: number;   // cents
  itemValue: number;    // cents
  guaranteeGap: number; // cents — portion covered by RentMy Protection
}

function formatDollars(cents: number): string {
  return `$${(cents / 100).toFixed(0)}`;
}

export default function HoldExplainer({
  holdAmount,
  itemValue,
  guaranteeGap,
}: HoldExplainerProps) {
  return (
    <View className="bg-amber-50 border border-amber-100 rounded-2xl p-4 gap-y-2">
      <View className="flex-row items-center gap-x-2">
        <Ionicons name="lock-closed-outline" size={16} color="#d97706" />
        <Text className="text-sm font-semibold text-amber-800">
          Temporary hold: {formatDollars(holdAmount)}
        </Text>
      </View>

      <Text className="text-xs text-amber-700 leading-relaxed">
        Based on item value of {formatDollars(itemValue)}, a temporary
        authorization of {formatDollars(holdAmount)} will be placed on your
        card. This is not a charge — it is released when the item is returned
        in good condition.
      </Text>

      {guaranteeGap > 0 && (
        <View className="flex-row items-start gap-x-2 mt-1">
          <Ionicons name="shield-checkmark-outline" size={14} color="#0284c7" />
          <Text className="text-xs text-sky-700 flex-1 leading-relaxed">
            The remaining {formatDollars(guaranteeGap)} is covered by{" "}
            <Text className="font-semibold">RentMy Protection</Text>.
          </Text>
        </View>
      )}
    </View>
  );
}
