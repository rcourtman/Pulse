import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';
import type { DeployTarget, DeployTargetStatus, SourceAgentInfo } from '@/types/agentDeploy';
import type { DeployWizardState } from '@/hooks/useDeployWizard';

/* ── component import ──────────────────────────────────────────── */
import { PreflightStep } from '../PreflightStep';

/* ── helpers ────────────────────────────────────────────────────── */

function makeTarget(overrides: Partial<DeployTarget> = {}): DeployTarget {
  return {
    id: 'target-1',
    jobId: 'job-1',
    nodeId: 'node-1',
    nodeName: 'pve1',
    nodeIP: '192.168.1.10',
    arch: 'amd64',
    status: 'pending',
    errorMessage: undefined,
    attempts: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/**
 * Returns the progress summary element (the one with aria-live="polite").
 * DeployStatusBadge also uses role="status", so we need to distinguish.
 */
function getProgressSummary(): HTMLElement {
  const statusElements = screen.getAllByRole('status');
  const progress = statusElements.find((el) => el.getAttribute('aria-live') === 'polite');
  if (!progress) throw new Error('Could not find progress summary element with aria-live="polite"');
  return progress;
}

/** Creates a mock DeployWizardState with sensible defaults for PreflightStep. */
function createMockWizard(
  overrides: Partial<{
    preflightTargets: DeployTarget[];
    preflightError: string;
  }> = {},
): DeployWizardState {
  const targets = overrides.preflightTargets ?? [];
  const [preflightError] = createSignal(overrides.preflightError ?? '');

  return {
    preflightTargets: (() => targets) as Accessor<DeployTarget[]>,
    preflightError,
    // Stubs for fields not used by PreflightStep but required by the type.
    step: (() => 'preflight') as Accessor<string>,
    setStep: vi.fn(),
    candidates: (() => []) as Accessor<unknown[]>,
    candidatesLoading: (() => false) as Accessor<boolean>,
    candidatesError: (() => '') as Accessor<string>,
    selectedNodeIds: (() => new Set<string>()) as Accessor<Set<string>>,
    deployableNodes: (() => []) as Accessor<unknown[]>,
    onlineSourceAgents: (() => []) as Accessor<SourceAgentInfo[]>,
    sourceAgents: (() => []) as Accessor<SourceAgentInfo[]>,
    selectedSourceAgent: (() => '') as Accessor<string>,
    setSelectedSourceAgent: vi.fn(),
    toggleNodeSelection: vi.fn(),
    selectAllNodes: vi.fn(),
    deselectAllNodes: vi.fn(),
    preflightId: (() => '') as Accessor<string>,
    preflightStatus: (() => '') as Accessor<string>,
    preflightStream: {} as unknown,
    readyNodes: (() => []) as Accessor<unknown[]>,
    failedPreflightNodes: (() => []) as Accessor<unknown[]>,
    confirmSelectedNodeIds: (() => new Set<string>()) as Accessor<Set<string>>,
    toggleConfirmNode: vi.fn(),
    jobId: (() => '') as Accessor<string>,
    jobTargets: (() => []) as Accessor<unknown[]>,
    jobStatus: (() => '') as Accessor<string>,
    deployError: (() => '') as Accessor<string>,
    deployStream: {} as unknown,
    succeededTargets: (() => []) as Accessor<unknown[]>,
    failedTargets: (() => []) as Accessor<unknown[]>,
    retryableTargets: (() => []) as Accessor<unknown[]>,
    skippedTargets: (() => []) as Accessor<unknown[]>,
    canceledTargets: (() => []) as Accessor<unknown[]>,
    maxAgentSlots: (() => 0) as Accessor<number>,
    startingPreflight: (() => false) as Accessor<boolean>,
    startingDeploy: (() => false) as Accessor<boolean>,
    retrying: (() => false) as Accessor<boolean>,
    canceling: (() => false) as Accessor<boolean>,
    isOperationActive: (() => false) as Accessor<boolean>,
    loadCandidates: vi.fn(),
    startPreflight: vi.fn(),
    startDeploy: vi.fn(),
    cancelDeploy: vi.fn(),
    retryFailed: vi.fn(),
  } as unknown as DeployWizardState;
}

/* ── lifecycle ──────────────────────────────────────────────────── */
beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  cleanup();
});

/* ================================================================
   Tests
   ================================================================ */

describe('PreflightStep', () => {
  /* ── Error banner ────────────────────────────────────────────── */

  describe('error banner', () => {
    it('shows the preflight error when present', () => {
      const wizard = createMockWizard({ preflightError: 'SSE connection lost' });
      render(() => <PreflightStep wizard={wizard} />);

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
      expect(alert).toHaveTextContent('SSE connection lost');
    });

    it('does not render an alert when there is no error', () => {
      const wizard = createMockWizard({ preflightError: '' });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  /* ── Progress summary ────────────────────────────────────────── */

  describe('progress summary', () => {
    it('shows "Checking X of Y nodes..." while checks are in progress', () => {
      const targets = [
        makeTarget({ id: 't1', nodeName: 'pve1', status: 'ready' }),
        makeTarget({ id: 't2', nodeName: 'pve2', status: 'preflighting' }),
        makeTarget({ id: 't3', nodeName: 'pve3', status: 'pending' }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Checking 1 of 3 nodes...');
    });

    it('shows "Preflight checks complete" when all nodes are done', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'ready' }),
        makeTarget({ id: 't2', status: 'failed_permanent' }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Preflight checks complete');
    });

    it('shows "Preflight checks complete" when a single node completes', () => {
      const targets = [makeTarget({ id: 't1', status: 'ready' })];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Preflight checks complete');
    });

    it('counts "pending" as not completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'pending' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Checking 0 of 2 nodes...');
    });

    it('counts "preflighting" as not completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'preflighting' }),
        makeTarget({ id: 't2', status: 'ready' }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Checking 1 of 2 nodes...');
    });
  });

  /* ── Completed count logic ───────────────────────────────────── */

  describe('completed count logic', () => {
    const terminalStatuses: DeployTargetStatus[] = [
      'ready',
      'failed_retryable',
      'failed_permanent',
      'skipped_already_agent',
      'skipped_license',
      'canceled',
      'succeeded',
      'installing',
      'enrolling',
      'verifying',
    ];

    it.each(terminalStatuses)('counts status "%s" as completed', (status) => {
      const targets = [makeTarget({ id: 't1', status })];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Preflight checks complete');
    });

    it('counts only non-pending/non-preflighting as completed in a mixed set', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'ready' }),
        makeTarget({ id: 't2', status: 'failed_permanent' }),
        makeTarget({ id: 't3', status: 'preflighting' }),
        makeTarget({ id: 't4', status: 'pending' }),
        makeTarget({ id: 't5', status: 'canceled' }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      // 3 completed (ready, failed_permanent, canceled) out of 5
      expect(getProgressSummary()).toHaveTextContent('Checking 3 of 5 nodes...');
    });
  });

  /* ── Table structure ─────────────────────────────────────────── */

  describe('table structure', () => {
    it('renders column headers', () => {
      const wizard = createMockWizard({ preflightTargets: [makeTarget()] });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Node')).toBeInTheDocument();
      expect(screen.getByText('IP')).toBeInTheDocument();
      expect(screen.getByText('Status')).toBeInTheDocument();
      expect(screen.getByText('Details')).toBeInTheDocument();
    });

    it('renders a row for each target', () => {
      const targets = [
        makeTarget({ id: 't1', nodeName: 'pve1', nodeIP: '10.0.0.1' }),
        makeTarget({ id: 't2', nodeName: 'pve2', nodeIP: '10.0.0.2' }),
        makeTarget({ id: 't3', nodeName: 'pve3', nodeIP: '10.0.0.3' }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('pve1')).toBeInTheDocument();
      expect(screen.getByText('pve2')).toBeInTheDocument();
      expect(screen.getByText('pve3')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.2')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.3')).toBeInTheDocument();
    });

    it('renders no rows when targets list is empty', () => {
      const wizard = createMockWizard({ preflightTargets: [] });
      render(() => <PreflightStep wizard={wizard} />);

      // Table should exist (headers) but no target content
      expect(screen.getByText('Node')).toBeInTheDocument();
      expect(screen.queryByText('pve1')).not.toBeInTheDocument();
    });
  });

  /* ── Status badges ───────────────────────────────────────────── */

  describe('status badges', () => {
    it('shows "Pending" badge for pending targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'pending' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Pending')).toBeInTheDocument();
    });

    it('shows "Checking" badge for preflighting targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'preflighting' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Checking')).toBeInTheDocument();
    });

    it('shows "Ready" badge for ready targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'ready' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Ready')).toBeInTheDocument();
    });

    it('shows "Failed" badge for failed_permanent targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'failed_permanent' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Failed')).toBeInTheDocument();
    });

    it('shows "Failed" badge for failed_retryable targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'failed_retryable' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Failed')).toBeInTheDocument();
    });

    it('shows "Canceled" badge for canceled targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'canceled' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Canceled')).toBeInTheDocument();
    });

    it('shows "Installing" badge for installing targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'installing' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Installing')).toBeInTheDocument();
    });

    it('shows "Enrolling" badge for enrolling targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'enrolling' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Enrolling')).toBeInTheDocument();
    });

    it('shows "Verifying" badge for verifying targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'verifying' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Verifying')).toBeInTheDocument();
    });

    it('shows "Deployed" badge for succeeded targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'succeeded' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Deployed')).toBeInTheDocument();
    });

    it('shows "Already monitored" badge for skipped_already_agent targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'skipped_already_agent' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('Already monitored')).toBeInTheDocument();
    });

    it('shows "License limit" badge for skipped_license targets', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'skipped_license' })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText('License limit')).toBeInTheDocument();
    });
  });

  /* ── Error details ───────────────────────────────────────────── */

  describe('error details', () => {
    it('shows error message for a target with an error', () => {
      const wizard = createMockWizard({
        preflightTargets: [
          makeTarget({ status: 'failed_permanent', errorMessage: 'connection refused' }),
        ],
      });
      render(() => <PreflightStep wizard={wizard} />);

      expect(screen.getByText(/connection refused/)).toBeInTheDocument();
    });

    it('does not show error text for targets without errors', () => {
      const wizard = createMockWizard({
        preflightTargets: [makeTarget({ status: 'ready', errorMessage: undefined })],
      });
      render(() => <PreflightStep wizard={wizard} />);

      // ErrorDetail renders nothing when message is undefined — no error UI elements
      expect(screen.queryByText('more')).not.toBeInTheDocument();
      expect(screen.queryByText('less')).not.toBeInTheDocument();
      // No red error text containers rendered (ErrorDetail wraps in text-red-600)
      const redText = document.querySelector('.text-red-600, .text-red-400');
      expect(redText).toBeNull();
    });
  });

  /* ── Mixed scenario ──────────────────────────────────────────── */

  describe('mixed scenario', () => {
    it('renders correct state for multiple targets in various states', () => {
      const targets = [
        makeTarget({
          id: 't1',
          nodeName: 'node-ready',
          nodeIP: '10.0.0.1',
          status: 'ready',
        }),
        makeTarget({
          id: 't2',
          nodeName: 'node-checking',
          nodeIP: '10.0.0.2',
          status: 'preflighting',
        }),
        makeTarget({
          id: 't3',
          nodeName: 'node-failed',
          nodeIP: '10.0.0.3',
          status: 'failed_permanent',
          errorMessage: 'permission denied',
        }),
        makeTarget({
          id: 't4',
          nodeName: 'node-pending',
          nodeIP: '10.0.0.4',
          status: 'pending',
        }),
      ];
      const wizard = createMockWizard({ preflightTargets: targets });
      render(() => <PreflightStep wizard={wizard} />);

      // All node names rendered
      expect(screen.getByText('node-ready')).toBeInTheDocument();
      expect(screen.getByText('node-checking')).toBeInTheDocument();
      expect(screen.getByText('node-failed')).toBeInTheDocument();
      expect(screen.getByText('node-pending')).toBeInTheDocument();

      // All IPs rendered
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.2')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.3')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.4')).toBeInTheDocument();

      // Status badges: Ready, Checking, Failed, Pending
      expect(screen.getByText('Ready')).toBeInTheDocument();
      expect(screen.getByText('Checking')).toBeInTheDocument();
      expect(screen.getByText('Failed')).toBeInTheDocument();
      expect(screen.getByText('Pending')).toBeInTheDocument();

      // Error message present for failed node
      expect(screen.getByText(/permission denied/)).toBeInTheDocument();

      // Progress: 2 completed (ready + failed_permanent) out of 4
      expect(getProgressSummary()).toHaveTextContent('Checking 2 of 4 nodes...');
    });
  });

  /* ── Error banner + targets together ─────────────────────────── */

  describe('error banner with targets', () => {
    it('shows both the error banner and the targets table', () => {
      const wizard = createMockWizard({
        preflightError: 'Stream disconnected',
        preflightTargets: [
          makeTarget({ id: 't1', nodeName: 'pve1', status: 'ready' }),
          makeTarget({ id: 't2', nodeName: 'pve2', status: 'pending' }),
        ],
      });
      render(() => <PreflightStep wizard={wizard} />);

      // Error banner visible
      expect(screen.getByRole('alert')).toHaveTextContent('Stream disconnected');

      // Targets still rendered
      expect(screen.getByText('pve1')).toBeInTheDocument();
      expect(screen.getByText('pve2')).toBeInTheDocument();
    });
  });

  /* ── Empty targets with no error ─────────────────────────────── */

  describe('empty targets', () => {
    it('shows "Preflight checks complete" with 0 of 0 nodes', () => {
      const wizard = createMockWizard({ preflightTargets: [] });
      render(() => <PreflightStep wizard={wizard} />);

      // completedCount=0, totalCount=0 → 0 < 0 is false → shows "Preflight checks complete"
      expect(getProgressSummary()).toHaveTextContent('Preflight checks complete');
    });
  });
});
