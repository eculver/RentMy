import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { Booking } from "./useBooking";

export interface Conversation {
  transactionId: string;
  otherPartyId: string;
  otherPartyName: string;
  listingTitle: string;
  lastMessage?: string;
  lastMessageAt?: string;
  unreadCount: number;
  bookingStatus: Booking["status"];
}

interface ConversationsResponse {
  conversations: Conversation[];
}

/**
 * Fetches the current user's active conversations (bookings that have messages
 * or are in a non-terminal state).
 */
export function useConversations() {
  return useQuery<ConversationsResponse>({
    queryKey: ["conversations"],
    queryFn: () =>
      api.get("api/v1/users/me/conversations").json<ConversationsResponse>(),
  });
}

/** Unread message count across all conversations (for tab badge). */
export function useUnreadCount() {
  return useQuery<{ count: number }>({
    queryKey: ["notifications", "unread-count"],
    queryFn: () =>
      api.get("api/v1/notifications/unread-count").json<{ count: number }>(),
    refetchInterval: 30_000,
  });
}
