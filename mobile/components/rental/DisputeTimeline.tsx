import { View, Text } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import type { DisputeStatus } from "../../lib/hooks/useDispute";

interface TimelineStep {
  status: DisputeStatus;
  label: string;
  description: string;
  icon: string;
}

const TIMELINE_STEPS: TimelineStep[] = [
  {
    status: "PENDING",
    label: "Filed",
    description: "Dispute submitted",
    icon: "flag-outline",
  },
  {
    status: "EVIDENCE_GATHERING",
    label: "Evidence gathered",
    description: "Photos and transaction data collected",
    icon: "images-outline",
  },
  {
    status: "UNDER_REVIEW",
    label: "Under review",
    description: "Agent or human reviewer assessing",
    icon: "eye-outline",
  },
  {
    status: "RESOLVED",
    label: "Resolved",
    description: "Decision issued",
    icon: "checkmark-circle-outline",
  },
];

const STATUS_ORDER: DisputeStatus[] = [
  "PENDING",
  "EVIDENCE_GATHERING",
  "UNDER_REVIEW",
  "RESOLVED",
];

function stepIndex(status: DisputeStatus): number {
  const idx = STATUS_ORDER.indexOf(status);
  return idx === -1 ? STATUS_ORDER.length - 1 : idx;
}

interface DisputeTimelineProps {
  currentStatus: DisputeStatus;
}

/**
 * Vertical timeline showing dispute progression from filing to resolution.
 */
export default function DisputeTimeline({ currentStatus }: DisputeTimelineProps) {
  const currentIdx = stepIndex(currentStatus);

  return (
    <View className="bg-gray-50 rounded-2xl overflow-hidden">
      <View className="px-4 py-3 border-b border-gray-200">
        <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
          Dispute status
        </Text>
      </View>

      <View className="px-4 py-4">
        {TIMELINE_STEPS.map((step, i) => {
          const isComplete = i < currentIdx;
          const isActive = i === currentIdx;
          const isFuture = i > currentIdx;

          const dotColor = isComplete
            ? "#16a34a"
            : isActive
            ? "#0284c7"
            : "#d1d5db";
          const labelColor = isFuture ? "#9ca3af" : "#111827";

          return (
            <View key={step.status} className="flex-row gap-x-3">
              {/* Dot + connector column */}
              <View className="items-center" style={{ width: 20 }}>
                <View
                  className="w-5 h-5 rounded-full items-center justify-center"
                  style={{ backgroundColor: isActive ? "#e0f2fe" : isComplete ? "#dcfce7" : "#f3f4f6" }}
                >
                  <Ionicons
                    name={step.icon as React.ComponentProps<typeof Ionicons>["name"]}
                    size={12}
                    color={dotColor}
                  />
                </View>
                {/* Vertical connector (skip for last item) */}
                {i < TIMELINE_STEPS.length - 1 && (
                  <View
                    className="flex-1 w-0.5 my-1"
                    style={{
                      backgroundColor: i < currentIdx ? "#16a34a" : "#e5e7eb",
                      minHeight: 16,
                    }}
                  />
                )}
              </View>

              {/* Text column */}
              <View className="flex-1 pb-4">
                <Text
                  className="text-sm font-semibold"
                  style={{ color: labelColor }}
                >
                  {step.label}
                  {isActive && (
                    <Text className="text-sky-600"> ← current</Text>
                  )}
                </Text>
                <Text className="text-xs text-gray-400 mt-0.5">
                  {step.description}
                </Text>
              </View>
            </View>
          );
        })}
      </View>
    </View>
  );
}
