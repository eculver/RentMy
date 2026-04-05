import { View, Text, FlatList, Pressable, ActivityIndicator } from "react-native";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import Avatar from "../ui/Avatar";
import type { Conversation } from "../../lib/hooks/useConversations";

function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

interface ConversationRowProps {
  conversation: Conversation;
}

function ConversationRow({ conversation }: ConversationRowProps) {
  const router = useRouter();

  return (
    <Pressable
      className="flex-row items-center px-4 py-4 bg-white border-b border-gray-50 active:bg-gray-50"
      onPress={() =>
        router.push({
          pathname: "/(tabs)/(messages)/conversation" as never,
          params: {
            transactionId: conversation.transactionId,
            otherPartyName: conversation.otherPartyName,
          },
        })
      }
    >
      <View className="relative mr-3">
        <Avatar name={conversation.otherPartyName} size="md" />
        {conversation.unreadCount > 0 && (
          <View className="absolute -top-1 -right-1 w-5 h-5 rounded-full bg-sky-600 items-center justify-center">
            <Text className="text-white text-xs font-bold">
              {conversation.unreadCount > 9 ? "9+" : String(conversation.unreadCount)}
            </Text>
          </View>
        )}
      </View>

      <View className="flex-1 min-w-0">
        <View className="flex-row items-center justify-between mb-0.5">
          <Text className="text-sm font-semibold text-gray-900 flex-1 mr-2" numberOfLines={1}>
            {conversation.otherPartyName}
          </Text>
          {conversation.lastMessageAt && (
            <Text className="text-xs text-gray-400 flex-shrink-0">
              {formatRelativeTime(conversation.lastMessageAt)}
            </Text>
          )}
        </View>
        <Text className="text-xs text-gray-500 mb-0.5" numberOfLines={1}>
          {conversation.listingTitle}
        </Text>
        {conversation.lastMessage ? (
          <Text
            className={`text-sm ${conversation.unreadCount > 0 ? "font-medium text-gray-900" : "text-gray-500"}`}
            numberOfLines={1}
          >
            {conversation.lastMessage}
          </Text>
        ) : (
          <Text className="text-sm text-gray-400 italic">No messages yet</Text>
        )}
      </View>

      <Ionicons name="chevron-forward" size={16} color="#d1d5db" className="ml-2" />
    </Pressable>
  );
}

interface ConversationListProps {
  conversations: Conversation[];
  isLoading: boolean;
  error: Error | null;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export default function ConversationList({
  conversations,
  isLoading,
  error,
  onRefresh,
  isRefreshing,
}: ConversationListProps) {
  if (isLoading) {
    return (
      <View className="flex-1 items-center justify-center">
        <ActivityIndicator size="large" color="#0284c7" />
      </View>
    );
  }

  if (error) {
    return (
      <View className="flex-1 items-center justify-center px-8">
        <Text className="text-gray-500 text-center mb-4">
          Unable to load conversations. Please try again.
        </Text>
        <Pressable
          className="px-6 py-3 bg-sky-600 rounded-xl"
          onPress={onRefresh}
        >
          <Text className="text-white font-semibold">Retry</Text>
        </Pressable>
      </View>
    );
  }

  if (conversations.length === 0) {
    return (
      <View className="flex-1 items-center justify-center px-8">
        <Ionicons name="chatbubbles-outline" size={48} color="#d1d5db" />
        <Text className="text-gray-900 font-semibold text-lg mt-4">No messages yet</Text>
        <Text className="text-gray-400 text-sm text-center mt-2">
          Messages with renters and hosts will appear here once you have active bookings.
        </Text>
      </View>
    );
  }

  return (
    <FlatList
      data={conversations}
      keyExtractor={(item) => item.transactionId}
      renderItem={({ item }) => <ConversationRow conversation={item} />}
      refreshing={isRefreshing}
      onRefresh={onRefresh}
      className="flex-1 bg-white"
    />
  );
}
