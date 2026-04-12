import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useBooking } from "./useBooking";
import { useTransactionDisputes } from "./useDispute";
import { useTransactionRatings } from "./useRatings";
import { usePusher } from "./usePusher";

export type PostRentalStep =
  | "idle"
  | "return_confirmed"
  | "rate_prompt"
  | "hold_releasing"
  | "done";

export interface PostRentalState {
  step: PostRentalStep;
  hasOpenDispute: boolean;
  hasRated: boolean;
  holdReleased: boolean;
  showRatingPrompt: boolean;
  dismissRatingPrompt: () => void;
}

/**
 * Orchestrates the post-rental flow for a completed transaction.
 *
 * Detects transaction completion, tracks rating and dispute state,
 * and determines which UI elements should be shown.
 */
export function usePostRental(
  transactionId: string | null,
  currentUserId: string | null,
): PostRentalState {
  const queryClient = useQueryClient();
  const [showRatingPrompt, setShowRatingPrompt] = useState(false);

  const { data: bookingData } = useBooking(transactionId);
  const { data: disputes } = useTransactionDisputes(transactionId);
  const { data: ratings } = useTransactionRatings(transactionId);

  const booking = bookingData?.booking ?? null;
  const isCompleted = booking?.status === "COMPLETED";
  const isDisputed = booking?.status === "DISPUTED";

  const hasOpenDispute =
    (disputes ?? []).some(
      (d) => d.status !== "RESOLVED" && d.status !== "CLOSED",
    ) || isDisputed;

  const hasRated =
    (ratings ?? []).some((r) => r.fromUserId === currentUserId) ?? false;

  // Show rating prompt once the booking reaches COMPLETED and user hasn't rated yet
  useEffect(() => {
    if (isCompleted && !hasRated && !hasOpenDispute) {
      setShowRatingPrompt(true);
    }
  }, [isCompleted, hasRated, hasOpenDispute]);

  // Listen for hold-released push events to refresh booking data
  usePusher(
    transactionId ? `private-transaction-${transactionId}` : null,
    "hold-released",
    () => {
      void queryClient.invalidateQueries({
        queryKey: ["bookings", transactionId],
      });
    },
  );

  // Listen for damage-detected events to refresh disputes
  usePusher(
    transactionId ? `private-transaction-${transactionId}` : null,
    "damage-detected",
    () => {
      void queryClient.invalidateQueries({
        queryKey: ["disputes", "transaction", transactionId],
      });
    },
  );

  let step: PostRentalStep = "idle";
  if (isCompleted || isDisputed) {
    if (hasOpenDispute) {
      step = "return_confirmed";
    } else if (!hasRated) {
      step = "rate_prompt";
    } else {
      step = "done";
    }
  }

  return {
    step,
    hasOpenDispute,
    hasRated,
    holdReleased: isCompleted && !hasOpenDispute,
    showRatingPrompt,
    dismissRatingPrompt: () => setShowRatingPrompt(false),
  };
}
