import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

export type AppraisalStatus = "PENDING" | "COMPLETE" | "FAILED";

export interface AppraisalResult {
  id: string;
  listingId: string;
  status: AppraisalStatus;
  itemName?: string;
  category?: string;
  condition?: string;
  estimatedValueCents?: number;
  suggestedPricePerHourCents?: number;
  suggestedPricePerDayCents?: number;
  description?: string;
  tags?: string[];
  confidence?: number;
  overrideApproved?: boolean;
  overrideReasoning?: string;
  failureReason?: string;
}

export interface OverrideResult {
  approved: boolean;
  reasoning: string;
  confidence: number;
}

export function useAppraisal(listingId: string | null) {
  return useQuery<AppraisalResult>({
    queryKey: ["appraisal", listingId],
    queryFn: () =>
      api.get(`api/v1/listings/${listingId}/appraisal`).json<AppraisalResult>(),
    enabled: !!listingId,
    refetchInterval: (query) => {
      const data = query.state.data;
      if (!data || data.status === "PENDING") return 2000;
      return false;
    },
  });
}

export function useOverride(listingId: string) {
  const queryClient = useQueryClient();
  return useMutation<
    OverrideResult,
    Error,
    { declaredValueCents: number; justification: string }
  >({
    mutationFn: (body) =>
      api
        .post(`api/v1/listings/${listingId}/override`, { json: body })
        .json<OverrideResult>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["appraisal", listingId] });
    },
  });
}
