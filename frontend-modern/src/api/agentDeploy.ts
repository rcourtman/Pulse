import { apiFetchJSON } from '@/utils/apiClient';

/**
 * Cluster agent deploy: Pulse installs its agent on uncovered Proxmox cluster
 * members by asking a member that already runs the agent to install it on its
 * peers over the cluster's own SSH trust. The server mints a per-node bootstrap
 * credential for each target, so no token is ever shown to, copied by, or
 * reused across operators or hosts.
 */

export type ClusterDeployCandidateReason = 'already_agent' | 'no_address' | (string & {});

export interface ClusterDeployCandidateNode {
  nodeId: string;
  name: string;
  ip?: string;
  hasAgent: boolean;
  deployable: boolean;
  reason?: ClusterDeployCandidateReason;
}

export interface ClusterDeploySourceAgent {
  agentId: string;
  nodeId: string;
  online: boolean;
}

export interface ClusterDeployCandidates {
  clusterId: string;
  clusterName: string;
  sourceAgents: ClusterDeploySourceAgent[] | null;
  nodes: ClusterDeployCandidateNode[] | null;
}

export type ClusterDeployJobStatus =
  | 'queued'
  | 'waiting_source'
  | 'running'
  | 'succeeded'
  | 'partial_success'
  | 'failed'
  | 'canceling'
  | 'canceled';

export type ClusterDeployTargetStatus =
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
  | 'canceled';

export interface ClusterDeployTarget {
  id: string;
  nodeId: string;
  nodeName: string;
  nodeIP: string;
  status: ClusterDeployTargetStatus;
  errorMessage?: string;
}

export interface ClusterDeployJob {
  id: string;
  clusterId: string;
  status: ClusterDeployJobStatus;
  targets?: ClusterDeployTarget[] | null;
}

export interface ClusterDeployPreflightCreated {
  preflightId: string;
  status: ClusterDeployJobStatus;
}

export interface ClusterDeployJobCreated {
  jobId: string;
  acceptedTargets: string[] | null;
  skippedTargets: { nodeId: string; reason: string }[] | null;
}

const ACTIVE_JOB_STATUSES: ReadonlySet<ClusterDeployJobStatus> = new Set([
  'queued',
  'waiting_source',
  'running',
  'canceling',
]);

export const isClusterDeployJobActive = (status: ClusterDeployJobStatus): boolean =>
  ACTIVE_JOB_STATUSES.has(status);

const clusterPath = (clusterName: string) =>
  `/api/clusters/${encodeURIComponent(clusterName)}/agent-deploy`;

export const ClusterAgentDeployAPI = {
  getCandidates: (clusterName: string) =>
    apiFetchJSON<ClusterDeployCandidates>(`${clusterPath(clusterName)}/candidates`),

  createPreflight: (clusterName: string, sourceAgentId: string, targetNodeIds: string[]) =>
    apiFetchJSON<ClusterDeployPreflightCreated>(`${clusterPath(clusterName)}/preflights`, {
      method: 'POST',
      body: JSON.stringify({ sourceAgentId, targetNodeIds, maxParallel: 2 }),
    }),

  getPreflight: (preflightId: string) =>
    apiFetchJSON<ClusterDeployJob>(
      `/api/agent-deploy/preflights/${encodeURIComponent(preflightId)}`,
    ),

  createJob: (
    clusterName: string,
    sourceAgentId: string,
    preflightId: string,
    targetNodeIds: string[],
  ) =>
    apiFetchJSON<ClusterDeployJobCreated>(`${clusterPath(clusterName)}/jobs`, {
      method: 'POST',
      body: JSON.stringify({ sourceAgentId, preflightId, targetNodeIds, maxParallel: 2 }),
    }),

  getJob: (jobId: string) =>
    apiFetchJSON<ClusterDeployJob>(`/api/agent-deploy/jobs/${encodeURIComponent(jobId)}`),
};
