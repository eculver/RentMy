/**
 * Push notification type definitions and handlers for post-rental events.
 *
 * These types align with the notification payloads emitted by the backend
 * River workers (DisputeResolutionWorker, LateReturnWorker, etc.).
 */

/** All push notification types sent by the backend. */
export type NotificationType =
  // Post-rental flow
  | "RATING_PROMPT"
  | "HOLD_RELEASED"
  | "DAMAGE_DETECTED"
  | "DISPUTE_FILED"
  | "DISPUTE_RESOLVED"
  | "PHOTO_REPROMPT"
  // Booking lifecycle
  | "BOOKING_ACCEPTED"
  | "BOOKING_DECLINED"
  | "BOOKING_CANCELLED"
  // Late return
  | "LATE_RETURN_WARNING"
  | "LATE_FEE_CHARGED"
  // Messages
  | "NEW_MESSAGE";

export interface RatingPromptPayload {
  type: "RATING_PROMPT";
  transactionId: string;
  counterpartyName: string;
}

export interface HoldReleasedPayload {
  type: "HOLD_RELEASED";
  transactionId: string;
  releasedAmountCents: number;
}

export interface DamageDetectedPayload {
  type: "DAMAGE_DETECTED";
  transactionId: string;
  disputeId: string;
  classification: string;
  confidence: number;
}

export interface DisputeFiledPayload {
  type: "DISPUTE_FILED";
  transactionId: string;
  disputeId: string;
  reason: string;
}

export interface DisputeResolvedPayload {
  type: "DISPUTE_RESOLVED";
  transactionId: string;
  disputeId: string;
  decision: string;
  damageChargeCents: number | null;
}

export interface PhotoRepromptPayload {
  type: "PHOTO_REPROMPT";
  transactionId: string;
  disputeId: string;
  expiresAt: string;
}

export type NotificationPayload =
  | RatingPromptPayload
  | HoldReleasedPayload
  | DamageDetectedPayload
  | DisputeFiledPayload
  | DisputeResolvedPayload
  | PhotoRepromptPayload;

/** Route map: notification type → screen pathname */
export const NOTIFICATION_ROUTES: Partial<Record<NotificationType, string>> = {
  RATING_PROMPT: "/(tabs)/(rentals)/rate",
  HOLD_RELEASED: "/(tabs)/(rentals)/return-confirmation",
  DAMAGE_DETECTED: "/(tabs)/(rentals)/dispute-status",
  DISPUTE_FILED: "/(tabs)/(rentals)/dispute-status",
  DISPUTE_RESOLVED: "/(tabs)/(rentals)/dispute-status",
  PHOTO_REPROMPT: "/(tabs)/(rentals)/dispute-status",
  BOOKING_ACCEPTED: "/(tabs)/(feed)/booking-status",
  BOOKING_DECLINED: "/(tabs)/(feed)/booking-status",
  BOOKING_CANCELLED: "/(tabs)/(feed)/booking-status",
  LATE_RETURN_WARNING: "/(tabs)/(feed)/active-rental",
  LATE_FEE_CHARGED: "/(tabs)/(feed)/active-rental",
  NEW_MESSAGE: "/(tabs)/(messages)",
};

/**
 * Returns the Expo Router params for a notification payload.
 * The caller is responsible for navigating to NOTIFICATION_ROUTES[payload.type].
 */
export function getNotificationParams(
  payload: NotificationPayload,
): Record<string, string> {
  switch (payload.type) {
    case "RATING_PROMPT":
      return { transactionId: payload.transactionId };
    case "HOLD_RELEASED":
      return { transactionId: payload.transactionId };
    case "DAMAGE_DETECTED":
      return { transactionId: payload.transactionId, disputeId: payload.disputeId };
    case "DISPUTE_FILED":
      return { transactionId: payload.transactionId, disputeId: payload.disputeId };
    case "DISPUTE_RESOLVED":
      return { transactionId: payload.transactionId, disputeId: payload.disputeId };
    case "PHOTO_REPROMPT":
      return { transactionId: payload.transactionId, disputeId: payload.disputeId };
  }
}
