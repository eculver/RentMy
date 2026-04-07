// ── Ops metrics ──

export interface MetricValue {
  name: string
  value: number
  previousValue: number
  trend: 'up' | 'down' | 'flat'
  period: string
}

export interface BusinessMetrics {
  activeListings: MetricValue
  activeUsers: MetricValue
  bookingConversionRate: MetricValue
  grossRevenueCents: MetricValue
  netRevenueCents: MetricValue
  avgTransactionCents: MetricValue
  hostPayoutVelocityHours: MetricValue
}

export interface TrustMetrics {
  fraudFlagRate: MetricValue
  disputeRate: MetricValue
  avgAgentConfidence: MetricValue
  collusionAlertCount: MetricValue
}

export interface SupplyMetrics {
  newHostSignups7d: MetricValue
  hostChurnRate: MetricValue
  avgResponseRate: MetricValue
}

export interface DemandMetrics {
  repeatRenterRate: MetricValue
  failedBookingRate: MetricValue
}

export interface HealthSnapshot {
  id: string
  business: BusinessMetrics
  trust: TrustMetrics
  supply: SupplyMetrics
  demand: DemandMetrics
  anomalies: string[]
  capturedAt: string
}

// ── Alerts ──

export type Severity = 'INFO' | 'WARNING' | 'CRITICAL'
export type Channel = 'SLACK' | 'PAGERDUTY' | 'BOTH'
export type Operator = 'GT' | 'LT' | 'DEVIATION'

export interface AlertRule {
  id: string
  metricName: string
  operator: Operator
  threshold: number
  severity: Severity
  channel: Channel
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface Alert {
  id: string
  ruleId: string
  metricName: string
  currentValue: number
  threshold: number
  severity: Severity
  channel: Channel
  firedAt: string
  acknowledgedAt?: string
  acknowledgedBy?: string
}

// ── Fraud ──

export type SignalType =
  | 'DEVICE_FINGERPRINT'
  | 'PAYMENT_INSTRUMENT'
  | 'CARRIER_BATCH'
  | 'SIMULTANEOUS_CREATION'
  | 'EXCLUSIVE_PAIR'
  | 'WIFI_NETWORK'
  | 'DAMAGE_PATTERN'
  | 'VALUE_SPIKE'

export type FraudAction = 'MONITOR' | 'FLAG' | 'SUSPEND'

export interface FraudSignal {
  type: SignalType
  userId: string
  relatedUserId?: string
  score: number
  isCompoundOnly: boolean
  evidence?: unknown
  detectedAt: string
}

export interface FraudFlag {
  id: string
  userId: string
  signals: FraudSignal[]
  totalScore: number
  action: FraudAction
  agentDecisionId?: string
  resolvedAt?: string
  resolvedBy?: string
  resolutionNotes?: string
  createdAt: string
}

// ── Agent decisions ──

export interface AgentDecision {
  id: string
  agentType: string
  transactionId: string
  confidence: number
  escalated: boolean
  outcomeCorrect?: boolean
  overrideOf?: string
  input?: unknown
  decision?: unknown
  model?: string
  promptVersion?: string
  reasoning?: string
  createdAt: string
}

// ── Calibration ──

export interface CalibrationBucket {
  bucketMin: number
  bucketMax: number
  decisionCount: number
  correctCount: number
  expectedAccuracy: number
  actualAccuracy: number
}

// ── Referrals ──

export type ReferralStatus =
  | 'PENDING'
  | 'SIGNED_UP'
  | 'FIRST_RENTAL_COMPLETED'
  | 'PAID'
  | 'FRAUDULENT'

export interface Referral {
  id: string
  referrerName: string
  refereeName: string
  code: string
  status: ReferralStatus
  createdAt: string
  completedAt?: string
  payoutAmount?: number
}
