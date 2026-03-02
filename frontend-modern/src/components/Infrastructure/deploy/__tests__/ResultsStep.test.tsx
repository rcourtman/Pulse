import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@solidjs/testing-library';
import type { Accessor } from 'solid-js';
import type { DeployTarget, SourceAgentInfo } from '@/types/agentDeploy';
import type { DeployWizardState } from '@/hooks/useDeployWizard';

/* ── hoisted mocks (must be before imports) ──────────────────── */

const { nodesAPIMock, copyToClipboardMock } = vi.hoisted(() => ({
  nodesAPIMock: {
    getAgentInstallCommand: vi.fn(),
  },
  copyToClipboardMock: vi.fn(),
}));

vi.mock('@/api/nodes', () => ({ NodesAPI: nodesAPIMock }));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: copyToClipboardMock }));

// Stub lucide icons — they don't render visually in jsdom
vi.mock('lucide-solid/icons/check-circle-2', () => ({
  default: () => <span data-testid="icon-check-circle" />,
}));
vi.mock('lucide-solid/icons/x-circle', () => ({
  default: () => <span data-testid="icon-x-circle" />,
}));
vi.mock('lucide-solid/icons/alert-circle', () => ({
  default: () => <span data-testid="icon-alert-circle" />,
}));
vi.mock('lucide-solid/icons/chevron-down', () => ({
  default: () => <span data-testid="icon-chevron-down" />,
}));
vi.mock('lucide-solid/icons/chevron-right', () => ({
  default: () => <span data-testid="icon-chevron-right" />,
}));
vi.mock('lucide-solid/icons/copy', () => ({
  default: () => <span data-testid="icon-copy" />,
}));
vi.mock('lucide-solid/icons/check', () => ({
  default: () => <span data-testid="icon-check" />,
}));

/* ── component import ──────────────────────────────────────────── */
import { ResultsStep } from '../ResultsStep';

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

/** Creates a mock DeployWizardState with sensible defaults for ResultsStep. */
function createMockWizard(
  overrides: Partial<{
    succeededTargets: DeployTarget[];
    failedTargets: DeployTarget[];
    skippedTargets: DeployTarget[];
    canceledTargets: DeployTarget[];
  }> = {},
): DeployWizardState {
  return {
    succeededTargets: (() => overrides.succeededTargets ?? []) as Accessor<DeployTarget[]>,
    failedTargets: (() => overrides.failedTargets ?? []) as Accessor<DeployTarget[]>,
    skippedTargets: (() => overrides.skippedTargets ?? []) as Accessor<DeployTarget[]>,
    canceledTargets: (() => overrides.canceledTargets ?? []) as Accessor<DeployTarget[]>,

    // Stubs for fields not used by ResultsStep but required by the type.
    step: (() => 'results') as Accessor<string>,
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
    jobTargets: (() => []) as Accessor<DeployTarget[]>,
    deployError: (() => '') as Accessor<string>,
    deployStream: {} as unknown,
    retryableTargets: (() => []) as Accessor<unknown[]>,
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

/* ── lifecycle ──────────────────────────────────────────────────── */
beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
});

/* ================================================================
   Tests
   ================================================================ */

describe('ResultsStep', () => {
  /* ── Succeeded section ──────────────────────────────────────── */

  describe('succeeded targets', () => {
    it('shows the Deployed header with count when there are succeeded targets', () => {
      const wizard = createMockWizard({
        succeededTargets: [
          makeTarget({ id: 't1', nodeName: 'pve1', status: 'succeeded' }),
          makeTarget({ id: 't2', nodeName: 'pve2', status: 'succeeded' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('Deployed (2)')).toBeInTheDocument();
    });

    it('renders node names and IPs in the succeeded table', () => {
      const wizard = createMockWizard({
        succeededTargets: [
          makeTarget({ id: 't1', nodeName: 'pve1', nodeIP: '10.0.0.1', status: 'succeeded' }),
          makeTarget({ id: 't2', nodeName: 'pve2', nodeIP: '10.0.0.2', status: 'succeeded' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('pve1')).toBeInTheDocument();
      expect(screen.getByText('pve2')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.2')).toBeInTheDocument();
    });

    it('renders DeployStatusBadge for each succeeded target', () => {
      const wizard = createMockWizard({
        succeededTargets: [
          makeTarget({ id: 't1', status: 'succeeded' }),
          makeTarget({ id: 't2', status: 'succeeded' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      // DeployStatusBadge renders with role="status" — one per succeeded row
      const badges = screen.getAllByRole('status');
      expect(badges).toHaveLength(2);
    });

    it('does not show the Deployed section when there are no succeeded targets', () => {
      const wizard = createMockWizard({ succeededTargets: [] });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.queryByText(/Deployed \(/)).not.toBeInTheDocument();
    });
  });

  /* ── Failed section ─────────────────────────────────────────── */

  describe('failed targets', () => {
    it('shows the Failed header with count', () => {
      const wizard = createMockWizard({
        failedTargets: [
          makeTarget({ id: 't1', status: 'failed_permanent' }),
          makeTarget({ id: 't2', status: 'failed_retryable' }),
          makeTarget({ id: 't3', status: 'failed_permanent' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('Failed (3)')).toBeInTheDocument();
    });

    it('renders node names, IPs, and error details for failed targets', () => {
      const wizard = createMockWizard({
        failedTargets: [
          makeTarget({
            id: 't1',
            nodeName: 'pve3',
            nodeIP: '10.0.0.3',
            status: 'failed_permanent',
            errorMessage: 'SSH connection refused',
          }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('pve3')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.3')).toBeInTheDocument();
      expect(screen.getByText('SSH connection refused')).toBeInTheDocument();
    });

    it('does not show the Failed section when there are no failed targets', () => {
      const wizard = createMockWizard({ failedTargets: [] });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.queryByText(/Failed \(/)).not.toBeInTheDocument();
    });
  });

  /* ── Skipped section ────────────────────────────────────────── */

  describe('skipped targets', () => {
    it('shows the Skipped header with count', () => {
      const wizard = createMockWizard({
        skippedTargets: [
          makeTarget({ id: 't1', status: 'skipped_already_agent' }),
          makeTarget({ id: 't2', status: 'skipped_license' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('Skipped (2)')).toBeInTheDocument();
    });

    it('renders node names and IPs in the skipped table', () => {
      const wizard = createMockWizard({
        skippedTargets: [
          makeTarget({ id: 't1', nodeName: 'pve4', nodeIP: '10.0.0.4', status: 'skipped_already_agent' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('pve4')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.4')).toBeInTheDocument();
    });

    it('does not show the Skipped section when there are no skipped targets', () => {
      const wizard = createMockWizard({ skippedTargets: [] });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.queryByText(/Skipped \(/)).not.toBeInTheDocument();
    });
  });

  /* ── Canceled section ───────────────────────────────────────── */

  describe('canceled targets', () => {
    it('shows the Canceled header with count', () => {
      const wizard = createMockWizard({
        canceledTargets: [
          makeTarget({ id: 't1', status: 'canceled' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('Canceled (1)')).toBeInTheDocument();
    });

    it('renders node names and IPs in the canceled table', () => {
      const wizard = createMockWizard({
        canceledTargets: [
          makeTarget({ id: 't1', nodeName: 'pve5', nodeIP: '10.0.0.5', status: 'canceled' }),
        ],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('pve5')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.5')).toBeInTheDocument();
    });

    it('does not show the Canceled section when there are no canceled targets', () => {
      const wizard = createMockWizard({ canceledTargets: [] });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.queryByText(/Canceled \(/)).not.toBeInTheDocument();
    });
  });

  /* ── Empty state ────────────────────────────────────────────── */

  describe('empty state', () => {
    it('renders nothing visible when all target arrays are empty', () => {
      const wizard = createMockWizard();
      const { container } = render(() => <ResultsStep wizard={wizard} />);

      // The root div exists but has no visible sections
      expect(screen.queryByText(/Deployed/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Failed/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Skipped/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Canceled/)).not.toBeInTheDocument();
      // Only the outer container div
      expect(container.querySelector('table')).not.toBeInTheDocument();
    });
  });

  /* ── Multiple sections ──────────────────────────────────────── */

  describe('multiple sections visible', () => {
    it('renders all four sections when each has targets', () => {
      const wizard = createMockWizard({
        succeededTargets: [makeTarget({ id: 's1', nodeName: 'pve-ok', status: 'succeeded' })],
        failedTargets: [makeTarget({ id: 'f1', nodeName: 'pve-fail', status: 'failed_permanent' })],
        skippedTargets: [makeTarget({ id: 'sk1', nodeName: 'pve-skip', status: 'skipped_already_agent' })],
        canceledTargets: [makeTarget({ id: 'c1', nodeName: 'pve-cancel', status: 'canceled' })],
      });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.getByText('Deployed (1)')).toBeInTheDocument();
      expect(screen.getByText('Failed (1)')).toBeInTheDocument();
      expect(screen.getByText('Skipped (1)')).toBeInTheDocument();
      expect(screen.getByText('Canceled (1)')).toBeInTheDocument();

      expect(screen.getByText('pve-ok')).toBeInTheDocument();
      expect(screen.getByText('pve-fail')).toBeInTheDocument();
      expect(screen.getByText('pve-skip')).toBeInTheDocument();
      expect(screen.getByText('pve-cancel')).toBeInTheDocument();
    });
  });

  /* ── Manual install accordion ───────────────────────────────── */

  describe('manual install accordion', () => {
    function renderWithFailedTargets() {
      const wizard = createMockWizard({
        failedTargets: [
          makeTarget({ id: 'f1', status: 'failed_permanent', errorMessage: 'Timeout' }),
        ],
      });
      return render(() => <ResultsStep wizard={wizard} />);
    }

    it('shows the "Manual install instructions" button when failed targets exist', () => {
      renderWithFailedTargets();

      expect(screen.getByText('Manual install instructions')).toBeInTheDocument();
    });

    it('does not show manual install button when there are no failed targets', () => {
      const wizard = createMockWizard({ failedTargets: [] });
      render(() => <ResultsStep wizard={wizard} />);

      expect(screen.queryByText('Manual install instructions')).not.toBeInTheDocument();
    });

    it('starts with accordion collapsed (aria-expanded=false)', () => {
      renderWithFailedTargets();

      const btn = screen.getByText('Manual install instructions');
      expect(btn.closest('button')).toHaveAttribute('aria-expanded', 'false');
    });

    it('expands the accordion on click and fetches install command', async () => {
      nodesAPIMock.getAgentInstallCommand.mockResolvedValueOnce({ command: 'curl -s https://example.com | bash' });
      renderWithFailedTargets();

      const btn = screen.getByText('Manual install instructions');
      fireEvent.click(btn);

      // Button should now be expanded
      expect(btn.closest('button')).toHaveAttribute('aria-expanded', 'true');

      // Should have called the API
      expect(nodesAPIMock.getAgentInstallCommand).toHaveBeenCalledWith({
        type: 'pve',
        enableProxmox: true,
      });

      // Instruction text appears
      expect(screen.getByText(/For nodes that failed SSH-based deployment/)).toBeInTheDocument();

      // Wait for command to load
      await waitFor(() => {
        expect(screen.getByText('curl -s https://example.com | bash')).toBeInTheDocument();
      });
    });

    it('collapses the accordion on second click', async () => {
      nodesAPIMock.getAgentInstallCommand.mockResolvedValueOnce({ command: 'curl ...' });
      renderWithFailedTargets();

      const btn = screen.getByText('Manual install instructions');
      fireEvent.click(btn); // open
      expect(btn.closest('button')).toHaveAttribute('aria-expanded', 'true');

      fireEvent.click(btn); // close
      expect(btn.closest('button')).toHaveAttribute('aria-expanded', 'false');
    });

    it('shows loading state while fetching the install command', async () => {
      // Never-resolving promise to keep loading
      nodesAPIMock.getAgentInstallCommand.mockReturnValueOnce(new Promise(() => {}));
      renderWithFailedTargets();

      fireEvent.click(screen.getByText('Manual install instructions'));

      expect(screen.getByText('Loading install command...')).toBeInTheDocument();
    });

    it('shows error when install command fetch fails', async () => {
      nodesAPIMock.getAgentInstallCommand.mockRejectedValueOnce(new Error('Network error'));
      renderWithFailedTargets();

      fireEvent.click(screen.getByText('Manual install instructions'));

      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent('Network error');
      });
    });

    it('shows generic error message for non-Error exceptions', async () => {
      nodesAPIMock.getAgentInstallCommand.mockRejectedValueOnce('string error');
      renderWithFailedTargets();

      fireEvent.click(screen.getByText('Manual install instructions'));

      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent('Failed to load install command');
      });
    });

    it('only fetches the install command once even if accordion is toggled multiple times', async () => {
      nodesAPIMock.getAgentInstallCommand.mockResolvedValueOnce({ command: 'curl install' });
      renderWithFailedTargets();

      const btn = screen.getByText('Manual install instructions');
      fireEvent.click(btn); // open — triggers fetch

      await waitFor(() => {
        expect(screen.getByText('curl install')).toBeInTheDocument();
      });

      fireEvent.click(btn); // close
      fireEvent.click(btn); // open again — should NOT fetch again

      expect(nodesAPIMock.getAgentInstallCommand).toHaveBeenCalledTimes(1);
    });

    it('does not call API before accordion is opened', () => {
      renderWithFailedTargets();

      // Accordion exists but API should not have been called
      expect(nodesAPIMock.getAgentInstallCommand).not.toHaveBeenCalled();
    });

    it('allows retry after fetch error', async () => {
      nodesAPIMock.getAgentInstallCommand
        .mockRejectedValueOnce(new Error('first failure'))
        .mockResolvedValueOnce({ command: 'curl retry' });
      renderWithFailedTargets();

      const btn = screen.getByText('Manual install instructions');
      fireEvent.click(btn); // open — triggers fetch (will fail)

      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent('first failure');
      });

      // Close and reopen to retry
      fireEvent.click(btn);
      fireEvent.click(btn);

      await waitFor(() => {
        expect(screen.getByText('curl retry')).toBeInTheDocument();
      });

      expect(nodesAPIMock.getAgentInstallCommand).toHaveBeenCalledTimes(2);
    });

    it('clears error UI when retry succeeds', async () => {
      nodesAPIMock.getAgentInstallCommand
        .mockRejectedValueOnce(new Error('temporary failure'))
        .mockResolvedValueOnce({ command: 'curl success' });
      renderWithFailedTargets();

      const btn = screen.getByText('Manual install instructions');
      fireEvent.click(btn);

      // Error appears
      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent('temporary failure');
      });

      // Close and reopen to retry
      fireEvent.click(btn);
      fireEvent.click(btn);

      // Error should be cleared and command shown
      await waitFor(() => {
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        expect(screen.getByText('curl success')).toBeInTheDocument();
      });
    });
  });

  /* ── Copy button ────────────────────────────────────────────── */

  describe('copy to clipboard', () => {
    async function renderWithCommand() {
      nodesAPIMock.getAgentInstallCommand.mockResolvedValueOnce({ command: 'curl install-cmd' });
      const wizard = createMockWizard({
        failedTargets: [makeTarget({ id: 'f1', status: 'failed_permanent' })],
      });
      render(() => <ResultsStep wizard={wizard} />);

      // Open accordion and wait for command
      fireEvent.click(screen.getByText('Manual install instructions'));
      await waitFor(() => {
        expect(screen.getByText('curl install-cmd')).toBeInTheDocument();
      });
    }

    it('renders the copy button with "Copy to clipboard" label', async () => {
      await renderWithCommand();

      expect(screen.getByLabelText('Copy to clipboard')).toBeInTheDocument();
    });

    it('copies the install command on click', async () => {
      copyToClipboardMock.mockResolvedValueOnce(true);
      await renderWithCommand();

      fireEvent.click(screen.getByLabelText('Copy to clipboard'));

      expect(copyToClipboardMock).toHaveBeenCalledWith('curl install-cmd');
    });

    it('shows "Copied" label after successful copy', async () => {
      copyToClipboardMock.mockResolvedValueOnce(true);
      await renderWithCommand();

      fireEvent.click(screen.getByLabelText('Copy to clipboard'));

      await waitFor(() => {
        expect(screen.getByLabelText('Copied')).toBeInTheDocument();
      });
    });

    it('does not change label when copy fails', async () => {
      copyToClipboardMock.mockResolvedValueOnce(false);
      await renderWithCommand();

      fireEvent.click(screen.getByLabelText('Copy to clipboard'));

      // Wait for the async copy call to resolve, then verify label is unchanged
      await waitFor(() => {
        expect(copyToClipboardMock).toHaveBeenCalled();
      });
      expect(screen.getByLabelText('Copy to clipboard')).toBeInTheDocument();
      expect(screen.queryByLabelText('Copied')).not.toBeInTheDocument();
    });

    it('reverts "Copied" label back to "Copy to clipboard" after timeout', async () => {
      vi.useFakeTimers();
      copyToClipboardMock.mockResolvedValueOnce(true);
      await renderWithCommand();

      fireEvent.click(screen.getByLabelText('Copy to clipboard'));

      await waitFor(() => {
        expect(screen.getByLabelText('Copied')).toBeInTheDocument();
      });

      // Advance past the 2s timer
      vi.advanceTimersByTime(2100);

      await waitFor(() => {
        expect(screen.getByLabelText('Copy to clipboard')).toBeInTheDocument();
      });

      vi.useRealTimers();
    });
  });
});
