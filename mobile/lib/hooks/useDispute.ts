import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

// Matches backend dispute.Status constants exactly.
export type DisputeStatus =
  | "PENDING"
  | "GATHERING"
  | "ANALYZING"
  | "AUTO_RESOLVED"
  | "AUDIT_QUEUED"
  | "HUMAN_REVIEW"
  | "RESOLVED"
  | "INCONCLUSIVE";

// Terminal statuses — no further agent/human processing will happen automatically.
export const TERMINAL_STATUSES: DisputeStatus[] = [
  "RESOLVED",
  "AUTO_RESOLVED",
  "AUDIT_QUEUED",
  "INCONCLUSIVE",
];

// Statuses where the dispute is effectively closed (decision made).
export const CLOSED_STATUSES: DisputeStatus[] = [
  "RESOLVED",
  "AUTO_RESOLVED",
  "AUDIT_QUEUED",
];

export type EscalationRoute =
  | "AUTO_RESOLVE"
  | "AUTO_RESOLVE_AUDIT"
  | "HUMAN_REVIEW";

export type DisputeReason =
  | "DAMAGE"
  | "MISSING_ITEM"
  | "OTHER";

// Matches the EvidencePackage shape returned by the backend.
export interface DisputeEvidence {
  photoDiffResult?: "NO_DAMAGE" | "MINOR_DAMAGE" | "MAJOR_DAMAGE" | "INCONCLUSIVE";
  photoDiffConfidence?: number;
  checkInMedia?: Array<{ id: string; url: string }>;
  checkOutMedia?: Array<{ id: string; url: string }>;
  [key: string]: unknown;
}

export interface Dispute {
  id: string;
  transactionId: string;
  reporterId: string;
  reason: DisputeReason;
  description: string;
  status: DisputeStatus;
  // Backend JSON tags after BUG-3 fix:
  escalationRoute: EscalationRoute | null;
  agentDecisionId: string | null;
  agentConfidence: number | null;
  damageChargeCents: number | null;
  resolvedBy: string | null;
  reviewerNotes: string | null;
  slaDeadline: string | null;
  evidence: DisputeEvidence | null;
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
        (d) => !TERMINAL_STATUSES.includes(d.status),
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
      return TERMINAL_STATUSES.includes(dispute.status) ? false : 15_000;
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
