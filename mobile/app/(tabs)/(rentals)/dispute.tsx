/**
 * Dispute filing screen — allows a user to report damage, a missing item,
 * or another issue with a completed rental.
 *
 * Submits via POST /api/v1/transactions/:id/disputes and navigates to the
 * dispute-status screen on success.
 */
import { useState } from "react";
import {
  View,
  Text,
  ScrollView,
  Pressable,
  TextInput,
  ActivityIndicator,
  Alert,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useFileDispute, type DisputeReason } from "../../../lib/hooks/useDispute";

type Params = {
  transactionId: string;
};

const REASON_OPTIONS: { value: DisputeReason; label: string; description: string; icon: string }[] = [
  {
    value: "DAMAGE",
    label: "Damage",
    description: "The item was returned with damage",
    icon: "construct-outline",
  },
  {
    value: "MISSING_ITEM",
    label: "Missing item",
    description: "Parts of the item are missing",
    icon: "search-outline",
  },
  {
    value: "OTHER",
    label: "Other",
    description: "Something else went wrong",
    icon: "help-circle-outline",
  },
];

export default function DisputeScreen() {
  const router = useRouter();
  const { transactionId } = useLocalSearchParams<Params>();

  const [selectedReason, setSelectedReason] = useState<DisputeReason | null>(null);
  const [description, setDescription] = useState("");

  const { mutate, isPending } = useFileDispute(transactionId ?? "");

  const canSubmit =
    selectedReason !== null && description.trim().length >= 10 && !isPending;

  const handleSubmit = () => {
    if (!canSubmit || !selectedReason) return;

    mutate(
      { reason: selectedReason, description: description.trim() },
      {
        onSuccess: (dispute) => {
          router.replace({
            pathname: "/(tabs)/(rentals)/dispute-status" as never,
            params: { transactionId: transactionId ?? "", disputeId: dispute.id },
          });
        },
        onError: () => {
          Alert.alert("Error", "Failed to file dispute. Please try again.");
        },
      },
    );
  };

  return (
    <View testID="screen-dispute" className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2">
          File a dispute
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ paddingVertical: 24, paddingHorizontal: 16, gap: 16 }}
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
      >
        {/* Intro */}
        <View className="bg-amber-50 rounded-2xl px-4 py-3 flex-row items-start gap-x-3">
          <Ionicons name="information-circle-outline" size={18} color="#b45309" />
          <Text className="text-sm text-amber-800 flex-1 leading-relaxed">
            Filing a dispute will pause hold release while our team reviews the
            rental. Provide as much detail as possible.
          </Text>
        </View>

        {/* Reason selector */}
        <View>
          <Text className="text-sm font-semibold text-gray-700 mb-3">
            What happened?
          </Text>
          <View className="gap-y-2">
            {REASON_OPTIONS.map((opt) => {
              const isSelected = selectedReason === opt.value;
              return (
                <Pressable
                  key={opt.value}
                  testID={`dispute-reason-${opt.value}`}
                  className={[
                    "flex-row items-center gap-x-3 p-4 rounded-2xl border",
                    isSelected
                      ? "border-sky-500 bg-sky-50"
                      : "border-gray-200 bg-gray-50",
                  ].join(" ")}
                  onPress={() => setSelectedReason(opt.value)}
                >
                  <View
                    className={[
                      "w-8 h-8 rounded-full items-center justify-center",
                      isSelected ? "bg-sky-100" : "bg-white",
                    ].join(" ")}
                  >
                    <Ionicons
                      name={opt.icon as React.ComponentProps<typeof Ionicons>["name"]}
                      size={16}
                      color={isSelected ? "#0284c7" : "#6b7280"}
                    />
                  </View>
                  <View className="flex-1">
                    <Text
                      className={`text-sm font-semibold ${isSelected ? "text-sky-700" : "text-gray-800"}`}
                    >
                      {opt.label}
                    </Text>
                    <Text className="text-xs text-gray-500 mt-0.5">
                      {opt.description}
                    </Text>
                  </View>
                  {isSelected && (
                    <Ionicons name="checkmark-circle" size={20} color="#0284c7" />
                  )}
                </Pressable>
              );
            })}
          </View>
        </View>

        {/* Description */}
        <View>
          <Text className="text-sm font-semibold text-gray-700 mb-2">
            Describe the issue
          </Text>
          <TextInput
            testID="input-dispute-description"
            className="bg-gray-50 rounded-2xl px-4 py-3 text-sm text-gray-900 border border-gray-200 min-h-[100px]"
            placeholder="Provide details about what happened (minimum 10 characters)…"
            placeholderTextColor="#9ca3af"
            value={description}
            onChangeText={setDescription}
            multiline
            textAlignVertical="top"
          />
          <Text className="text-xs text-gray-400 mt-1 text-right">
            {description.trim().length} chars{" "}
            {description.trim().length < 10 && "(min 10)"}
          </Text>
        </View>

        {/* Evidence note */}
        <View className="bg-gray-50 rounded-2xl px-4 py-3 flex-row items-start gap-x-3">
          <Ionicons name="images-outline" size={18} color="#6b7280" />
          <Text className="text-sm text-gray-600 flex-1 leading-relaxed">
            Your check-in and check-out photos are automatically attached as
            evidence.
          </Text>
        </View>

        {/* Submit */}
        <Pressable
          testID="btn-submit-dispute"
          className={[
            "rounded-2xl py-4 items-center flex-row justify-center gap-x-2",
            canSubmit ? "bg-red-600" : "bg-red-300",
          ].join(" ")}
          onPress={handleSubmit}
          disabled={!canSubmit}
        >
          {isPending ? (
            <ActivityIndicator color="white" />
          ) : (
            <>
              <Ionicons name="flag-outline" size={18} color="white" />
              <Text className="text-white font-semibold text-base">
                Submit dispute
              </Text>
            </>
          )}
        </Pressable>

        <Pressable
          testID="btn-cancel-dispute"
          className="py-3 items-center"
          onPress={() => router.back()}
        >
          <Text className="text-sm text-gray-500">Cancel</Text>
        </Pressable>
      </ScrollView>
    </View>
  );
}
