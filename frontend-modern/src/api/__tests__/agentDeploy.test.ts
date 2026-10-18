import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ClusterAgentDeployAPI, isClusterDeployJobActive } from '../agentDeploy';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('ClusterAgentDeployAPI', () => {
  beforeEach(() => {
    vi.mocked(apiFetchJSON).mockReset();
    vi.mocked(apiFetchJSON).mockResolvedValue({});
  });

  it('keys cluster calls on the encoded Proxmox cluster name', async () => {
    await ClusterAgentDeployAPI.getCandidates('home lab/1');
    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/clusters/home%20lab%2F1/agent-deploy/candidates',
    );
  });

  it('sends the source agent and targets for a preflight', async () => {
    await ClusterAgentDeployAPI.createPreflight('homelab', 'agent-delly', ['n-delly2']);
    const [url, options] = vi.mocked(apiFetchJSON).mock.calls[0];
    expect(url).toBe('/api/clusters/homelab/agent-deploy/preflights');
    expect(options?.method).toBe('POST');
    expect(JSON.parse(String(options?.body))).toEqual({
      sourceAgentId: 'agent-delly',
      targetNodeIds: ['n-delly2'],
      maxParallel: 2,
    });
  });

  it('creates a job from a passed preflight', async () => {
    await ClusterAgentDeployAPI.createJob('homelab', 'agent-delly', 'pf_1', ['n-delly2']);
    const [url, options] = vi.mocked(apiFetchJSON).mock.calls[0];
    expect(url).toBe('/api/clusters/homelab/agent-deploy/jobs');
    expect(JSON.parse(String(options?.body))).toMatchObject({
      sourceAgentId: 'agent-delly',
      preflightId: 'pf_1',
      targetNodeIds: ['n-delly2'],
    });
  });

  it('reads preflights and jobs from their own status routes', async () => {
    await ClusterAgentDeployAPI.getPreflight('pf_1');
    await ClusterAgentDeployAPI.getJob('dep_1');
    expect(apiFetchJSON).toHaveBeenNthCalledWith(1, '/api/agent-deploy/preflights/pf_1');
    expect(apiFetchJSON).toHaveBeenNthCalledWith(2, '/api/agent-deploy/jobs/dep_1');
  });
});

describe('isClusterDeployJobActive', () => {
  it('treats only in-flight job states as active', () => {
    expect(isClusterDeployJobActive('queued')).toBe(true);
    expect(isClusterDeployJobActive('waiting_source')).toBe(true);
    expect(isClusterDeployJobActive('running')).toBe(true);
    expect(isClusterDeployJobActive('canceling')).toBe(true);
    expect(isClusterDeployJobActive('succeeded')).toBe(false);
    expect(isClusterDeployJobActive('partial_success')).toBe(false);
    expect(isClusterDeployJobActive('failed')).toBe(false);
    expect(isClusterDeployJobActive('canceled')).toBe(false);
  });
});
