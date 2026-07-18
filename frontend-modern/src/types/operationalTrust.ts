export type EvidenceCompleteness = 'complete' | 'partial' | 'unavailable';
export type EvidenceConfidence = 'confirmed' | 'inferred' | 'unknown';
export type EvidencePermissions = 'sufficient' | 'partial' | 'denied' | 'unknown';
export type EvidenceFreshness = 'fresh' | 'stale' | 'unknown';

export interface EvidenceSource {
  provider: string;
  collector: string;
  instance?: string;
}

export interface EvidenceSubject {
  resourceId?: string;
  providerRef?: string;
  providerScope?: string;
}

export interface EvidenceReason {
  code: string;
  message?: string;
}

export interface EvidencePayloadRef {
  kind: string;
  id: string;
}

export interface IdentityCorrelation {
  rule: string;
  matchedFields: Record<string, string>;
  candidateCount: number;
}

export interface EvidenceEnvelope {
  id: string;
  source: EvidenceSource;
  subject: EvidenceSubject;
  observedAt: string;
  ingestedAt: string;
  validUntil?: string;
  completeness: EvidenceCompleteness;
  confidence: EvidenceConfidence;
  reason?: EvidenceReason;
  permissions: EvidencePermissions;
  payloadRef?: EvidencePayloadRef;
  correlation?: IdentityCorrelation;
}

export type OperationalState =
  | 'observing'
  | 'open'
  | 'acknowledged'
  | 'suppressed'
  | 'resolving'
  | 'resolved'
  | 'stale'
  | 'unknown';

export type OperationalSeverity = 'info' | 'warning' | 'critical' | 'unknown';

export interface OperationalAcknowledgement {
  at: string;
  by: string;
  note?: string;
}

export interface OperationalSuppression {
  at: string;
  by: string;
  reason: string;
  expiresAt?: string;
}

export interface OperationalRecord {
  id: string;
  canonicalSpecId: string;
  subjectResourceId: string;
  state: OperationalState;
  severity: OperationalSeverity;
  firstObservedAt: string;
  lastObservedAt: string;
  stateChangedAt: string;
  resolvedAt?: string;
  acknowledgement?: OperationalAcknowledgement;
  suppression?: OperationalSuppression;
  evidenceIds: string[];
  causeKey: string;
  relatedResourceIds: string[];
  impactSummary?: string;
  recommendedNextStep?: string;
}

export type TransitionCause =
  | 'detector_decision'
  | 'acknowledgement'
  | 'unacknowledgement'
  | 'suppression'
  | 'suppression_expired'
  | 'recovery_evidence'
  | 'collection_stale'
  | 'collection_unknown';

export interface LifecycleTransition {
  id: string;
  operationalRecordId: string;
  from: OperationalState;
  to: OperationalState;
  at: string;
  cause: TransitionCause;
  causeKey: string;
  evidenceIds: string[];
  reason?: string;
}

export type NotificationDeliveryState =
  'queued' | 'delivering' | 'delivered' | 'retrying' | 'cancelled' | 'failed' | 'dead_letter';

export interface NotificationLink {
  notificationId: string;
  operationalRecordId: string;
  transitionId: string;
  lifecycleState: OperationalState;
  causeKey: string;
  destinationId: string;
  deliveryState: NotificationDeliveryState;
  attemptedAt?: string;
  completedAt?: string;
}
