import type { EvidenceEnvelope, EvidencePermissions } from '@/types/operationalTrust';

export type RecoveryPlatform = string;
export type RecoveryKind = 'snapshot' | 'backup' | 'other' | (string & {});
export type RecoveryMode = 'snapshot' | 'local' | 'remote' | (string & {});
export type RecoveryOutcome =
  'success' | 'warning' | 'failed' | 'running' | 'unknown' | (string & {});

export interface RecoveryExternalRef {
  type: string;
  namespace?: string;
  name?: string;
  uid?: string;
  id?: string;
  class?: string;
  extra?: Record<string, string>;
}

export interface RecoveryPointDisplay {
  itemLabel?: string;
  itemType?: string;
  isWorkload?: boolean;
  clusterLabel?: string;
  nodeHostLabel?: string;
  nodeAgentLabel?: string;
  namespaceLabel?: string;
  entityIdLabel?: string;
  repositoryLabel?: string;
  detailsSummary?: string;
}

// The backend display names the item label and type subjectLabel and
// subjectType. recoveryPlatformModel folds them onto itemLabel and itemType,
// so normalized points never carry them.
export interface RecoveryPointDisplayTransport extends RecoveryPointDisplay {
  subjectLabel?: string;
  subjectType?: string;
}

export interface RecoveryPoint {
  id: string;
  platform?: RecoveryPlatform;
  kind: RecoveryKind;
  mode: RecoveryMode;
  outcome: RecoveryOutcome;

  // Optional dimensions used for filtering and display.
  entityId?: string | null;
  cluster?: string | null;
  node?: string | null;
  namespace?: string | null;

  startedAt?: string | null;
  completedAt?: string | null;

  sizeBytes?: number | null;
  verified?: boolean | null;
  encrypted?: boolean | null;
  immutable?: boolean | null;

  itemResourceId?: string;
  repositoryResourceId?: string;
  providerScope?: string;
  evidence?: EvidenceEnvelope | null;
  itemRef?: RecoveryExternalRef | null;
  repositoryRef?: RecoveryExternalRef | null;
  details?: Record<string, unknown> | null;

  display?: RecoveryPointDisplay | null;
}

export interface RecoveryPointTransport extends RecoveryPoint {
  display?: RecoveryPointDisplayTransport | null;
  provider?: RecoveryPlatform;
  subjectResourceId?: string;
  subjectRef?: RecoveryExternalRef | null;
}

export interface RecoveryPointsResponse {
  data: RecoveryPoint[];
}

export interface RecoveryPointsTransportResponse {
  data: RecoveryPointTransport[];
}

export type ProtectionState = 'protected' | 'attention' | 'unprotected' | 'unknown';
export type ProtectionFreshness = 'current' | 'stale' | 'unknown';
export type ProtectionVerification = 'verified' | 'unverified' | 'stale' | 'unknown';
export type ProtectionCoverage = 'complete' | 'partial' | 'none' | 'unknown';
export type ProtectionHistoryCompleteness = 'complete' | 'partial' | 'unavailable' | 'unknown';

export interface ProtectionProviderState {
  provider: RecoveryPlatform;
  source: string;
  scope: string;
  jobState: RecoveryOutcome;
  historyCompleteness: ProtectionHistoryCompleteness;
  permissions: EvidencePermissions;
  lastAttemptAt?: string | null;
  lastSuccessAt?: string | null;
  lastVerifiedAt?: string | null;
  evidenceIds: string[];
  verificationExpected?: boolean;
}

export interface ProtectionPosture {
  subjectResourceId: string;
  state: ProtectionState;
  lastAttemptAt?: string | null;
  lastSuccessfulPointAt?: string | null;
  lastVerifiedAt?: string | null;
  freshness: ProtectionFreshness;
  verification: ProtectionVerification;
  coverage: ProtectionCoverage;
  providerStates: ProtectionProviderState[];
  repositoryResourceIds: string[];
  evidenceIds: string[];
  explanation: string;
  evaluatedAt: string;
}

export interface ProtectionPosturePolicy {
  freshnessWindowSeconds: number;
  verificationWindowSeconds: number;
  requireVerification: boolean;
}

export interface ProtectionPosturesResponse {
  data: ProtectionPosture[];
  policy: ProtectionPosturePolicy;
}
