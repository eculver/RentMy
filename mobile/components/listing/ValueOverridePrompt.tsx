import { useState } from "react";
import {
  Modal,
  View,
  Text,
  TextInput,
  Pressable,
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useOverride, type OverrideResult } from "../../lib/hooks/useAppraisal";

interface ValueOverridePromptProps {
  visible: boolean;
  listingId: string;
  aiEstimateCents: number;
  hostValueCents: number;
  onClose: () => void;
  onResult: (result: OverrideResult) => void;
}

function formatDollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function ValueOverridePrompt({
  visible,
  listingId,
  aiEstimateCents,
  hostValueCents,
  onClose,
  onResult,
}: ValueOverridePromptProps) {
  const [justification, setJustification] = useState("");
  const [result, setResult] = useState<OverrideResult | null>(null);
  const { mutate: submitOverride, isPending, isError } = useOverride(listingId);

  const handleSubmit = () => {
    if (!justification.trim()) return;
    submitOverride(
      { declaredValueCents: hostValueCents, justification: justification.trim() },
      {
        onSuccess: (res) => {
          setResult(res);
          onResult(res);
        },
      }
    );
  };

  const handleClose = () => {
    setJustification("");
    setResult(null);
    onClose();
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={handleClose}
    >
      <KeyboardAvoidingView
        className="flex-1 justify-end"
        behavior={Platform.OS === "ios" ? "padding" : "height"}
      >
        <View className="bg-white rounded-t-2xl px-6 pt-6 pb-10">
          <Text className="text-lg font-bold text-gray-900 mb-1">
            Value Override Required
          </Text>
          <Text className="text-sm text-gray-500 mb-5">
            Your declared value is significantly higher than our AI estimate.
            Please provide a justification.
          </Text>

          <View className="flex-row justify-between mb-5">
            <View className="flex-1 bg-gray-50 rounded-xl p-3 mr-2 items-center">
              <Text className="text-xs text-gray-400 mb-1">AI Estimate</Text>
              <Text className="text-base font-semibold text-gray-700">
                {formatDollars(aiEstimateCents)}
              </Text>
            </View>
            <View className="flex-1 bg-amber-50 rounded-xl p-3 ml-2 items-center">
              <Text className="text-xs text-amber-600 mb-1">Your Value</Text>
              <Text className="text-base font-semibold text-amber-700">
                {formatDollars(hostValueCents)}
              </Text>
            </View>
          </View>

          {result ? (
            <View
              className={`rounded-xl p-4 mb-5 ${
                result.approved ? "bg-green-50" : "bg-red-50"
              }`}
            >
              <Text
                className={`text-sm font-semibold mb-1 ${
                  result.approved ? "text-green-700" : "text-red-700"
                }`}
              >
                {result.approved ? "Override Approved" : "Override Rejected"}
              </Text>
              <Text
                className={`text-sm ${
                  result.approved ? "text-green-600" : "text-red-600"
                }`}
              >
                {result.reasoning}
              </Text>
            </View>
          ) : (
            <>
              <Text className="text-sm font-medium text-gray-700 mb-1">
                Justification
              </Text>
              <TextInput
                className="border border-gray-300 rounded-xl px-4 py-3 text-sm text-gray-800 mb-4 min-h-[80px]"
                placeholder="e.g. Limited edition, recent purchase with receipt, collector item…"
                value={justification}
                onChangeText={setJustification}
                multiline
                numberOfLines={3}
                textAlignVertical="top"
              />
              {isError && (
                <Text className="text-red-500 text-sm mb-3">
                  Override review failed. Please try again.
                </Text>
              )}
              <Pressable
                onPress={handleSubmit}
                disabled={isPending || !justification.trim()}
                className={`rounded-xl py-3 items-center ${
                  isPending || !justification.trim()
                    ? "bg-gray-200"
                    : "bg-sky-600"
                }`}
              >
                {isPending ? (
                  <ActivityIndicator color="white" />
                ) : (
                  <Text
                    className={`text-sm font-semibold ${
                      !justification.trim() ? "text-gray-400" : "text-white"
                    }`}
                  >
                    Submit for Review
                  </Text>
                )}
              </Pressable>
            </>
          )}

          <Pressable onPress={handleClose} className="mt-4 items-center">
            <Text className="text-sm text-gray-400">
              {result ? "Close" : "Cancel"}
            </Text>
          </Pressable>
        </View>
      </KeyboardAvoidingView>
    </Modal>
  );
}
