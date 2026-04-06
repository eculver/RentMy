import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

export type RatingBubble =
  // Renter-rates-Host
  | "GOOD_COMMUNICATION"
  | "ON_TIME"
  | "ITEM_AS_DESCRIBED"
  | "EASY_PICKUP"
  | "FRIENDLY"
  // Host-rates-Renter
  | "ON_TIME_RETURN"
  | "CAREFUL_WITH_ITEM"
  | "EASY_HANDOFF"
  | "RESPECTFUL";

export interface Rating {
  id: string;
  transactionId: string;
  fromUserId: string;
  toUserId: string;
  bubbles: RatingBubble[];
  createdAt: string;
}

export interface BubbleSummaryItem {
  bubble: RatingBubble;
  count: number;
}

interface UserRatingsResponse {
  ratings: Rating[];
  total: number;
  page: number;
}

interface BubbleSummaryResponse {
  summary: BubbleSummaryItem[];
}

/** Friendly display label for each bubble. */
export const BUBBLE_LABELS: Record<RatingBubble, string> = {
  GOOD_COMMUNICATION: "Good communication",
  ON_TIME: "On time",
  ITEM_AS_DESCRIBED: "Item as described",
  EASY_PICKUP: "Easy pickup",
  FRIENDLY: "Friendly",
  ON_TIME_RETURN: "On time return",
  CAREFUL_WITH_ITEM: "Careful with item",
  EASY_HANDOFF: "Easy handoff",
  RESPECTFUL: "Respectful",
};

/** Bubbles available when a renter rates a host. */
export const RENTER_BUBBLES: RatingBubble[] = [
  "GOOD_COMMUNICATION",
  "ON_TIME",
  "ITEM_AS_DESCRIBED",
  "EASY_PICKUP",
  "FRIENDLY",
];

/** Bubbles available when a host rates a renter. */
export const HOST_BUBBLES: RatingBubble[] = [
  "GOOD_COMMUNICATION",
  "ON_TIME_RETURN",
  "CAREFUL_WITH_ITEM",
  "EASY_HANDOFF",
  "RESPECTFUL",
];

/** Fetches all ratings for a transaction. */
export function useTransactionRatings(transactionId: string | null) {
  return useQuery<Rating[]>({
    queryKey: ["ratings", "transaction", transactionId],
    queryFn: () =>
      api
        .get(`api/v1/transactions/${transactionId}/ratings`)
        .json<Rating[]>(),
    enabled: !!transactionId,
  });
}

/** Fetches paginated ratings received by a user. */
export function useUserRatings(userId: string | null, page = 1) {
  return useQuery<UserRatingsResponse>({
    queryKey: ["ratings", "user", userId, page],
    queryFn: () =>
      api
        .get(`api/v1/users/${userId}/ratings`, {
          searchParams: { page: String(page) },
        })
        .json<UserRatingsResponse>(),
    enabled: !!userId,
  });
}

/** Fetches bubble count summary for a user's received ratings. */
export function useUserRatingsSummary(userId: string | null) {
  return useQuery<BubbleSummaryResponse>({
    queryKey: ["ratings", "summary", userId],
    queryFn: () =>
      api
        .get(`api/v1/users/${userId}/ratings/summary`)
        .json<BubbleSummaryResponse>(),
    enabled: !!userId,
  });
}

/** Submits a rating for a completed transaction. */
export function useSubmitRating(transactionId: string) {
  const queryClient = useQueryClient();

  return useMutation<Rating, Error, { bubbles: RatingBubble[] }>({
    mutationFn: (body) =>
      api
        .post(`api/v1/transactions/${transactionId}/ratings`, { json: body })
        .json<Rating>(),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["ratings", "transaction", transactionId],
      });
    },
  });
}
