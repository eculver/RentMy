import { View, Text } from "react-native";
import type { Message } from "../../lib/hooks/useMessages";

interface MessageBubbleProps {
  message: Message;
  isOwn: boolean;
  senderName: string;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
  });
}

export default function MessageBubble({ message, isOwn, senderName }: MessageBubbleProps) {
  return (
    <View className={`mb-3 max-w-[78%] ${isOwn ? "self-end items-end" : "self-start items-start"}`}>
      {!isOwn && (
        <Text className="text-xs text-gray-400 mb-1 ml-1">{senderName}</Text>
      )}
      <View
        className={`px-4 py-3 rounded-2xl ${
          isOwn
            ? "bg-sky-600 rounded-br-sm"
            : "bg-gray-100 rounded-bl-sm"
        }`}
      >
        <Text className={`text-sm leading-relaxed ${isOwn ? "text-white" : "text-gray-900"}`}>
          {message.content}
        </Text>
      </View>
      <Text className="text-xs text-gray-400 mt-1 mx-1">{formatTime(message.createdAt)}</Text>
    </View>
  );
}
