import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { usePusher } from "./usePusher";

export type BookingStatus =
  | "REQUESTED"
  | "ACCEPTED"
  | "DECLINED"
  | "AUTO_DECLINED"
  | "ACTIVE"
  | "COMPLETED"
  | "DISPUTED"
  | "CANCELLED";

export interface Booking {
  id: string;
  renterId: string;
  hostId: string;
  listingId: string;
  scheduledStart: string; // ISO 8601
  scheduledEnd: string;
  status: BookingStatus;
  cancelledBy?: string;
  cancellationFee?: number; // cents
  actualStart?: string;
  actualEnd?: string;
  createdAt: string;
}

interface BookingResponse {
  booking: Booking;
}

/**
 * Fetches a single booking by ID.
 * Subscribes to the Pusher channel for real-time status updates —
 * any `booking-status-changed` event invalidates the query.
 */
export function useBooking(id: string | null) {
  const queryClient = useQueryClient();

  usePusher(
    id ? `private-transaction-${id}` : null,
    "booking-status-changed",
    () => {
      void queryClient.invalidateQueries({ queryKey: ["bookings", id] });
    },
  );

  return useQuery<BookingResponse>({
    queryKey: ["bookings", id],
    queryFn: () =>
      api.get(`api/v1/bookings/${id}`).json<BookingResponse>(),
    enabled: id !== null,
  });
}
