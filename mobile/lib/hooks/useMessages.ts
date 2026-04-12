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
 *
 * Pagination direction: pages[0] always holds the newest messages; each
 * subsequent page (fetched via fetchNextPage / onStartReached) holds older
 * messages. Within each page, messages are sorted oldest-first. The
 * conversation screen reverses the page order before rendering so that the
 * FlatList displays oldest-at-top, newest-at-bottom.
 *
 * Subscribes to `private-transaction-{id}` / `new-message` via Pusher —
 * each new event appends to pages[0] (the newest page) so it appears
 * instantly without a full refetch.
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
          // Append to pages[0] — the newest page — so the message appears at
          // the bottom of the chat after the caller reverses page order.
          const pages = old.pages.map((page, i) => {
            if (i !== 0) return page;
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
    // nextCursor points to the oldest message in the current page; fetching
    // it loads the next older batch (correct for scroll-to-top load-more).
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    enabled: transactionId !== null,
  });
}

/**
 * Sends a message to the given booking thread.
 * On success, appends the returned message to pages[0] (newest page) so it
 * appears at the bottom of the chat.
 * Returns a Promise so callers can await success before clearing the input.
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
          // Append to pages[0] — the newest page.
          const pages = old.pages.map((page, i) => {
            if (i !== 0) return page;
            return { ...page, messages: [...page.messages, message] };
          });
          return { ...old, pages };
        },
      );
    },
  });
}
