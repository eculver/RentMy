import { useEffect, useRef } from "react";
import { View, Text, Animated } from "react-native";

interface AIAutofillOverlayProps {
  isLoading: boolean;
  error?: string | null;
}

function ShimmerBar({ width, delay }: { width: string; delay: number }) {
  const opacity = useRef(new Animated.Value(0.3)).current;

  useEffect(() => {
    const anim = Animated.loop(
      Animated.sequence([
        Animated.timing(opacity, {
          toValue: 1,
          duration: 700,
          delay,
          useNativeDriver: true,
        }),
        Animated.timing(opacity, {
          toValue: 0.3,
          duration: 700,
          useNativeDriver: true,
        }),
      ])
    );
    anim.start();
    return () => anim.stop();
  }, [opacity, delay]);

  return (
    <Animated.View
      style={{ opacity, width: width as `${number}%` }}
      className="h-4 bg-gray-200 rounded-md mb-2"
    />
  );
}

export default function AIAutofillOverlay({
  isLoading,
  error,
}: AIAutofillOverlayProps) {
  if (!isLoading && !error) return null;

  return (
    <View className="absolute inset-0 bg-white/95 z-10 justify-center px-6">
      {error ? (
        <View className="items-center">
          <Text className="text-2xl mb-3">🤖</Text>
          <Text className="text-base font-semibold text-gray-800 text-center mb-1">
            AI couldn't identify this item
          </Text>
          <Text className="text-sm text-gray-500 text-center">
            {error} Please fill in the details manually.
          </Text>
        </View>
      ) : (
        <View>
          <View className="flex-row items-center mb-6">
            <Text className="text-2xl mr-2">🤖</Text>
            <Text className="text-base font-semibold text-gray-800">
              AI is identifying your item…
            </Text>
          </View>

          {/* Title skeleton */}
          <Text className="text-xs text-gray-400 mb-1 uppercase tracking-wide">
            Title
          </Text>
          <ShimmerBar width="85%" delay={0} />

          {/* Description skeleton */}
          <Text className="text-xs text-gray-400 mb-1 mt-3 uppercase tracking-wide">
            Description
          </Text>
          <ShimmerBar width="100%" delay={100} />
          <ShimmerBar width="90%" delay={200} />
          <ShimmerBar width="70%" delay={300} />

          {/* Pricing skeleton */}
          <Text className="text-xs text-gray-400 mb-1 mt-3 uppercase tracking-wide">
            Pricing
          </Text>
          <View className="flex-row gap-3">
            <ShimmerBar width="45%" delay={150} />
            <ShimmerBar width="45%" delay={250} />
          </View>

          {/* Tags skeleton */}
          <Text className="text-xs text-gray-400 mb-1 mt-3 uppercase tracking-wide">
            Tags
          </Text>
          <View className="flex-row gap-2">
            <ShimmerBar width="25%" delay={100} />
            <ShimmerBar width="20%" delay={200} />
            <ShimmerBar width="30%" delay={300} />
          </View>
        </View>
      )}
    </View>
  );
}
