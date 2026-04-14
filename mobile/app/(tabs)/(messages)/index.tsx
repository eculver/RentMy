import { View, SafeAreaView } from "react-native";
import { useQueryClient } from "@tanstack/react-query";
import { useConversations } from "../../../lib/hooks/useConversations";
import ConversationList from "../../../components/messaging/ConversationList";

export default function MessagesScreen() {
  const queryClient = useQueryClient();
  const { data, isLoading, error, refetch, isRefetching } = useConversations();

  const conversations = data?.conversations ?? [];

  const handleRefresh = () => {
    void refetch();
    void queryClient.invalidateQueries({ queryKey: ["notifications", "unread-count"] });
  };

  return (
    <View testID="screen-messages" className="flex-1 bg-white">
      <ConversationList
        conversations={conversations}
        isLoading={isLoading}
        error={error}
        onRefresh={handleRefresh}
        isRefreshing={isRefetching}
      />
    </View>
  );
}
