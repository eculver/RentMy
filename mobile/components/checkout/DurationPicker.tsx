import { View, Text, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";

// 7-day maximum booking ceiling (PRD §7)
const MAX_DURATION_MS = 7 * 24 * 60 * 60 * 1000;

interface DurationPickerProps {
  start: Date | null;
  end: Date | null;
  onChangeStart: (date: Date) => void;
  onChangeEnd: (date: Date) => void;
}

function formatDateLabel(date: Date | null): string {
  if (!date) return "Select";
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function durationLabel(start: Date | null, end: Date | null): string | null {
  if (!start || !end) return null;
  const ms = end.getTime() - start.getTime();
  if (ms <= 0) return null;
  const hours = ms / (1000 * 60 * 60);
  if (hours < 24) return `${Math.round(hours)} hr${Math.round(hours) !== 1 ? "s" : ""}`;
  const days = Math.floor(hours / 24);
  const remainHours = Math.round(hours % 24);
  if (remainHours === 0) return `${days} day${days !== 1 ? "s" : ""}`;
  return `${days}d ${remainHours}h`;
}

function nextHour(from: Date): Date {
  const d = new Date(from);
  d.setMinutes(0, 0, 0);
  d.setHours(d.getHours() + 1);
  return d;
}

// Simple +/- hour step controls — a full calendar library is deferred to Phase 5
function StepControl({
  label,
  date,
  onChange,
  minDate,
  maxDate,
}: {
  label: string;
  date: Date | null;
  onChange: (d: Date) => void;
  minDate?: Date;
  maxDate?: Date;
}) {
  const canDecrease = date != null && (!minDate || date.getTime() - 3600000 >= minDate.getTime());
  const canIncrease = date != null && (!maxDate || date.getTime() + 3600000 <= maxDate.getTime());

  const step = (delta: number) => {
    const base = date ?? nextHour(new Date());
    onChange(new Date(base.getTime() + delta * 3600000));
  };

  const initIfNull = () => {
    if (!date) {
      const d = nextHour(new Date());
      onChange(d);
    }
  };

  return (
    <View className="flex-1">
      <Text className="text-xs text-gray-500 mb-1">{label}</Text>
      <View className="flex-row items-center border border-gray-200 rounded-xl overflow-hidden">
        <Pressable
          onPress={() => (date ? step(-1) : initIfNull())}
          disabled={!canDecrease}
          className={`px-3 py-3 ${!canDecrease ? "opacity-30" : ""}`}
        >
          <Ionicons name="remove" size={16} color="#374151" />
        </Pressable>
        <Pressable onPress={initIfNull} className="flex-1 items-center py-3">
          <Text className="text-sm font-medium text-gray-800 text-center">
            {formatDateLabel(date)}
          </Text>
        </Pressable>
        <Pressable
          onPress={() => (date ? step(1) : initIfNull())}
          disabled={!canIncrease}
          className={`px-3 py-3 ${!canIncrease ? "opacity-30" : ""}`}
        >
          <Ionicons name="add" size={16} color="#374151" />
        </Pressable>
      </View>
    </View>
  );
}

export default function DurationPicker({
  start,
  end,
  onChangeStart,
  onChangeEnd,
}: DurationPickerProps) {
  const now = new Date();

  const handleStartChange = (newStart: Date) => {
    onChangeStart(newStart);
    // If end is now invalid, push it to start + 1h
    if (end && end.getTime() <= newStart.getTime()) {
      onChangeEnd(new Date(newStart.getTime() + 3600000));
    }
  };

  const handleEndChange = (newEnd: Date) => {
    if (start && newEnd.getTime() <= start.getTime()) return;
    if (start && newEnd.getTime() - start.getTime() > MAX_DURATION_MS) return;
    onChangeEnd(newEnd);
  };

  const maxEnd = start ? new Date(start.getTime() + MAX_DURATION_MS) : undefined;
  const duration = durationLabel(start, end);
  const exceeds7Days =
    start && end && end.getTime() - start.getTime() > MAX_DURATION_MS;

  return (
    <View className="gap-y-3">
      <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
        Rental period
      </Text>
      <View className="flex-row gap-x-3">
        <StepControl
          label="Start"
          date={start}
          onChange={handleStartChange}
          minDate={now}
        />
        <StepControl
          label="End"
          date={end}
          onChange={handleEndChange}
          minDate={start ?? now}
          maxDate={maxEnd}
        />
      </View>
      {duration && (
        <View className="flex-row items-center gap-x-1">
          <Ionicons name="time-outline" size={14} color="#6b7280" />
          <Text className="text-sm text-gray-500">Duration: {duration}</Text>
        </View>
      )}
      {exceeds7Days && (
        <View className="flex-row items-center gap-x-1 bg-red-50 px-3 py-2 rounded-lg">
          <Ionicons name="warning-outline" size={14} color="#ef4444" />
          <Text className="text-sm text-red-600">
            Maximum rental duration is 7 days
          </Text>
        </View>
      )}
    </View>
  );
}
