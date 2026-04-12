import { View, Text } from "react-native";

interface HoldAllocation {
  /** Total amount authorized on the card, in cents. */
  authorizedCents: number;
  /** Amount captured for late return fee, in cents. */
  capturedLateCents: number;
  /** Amount captured for damage, in cents. */
  capturedDamageCents: number;
  /** Amount reserved for the guarantee fund, in cents. */
  damageReserveCents: number;
  /** Amount released back to the renter, in cents. */
  releasedCents: number;
}

interface HoldStatusCardProps {
  allocation: HoldAllocation;
}

function formatDollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

interface BarSegmentProps {
  label: string;
  cents: number;
  total: number;
  color: string;
  textColor: string;
}

function BarSegment({ label, cents, total, color, textColor }: BarSegmentProps) {
  if (cents <= 0) return null;
  const pct = Math.min(100, (cents / total) * 100);

  return (
    <View className="mb-3">
      <View className="flex-row justify-between mb-1">
        <Text className="text-xs text-gray-600">{label}</Text>
        <Text className="text-xs font-medium text-gray-800">
          {formatDollars(cents)}
        </Text>
      </View>
      <View className="h-2 bg-gray-100 rounded-full overflow-hidden">
        <View
          className={`h-2 rounded-full ${color}`}
          style={{ width: `${pct}%` }}
        />
      </View>
      {/* suppress unused textColor — reserved for future label styling */}
      {textColor === "" && null}
    </View>
  );
}

/**
 * Visualizes the hold allocation breakdown for a completed rental.
 * Shows a bar for each allocation bucket relative to the authorized total.
 */
export default function HoldStatusCard({ allocation }: HoldStatusCardProps) {
  const {
    authorizedCents,
    capturedLateCents,
    capturedDamageCents,
    damageReserveCents,
    releasedCents,
  } = allocation;

  const totalAuthorized = authorizedCents;

  return (
    <View className="bg-gray-50 rounded-2xl overflow-hidden">
      <View className="px-4 py-3 border-b border-gray-200">
        <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
          Hold breakdown
        </Text>
      </View>

      <View className="px-4 py-4">
        {/* Authorized */}
        <View className="flex-row justify-between items-center mb-4">
          <Text className="text-sm text-gray-600">Total authorized</Text>
          <Text className="text-sm font-semibold text-gray-900">
            {formatDollars(totalAuthorized)}
          </Text>
        </View>

        <BarSegment
          label="Late return fee"
          cents={capturedLateCents}
          total={totalAuthorized}
          color="bg-amber-400"
          textColor="text-amber-700"
        />
        <BarSegment
          label="Damage charge"
          cents={capturedDamageCents}
          total={totalAuthorized}
          color="bg-red-400"
          textColor="text-red-700"
        />
        <BarSegment
          label="Damage reserve"
          cents={damageReserveCents}
          total={totalAuthorized}
          color="bg-orange-300"
          textColor="text-orange-700"
        />
        <BarSegment
          label="Released"
          cents={releasedCents}
          total={totalAuthorized}
          color="bg-green-400"
          textColor="text-green-700"
        />

        {/* Released status callout */}
        {releasedCents > 0 && (
          <View className="mt-2 bg-green-50 rounded-xl px-3 py-2">
            <Text className="text-xs text-green-700">
              {formatDollars(releasedCents)} released back to your payment method
            </Text>
          </View>
        )}

        {releasedCents === 0 &&
          capturedDamageCents === 0 &&
          capturedLateCents === 0 && (
            <View className="mt-2 bg-sky-50 rounded-xl px-3 py-2">
              <Text className="text-xs text-sky-700">
                Hold release is pending — check back shortly
              </Text>
            </View>
          )}
      </View>
    </View>
  );
}
