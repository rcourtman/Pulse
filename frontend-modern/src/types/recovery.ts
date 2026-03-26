export type RecoveryPlatform = string;
export type RecoveryKind = 'snapshot' | 'backup' | 'other' | (string & {});
export type RecoveryMode = 'snapshot' | 'local' | 'remote' | (string & {});
export type RecoveryOutcome =
  | 'success'
  | 'warning'
  | 'failed'
  | 'running'
  | 'unknown'
  | (string & {});

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
  subjectRef?: RecoveryExternalRef | null;
  repositoryRef?: RecoveryExternalRef | null;
  details?: Record<string, unknown> | null;

  display?: RecoveryPointDisplay | null;
}

export interface RecoveryPointTransport extends RecoveryPoint {
  display?: RecoveryPointDisplayTransport | null;
  provider?: RecoveryPlatform;
  subjectResourceId?: string;
}

export interface RecoveryResponseMeta {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

export interface RecoveryPointsResponse {
  data: RecoveryPoint[];
  meta: RecoveryResponseMeta;
}

export interface RecoveryPointsTransportResponse {
  data: RecoveryPointTransport[];
  meta: RecoveryResponseMeta;
}

export interface ProtectionRollup {
  rollupId: string;
  itemResourceId?: string;
  subjectRef?: RecoveryExternalRef | null;
  display?: RecoveryPointDisplay | null;

  lastAttemptAt?: string | null;
  lastSuccessAt?: string | null;
  lastOutcome: RecoveryOutcome;

  platforms?: RecoveryPlatform[];
}

export interface ProtectionRollupTransport extends ProtectionRollup {
  display?: RecoveryPointDisplayTransport | null;
  providers?: RecoveryPlatform[];
  subjectResourceId?: string;
}

export interface RecoveryRollupsResponse {
  data: ProtectionRollup[];
  meta: RecoveryResponseMeta;
}

export interface RecoveryRollupsTransportResponse {
  data: ProtectionRollupTransport[];
  meta: RecoveryResponseMeta;
}

export interface RecoveryPointsSeriesBucket {
  day: string; // YYYY-MM-DD (client timezone)
  total: number;
  snapshot: number;
  local: number;
  remote: number;
}

export interface RecoveryPointsSeriesResponse {
  data: RecoveryPointsSeriesBucket[];
}

export interface RecoveryPointsFacets {
  clusters?: string[];
  nodesAgents?: string[];
  namespaces?: string[];
  itemTypes?: string[];
  hasSize?: boolean;
  hasVerification?: boolean;
  hasEntityId?: boolean;
}

export interface RecoveryPointsFacetsResponse {
  data: RecoveryPointsFacets;
}
