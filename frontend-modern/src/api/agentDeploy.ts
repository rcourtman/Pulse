import { apiFetchJSON } from '@/utils/apiClient';
import type {
  CandidatesResponse,
  CreatePreflightRequest,
  CreatePreflightResponse,
  CreateJobRequest,
  CreateJobResponse,
  DeployJob,
  RetryJobResponse,
} from '@/types/agentDeploy';

export class AgentDeployAPI {
  static async getCandidates(clusterId: string): Promise<CandidatesResponse> {
    return apiFetchJSON(
      `/api/clusters/${encodeURIComponent(clusterId)}/agent-deploy/candidates`,
    ) as Promise<CandidatesResponse>;
  }

  static async createPreflight(
    clusterId: string,
    req: CreatePreflightRequest,
  ): Promise<CreatePreflightResponse> {
    return apiFetchJSON(`/api/clusters/${encodeURIComponent(clusterId)}/agent-deploy/preflights`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }) as Promise<CreatePreflightResponse>;
  }

  static async getPreflight(preflightId: string): Promise<DeployJob> {
    return apiFetchJSON(
      `/api/agent-deploy/preflights/${encodeURIComponent(preflightId)}`,
    ) as Promise<DeployJob>;
  }

  static async createJob(clusterId: string, req: CreateJobRequest): Promise<CreateJobResponse> {
    return apiFetchJSON(`/api/clusters/${encodeURIComponent(clusterId)}/agent-deploy/jobs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }) as Promise<CreateJobResponse>;
  }

  static async getJob(jobId: string): Promise<DeployJob> {
    return apiFetchJSON(
      `/api/agent-deploy/jobs/${encodeURIComponent(jobId)}`,
    ) as Promise<DeployJob>;
  }

  static async cancelJob(jobId: string): Promise<void> {
    await apiFetchJSON(`/api/agent-deploy/jobs/${encodeURIComponent(jobId)}/cancel`, {
      method: 'POST',
    });
  }

  static async retryJob(jobId: string, targetIds?: string[]): Promise<RetryJobResponse> {
    return apiFetchJSON(`/api/agent-deploy/jobs/${encodeURIComponent(jobId)}/retry`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(targetIds ? { targetIds } : {}),
    }) as Promise<RetryJobResponse>;
  }
}
