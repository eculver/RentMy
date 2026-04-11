import { useState } from "react";
import { ActivityIndicator, Modal, Pressable, Text, View } from "react-native";
import {
  HOST_BUBBLES,
  RENTER_BUBBLES,
  RatingBubble,
  useSubmitRating,
} from "../../lib/hooks/useRatings";
import RatingBubbles from "./RatingBubbles";

interface RatingPromptProps {
  transactionId: string;
  /** The ID of the currently authenticated user. */
  currentUserId: string;
  /** The ID of the renter for this transaction. */
  renterId: string;
  visible: boolean;
  onDismiss: () => void;
}

/**
 * Post-rental rating prompt modal.
 * Shown after a transaction reaches COMPLETED status.
 * Determines the correct bubble set based on whether the current user is the
 * renter or the host, then submits the rating on confirmation.
 */
export default function RatingPrompt({
  transactionId,
  currentUserId,
  renterId,
  visible,
  onDismiss,
}: RatingPromptProps) {
  const isRenter = currentUserId === renterId;
  const availableBubbles: RatingBubble[] = isRenter
    ? RENTER_BUBBLES
    : HOST_BUBBLES;

  const [selected, setSelected] = useState<RatingBubble[]>([]);
  const { mutate, isPending, isSuccess, isError } =
    useSubmitRating(transactionId);

  const toggleBubble = (bubble: RatingBubble) => {
    setSelected((prev) =>
      prev.includes(bubble) ? prev.filter((b) => b !== bubble) : [...prev, bubble],
    );
  };

  const handleSubmit = () => {
    if (selected.length === 0) return;
    mutate(
      { bubbles: selected },
      {
        onSuccess: () => {
          setTimeout(onDismiss, 800);
        },
      },
    );
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={onDismiss}
    >
      <View className="flex-1 justify-end bg-black/40">
        <View className="bg-white rounded-t-3xl p-6 pb-10">
          <Text className="text-xl font-bold text-gray-900 mb-1">
            Rate your experience
          </Text>
          <Text className="text-sm text-gray-500 mb-6">
            Tap the bubbles that best describe your rental.
          </Text>

          <RatingBubbles
            availableBubbles={availableBubbles}
            selected={selected}
            onToggle={toggleBubble}
          />

          {isError && (
            <Text className="text-red-500 text-sm mt-4">
              Something went wrong. Please try again.
            </Text>
          )}

          {isSuccess && (
            <Text className="text-green-600 text-sm mt-4 font-medium">
              Rating submitted!
            </Text>
          )}

          <Pressable
            className={[
              "mt-6 py-4 rounded-xl items-center",
              selected.length > 0 && !isPending
                ? "bg-primary-600"
                : "bg-primary-300",
            ].join(" ")}
            onPress={handleSubmit}
            disabled={selected.length === 0 || isPending}
          >
            {isPending ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text className="text-white font-semibold text-base">
                Submit rating
              </Text>
            )}
          </Pressable>

          <Pressable className="mt-3 items-center py-2" onPress={onDismiss}>
            <Text className="text-gray-500 text-sm">Skip</Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}
