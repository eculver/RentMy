import { useState } from "react";
import { View, TextInput, Pressable, ActivityIndicator } from "react-native";
import { Ionicons } from "@expo/vector-icons";

const MAX_LENGTH = 2000;

interface MessageInputProps {
  onSend: (content: string) => void;
  isSending: boolean;
  disabled?: boolean;
}

export default function MessageInput({ onSend, isSending, disabled = false }: MessageInputProps) {
  const [text, setText] = useState("");

  const handleSend = () => {
    const trimmed = text.trim();
    if (!trimmed || isSending || disabled) return;
    onSend(trimmed);
    setText("");
  };

  const canSend = text.trim().length > 0 && !isSending && !disabled;

  return (
    <View className="flex-row items-end px-4 py-3 bg-white border-t border-gray-100 gap-x-3">
      <TextInput
        className="flex-1 min-h-[40px] max-h-[120px] bg-gray-100 rounded-2xl px-4 py-2.5 text-sm text-gray-900"
        placeholder="Message…"
        placeholderTextColor="#9ca3af"
        value={text}
        onChangeText={setText}
        multiline
        maxLength={MAX_LENGTH}
        editable={!disabled && !isSending}
        returnKeyType="default"
        blurOnSubmit={false}
      />
      <Pressable
        className={`w-10 h-10 rounded-full items-center justify-center ${canSend ? "bg-sky-600" : "bg-gray-200"}`}
        onPress={handleSend}
        disabled={!canSend}
        hitSlop={8}
      >
        {isSending ? (
          <ActivityIndicator size="small" color="#0284c7" />
        ) : (
          <Ionicons name="send" size={16} color={canSend ? "#fff" : "#9ca3af"} />
        )}
      </Pressable>
    </View>
  );
}
