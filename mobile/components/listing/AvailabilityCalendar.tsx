import { View, Text } from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface TimeSlot {
  start: string;
  end: string;
}

interface AvailabilityCalendarProps {
  availability: unknown; // raw JSON from listing API — array of TimeSlot or null
}

function parseSlots(raw: unknown): TimeSlot[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter(
    (s): s is TimeSlot =>
      typeof s === "object" &&
      s !== null &&
      typeof (s as Record<string, unknown>).start === "string" &&
      typeof (s as Record<string, unknown>).end === "string",
  );
}

function formatSlot(slot: TimeSlot): string {
  const start = new Date(slot.start);
  const end = new Date(slot.end);
  const opts: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  };
  return `${start.toLocaleDateString("en-US", opts)} – ${end.toLocaleDateString("en-US", opts)}`;
}

export default function AvailabilityCalendar({
  availability,
}: AvailabilityCalendarProps) {
  const slots = parseSlots(availability);

  return (
    <View>
      <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">
        Availability
      </Text>

      {slots.length === 0 ? (
        <View className="flex-row items-center gap-x-2 bg-green-50 border border-green-100 rounded-2xl p-4">
          <Ionicons name="checkmark-circle-outline" size={18} color="#16a34a" />
          <Text className="text-sm text-green-800">
            Available anytime — contact host to arrange pickup.
          </Text>
        </View>
      ) : (
        <View className="bg-gray-50 rounded-2xl p-4 gap-y-2">
          {slots.map((slot, i) => (
            <View key={i} className="flex-row items-center gap-x-2">
              <Ionicons name="time-outline" size={14} color="#6b7280" />
              <Text className="text-sm text-gray-700">{formatSlot(slot)}</Text>
            </View>
          ))}
        </View>
      )}
    </View>
  );
}
