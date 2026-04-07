import { View, Text } from "react-native";
import { Referral } from "../../lib/hooks/useReferrals";

interface ReferralCardProps {
  referral: Referral;
}

const statusLabel: Record<Referral["status"], string> = {
  SIGNED_UP: "Signed Up",
  FIRST_RENTAL_COMPLETED: "Rental Completed",
  PAID: "Paid",
  FRAUDULENT: "Flagged",
};

const statusColor: Record<Referral["status"], string> = {
  SIGNED_UP: "bg-blue-100 text-blue-700",
  FIRST_RENTAL_COMPLETED: "bg-yellow-100 text-yellow-700",
  PAID: "bg-green-100 text-green-700",
  FRAUDULENT: "bg-red-100 text-red-700",
};

export default function ReferralCard({ referral }: ReferralCardProps) {
  const payout = referral.status === "PAID"
    ? `$${(referral.referrerPayout / 100).toFixed(2)}`
    : null;

  return (
    <View className="flex-row items-center py-3 border-b border-gray-100">
      {/* Avatar placeholder */}
      <View className="w-10 h-10 rounded-full bg-sky-100 items-center justify-center mr-3">
        <Text className="text-sky-700 font-semibold text-sm">R</Text>
      </View>

      <View className="flex-1">
        <Text className="text-sm font-medium text-gray-900">
          Referee {referral.refereeId.slice(-6).toUpperCase()}
        </Text>
        <Text className="text-xs text-gray-400 mt-0.5">
          {new Date(referral.createdAt).toLocaleDateString()}
        </Text>
      </View>

      <View className="items-end gap-y-1">
        <View className={`px-2 py-0.5 rounded-full ${statusColor[referral.status].split(" ")[0]}`}>
          <Text className={`text-xs font-medium ${statusColor[referral.status].split(" ")[1]}`}>
            {statusLabel[referral.status]}
          </Text>
        </View>
        {payout && (
          <Text className="text-xs text-green-600 font-semibold">{payout}</Text>
        )}
      </View>
    </View>
  );
}
