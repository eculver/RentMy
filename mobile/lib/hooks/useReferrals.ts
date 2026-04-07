import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

export interface ReferralCode {
  id: string;
  code: string;
  userId: string;
  expiresAt?: string;
  maxUses: number;
  useCount: number;
  createdAt: string;
}

export interface Referral {
  id: string;
  referralCodeId: string;
  referrerId: string;
  refereeId: string;
  status: "SIGNED_UP" | "FIRST_RENTAL_COMPLETED" | "PAID" | "FRAUDULENT";
  referrerPayout: number;
  refereePayout: number;
  completedAt?: string;
  paidAt?: string;
  createdAt: string;
}

interface ReferralsResponse {
  referrals: Referral[];
  page: number;
  limit: number;
}

/**
 * Returns the caller's referral code, auto-generating one if none exists.
 */
export function useReferralCode() {
  const queryClient = useQueryClient();

  const query = useQuery<ReferralCode>({
    queryKey: ["referral", "code"],
    queryFn: async () => {
      try {
        return await api.get("api/v1/referrals/code").json<ReferralCode>();
      } catch {
        // Auto-generate if not found.
        const created = await api
          .post("api/v1/referrals/code")
          .json<ReferralCode>();
        queryClient.setQueryData(["referral", "code"], created);
        return created;
      }
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  return query;
}

/**
 * Returns the referrals where the authenticated user is the referrer.
 */
export function useMyReferrals(page = 1, limit = 20) {
  return useQuery<ReferralsResponse>({
    queryKey: ["referral", "mine", page],
    queryFn: () =>
      api
        .get("api/v1/referrals/mine", { searchParams: { page, limit } })
        .json<ReferralsResponse>(),
  });
}

/**
 * Applies a referral code for the authenticated user.
 */
export function useApplyReferralCode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (code: string) =>
      api
        .post("api/v1/referrals/apply", { json: { code } })
        .json<Referral>(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["referral"] });
    },
  });
}
