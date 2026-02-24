export type RecoveryProvider = string;
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
  subjectLabel?: string;
  subjectType?: string;
  isWorkload?: boolean;
  clusterLabel?: string;
  nodeHostLabel?: string;
  namespaceLabel?: string;
  entityIdLabel?: string;
  repositoryLabel?: string;
  detailsSummary?: string;
}

export interface RecoveryPoint {
  id: string;
  provider: RecoveryProvider;
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

  subjectResourceId?: string;
  repositoryResourceId?: string;
  subjectRef?: RecoveryExternalRef | null;
  repositoryRef?: RecoveryExternalRef | null;
  details?: Record<string, unknown> | null;

  display?: RecoveryPointDisplay | null;
}

export interface RecoveryPointsResponse {
  data: RecoveryPoint[];
  meta: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
  };
}

export interface ProtectionRollup {
  rollupId: string;
  subjectResourceId?: string;
  subjectRef?: RecoveryExternalRef | null;

  lastAttemptAt?: string | null;
  lastSuccessAt?: string | null;
  lastOutcome: RecoveryOutcome;

  providers?: RecoveryProvider[];
}

export interface RecoveryRollupsResponse {
  data: ProtectionRollup[];
  meta: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
  };
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
  nodesHosts?: string[];
  namespaces?: string[];
  hasSize?: boolean;
  hasVerification?: boolean;
  hasEntityId?: boolean;
}

export interface RecoveryPointsFacetsResponse {
  data: RecoveryPointsFacets;
}
