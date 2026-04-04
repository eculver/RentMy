import { useQuery } from "@tanstack/react-query";
import { api } from "../api";

export interface Listing {
  id: string;
  hostId: string;
  title: string;
  description: string;
  pricePerHour?: number;
  pricePerDay?: number;
  status: "PENDING" | "ACTIVE" | "FLAGGED" | "SUSPENDED";
  createdAt: string;
  thumbnailUrl?: string;
}

interface ListingsResponse {
  listings: Listing[];
  total: number;
  page: number;
}

export function useMyListings(page = 1, limit = 20) {
  return useQuery<ListingsResponse>({
    queryKey: ["listings", "mine", page],
    queryFn: () =>
      api.get("api/v1/users/me/listings", { searchParams: { page, limit } }).json<ListingsResponse>(),
  });
}
