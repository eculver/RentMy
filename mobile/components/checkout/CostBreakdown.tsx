import { useState } from "react";
import { View, Text, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface CostBreakdownProps {
  rentalFee: number; // cents
  holdAmount: number; // cents
  totalImpact: number; // cents
}

function dollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function CostBreakdown({
  rentalFee,
  holdAmount,
  totalImpact,
}: CostBreakdownProps) {
  const [holdExpanded, setHoldExpanded] = useState(false);

  return (
    <View className="gap-y-3">
      <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
        Cost breakdown
      </Text>

      <View className="border border-gray-100 rounded-xl divide-y divide-gray-100">
        {/* Rental fee row */}
        <View className="flex-row items-center justify-between px-4 py-3">
          <Text className="text-sm text-gray-700">Rental fee</Text>
          <Text className="text-sm font-medium text-gray-900">
            {dollars(rentalFee)}
          </Text>
        </View>

        {/* Hold row with expandable info */}
        <View>
          <Pressable
            onPress={() => setHoldExpanded((v) => !v)}
            className="flex-row items-center justify-between px-4 py-3"
          >
            <View className="flex-row items-center gap-x-1">
              <Text className="text-sm text-gray-700">
                Temporary hold
              </Text>
              <Ionicons
                name={holdExpanded ? "chevron-up" : "information-circle-outline"}
                size={14}
                color="#9ca3af"
              />
            </View>
            <Text className="text-sm font-medium text-gray-900">
              {dollars(holdAmount)}
            </Text>
          </Pressable>
          {holdExpanded && (
            <View className="px-4 pb-3">
              <Text className="text-xs text-gray-500 leading-relaxed">
                This is a temporary authorization on your card, not a charge. It
                is released automatically when the item is returned in good
                condition. RentMy uses a tiered hold based on the item's value.
              </Text>
            </View>
          )}
        </View>

        {/* Total row */}
        <View className="flex-row items-center justify-between px-4 py-3 bg-gray-50 rounded-b-xl">
          <View>
            <Text className="text-sm font-semibold text-gray-900">
              Total card impact
            </Text>
            <Text className="text-xs text-gray-500">Rental fee + hold</Text>
          </View>
          <Text className="text-lg font-bold text-gray-900">
            {dollars(totalImpact)}
          </Text>
        </View>
      </View>
    </View>
  );
}
