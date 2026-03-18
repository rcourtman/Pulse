import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ── hoisted mocks ──────────────────────────────────────────── */

const { apiMock, loggerMock, getLimitMock } = vi.hoisted(() => ({
  apiMock: {
    getCandidates: vi.fn(),
    createPreflight: vi.fn(),
    getPreflight: vi.fn(),
    createJob: vi.fn(),
    getJob: vi.fn(),
    cancelJob: vi.fn(),
    retryJob: vi.fn(),
  },
  loggerMock: { info: vi.fn(), warn: vi.fn() },
  getLimitMock: vi.fn(),
}));

vi.mock('@/api/agentDeploy', () => ({ AgentDeployAPI: apiMock }));
vi.mock('@/utils/logger', () => ({ logger: loggerMock }));
vi.mock('@/stores/license', () => ({ getLimit: getLimitMock }));

// Stub useDeployStream — wizard only needs the returned state, not real SSE.
vi.mock('@/hooks/useDeployStream', () => ({
  useDeployStream: () => ({
    isStreaming: () => false,
    events: () => [],
    lastError: () => '',
  }),
}));

import { useDeployWizard, type DeployWizardState } from '@/hooks/useDeployWizard';

/* ── helpers ────────────────────────────────────────────────── */

function withRoot(fn: () => DeployWizardState): { wizard: DeployWizardState; dispose: () => void } {
  let wizard!: DeployWizardState;
  const dispose = createRoot((d) => {
    wizard = fn();
    return d;
  });
  return { wizard, dispose };
}

const defaultOpts = { clusterId: 'cluster-1', clusterName: 'My Cluster' };

/* ── lifecycle ──────────────────────────────────────────────── */

beforeEach(() => {
  vi.clearAllMocks();
  getLimitMock.mockReturnValue(null);
  // Default: getCandidates returns empty (called immediately on creation).
  apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
});

afterEach(() => {
  vi.restoreAllMocks();
});

/* ── tests ──────────────────────────────────────────────────── */

describe('useDeployWizard', () => {
  describe('initialization', () => {
    it('starts on candidates step', () => {
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      expect(wizard.step()).toBe('candidates');
      dispose();
    });

    it('loads candidates immediately', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [
          { nodeId: 'n1', name: 'node1', ip: '10.0.0.1', hasAgent: false, deployable: true },
          { nodeId: 'n2', name: 'node2', ip: '10.0.0.2', hasAgent: true, deployable: false },
        ],
        sourceAgents: [{ agentId: 'a1', nodeId: 'n2', online: true }],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.candidates()).toHaveLength(2);
      expect(wizard.sourceAgents()).toHaveLength(1);
      dispose();
    });

    it('auto-selects deployable nodes', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [
          { nodeId: 'n1', name: 'node1', ip: '10.0.0.1', hasAgent: false, deployable: true },
          { nodeId: 'n2', name: 'node2', ip: '10.0.0.2', hasAgent: true, deployable: false },
          { nodeId: 'n3', name: 'node3', ip: '10.0.0.3', hasAgent: false, deployable: true },
        ],
        sourceAgents: [],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.selectedNodeIds().has('n1')).toBe(true);
      expect(wizard.selectedNodeIds().has('n3')).toBe(true);
      expect(wizard.selectedNodeIds().has('n2')).toBe(false);
      dispose();
    });

    it('auto-selects source agent when only one is online', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [],
        sourceAgents: [
          { agentId: 'a1', nodeId: 'n1', online: true },
          { agentId: 'a2', nodeId: 'n2', online: false },
        ],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.selectedSourceAgent()).toBe('a1');
      dispose();
    });

    it('does not auto-select source agent when multiple are online', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [],
        sourceAgents: [
          { agentId: 'a1', nodeId: 'n1', online: true },
          { agentId: 'a2', nodeId: 'n2', online: true },
        ],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.selectedSourceAgent()).toBe('');
      dispose();
    });

    it('sets candidatesError on API failure', async () => {
      apiMock.getCandidates.mockRejectedValue(new Error('Network error'));

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.candidatesError()).toBe('Network error');
      dispose();
    });
  });

  describe('node selection', () => {
    it('toggleNodeSelection adds and removes nodes', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.toggleNodeSelection('n1');
      expect(wizard.selectedNodeIds().has('n1')).toBe(true);

      wizard.toggleNodeSelection('n1');
      expect(wizard.selectedNodeIds().has('n1')).toBe(false);
      dispose();
    });

    it('selectAllNodes selects all deployable nodes', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [
          { nodeId: 'n1', name: 'a', ip: '1', hasAgent: false, deployable: true },
          { nodeId: 'n2', name: 'b', ip: '2', hasAgent: true, deployable: false },
          { nodeId: 'n3', name: 'c', ip: '3', hasAgent: false, deployable: true },
        ],
        sourceAgents: [],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.deselectAllNodes();
      expect(wizard.selectedNodeIds().size).toBe(0);

      wizard.selectAllNodes();
      expect(wizard.selectedNodeIds().size).toBe(2);
      expect(wizard.selectedNodeIds().has('n1')).toBe(true);
      expect(wizard.selectedNodeIds().has('n3')).toBe(true);
      dispose();
    });
  });

  describe('startPreflight', () => {
    it('does nothing when no nodes selected', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      await wizard.startPreflight();
      expect(apiMock.createPreflight).not.toHaveBeenCalled();
      dispose();
    });

    it('does nothing when no source agent selected', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.toggleNodeSelection('n1');
      await wizard.startPreflight();
      expect(apiMock.createPreflight).not.toHaveBeenCalled();
      dispose();
    });

    it('calls createPreflight and transitions to preflight step', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createPreflight.mockResolvedValue({
        preflightId: 'pf-1',
        status: 'running',
        eventsUrl: '/api/agent-deploy/preflights/pf-1/events',
      });
      apiMock.getPreflight.mockResolvedValue({
        targets: [
          { id: 't1', nodeId: 'n1', nodeName: 'node1', nodeIP: '10.0.0.1', status: 'pending' },
        ],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.toggleNodeSelection('n1');
      wizard.setSelectedSourceAgent('a1');
      await wizard.startPreflight();

      expect(apiMock.createPreflight).toHaveBeenCalledWith('cluster-1', {
        sourceAgentId: 'a1',
        targetNodeIds: ['n1'],
      });
      expect(wizard.step()).toBe('preflight');
      expect(wizard.preflightId()).toBe('pf-1');
      dispose();
    });

    it('sets preflightError on API failure', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createPreflight.mockRejectedValue(new Error('Server error'));

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.toggleNodeSelection('n1');
      wizard.setSelectedSourceAgent('a1');
      await wizard.startPreflight();

      expect(wizard.preflightError()).toBe('Server error');
      expect(wizard.step()).toBe('candidates'); // didn't transition
      dispose();
    });
  });

  describe('confirm step selection', () => {
    it('toggleConfirmNode adds and removes nodes', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.toggleConfirmNode('n1');
      expect(wizard.confirmSelectedNodeIds().has('n1')).toBe(true);

      wizard.toggleConfirmNode('n1');
      expect(wizard.confirmSelectedNodeIds().has('n1')).toBe(false);
      dispose();
    });
  });

  describe('startDeploy', () => {
    it('calls createJob and transitions to deploying step', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createJob.mockResolvedValue({
        jobId: 'job-1',
        eventsUrl: '/api/agent-deploy/jobs/job-1/events',
      });
      apiMock.getJob.mockResolvedValue({
        targets: [{ id: 't1', nodeId: 'n1', status: 'pending' }],
        status: 'running',
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.setSelectedSourceAgent('a1');
      wizard.toggleConfirmNode('n1');

      // Simulate being on confirm step
      wizard.setStep('confirm');
      await wizard.startDeploy();

      expect(apiMock.createJob).toHaveBeenCalledWith(
        'cluster-1',
        expect.objectContaining({
          sourceAgentId: 'a1',
          targetNodeIds: ['n1'],
        }),
      );
      expect(wizard.step()).toBe('deploying');
      expect(wizard.jobId()).toBe('job-1');
      dispose();
    });

    it('accepts explicit nodeIds parameter', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createJob.mockResolvedValue({
        jobId: 'job-2',
        eventsUrl: '/api/events',
      });
      apiMock.getJob.mockResolvedValue({ targets: [], status: 'running' });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.setSelectedSourceAgent('a1');
      await wizard.startDeploy(['n1', 'n2']);

      expect(apiMock.createJob).toHaveBeenCalledWith(
        'cluster-1',
        expect.objectContaining({
          targetNodeIds: ['n1', 'n2'],
        }),
      );
      dispose();
    });

    it('does nothing with empty targetNodeIds', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      await wizard.startDeploy([]);
      expect(apiMock.createJob).not.toHaveBeenCalled();
      dispose();
    });

    it('sets deployError on API failure', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createJob.mockRejectedValue(new Error('Deploy failed'));

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.setSelectedSourceAgent('a1');
      wizard.toggleConfirmNode('n1');
      await wizard.startDeploy();

      expect(wizard.deployError()).toBe('Deploy failed');
      dispose();
    });
  });

  describe('cancelDeploy', () => {
    it('calls cancelJob API', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.cancelJob.mockResolvedValue(undefined);
      apiMock.createJob.mockResolvedValue({ jobId: 'job-1', eventsUrl: '/api/events' });
      apiMock.getJob.mockResolvedValue({ targets: [], status: 'running' });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.setSelectedSourceAgent('a1');
      await wizard.startDeploy(['n1']);
      await wizard.cancelDeploy();

      expect(apiMock.cancelJob).toHaveBeenCalledWith('job-1');
      dispose();
    });

    it('does nothing when no jobId', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      await wizard.cancelDeploy();
      expect(apiMock.cancelJob).not.toHaveBeenCalled();
      dispose();
    });
  });

  describe('retryFailed', () => {
    it('calls retryJob with retryable target IDs', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createJob.mockResolvedValue({ jobId: 'job-1', eventsUrl: '/api/events' });
      apiMock.getJob.mockResolvedValue({
        targets: [
          { id: 't1', nodeId: 'n1', status: 'failed_retryable' },
          { id: 't2', nodeId: 'n2', status: 'succeeded' },
          { id: 't3', nodeId: 'n3', status: 'failed_retryable' },
        ],
        status: 'partial_success',
      });
      apiMock.retryJob.mockResolvedValue({
        jobId: 'job-1',
        retryTargets: 2,
        status: 'running',
        eventsUrl: '/api/events/retry',
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.setSelectedSourceAgent('a1');
      await wizard.startDeploy(['n1', 'n2', 'n3']);
      await wizard.retryFailed();

      expect(apiMock.retryJob).toHaveBeenCalledWith('job-1', ['t1', 't3']);
      expect(wizard.step()).toBe('deploying');
      dispose();
    });
  });

  describe('derived state', () => {
    it('deployableNodes filters to deployable nodes without agents', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [
          { nodeId: 'n1', name: 'a', ip: '1', hasAgent: false, deployable: true },
          { nodeId: 'n2', name: 'b', ip: '2', hasAgent: true, deployable: false },
          { nodeId: 'n3', name: 'c', ip: '3', hasAgent: false, deployable: false },
        ],
        sourceAgents: [],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.deployableNodes()).toHaveLength(1);
      expect(wizard.deployableNodes()[0].nodeId).toBe('n1');
      dispose();
    });

    it('onlineSourceAgents filters to online agents', async () => {
      apiMock.getCandidates.mockResolvedValue({
        nodes: [],
        sourceAgents: [
          { agentId: 'a1', nodeId: 'n1', online: true },
          { agentId: 'a2', nodeId: 'n2', online: false },
        ],
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.onlineSourceAgents()).toHaveLength(1);
      expect(wizard.onlineSourceAgents()[0].agentId).toBe('a1');
      dispose();
    });

    it('succeededTargets includes succeeded, enrolling, and verifying', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      apiMock.createJob.mockResolvedValue({ jobId: 'j1', eventsUrl: '/e' });
      apiMock.getJob.mockResolvedValue({
        targets: [
          { id: 't1', status: 'succeeded' },
          { id: 't2', status: 'enrolling' },
          { id: 't3', status: 'verifying' },
          { id: 't4', status: 'failed_retryable' },
        ],
        status: 'partial_success',
      });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      wizard.setSelectedSourceAgent('a1');
      await wizard.startDeploy(['n1']);

      expect(wizard.succeededTargets()).toHaveLength(3);
      expect(wizard.failedTargets()).toHaveLength(1);
      dispose();
    });

    it('isOperationActive is true during preflight and deploying', async () => {
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });
      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.isOperationActive()).toBe(false);

      wizard.setStep('preflight');
      expect(wizard.isOperationActive()).toBe(true);

      wizard.setStep('deploying');
      expect(wizard.isOperationActive()).toBe(true);

      wizard.setStep('results');
      expect(wizard.isOperationActive()).toBe(false);
      dispose();
    });

    it('maxAgentSlots returns limit from license store', async () => {
      getLimitMock.mockReturnValue({ limit: 10 });
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.maxAgentSlots()).toBe(10);
      dispose();
    });

    it('maxAgentSlots returns 0 when no license limit', async () => {
      getLimitMock.mockReturnValue(null);
      apiMock.getCandidates.mockResolvedValue({ nodes: [], sourceAgents: [] });

      const { wizard, dispose } = withRoot(() => useDeployWizard(defaultOpts));
      await vi.waitFor(() => expect(wizard.candidatesLoading()).toBe(false));

      expect(wizard.maxAgentSlots()).toBe(0);
      dispose();
    });
  });
});
