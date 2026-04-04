import { useQuery } from "@tanstack/react-query";
import { api } from "../api";

export interface ListingDetail {
  id: string;
  hostId: string;
  title: string;
  description: string;
  pricePerHour?: number;
  pricePerDay?: number;
  minDuration?: string;
  maxDuration?: string;
  location?: { lat: number; lng: number };
  availability: unknown;
  hasVideo: boolean;
  status: "PENDING" | "ACTIVE" | "FLAGGED" | "SUSPENDED";
  createdAt: string;
  estimatedValue?: number;
  hostDeclaredValue?: number;
  aiGeneratedTags?: string[];
}

export function useListing(id: string | null) {
  return useQuery<{ listing: ListingDetail }>({
    queryKey: ["listings", id],
    queryFn: () =>
      api.get(`api/v1/listings/${id}`).json<{ listing: ListingDetail }>(),
    enabled: id !== null && id !== "",
  });
}
