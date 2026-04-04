import { View, Image, FlatList, Dimensions, Pressable } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useState, useRef } from "react";

const { width: SCREEN_WIDTH } = Dimensions.get("window");
const GALLERY_HEIGHT = 260;

interface PhotoGalleryProps {
  photos: string[];
  onPress?: () => void;
}

function PhotoPlaceholder() {
  return (
    <View
      style={{ width: SCREEN_WIDTH, height: GALLERY_HEIGHT }}
      className="bg-gray-100 items-center justify-center"
    >
      <Ionicons name="image-outline" size={48} color="#9ca3af" />
    </View>
  );
}

export default function PhotoGallery({ photos, onPress }: PhotoGalleryProps) {
  const [activeIndex, setActiveIndex] = useState(0);
  const flatListRef = useRef<FlatList>(null);

  if (photos.length === 0) {
    return (
      <Pressable onPress={onPress}>
        <PhotoPlaceholder />
      </Pressable>
    );
  }

  return (
    <View style={{ height: GALLERY_HEIGHT }}>
      <FlatList
        ref={flatListRef}
        data={photos}
        keyExtractor={(_, i) => String(i)}
        renderItem={({ item }) => (
          <Pressable onPress={onPress}>
            <Image
              source={{ uri: item }}
              style={{ width: SCREEN_WIDTH, height: GALLERY_HEIGHT }}
              resizeMode="cover"
            />
          </Pressable>
        )}
        horizontal
        pagingEnabled
        showsHorizontalScrollIndicator={false}
        onMomentumScrollEnd={(e) => {
          const index = Math.round(
            e.nativeEvent.contentOffset.x / SCREEN_WIDTH,
          );
          setActiveIndex(index);
        }}
      />

      {/* Pagination dots */}
      {photos.length > 1 && (
        <View className="absolute bottom-3 left-0 right-0 flex-row justify-center gap-x-1.5">
          {photos.map((_, i) => (
            <View
              key={i}
              className={`rounded-full ${
                i === activeIndex
                  ? "w-2 h-2 bg-white"
                  : "w-1.5 h-1.5 bg-white/50"
              }`}
            />
          ))}
        </View>
      )}
    </View>
  );
}
