import { Pressable, Text, View } from "react-native";
import { BUBBLE_LABELS, RatingBubble } from "../../lib/hooks/useRatings";

interface RatingBubblesProps {
  /** Available bubbles for the current user's role. */
  availableBubbles: RatingBubble[];
  /** Currently selected bubbles. */
  selected: RatingBubble[];
  /** Called when the user taps a bubble to toggle it. */
  onToggle: (bubble: RatingBubble) => void;
  /** When true, bubbles are rendered as read-only tags (no interaction). */
  readOnly?: boolean;
}

/**
 * Renders a row of tappable pill-shaped bubble tags.
 * When readOnly is true, the selected bubbles are displayed without interaction.
 */
export default function RatingBubbles({
  availableBubbles,
  selected,
  onToggle,
  readOnly = false,
}: RatingBubblesProps) {
  return (
    <View className="flex-row flex-wrap gap-2">
      {availableBubbles.map((bubble) => {
        const isSelected = selected.includes(bubble);
        return (
          <Pressable
            key={bubble}
            testID={`rating-bubble-${bubble}`}
            onPress={readOnly ? undefined : () => onToggle(bubble)}
            disabled={readOnly}
            className={[
              "px-4 py-2 rounded-full border",
              isSelected
                ? "bg-primary-600 border-primary-600"
                : "bg-white border-gray-300",
            ].join(" ")}
          >
            <Text
              className={[
                "text-sm font-medium",
                isSelected ? "text-white" : "text-gray-700",
              ].join(" ")}
            >
              {BUBBLE_LABELS[bubble]}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}
