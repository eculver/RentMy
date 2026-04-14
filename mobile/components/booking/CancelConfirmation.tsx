import { View, Text, Pressable, Modal, ActivityIndicator } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useState } from "react";

interface CancelConfirmationProps {
  visible: boolean;
  scheduledStart: string;
  onConfirm: () => Promise<void>;
  onDismiss: () => void;
}

function cancellationNote(scheduledStart: string): string {
  const start = new Date(scheduledStart);
  const now = Date.now();
  const hoursUntil = (start.getTime() - now) / (1000 * 60 * 60);

  if (hoursUntil > 2) return "No cancellation fee — more than 2 hours before pickup.";
  if (hoursUntil > 1) return "25% cancellation fee applies — 1–2 hours before pickup.";
  if (hoursUntil > 0) return "50% cancellation fee applies — less than 1 hour before pickup.";
  return "100% cancellation fee applies — the rental period has already started.";
}

export default function CancelConfirmation({
  visible,
  scheduledStart,
  onConfirm,
  onDismiss,
}: CancelConfirmationProps) {
  const [confirming, setConfirming] = useState(false);

  const handleConfirm = async () => {
    setConfirming(true);
    try {
      await onConfirm();
    } finally {
      setConfirming(false);
    }
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={onDismiss}
    >
      <View className="flex-1 bg-black/40 items-center justify-center px-6">
        <View className="w-full bg-white rounded-2xl overflow-hidden">
          {/* Icon + heading */}
          <View className="items-center pt-6 pb-4 px-6">
            <View className="w-14 h-14 bg-red-50 rounded-full items-center justify-center mb-3">
              <Ionicons name="close-circle-outline" size={32} color="#dc2626" />
            </View>
            <Text className="text-lg font-bold text-gray-900 text-center">
              Cancel this booking?
            </Text>
          </View>

          {/* Fee note */}
          <View className="mx-6 mb-5 bg-amber-50 rounded-xl px-4 py-3">
            <Text className="text-sm text-amber-800">
              {cancellationNote(scheduledStart)}
            </Text>
          </View>

          {/* Buttons */}
          <View className="border-t border-gray-100 flex-row">
            <Pressable
              onPress={onDismiss}
              disabled={confirming}
              className="flex-1 py-4 items-center border-r border-gray-100"
            >
              <Text className="text-sm font-semibold text-gray-700">
                Keep booking
              </Text>
            </Pressable>
            <Pressable
              testID="btn-confirm-cancel"
              onPress={handleConfirm}
              disabled={confirming}
              className="flex-1 py-4 items-center"
            >
              {confirming ? (
                <ActivityIndicator size="small" color="#dc2626" />
              ) : (
                <Text className="text-sm font-semibold text-red-600">
                  Cancel booking
                </Text>
              )}
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}
