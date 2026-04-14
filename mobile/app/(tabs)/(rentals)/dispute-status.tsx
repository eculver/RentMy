/**
 * Dispute status screen — tracks the lifecycle of an open dispute.
 *
 * Shows a vertical timeline (filed → evidence → under review → resolved),
 * the agent/human decision when available, hold charge/release details,
 * and a photo re-prompt upload CTA when the status is INCONCLUSIVE.
 */
import {
  View,
  Text,
  ScrollView,
  Pressable,
  ActivityIndicator,
  Alert,
  RefreshControl,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import {
  useTransactionDisputes,
  useDispute,
  type Dispute,
} from "../../../lib/hooks/useDispute";
import DisputeTimeline from "../../../components/rental/DisputeTimeline";

type Params = {
  transactionId: string;
  disputeId?: string;
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function formatDollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

const REASON_LABELS: Record<string, string> = {
  DAMAGE: "Damage",
  MISSING_ITEM: "Missing item",
  OTHER: "Other",
};

const ESCALATION_LABELS: Record<string, string> = {
  AUTO_RESOLVE: "Auto-resolved",
  AUTO_RESOLVE_AUDIT: "Auto-resolved (audited)",
  HUMAN_REVIEW: "Human review",
};

// ── DisputeDetail ─────────────────────────────────────────────────────────────

function DisputeDetail({ dispute }: { dispute: Dispute }) {
  const router = useRouter();

  const isInconclusive = dispute.agentDecision === "INCONCLUSIVE";
  const isResolved =
    dispute.status === "RESOLVED" || dispute.status === "CLOSED";

  const handleReprompt = () => {
    Alert.alert(
      "Upload additional photos",
      "Please take new photos of the item and submit them to support the dispute review.",
      [{ text: "OK" }],
    );
  };

  return (
    <View className="gap-y-4">
      {/* Timeline */}
      <View testID="dispute-timeline">
        <DisputeTimeline currentStatus={dispute.status} />
      </View>

      {/* INCONCLUSIVE re-prompt banner */}
      {!isResolved && isInconclusive && (
        <View className="bg-amber-50 border border-amber-200 rounded-2xl px-4 py-3 gap-y-2">
          <View className="flex-row items-start gap-x-2">
            <Ionicons name="camera-outline" size={18} color="#b45309" />
            <View className="flex-1">
              <Text className="text-sm font-semibold text-amber-800">
                Additional photos requested
              </Text>
              <Text className="text-xs text-amber-700 mt-0.5 leading-relaxed">
                The photo comparison was inconclusive. Please upload new photos
                of the item to help resolve the dispute.
                {dispute.slaDeadline && (
                  <Text className="font-semibold">
                    {" "}Deadline: {formatDate(dispute.slaDeadline)}
                  </Text>
                )}
              </Text>
            </View>
          </View>
          <Pressable
            className="bg-amber-600 rounded-xl py-2.5 items-center"
            onPress={handleReprompt}
          >
            <Text className="text-white text-sm font-semibold">
              Upload photos
            </Text>
          </Pressable>
        </View>
      )}

      {/* Dispute details */}
      <View className="bg-gray-50 rounded-2xl overflow-hidden">
        <View className="px-4 py-3 border-b border-gray-200">
          <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
            Dispute details
          </Text>
        </View>
        <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
          <Text className="text-sm text-gray-600">Dispute ID</Text>
          <Text className="text-xs font-mono text-gray-500">
            {dispute.id.slice(-8).toUpperCase()}
          </Text>
        </View>
        <View className="px-4 py-3 border-b border-gray-100">
          <Text className="text-sm text-gray-600 mb-0.5">Reason</Text>
          <Text className="text-sm font-medium text-gray-900">
            {REASON_LABELS[dispute.reason] ?? dispute.reason}
          </Text>
        </View>
        <View className="px-4 py-3 border-b border-gray-100">
          <Text className="text-sm text-gray-600 mb-0.5">Description</Text>
          <Text className="text-sm text-gray-700 leading-relaxed">
            {dispute.description}
          </Text>
        </View>
        <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
          <Text className="text-sm text-gray-600">Filed</Text>
          <Text className="text-sm text-gray-700">
            {formatDate(dispute.createdAt)}
          </Text>
        </View>
        {dispute.slaDeadline && !isResolved && (
          <View className="px-4 py-3 flex-row justify-between">
            <Text className="text-sm text-gray-600">SLA deadline</Text>
            <Text className="text-sm font-medium text-amber-700">
              {formatDate(dispute.slaDeadline)}
            </Text>
          </View>
        )}
      </View>

      {/* Decision (when available) */}
      {isResolved && (
        <View className="bg-gray-50 rounded-2xl overflow-hidden">
          <View className="px-4 py-3 border-b border-gray-200">
            <Text className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
              Decision
            </Text>
          </View>
          {dispute.escalationRoute && (
            <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
              <Text className="text-sm text-gray-600">Reviewed by</Text>
              <Text className="text-sm font-medium text-gray-900">
                {ESCALATION_LABELS[dispute.escalationRoute] ?? dispute.escalationRoute}
              </Text>
            </View>
          )}
          {dispute.agentDecision && (
            <View className="px-4 py-3 border-b border-gray-100">
              <Text className="text-sm text-gray-600 mb-0.5">Outcome</Text>
              <Text className="text-sm font-medium text-gray-900">
                {dispute.agentDecision}
              </Text>
            </View>
          )}
          {dispute.agentConfidence != null && (
            <View className="px-4 py-3 border-b border-gray-100 flex-row justify-between">
              <Text className="text-sm text-gray-600">Confidence</Text>
              <Text className="text-sm text-gray-700">
                {Math.round(dispute.agentConfidence * 100)}%
              </Text>
            </View>
          )}
          {dispute.damageChargeCents != null && dispute.damageChargeCents > 0 && (
            <View className="px-4 py-3">
              <Text className="text-sm text-gray-600 mb-0.5">Damage charge</Text>
              <Text className="text-sm font-semibold text-red-700">
                {formatDollars(dispute.damageChargeCents)}
              </Text>
            </View>
          )}
          {dispute.damageChargeCents === 0 && (
            <View className="px-4 py-3">
              <Text className="text-sm text-green-700 font-medium">
                No charge applied — full hold released
              </Text>
            </View>
          )}
        </View>
      )}

      {/* Back button */}
      <Pressable
        testID="btn-back-to-rentals-dispute"
        className="border border-gray-200 rounded-2xl py-4 items-center"
        onPress={() => router.replace("/(tabs)/(rentals)" as never)}
      >
        <Text className="text-gray-700 font-semibold text-base">
          Back to rentals
        </Text>
      </Pressable>
    </View>
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

export default function DisputeStatusScreen() {
  const router = useRouter();
  const { transactionId, disputeId } = useLocalSearchParams<Params>();

  // If we have a specific dispute ID (from notification), fetch it directly.
  // Otherwise, fetch all disputes for the transaction and show the first open one.
  const { data: singleDispute, isLoading: singleLoading } = useDispute(
    disputeId ?? null,
  );
  const {
    data: allDisputes,
    isLoading: allLoading,
    isRefetching,
    refetch,
  } = useTransactionDisputes(disputeId ? null : (transactionId ?? null));

  const isLoading = disputeId ? singleLoading : allLoading;

  const dispute: Dispute | null =
    singleDispute ??
    (allDisputes ?? []).find(
      (d) => d.status !== "RESOLVED" && d.status !== "CLOSED",
    ) ??
    (allDisputes ?? [])[0] ??
    null;

  return (
    <View testID="screen-dispute-status" className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          Dispute status
        </Text>
      </View>

      {isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator size="large" color="#0284c7" />
        </View>
      ) : !dispute ? (
        <View className="flex-1 items-center justify-center px-8">
          <Text className="text-gray-500 text-center">
            No dispute found for this rental.
          </Text>
          <Pressable
            className="mt-4 px-6 py-3 bg-sky-600 rounded-xl"
            onPress={() => router.back()}
          >
            <Text className="text-white font-semibold">Go back</Text>
          </Pressable>
        </View>
      ) : (
        <ScrollView
          className="flex-1"
          contentContainerStyle={{ paddingVertical: 24, paddingHorizontal: 16, gap: 16 }}
          refreshControl={
            <RefreshControl
              refreshing={isRefetching}
              onRefresh={refetch}
              tintColor="#0284c7"
            />
          }
        >
          <DisputeDetail dispute={dispute} />
        </ScrollView>
      )}
    </View>
  );
}
