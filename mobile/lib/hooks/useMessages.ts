import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { usePusher } from "./usePusher";

export interface Message {
  id: string;
  transactionId: string;
  senderId: string;
  content: string;
  createdAt: string;
}

interface MessagesPage {
  messages: Message[];
  nextCursor?: string;
}

/**
 * Cursor-paginated message list for a booking.
 * Subscribes to `private-transaction-{id}` / `new-message` via Pusher —
 * each new event prepends the incoming message to the first page of cache
 * so it appears instantly without a full refetch.
 */
export function useMessages(transactionId: string | null) {
  const queryClient = useQueryClient();

  usePusher(
    transactionId ? `private-transaction-${transactionId}` : null,
    "new-message",
    (data: unknown) => {
      const msg = data as Message;
      queryClient.setQueryData<{ pages: MessagesPage[]; pageParams: unknown[] }>(
        ["messages", transactionId],
        (old) => {
          if (!old) return old;
          // Append to the last page so the flat list sees it in chronological order
          const pages = old.pages.map((page, i) => {
            if (i !== old.pages.length - 1) return page;
            return { ...page, messages: [...page.messages, msg] };
          });
          return { ...old, pages };
        },
      );
    },
  );

  return useInfiniteQuery<MessagesPage>({
    queryKey: ["messages", transactionId],
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams({ limit: "50" });
      if (pageParam) params.set("cursor", pageParam as string);
      return api
        .get(`api/v1/bookings/${transactionId}/messages?${params.toString()}`)
        .json<MessagesPage>();
    },
    initialPageParam: undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    enabled: transactionId !== null,
  });
}

/**
 * Sends a message to the given booking thread.
 * On success, appends the returned message to the query cache.
 */
export function useSendMessage(transactionId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (content: string) =>
      api
        .post(`api/v1/bookings/${transactionId}/messages`, { json: { content } })
        .json<{ message: Message }>(),
    onSuccess: ({ message }) => {
      queryClient.setQueryData<{ pages: MessagesPage[]; pageParams: unknown[] }>(
        ["messages", transactionId],
        (old) => {
          if (!old) return old;
          const pages = old.pages.map((page, i) => {
            if (i !== old.pages.length - 1) return page;
            return { ...page, messages: [...page.messages, message] };
          });
          return { ...old, pages };
        },
      );
    },
  });
}
