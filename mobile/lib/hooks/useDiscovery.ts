import { useInfiniteQuery, useQuery, InfiniteData } from "@tanstack/react-query";
import { api } from "../api";

export interface RankedListing {
  id: string;
  hostId: string;
  title: string;
  description: string;
  pricePerDay?: number;
  pricePerHour?: number;
  status: string;
  hasVideo: boolean;
  createdAt: string;
  hostName: string;
  hostReputation: number;
  distanceMeters: number;
  driveTimeMin: number;
  rankScore: number;
  lat: number;
  lng: number;
  thumbnailUrl: string;
}

interface FeedResponse {
  listings: RankedListing[];
  count: number;
}

export interface SearchFilters {
  maxDriveMin?: number;
  minPrice?: number;
  maxPrice?: number;
}

export interface MapBounds {
  swLat: number;
  swLng: number;
  neLat: number;
  neLng: number;
}

export interface HoldEstimate {
  holdAmount: number;
  itemValue: number;
  guaranteeGap: number;
}

export function useFeed(lat: number | null, lng: number | null) {
  return useInfiniteQuery<FeedResponse, Error, InfiniteData<FeedResponse>, string[], string>({
    queryKey: ["discovery", "feed", String(lat), String(lng)],
    queryFn: ({ pageParam }) =>
      api
        .get("api/v1/discovery/feed", {
          searchParams: {
            lat: lat!,
            lng: lng!,
            ...(pageParam ? { cursor: pageParam } : {}),
          },
        })
        .json<FeedResponse>(),
    initialPageParam: "",
    getNextPageParam: (lastPage) => {
      if (!lastPage.listings || lastPage.listings.length === 0) return undefined;
      return lastPage.listings[lastPage.listings.length - 1].id;
    },
    enabled: lat !== null && lng !== null,
  });
}

export function useSearch(
  query: string,
  lat: number | null,
  lng: number | null,
  filters: SearchFilters = {},
) {
  return useInfiniteQuery<FeedResponse, Error, InfiniteData<FeedResponse>, string[], string>({
    queryKey: [
      "discovery",
      "search",
      query,
      String(lat),
      String(lng),
      JSON.stringify(filters),
    ],
    queryFn: ({ pageParam }) =>
      api
        .get("api/v1/discovery/search", {
          searchParams: {
            q: query,
            lat: lat!,
            lng: lng!,
            ...(filters.maxDriveMin != null ? { max_drive_min: filters.maxDriveMin } : {}),
            ...(filters.minPrice != null ? { min_price: filters.minPrice } : {}),
            ...(filters.maxPrice != null ? { max_price: filters.maxPrice } : {}),
            ...(pageParam ? { cursor: pageParam } : {}),
          },
        })
        .json<FeedResponse>(),
    initialPageParam: "",
    getNextPageParam: (lastPage) => {
      if (!lastPage.listings || lastPage.listings.length === 0) return undefined;
      return lastPage.listings[lastPage.listings.length - 1].id;
    },
    enabled: query.length > 0 && lat !== null && lng !== null,
  });
}

export function useMapListings(bounds: MapBounds | null) {
  return useQuery<FeedResponse>({
    queryKey: ["discovery", "map", JSON.stringify(bounds)],
    queryFn: () =>
      api
        .get("api/v1/discovery/map", {
          searchParams: {
            sw_lat: bounds!.swLat,
            sw_lng: bounds!.swLng,
            ne_lat: bounds!.neLat,
            ne_lng: bounds!.neLng,
          },
        })
        .json<FeedResponse>(),
    enabled: bounds !== null,
  });
}

export function useHoldEstimate(listingId: string | null) {
  return useQuery<HoldEstimate>({
    queryKey: ["listings", listingId, "hold-estimate"],
    queryFn: () =>
      api.get(`api/v1/listings/${listingId}/hold-estimate`).json<HoldEstimate>(),
    enabled: listingId !== null,
  });
}
