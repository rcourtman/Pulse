import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';
import type { DeployTarget, SourceAgentInfo } from '@/types/agentDeploy';
import type { DeployWizardState } from '@/hooks/useDeployWizard';

/* ── component import ──────────────────────────────────────────── */
import { DeployingStep } from '../DeployingStep';

/* ── helpers ────────────────────────────────────────────────────── */

function makeTarget(overrides: Partial<DeployTarget> = {}): DeployTarget {
  return {
    id: 'target-1',
    jobId: 'job-1',
    nodeId: 'node-1',
    nodeName: 'pve1',
    nodeIP: '192.168.1.10',
    status: 'pending',
    attempts: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/** Creates a mock DeployWizardState with sensible defaults for DeployingStep. */
function createMockWizard(
  overrides: Partial<{
    jobTargets: DeployTarget[];
    deployError: string;
  }> = {},
): DeployWizardState {
  const jobTargetsData = overrides.jobTargets ?? [];
  const [deployError] = createSignal(overrides.deployError ?? '');

  return {
    // Fields used by DeployingStep
    jobTargets: (() => jobTargetsData) as Accessor<DeployTarget[]>,
    deployError,

    // Stubs for fields not used by DeployingStep but required by the type.
    step: (() => 'deploying') as Accessor<string>,
    setStep: vi.fn(),
    candidates: (() => []) as Accessor<unknown[]>,
    candidatesLoading: (() => false) as Accessor<boolean>,
    candidatesError: (() => '') as Accessor<string>,
    sourceAgents: (() => []) as Accessor<SourceAgentInfo[]>,
    onlineSourceAgents: (() => []) as Accessor<SourceAgentInfo[]>,
    selectedSourceAgent: (() => '') as Accessor<string>,
    setSelectedSourceAgent: vi.fn(),
    selectedNodeIds: (() => new Set<string>()) as Accessor<Set<string>>,
    deployableNodes: (() => []) as Accessor<unknown[]>,
    toggleNodeSelection: vi.fn(),
    selectAllNodes: vi.fn(),
    deselectAllNodes: vi.fn(),
    loadCandidates: vi.fn(),
    preflightId: (() => '') as Accessor<string>,
    preflightTargets: (() => []) as Accessor<unknown[]>,
    preflightStatus: (() => '') as Accessor<string>,
    preflightError: (() => '') as Accessor<string>,
    preflightStream: {} as unknown,
    readyNodes: (() => []) as Accessor<unknown[]>,
    failedPreflightNodes: (() => []) as Accessor<unknown[]>,
    confirmSelectedNodeIds: (() => new Set<string>()) as Accessor<Set<string>>,
    toggleConfirmNode: vi.fn(),
    jobId: (() => '') as Accessor<string>,
    jobStatus: (() => '') as Accessor<string>,
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
    startPreflight: vi.fn(),
    startDeploy: vi.fn(),
    cancelDeploy: vi.fn(),
    retryFailed: vi.fn(),
  } as unknown as DeployWizardState;
}

/** Returns the progress summary element (the one with aria-live="polite"). */
function getProgressSummary(): HTMLElement {
  const el = document.querySelector('[role="status"][aria-live="polite"]');
  if (!el) throw new Error('Progress summary not found');
  return el as HTMLElement;
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

describe('DeployingStep', () => {
  /* ── Deploy error display ─────────────────────────────────────── */

  describe('deploy error', () => {
    it('shows error alert when deployError is set', () => {
      const wizard = createMockWizard({ deployError: 'Connection lost to source agent' });
      render(() => <DeployingStep wizard={wizard} />);

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
      expect(alert).toHaveTextContent('Connection lost to source agent');
    });

    it('does not show error alert when deployError is empty', () => {
      const wizard = createMockWizard({ deployError: '' });
      render(() => <DeployingStep wizard={wizard} />);

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  /* ── Progress summary ─────────────────────────────────────────── */

  describe('progress summary', () => {
    it('shows "Installing 0 of N nodes..." when all are pending', () => {
      const targets = [
        makeTarget({ id: 't1', nodeName: 'pve1', status: 'pending' }),
        makeTarget({ id: 't2', nodeName: 'pve2', status: 'pending' }),
        makeTarget({ id: 't3', nodeName: 'pve3', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 0 of 3 nodes...');
    });

    it('counts succeeded targets as completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 1 of 2 nodes...');
    });

    it('counts failed_retryable as completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'failed_retryable' }),
        makeTarget({ id: 't2', status: 'installing' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 1 of 2 nodes...');
    });

    it('counts failed_permanent as completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'failed_permanent' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 1 of 2 nodes...');
    });

    it('counts skipped_already_agent as completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'skipped_already_agent' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 1 of 2 nodes...');
    });

    it('counts skipped_license as completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'skipped_license' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 1 of 2 nodes...');
    });

    it('counts canceled as completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'canceled' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('Installing 1 of 2 nodes...');
    });

    it('shows in-progress count for installing/enrolling/verifying targets', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'installing' }),
        makeTarget({ id: 't2', status: 'enrolling' }),
        makeTarget({ id: 't3', status: 'verifying' }),
        makeTarget({ id: 't4', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).toHaveTextContent('(3 in progress)');
    });

    it('does not show in-progress count when no targets are in progress', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(getProgressSummary()).not.toHaveTextContent('in progress');
    });

    it('shows spinner icon when deployment is still in progress', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', status: 'installing' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      const { container } = render(() => <DeployingStep wizard={wizard} />);

      // Loader icon has animate-spin class
      const spinner = container.querySelector('.animate-spin');
      expect(spinner).toBeInTheDocument();
    });

    it('shows check icon when all targets are completed', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', status: 'failed_permanent' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      const { container } = render(() => <DeployingStep wizard={wizard} />);

      // Check icon has text-emerald-500 class
      const checkIcon = container.querySelector('.text-emerald-500');
      expect(checkIcon).toBeInTheDocument();
      // No spinner
      expect(container.querySelector('.animate-spin')).not.toBeInTheDocument();
    });
  });

  /* ── Table rendering ──────────────────────────────────────────── */

  describe('table rendering', () => {
    it('renders table headers', () => {
      const wizard = createMockWizard({
        jobTargets: [makeTarget()],
      });
      render(() => <DeployingStep wizard={wizard} />);

      expect(screen.getByText('Node')).toBeInTheDocument();
      expect(screen.getByText('IP')).toBeInTheDocument();
      expect(screen.getByText('Status')).toBeInTheDocument();
      expect(screen.getByText('Details')).toBeInTheDocument();
    });

    it('renders a row for each target with node name and IP', () => {
      const targets = [
        makeTarget({ id: 't1', nodeName: 'pve1', nodeIP: '10.0.0.1', status: 'installing' }),
        makeTarget({ id: 't2', nodeName: 'pve2', nodeIP: '10.0.0.2', status: 'succeeded' }),
        makeTarget({ id: 't3', nodeName: 'pve3', nodeIP: '10.0.0.3', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(screen.getByText('pve1')).toBeInTheDocument();
      expect(screen.getByText('pve2')).toBeInTheDocument();
      expect(screen.getByText('pve3')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.2')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.3')).toBeInTheDocument();
    });

    it('renders DeployStatusBadge for each target', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', status: 'failed_permanent' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      // DeployStatusBadge renders role="status" spans; progress summary also has role="status"
      const allStatusElements = screen.getAllByRole('status');
      // Exactly 1 progress summary + 2 badges = 3
      expect(allStatusElements).toHaveLength(3);
    });

    it('renders empty table body when no targets', () => {
      const wizard = createMockWizard({ jobTargets: [] });
      render(() => <DeployingStep wizard={wizard} />);

      // Table headers should still appear
      expect(screen.getByText('Node')).toBeInTheDocument();
      // Progress shows 0 of 0
      expect(getProgressSummary()).toHaveTextContent('Installing 0 of 0 nodes...');
    });
  });

  /* ── Mixed status scenario ────────────────────────────────────── */

  describe('mixed status scenario', () => {
    it('correctly computes counts with all deploy-phase statuses', () => {
      // Note: preflighting and ready are preflight-phase statuses,
      // not expected in jobTargets during the deploying step.
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', status: 'failed_retryable' }),
        makeTarget({ id: 't3', status: 'failed_permanent' }),
        makeTarget({ id: 't4', status: 'skipped_already_agent' }),
        makeTarget({ id: 't5', status: 'skipped_license' }),
        makeTarget({ id: 't6', status: 'canceled' }),
        makeTarget({ id: 't7', status: 'installing' }),
        makeTarget({ id: 't8', status: 'enrolling' }),
        makeTarget({ id: 't9', status: 'verifying' }),
        makeTarget({ id: 't10', status: 'pending' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      // 6 completed (succeeded, failed_retryable, failed_permanent, skipped_already_agent, skipped_license, canceled)
      // 3 in progress (installing, enrolling, verifying)
      // 1 pending
      const summary = getProgressSummary();
      expect(summary).toHaveTextContent('Installing 6 of 10 nodes...');
      expect(summary).toHaveTextContent('(3 in progress)');
    });

    it('treats preflighting and ready as neither completed nor in-progress', () => {
      // These are preflight-phase statuses — if they appear in jobTargets,
      // they should not be counted as completed or in-progress.
      const targets = [
        makeTarget({ id: 't1', status: 'preflighting' }),
        makeTarget({ id: 't2', status: 'ready' }),
        makeTarget({ id: 't3', status: 'succeeded' }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      // Only succeeded counts as completed
      const summary = getProgressSummary();
      expect(summary).toHaveTextContent('Installing 1 of 3 nodes...');
      // No in-progress since preflighting/ready aren't in the inProgressStatuses set
      expect(summary).not.toHaveTextContent('in progress');
    });
  });

  /* ── Error detail rendering ───────────────────────────────────── */

  describe('error details', () => {
    it('shows error message on failed targets', () => {
      const targets = [
        makeTarget({
          id: 't1',
          status: 'failed_permanent',
          errorMessage: 'SSH connection refused',
        }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      expect(screen.getByText('SSH connection refused')).toBeInTheDocument();
    });

    it('does not show error detail for targets without error messages', () => {
      const targets = [
        makeTarget({ id: 't1', status: 'succeeded', errorMessage: undefined }),
      ];
      const wizard = createMockWizard({ jobTargets: targets });
      render(() => <DeployingStep wizard={wizard} />);

      // The Details column cell (4th td) should be empty
      const rows = screen.getAllByRole('row');
      const detailsCells = rows[1].querySelectorAll('td');
      // 4th cell is the Details column
      expect(detailsCells[3].textContent).toBe('');
      // No expand/collapse buttons from ErrorDetail
      expect(screen.queryByText('more')).not.toBeInTheDocument();
      expect(screen.queryByText('less')).not.toBeInTheDocument();
    });
  });

  /* ── Both deploy error and target errors ──────────────────────── */

  describe('deploy error with target errors', () => {
    it('shows both deploy-level error and per-target errors', () => {
      const targets = [
        makeTarget({
          id: 't1',
          nodeName: 'pve1',
          status: 'failed_permanent',
          errorMessage: 'Permission denied',
        }),
        makeTarget({
          id: 't2',
          nodeName: 'pve2',
          status: 'succeeded',
        }),
      ];
      const wizard = createMockWizard({
        jobTargets: targets,
        deployError: 'Job timed out',
      });
      render(() => <DeployingStep wizard={wizard} />);

      // Deploy-level error
      expect(screen.getByRole('alert')).toHaveTextContent('Job timed out');
      // Per-target error
      expect(screen.getByText('Permission denied')).toBeInTheDocument();
      // Node names still visible
      expect(screen.getByText('pve1')).toBeInTheDocument();
      expect(screen.getByText('pve2')).toBeInTheDocument();
    });
  });
});
