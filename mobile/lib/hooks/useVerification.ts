import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "../api";
import { IdentityStatus } from "../auth";

interface StartVerificationResponse {
  sessionUrl: string;
  sessionId: string;
  ephemeralKeySecret?: string; // Stripe client_secret; present on new sessions, absent on idempotent returns
}

interface VerificationStatusResponse {
  status: IdentityStatus;
  updatedAt: string;
}

/**
 * Mutation to start a KYC verification session.
 * Returns sessionUrl (for Stripe Identity sheet) and sessionId.
 */
export function useStartVerification() {
  return useMutation<StartVerificationResponse, Error>({
    mutationFn: () =>
      api.post("api/v1/verification/start").json<StartVerificationResponse>(),
  });
}

/**
 * Query that polls verification status every 3 seconds while enabled.
 * Caller sets `enabled: true` during the Stripe Identity flow to receive
 * webhook-driven status updates as soon as they arrive.
 */
export function useVerificationStatus(enabled: boolean) {
  return useQuery<VerificationStatusResponse>({
    queryKey: ["verification", "status"],
    queryFn: () =>
      api
        .get("api/v1/verification/status")
        .json<VerificationStatusResponse>(),
    enabled,
    refetchInterval: enabled ? 3000 : false,
    staleTime: 0,
  });
}
