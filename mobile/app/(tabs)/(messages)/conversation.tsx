/**
 * Conversation screen — real-time chat for a single booking thread.
 *
 * Features:
 * - FlatList in chronological order (oldest at top, newest at bottom)
 * - Auto-scroll to bottom on mount and when a new message arrives
 * - Load-more on scroll to top (cursor pagination)
 * - Pusher subscription appends incoming messages live
 * - Text input with send button; disabled while sending
 */
import { useRef, useEffect, useCallback } from "react";
import {
  View,
  Text,
  FlatList,
  ActivityIndicator,
  SafeAreaView,
  Pressable,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useAuthStore } from "../../../lib/auth";
import { useMessages, useSendMessage, type Message } from "../../../lib/hooks/useMessages";
import MessageBubble from "../../../components/messaging/MessageBubble";
import MessageInput from "../../../components/messaging/MessageInput";

type ConversationParams = {
  transactionId: string;
  otherPartyName: string;
};

export default function ConversationScreen() {
  const router = useRouter();
  const { transactionId, otherPartyName } = useLocalSearchParams<ConversationParams>();
  const user = useAuthStore((s) => s.user);
  const listRef = useRef<FlatList<Message>>(null);

  const {
    data,
    isLoading,
    isFetchingNextPage,
    fetchNextPage,
    hasNextPage,
    error,
  } = useMessages(transactionId ?? null);

  const { mutate: sendMessage, isPending: isSending } = useSendMessage(transactionId ?? "");

  // Flatten pages into a single array (oldest first)
  const messages: Message[] = data?.pages.flatMap((p) => p.messages) ?? [];

  // Scroll to bottom when the first page loads or when a new message arrives
  const lastMessageId = messages[messages.length - 1]?.id;
  useEffect(() => {
    if (messages.length > 0) {
      listRef.current?.scrollToEnd({ animated: true });
    }
  }, [lastMessageId]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      void fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const handleSend = useCallback(
    (content: string) => {
      sendMessage(content);
    },
    [sendMessage],
  );

  if (isLoading) {
    return (
      <SafeAreaView className="flex-1 bg-white items-center justify-center">
        <ActivityIndicator size="large" color="#0284c7" />
      </SafeAreaView>
    );
  }

  if (error) {
    return (
      <SafeAreaView className="flex-1 bg-white">
        <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <Ionicons name="chevron-back" size={24} color="#111827" />
          </Pressable>
          <Text className="text-lg font-semibold text-gray-900 ml-2">
            {otherPartyName ?? "Conversation"}
          </Text>
        </View>
        <View className="flex-1 items-center justify-center px-8">
          <Text className="text-gray-500 text-center">
            Unable to load messages. Please try again.
          </Text>
        </View>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView className="flex-1 bg-white">
      {/* Header */}
      <View className="flex-row items-center px-4 pt-4 pb-3 border-b border-gray-100">
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <Ionicons name="chevron-back" size={24} color="#111827" />
        </Pressable>
        <Text className="text-lg font-semibold text-gray-900 ml-2 flex-1" numberOfLines={1}>
          {otherPartyName ?? "Conversation"}
        </Text>
      </View>

      <KeyboardAvoidingView
        className="flex-1"
        behavior={Platform.OS === "ios" ? "padding" : "height"}
        keyboardVerticalOffset={0}
      >
        {/* Load-more indicator at top */}
        {isFetchingNextPage && (
          <View className="py-2 items-center">
            <ActivityIndicator size="small" color="#9ca3af" />
          </View>
        )}

        {/* Message list */}
        <FlatList
          ref={listRef}
          data={messages}
          keyExtractor={(item) => item.id}
          renderItem={({ item }) => (
            <MessageBubble
              message={item}
              isOwn={item.senderId === user?.id}
              senderName={otherPartyName ?? "Them"}
            />
          )}
          contentContainerStyle={{ paddingHorizontal: 16, paddingVertical: 12 }}
          onStartReached={handleLoadMore}
          onStartReachedThreshold={0.2}
          ListEmptyComponent={
            <View className="flex-1 items-center justify-center py-16">
              <Ionicons name="chatbubble-ellipses-outline" size={40} color="#d1d5db" />
              <Text className="text-gray-400 text-sm mt-3">
                No messages yet. Say hello!
              </Text>
            </View>
          }
        />

        {/* Input bar */}
        <MessageInput
          onSend={handleSend}
          isSending={isSending}
        />
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
