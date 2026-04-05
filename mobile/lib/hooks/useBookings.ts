import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { Booking } from "./useBooking";

interface BookingsResponse {
  bookings: Booking[];
  total: number;
}

/** All bookings where the current user is the renter. */
export function useRenterBookings() {
  return useQuery<BookingsResponse>({
    queryKey: ["bookings", "renter"],
    queryFn: () =>
      api.get("api/v1/users/me/bookings").json<BookingsResponse>(),
  });
}

/** All bookings where the current user is the host. */
export function useHostBookings() {
  return useQuery<BookingsResponse>({
    queryKey: ["bookings", "host"],
    queryFn: () =>
      api.get("api/v1/users/me/hosted-bookings").json<BookingsResponse>(),
  });
}
