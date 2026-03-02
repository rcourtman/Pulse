// Types for cluster agent deployment, mirroring backend Go structs
// from internal/deploy/types.go and internal/api/deploy_handlers.go.

/** Lifecycle state of a deployment job. */
export type DeployJobStatus =
  | 'queued'
  | 'waiting_source'
  | 'running'
  | 'succeeded'
  | 'partial_success'
  | 'failed'
  | 'canceling'
  | 'canceled';

/** Lifecycle state of a single deployment target node. */
export type DeployTargetStatus =
  | 'pending'
  | 'preflighting'
  | 'ready'
  | 'installing'
  | 'enrolling'
  | 'verifying'
  | 'succeeded'
  | 'failed_retryable'
  | 'failed_permanent'
  | 'skipped_already_agent'
  | 'skipped_license'
  | 'canceled';

/** Event type classification for deployment audit log entries. */
export type DeployEventType =
  | 'job_created'
  | 'job_status_changed'
  | 'target_status_changed'
  | 'preflight_result'
  | 'install_output'
  | 'enroll_complete'
  | 'error'
  | 'job_complete';

// --- API response types ---

/** A candidate node in the cluster for agent deployment. */
export interface CandidateNode {
  nodeId: string;
  name: string;
  ip: string;
  hasAgent: boolean;
  deployable: boolean;
  reason?: string;
}

/** A connected agent that can act as a deployment source (SSH to peers). */
export interface SourceAgentInfo {
  agentId: string;
  nodeId: string;
  online: boolean;
}

/** Response from GET /api/clusters/{clusterId}/agent-deploy/candidates */
export interface CandidatesResponse {
  clusterId: string;
  clusterName: string;
  sourceAgents: SourceAgentInfo[];
  nodes: CandidateNode[];
}

/** A single target node within a deploy job. */
export interface DeployTarget {
  id: string;
  jobId: string;
  nodeId: string;
  nodeName: string;
  nodeIP: string;
  arch?: string;
  status: DeployTargetStatus;
  errorMessage?: string;
  attempts: number;
  createdAt: string;
  updatedAt: string;
}

/** A deploy or preflight job with its targets. */
export interface DeployJob {
  id: string;
  clusterId: string;
  clusterName: string;
  sourceAgentId: string;
  status: DeployJobStatus;
  targets: DeployTarget[];
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

/** An immutable audit log entry for a deployment lifecycle. */
export interface DeployEvent {
  id: string;
  jobId: string;
  targetId?: string;
  type: DeployEventType;
  message: string;
  data?: string;
  createdAt: string;
}

// --- Request/Response types ---

export interface CreatePreflightRequest {
  sourceAgentId: string;
  targetNodeIds: string[];
  maxParallel?: number;
}

/** Response from POST /api/clusters/{clusterId}/agent-deploy/preflights */
export interface CreatePreflightResponse {
  preflightId: string;
  status: string;
  eventsUrl: string;
}

export interface CreateJobRequest {
  sourceAgentId: string;
  preflightId: string;
  targetNodeIds: string[];
  mode?: string;
  maxParallel?: number;
  retryPolicy?: { maxAttempts: number };
}

export interface SkippedTarget {
  nodeId: string;
  reason: string;
}

/** Response from POST /api/clusters/{clusterId}/agent-deploy/jobs */
export interface CreateJobResponse {
  jobId: string;
  acceptedTargets: string[];
  skippedTargets: SkippedTarget[];
  reservedLicenseSlots: number;
  eventsUrl: string;
}

/** Response from POST /api/agent-deploy/jobs/{jobId}/retry */
export interface RetryJobResponse {
  jobId: string;
  retryTargets: number;
  status: string;
  eventsUrl: string;
}
