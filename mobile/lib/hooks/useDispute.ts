import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

export type DisputeStatus =
  | "PENDING"
  | "EVIDENCE_GATHERING"
  | "UNDER_REVIEW"
  | "RESOLVED"
  | "CLOSED";

export type EscalationRoute =
  | "AUTO_RESOLVE"
  | "AUTO_RESOLVE_AUDIT"
  | "HUMAN_REVIEW";

export type DisputeReason =
  | "DAMAGE"
  | "MISSING_ITEM"
  | "OTHER";

export interface Dispute {
  id: string;
  transactionId: string;
  reporterId: string;
  reason: DisputeReason;
  description: string;
  evidenceRefs: string[];
  status: DisputeStatus;
  escalationRoute: EscalationRoute | null;
  agentDecision: string | null;
  agentConfidence: number | null;
  damageChargeCents: number | null;
  resolvedBy: string | null;
  slaDeadline: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface FileDisputeInput {
  reason: DisputeReason;
  description: string;
  evidenceRefs?: string[];
}

/** Fetches all disputes for a transaction. */
export function useTransactionDisputes(transactionId: string | null) {
  return useQuery<Dispute[]>({
    queryKey: ["disputes", "transaction", transactionId],
    queryFn: () =>
      api
        .get(`api/v1/transactions/${transactionId}/disputes`)
        .json<Dispute[]>(),
    enabled: !!transactionId,
    refetchInterval: (query) => {
      const disputes = query.state.data;
      if (!disputes) return false;
      const hasOpenDispute = disputes.some(
        (d) => d.status !== "RESOLVED" && d.status !== "CLOSED",
      );
      return hasOpenDispute ? 15_000 : false;
    },
  });
}

/** Fetches a single dispute by ID. */
export function useDispute(disputeId: string | null) {
  return useQuery<Dispute>({
    queryKey: ["disputes", disputeId],
    queryFn: () =>
      api.get(`api/v1/disputes/${disputeId}`).json<Dispute>(),
    enabled: !!disputeId,
    refetchInterval: (query) => {
      const dispute = query.state.data;
      if (!dispute) return false;
      return dispute.status !== "RESOLVED" && dispute.status !== "CLOSED"
        ? 15_000
        : false;
    },
  });
}

/** Files a new dispute for a transaction. */
export function useFileDispute(transactionId: string) {
  const queryClient = useQueryClient();

  return useMutation<Dispute, Error, FileDisputeInput>({
    mutationFn: (body) =>
      api
        .post(`api/v1/transactions/${transactionId}/disputes`, { json: body })
        .json<Dispute>(),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["disputes", "transaction", transactionId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["bookings", transactionId],
      });
    },
  });
}
