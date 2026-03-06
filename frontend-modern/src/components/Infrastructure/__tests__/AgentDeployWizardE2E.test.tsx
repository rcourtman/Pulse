/**
 * E2E integration test for the full Agent Deploy Wizard flow.
 *
 * Unlike the unit tests in AgentDeployModal.test.tsx and DeployStepComponents.test.tsx,
 * this test renders real step components with the real useDeployWizard hook,
 * mocking only external dependencies (API, SSE, license store, clipboard, icons).
 */
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@solidjs/testing-library';

/* ── hoisted mocks (must be before imports) ──────────────────── */

const { agentDeployAPIMock, nodesAPIMock, getLimitMock, copyToClipboardMock, loggerMock } =
  vi.hoisted(() => ({
    agentDeployAPIMock: {
      getCandidates: vi.fn(),
      createPreflight: vi.fn(),
      getPreflight: vi.fn(),
      createJob: vi.fn(),
      getJob: vi.fn(),
      cancelJob: vi.fn(),
      retryJob: vi.fn(),
    },
    nodesAPIMock: {
      getAgentInstallCommand: vi.fn(),
    },
    getLimitMock: vi.fn(),
    copyToClipboardMock: vi.fn(),
    loggerMock: {
      info: vi.fn(),
      warn: vi.fn(),
      error: vi.fn(),
      debug: vi.fn(),
    },
  }));

vi.mock('@/api/agentDeploy', () => ({ AgentDeployAPI: agentDeployAPIMock }));
vi.mock('@/api/nodes', () => ({ NodesAPI: nodesAPIMock }));
vi.mock('@/stores/license', () => ({ getLimit: getLimitMock }));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: copyToClipboardMock }));
vi.mock('@/utils/logger', () => ({ logger: loggerMock }));

// Stub icons — they don't render visually in jsdom
vi.mock('lucide-solid/icons/check', () => ({ default: () => <span data-testid="icon-check" /> }));
vi.mock('lucide-solid/icons/check-circle-2', () => ({
  default: () => <span data-testid="icon-check-circle" />,
}));
vi.mock('lucide-solid/icons/x-circle', () => ({
  default: () => <span data-testid="icon-x-circle" />,
}));
vi.mock('lucide-solid/icons/alert-circle', () => ({
  default: () => <span data-testid="icon-alert-circle" />,
}));
vi.mock('lucide-solid/icons/loader-2', () => ({
  default: () => <span data-testid="icon-loader" />,
}));
vi.mock('lucide-solid/icons/chevron-down', () => ({
  default: () => <span data-testid="icon-chevron-down" />,
}));
vi.mock('lucide-solid/icons/chevron-right', () => ({
  default: () => <span data-testid="icon-chevron-right" />,
}));
vi.mock('lucide-solid/icons/x', () => ({ default: () => <span data-testid="icon-x" /> }));
vi.mock('lucide-solid/icons/copy', () => ({ default: () => <span data-testid="icon-copy" /> }));

/* ── MockEventSource ──────────────────────────────────────────── */

class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly url: string;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  readyState = 0;
  withCredentials = false;
  closed = false;

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  close() {
    this.closed = true;
    this.readyState = 2;
  }

  emitOpen() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  emitMessage(payload: unknown) {
    const evt = { data: JSON.stringify(payload), lastEventId: '' } as MessageEvent;
    this.onmessage?.(evt);
  }

  emitError() {
    this.onerror?.(new Event('error'));
  }
}

/* ── Component import (after mocks) ───────────────────────────── */

import { AgentDeployModal } from '../AgentDeployModal';
import type { DeployTarget, DeployJob } from '@/types/agentDeploy';

/* ── Test data factories ──────────────────────────────────────── */

function makeTarget(overrides: Partial<DeployTarget> = {}): DeployTarget {
  return {
    id: 't1',
    jobId: 'j1',
    nodeId: 'n1',
    nodeName: 'node1',
    nodeIP: '10.0.0.1',
    status: 'pending',
    attempts: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeJob(overrides: Partial<DeployJob> = {}): DeployJob {
  return {
    id: 'j1',
    clusterId: 'cluster-1',
    clusterName: 'My Cluster',
    sourceAgentId: 'agent-1',
    status: 'running',
    targets: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

const CANDIDATES_RESPONSE = {
  clusterId: 'cluster-1',
  clusterName: 'My Cluster',
  sourceAgents: [{ agentId: 'agent-1', nodeId: 'pve1', online: true }],
  nodes: [
    { nodeId: 'n1', name: 'pve1', ip: '10.0.0.1', hasAgent: true, deployable: false },
    { nodeId: 'n2', name: 'pve2', ip: '10.0.0.2', hasAgent: false, deployable: true },
    { nodeId: 'n3', name: 'pve3', ip: '10.0.0.3', hasAgent: false, deployable: true },
  ],
};

/* ── Lifecycle ─────────────────────────────────────────────────── */

const originalEventSource = globalThis.EventSource;

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers();
  MockEventSource.instances = [];
  (globalThis as unknown as Record<string, unknown>).EventSource =
    MockEventSource as unknown as typeof EventSource;
  getLimitMock.mockReturnValue({ limit: 10 });
  copyToClipboardMock.mockResolvedValue(true);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  globalThis.EventSource = originalEventSource;
  document.body.style.overflow = '';
});

/* ── Helpers ───────────────────────────────────────────────────── */

function renderWizard(onClose = vi.fn()) {
  return render(() => (
    <AgentDeployModal
      isOpen={true}
      clusterId="cluster-1"
      clusterName="My Cluster"
      onClose={onClose}
    />
  ));
}

/** Wait for candidates to load (API resolves, loading spinner disappears). */
async function waitForCandidates() {
  await vi.advanceTimersByTimeAsync(0);
  await waitFor(() => {
    expect(screen.getByText('pve2')).toBeInTheDocument();
  });
}

/** Get the latest MockEventSource instance. */
function latestES(): MockEventSource {
  const instances = MockEventSource.instances;
  return instances[instances.length - 1];
}

/* ================================================================
   Test Cases
   ================================================================ */

describe('Agent Deploy Wizard E2E', () => {
  /* ── Test 1: Happy path ──────────────────────────────────────── */

  it('completes full happy path: candidates → preflight → confirm → deploy → results → close', async () => {
    // Setup: getCandidates returns 2 deployable + 1 with agent
    agentDeployAPIMock.getCandidates.mockResolvedValue(CANDIDATES_RESPONSE);

    // Setup: createPreflight
    agentDeployAPIMock.createPreflight.mockResolvedValue({
      preflightId: 'pf1',
      status: 'running',
      eventsUrl: '/api/agent-deploy/preflights/pf1/events',
    });

    // Setup: getPreflight (initial fetch returns pending targets)
    const preflightTargets = [
      makeTarget({
        id: 'pt1',
        nodeId: 'n2',
        nodeName: 'pve2',
        nodeIP: '10.0.0.2',
        status: 'pending',
      }),
      makeTarget({
        id: 'pt2',
        nodeId: 'n3',
        nodeName: 'pve3',
        nodeIP: '10.0.0.3',
        status: 'pending',
      }),
    ];
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({ id: 'pf1', targets: preflightTargets }),
    );

    const onClose = vi.fn();
    renderWizard(onClose);

    // Step 1: Candidates step loads
    await waitForCandidates();

    // pve1 already has agent, pve2+pve3 should be selectable
    expect(screen.getByText('Already monitored')).toBeInTheDocument();
    expect(screen.getByText('2 of 2 nodes selected')).toBeInTheDocument();

    // Click "Run Preflight"
    const preflightBtn = screen.getByText('Run Preflight');
    expect(preflightBtn).not.toBeDisabled();
    await fireEvent.click(preflightBtn);
    await vi.advanceTimersByTimeAsync(0);

    // Step 2: Preflight step — SSE events arrive
    const preflightES = latestES();
    preflightES.emitOpen();

    // Emit preflight results for both nodes
    preflightES.emitMessage({
      id: 'e1',
      jobId: 'pf1',
      targetId: 'pt1',
      type: 'preflight_result',
      message: 'ready',
      data: JSON.stringify({ ssh_reachable: true, pulse_reachable: true, arch: 'amd64' }),
      createdAt: '2026-01-01T00:00:00Z',
    });
    preflightES.emitMessage({
      id: 'e2',
      jobId: 'pf1',
      targetId: 'pt2',
      type: 'preflight_result',
      message: 'ready',
      data: JSON.stringify({ ssh_reachable: true, pulse_reachable: true, arch: 'arm64' }),
      createdAt: '2026-01-01T00:00:00Z',
    });

    // Emit job_complete
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'ready',
            arch: 'amd64',
          }),
          makeTarget({
            id: 'pt2',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'ready',
            arch: 'arm64',
          }),
        ],
      }),
    );

    preflightES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    // Step 3: Confirm step — both nodes ready
    await waitFor(() => {
      expect(screen.getByText('Ready to deploy (2)')).toBeInTheDocument();
    });

    // Setup: createJob
    agentDeployAPIMock.createJob.mockResolvedValue({
      jobId: 'j1',
      acceptedTargets: ['n2', 'n3'],
      skippedTargets: [],
      reservedLicenseSlots: 2,
      eventsUrl: '/api/agent-deploy/jobs/j1/events',
    });

    const deployTargets = [
      makeTarget({
        id: 'dt1',
        jobId: 'j1',
        nodeId: 'n2',
        nodeName: 'pve2',
        nodeIP: '10.0.0.2',
        status: 'pending',
      }),
      makeTarget({
        id: 'dt2',
        jobId: 'j1',
        nodeId: 'n3',
        nodeName: 'pve3',
        nodeIP: '10.0.0.3',
        status: 'pending',
      }),
    ];
    agentDeployAPIMock.getJob.mockResolvedValue(makeJob({ id: 'j1', targets: deployTargets }));

    // Click "Deploy 2 Hosts"
    const deployBtn = screen.getByText('Deploy 2 Hosts');
    await fireEvent.click(deployBtn);
    await vi.advanceTimersByTimeAsync(0);

    // Step 4: Deploying step — SSE events arrive
    const deployES = latestES();
    deployES.emitOpen();

    // Emit job_complete with all succeeded
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'succeeded',
          }),
          makeTarget({
            id: 'dt2',
            jobId: 'j1',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'succeeded',
          }),
        ],
      }),
    );

    deployES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    // Step 5: Results step — both deployed
    await waitFor(() => {
      expect(screen.getByText('Deployed (2)')).toBeInTheDocument();
    });

    // Close the wizard
    await fireEvent.click(screen.getByText('Close'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  /* ── Test 2: Partial failure + retry ─────────────────────────── */

  it('handles partial failure and retry flow', async () => {
    agentDeployAPIMock.getCandidates.mockResolvedValue(CANDIDATES_RESPONSE);

    // Preflight setup — same as happy path
    agentDeployAPIMock.createPreflight.mockResolvedValue({
      preflightId: 'pf1',
      status: 'running',
      eventsUrl: '/api/agent-deploy/preflights/pf1/events',
    });
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
          makeTarget({
            id: 'pt2',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'pending',
          }),
        ],
      }),
    );

    renderWizard();
    await waitForCandidates();

    // Run preflight
    await fireEvent.click(screen.getByText('Run Preflight'));
    await vi.advanceTimersByTimeAsync(0);

    const pfES = latestES();
    pfES.emitOpen();

    // Both pass preflight
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'ready',
          }),
          makeTarget({
            id: 'pt2',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'ready',
          }),
        ],
      }),
    );
    pfES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    await waitFor(() => {
      expect(screen.getByText('Ready to deploy (2)')).toBeInTheDocument();
    });

    // Deploy — one succeeds, one fails
    agentDeployAPIMock.createJob.mockResolvedValue({
      jobId: 'j1',
      acceptedTargets: ['n2', 'n3'],
      skippedTargets: [],
      reservedLicenseSlots: 2,
      eventsUrl: '/api/agent-deploy/jobs/j1/events',
    });
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
          makeTarget({
            id: 'dt2',
            jobId: 'j1',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'pending',
          }),
        ],
      }),
    );

    await fireEvent.click(screen.getByText('Deploy 2 Hosts'));
    await vi.advanceTimersByTimeAsync(0);

    const deployES = latestES();
    deployES.emitOpen();

    // Job completes with partial success
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        status: 'partial_success',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'succeeded',
          }),
          makeTarget({
            id: 'dt2',
            jobId: 'j1',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'failed_retryable',
            errorMessage: 'SSH connection refused',
          }),
        ],
      }),
    );

    deployES.emitMessage({ type: 'job_complete', status: 'partial_success' });
    await vi.advanceTimersByTimeAsync(0);

    // Results step — 1 succeeded, 1 failed
    await waitFor(() => {
      expect(screen.getByText('Deployed (1)')).toBeInTheDocument();
      expect(screen.getByText('Failed (1)')).toBeInTheDocument();
    });

    // Retry button should be visible
    expect(screen.getByText('Retry 1 Failed')).toBeInTheDocument();

    // Setup retry
    agentDeployAPIMock.retryJob.mockResolvedValue({
      jobId: 'j1',
      retryTargets: 1,
      status: 'running',
      eventsUrl: '/api/agent-deploy/jobs/j1/events',
    });
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'succeeded',
          }),
          makeTarget({
            id: 'dt2',
            jobId: 'j1',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'pending',
          }),
        ],
      }),
    );

    // Click retry
    await fireEvent.click(screen.getByText('Retry 1 Failed'));
    await vi.advanceTimersByTimeAsync(0);

    // Should be back on deploying step
    const retryES = latestES();
    retryES.emitOpen();

    // Retry succeeds
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'succeeded',
          }),
          makeTarget({
            id: 'dt2',
            jobId: 'j1',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'succeeded',
          }),
        ],
      }),
    );

    retryES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    await waitFor(() => {
      expect(screen.getByText('Deployed (2)')).toBeInTheDocument();
    });
    expect(screen.queryByText(/Failed/)).not.toBeInTheDocument();
  });

  /* ── Test 3: Cancel during deploy ────────────────────────────── */

  it('handles cancel during deploy', async () => {
    agentDeployAPIMock.getCandidates.mockResolvedValue(CANDIDATES_RESPONSE);

    agentDeployAPIMock.createPreflight.mockResolvedValue({
      preflightId: 'pf1',
      status: 'running',
      eventsUrl: '/api/agent-deploy/preflights/pf1/events',
    });
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
        ],
      }),
    );

    renderWizard();
    await waitForCandidates();

    // Deselect pve3, keep only pve2
    const pve3Row = screen.getByText('pve3').closest('tr')!;
    await fireEvent.click(pve3Row);
    await vi.advanceTimersByTimeAsync(0);

    // Run preflight with only pve2
    await fireEvent.click(screen.getByText('Run Preflight'));
    await vi.advanceTimersByTimeAsync(0);

    const pfES = latestES();
    pfES.emitOpen();

    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'ready',
          }),
        ],
      }),
    );
    pfES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    await waitFor(() => {
      expect(screen.getByText('Ready to deploy (1)')).toBeInTheDocument();
    });

    // Start deploy
    agentDeployAPIMock.createJob.mockResolvedValue({
      jobId: 'j1',
      acceptedTargets: ['n2'],
      skippedTargets: [],
      reservedLicenseSlots: 1,
      eventsUrl: '/api/agent-deploy/jobs/j1/events',
    });
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'installing',
          }),
        ],
      }),
    );

    await fireEvent.click(screen.getByText('Deploy 1 Host'));
    await vi.advanceTimersByTimeAsync(0);

    const deployES = latestES();
    deployES.emitOpen();

    // Cancel the deployment
    agentDeployAPIMock.cancelJob.mockResolvedValue(undefined);
    await fireEvent.click(screen.getByText('Cancel Deployment'));
    await vi.advanceTimersByTimeAsync(0);

    expect(agentDeployAPIMock.cancelJob).toHaveBeenCalledWith('j1');

    // Backend sends job_complete with canceled status
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        status: 'canceled',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'canceled',
          }),
        ],
      }),
    );

    deployES.emitMessage({ type: 'job_complete', status: 'canceled' });
    await vi.advanceTimersByTimeAsync(0);

    // Results step — should show canceled
    await waitFor(() => {
      expect(screen.getByText('Canceled (1)')).toBeInTheDocument();
    });
  });

  /* ── Test 4: Manual install on failure ───────────────────────── */

  it('shows manual install with real API command and copy button', async () => {
    agentDeployAPIMock.getCandidates.mockResolvedValue({
      ...CANDIDATES_RESPONSE,
      nodes: [
        { nodeId: 'n1', name: 'pve1', ip: '10.0.0.1', hasAgent: true, deployable: false },
        { nodeId: 'n2', name: 'pve2', ip: '10.0.0.2', hasAgent: false, deployable: true },
      ],
    });

    agentDeployAPIMock.createPreflight.mockResolvedValue({
      preflightId: 'pf1',
      status: 'running',
      eventsUrl: '/api/agent-deploy/preflights/pf1/events',
    });
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
        ],
      }),
    );

    renderWizard();
    await waitForCandidates();

    // Run preflight
    await fireEvent.click(screen.getByText('Run Preflight'));
    await vi.advanceTimersByTimeAsync(0);

    const pfES = latestES();
    pfES.emitOpen();

    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'ready',
          }),
        ],
      }),
    );
    pfES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    await waitFor(() => {
      expect(screen.getByText('Ready to deploy (1)')).toBeInTheDocument();
    });

    // Deploy — fails
    agentDeployAPIMock.createJob.mockResolvedValue({
      jobId: 'j1',
      acceptedTargets: ['n2'],
      skippedTargets: [],
      reservedLicenseSlots: 1,
      eventsUrl: '/api/agent-deploy/jobs/j1/events',
    });
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
        ],
      }),
    );

    await fireEvent.click(screen.getByText('Deploy 1 Host'));
    await vi.advanceTimersByTimeAsync(0);

    const deployES = latestES();
    deployES.emitOpen();

    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        status: 'failed',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'failed_retryable',
            errorMessage: 'SSH connection refused',
          }),
        ],
      }),
    );

    deployES.emitMessage({ type: 'job_complete', status: 'failed' });
    await vi.advanceTimersByTimeAsync(0);

    // Results — failed, manual install accordion visible
    await waitFor(() => {
      expect(screen.getByText('Failed (1)')).toBeInTheDocument();
    });

    // Setup API for install command
    nodesAPIMock.getAgentInstallCommand.mockResolvedValue({
      command: 'curl -fsSL http://10.0.0.100:7655/api/agent/install.sh | PULSE_TOKEN=abc123 bash',
    });

    // Open manual install accordion
    const accordionBtn = screen.getByText('Manual install instructions');
    expect(accordionBtn).toHaveAttribute('aria-expanded', 'false');
    await fireEvent.click(accordionBtn);
    expect(accordionBtn).toHaveAttribute('aria-expanded', 'true');

    // Should call the API
    await vi.advanceTimersByTimeAsync(0);
    expect(nodesAPIMock.getAgentInstallCommand).toHaveBeenCalledWith({
      type: 'pve',
      enableProxmox: true,
    });

    // Command should appear
    await waitFor(() => {
      expect(
        screen.getByText(
          'curl -fsSL http://10.0.0.100:7655/api/agent/install.sh | PULSE_TOKEN=abc123 bash',
        ),
      ).toBeInTheDocument();
    });

    // Copy button should be visible
    const copyBtn = screen.getByLabelText('Copy to clipboard');
    await fireEvent.click(copyBtn);
    expect(copyToClipboardMock).toHaveBeenCalledWith(
      'curl -fsSL http://10.0.0.100:7655/api/agent/install.sh | PULSE_TOKEN=abc123 bash',
    );

    // Should show "Copied" state
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByLabelText('Copied')).toBeInTheDocument();

    // After 2s, reverts back
    await vi.advanceTimersByTimeAsync(2000);
    expect(screen.getByLabelText('Copy to clipboard')).toBeInTheDocument();
  });

  /* ── Test 5: Preflight failure ───────────────────────────────── */

  it('handles preflight failures — shows "Cannot deploy" section and deploys only ready nodes', async () => {
    agentDeployAPIMock.getCandidates.mockResolvedValue(CANDIDATES_RESPONSE);

    agentDeployAPIMock.createPreflight.mockResolvedValue({
      preflightId: 'pf1',
      status: 'running',
      eventsUrl: '/api/agent-deploy/preflights/pf1/events',
    });
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
          makeTarget({
            id: 'pt2',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'pending',
          }),
        ],
      }),
    );

    renderWizard();
    await waitForCandidates();

    // Run preflight
    await fireEvent.click(screen.getByText('Run Preflight'));
    await vi.advanceTimersByTimeAsync(0);

    const pfES = latestES();
    pfES.emitOpen();

    // One passes, one fails
    agentDeployAPIMock.getPreflight.mockResolvedValue(
      makeJob({
        id: 'pf1',
        status: 'partial_success',
        targets: [
          makeTarget({
            id: 'pt1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'ready',
          }),
          makeTarget({
            id: 'pt2',
            nodeId: 'n3',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'failed_permanent',
            errorMessage: 'SSH permission denied',
          }),
        ],
      }),
    );

    pfES.emitMessage({ type: 'job_complete', status: 'partial_success' });
    await vi.advanceTimersByTimeAsync(0);

    // Confirm step — shows ready and failed sections
    await waitFor(() => {
      expect(screen.getByText('Ready to deploy (1)')).toBeInTheDocument();
    });
    expect(screen.getByText('Cannot deploy (1)')).toBeInTheDocument();
    expect(screen.getByText('SSH permission denied')).toBeInTheDocument();

    // Deploy the 1 ready node
    agentDeployAPIMock.createJob.mockResolvedValue({
      jobId: 'j1',
      acceptedTargets: ['n2'],
      skippedTargets: [],
      reservedLicenseSlots: 1,
      eventsUrl: '/api/agent-deploy/jobs/j1/events',
    });
    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'pending',
          }),
        ],
      }),
    );

    await fireEvent.click(screen.getByText('Deploy 1 Host'));
    await vi.advanceTimersByTimeAsync(0);

    const deployES = latestES();
    deployES.emitOpen();

    agentDeployAPIMock.getJob.mockResolvedValue(
      makeJob({
        id: 'j1',
        status: 'succeeded',
        targets: [
          makeTarget({
            id: 'dt1',
            jobId: 'j1',
            nodeId: 'n2',
            nodeName: 'pve2',
            nodeIP: '10.0.0.2',
            status: 'succeeded',
          }),
        ],
      }),
    );

    deployES.emitMessage({ type: 'job_complete', status: 'succeeded' });
    await vi.advanceTimersByTimeAsync(0);

    await waitFor(() => {
      expect(screen.getByText('Deployed (1)')).toBeInTheDocument();
    });
  });
});
