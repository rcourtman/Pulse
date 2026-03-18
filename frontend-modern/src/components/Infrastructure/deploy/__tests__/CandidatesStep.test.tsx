import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';
import type { CandidateNode, SourceAgentInfo } from '@/types/agentDeploy';
import type { DeployWizardState } from '@/hooks/useDeployWizard';

/* ── component import ──────────────────────────────────────────── */
import { CandidatesStep } from '../CandidatesStep';

/* ── helpers ────────────────────────────────────────────────────── */

/** Creates a mock DeployWizardState with sensible defaults. */
function createMockWizard(
  overrides: Partial<{
    candidates: CandidateNode[];
    candidatesLoading: boolean;
    candidatesError: string;
    selectedNodeIds: Set<string>;
    deployableNodes: CandidateNode[];
    onlineSourceAgents: SourceAgentInfo[];
    selectedSourceAgent: string;
  }> = {},
): DeployWizardState {
  const candidatesData = overrides.candidates ?? [];
  const deployableData =
    overrides.deployableNodes ?? candidatesData.filter((n) => n.deployable && !n.hasAgent);
  const [candidatesLoading] = createSignal(overrides.candidatesLoading ?? false);
  const [candidatesError] = createSignal(overrides.candidatesError ?? '');
  const [selectedNodeIds, setSelectedNodeIds] = createSignal<Set<string>>(
    overrides.selectedNodeIds ?? new Set<string>(),
  );
  const [selectedSourceAgent, setSelectedSourceAgent] = createSignal(
    overrides.selectedSourceAgent ?? '',
  );

  const toggleNodeSelection = vi.fn((nodeId: string) => {
    setSelectedNodeIds((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  });

  const selectAllNodes = vi.fn(() => {
    setSelectedNodeIds(new Set(deployableData.map((n) => n.nodeId)));
  });

  const deselectAllNodes = vi.fn(() => {
    setSelectedNodeIds(new Set<string>());
  });

  // Build a minimal mock that satisfies the fields CandidatesStep accesses.
  // CandidatesStep only reads: candidatesError, candidatesLoading, candidates,
  // onlineSourceAgents, selectedSourceAgent, setSelectedSourceAgent,
  // deployableNodes, selectedNodeIds, selectAllNodes, deselectAllNodes,
  // toggleNodeSelection.
  return {
    candidates: (() => candidatesData) as Accessor<CandidateNode[]>,
    candidatesLoading,
    candidatesError,
    selectedNodeIds,
    deployableNodes: (() => deployableData) as Accessor<CandidateNode[]>,
    onlineSourceAgents: (() => overrides.onlineSourceAgents ?? []) as Accessor<SourceAgentInfo[]>,
    selectedSourceAgent,
    setSelectedSourceAgent,
    toggleNodeSelection,
    selectAllNodes,
    deselectAllNodes,
    // Stubs for fields not used by CandidatesStep but required by the type.
    step: (() => 'candidates') as Accessor<string>,
    setStep: vi.fn(),
    sourceAgents: (() => []) as Accessor<SourceAgentInfo[]>,
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

function makeCandidate(overrides: Partial<CandidateNode> = {}): CandidateNode {
  return {
    nodeId: 'node-1',
    name: 'pve1',
    ip: '192.168.1.10',
    hasAgent: false,
    deployable: true,
    ...overrides,
  };
}

function makeSourceAgent(overrides: Partial<SourceAgentInfo> = {}): SourceAgentInfo {
  return {
    agentId: 'agent-1',
    nodeId: 'node-1',
    online: true,
    ...overrides,
  };
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

describe('CandidatesStep', () => {
  /* ── Loading state ──────────────────────────────────────────── */

  describe('loading state', () => {
    it('shows a loading spinner and message while loading', () => {
      const wizard = createMockWizard({ candidatesLoading: true });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Loading cluster nodes...')).toBeInTheDocument();
    });

    it('does not show node table or empty state while loading', () => {
      const wizard = createMockWizard({ candidatesLoading: true });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.queryByText('Select All')).not.toBeInTheDocument();
      expect(screen.queryByText('No nodes found in this cluster.')).not.toBeInTheDocument();
    });
  });

  /* ── Error state ────────────────────────────────────────────── */

  describe('error state', () => {
    it('displays the error message', () => {
      const wizard = createMockWizard({ candidatesError: 'Network timeout' });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Network timeout')).toBeInTheDocument();
    });
  });

  /* ── Empty state ────────────────────────────────────────────── */

  describe('empty state', () => {
    it('shows empty message when no candidates and not loading', () => {
      const wizard = createMockWizard({ candidates: [], candidatesLoading: false });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('No nodes found in this cluster.')).toBeInTheDocument();
    });
  });

  /* ── No online source agents ────────────────────────────────── */

  describe('no online source agents', () => {
    it('shows a warning when no online source agents are available', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        onlineSourceAgents: [],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText(/No online source agents found/)).toBeInTheDocument();
    });
  });

  /* ── Source agent selector ──────────────────────────────────── */

  describe('source agent selector', () => {
    it('renders a source agent dropdown when there are multiple online agents', () => {
      const agents = [
        makeSourceAgent({ agentId: 'a1', nodeId: 'n1' }),
        makeSourceAgent({ agentId: 'a2', nodeId: 'n2' }),
      ];
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        onlineSourceAgents: agents,
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const select = screen.getByLabelText('Source Agent') as HTMLSelectElement;
      expect(select).toBeInTheDocument();
      // It should have a placeholder + 2 agent options
      expect(select.options.length).toBe(3);
    });

    it('does not render a source agent dropdown when there is exactly one agent', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        onlineSourceAgents: [makeSourceAgent()],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.queryByLabelText('Source Agent')).not.toBeInTheDocument();
    });

    it('calls setSelectedSourceAgent on dropdown change', () => {
      const agents = [
        makeSourceAgent({ agentId: 'a1', nodeId: 'n1' }),
        makeSourceAgent({ agentId: 'a2', nodeId: 'n2' }),
      ];
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        onlineSourceAgents: agents,
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const select = screen.getByLabelText('Source Agent') as HTMLSelectElement;
      fireEvent.change(select, { target: { value: 'a2' } });
      // The setSelectedSourceAgent is the real signal setter, check value updated
      expect(wizard.selectedSourceAgent()).toBe('a2');
    });
  });

  /* ── Node table rendering ──────────────────────────────────── */

  describe('node table', () => {
    it('renders a row for each candidate node', () => {
      const candidates = [
        makeCandidate({ nodeId: 'n1', name: 'pve1', ip: '10.0.0.1' }),
        makeCandidate({ nodeId: 'n2', name: 'pve2', ip: '10.0.0.2' }),
        makeCandidate({ nodeId: 'n3', name: 'pve3', ip: '10.0.0.3' }),
      ];
      const wizard = createMockWizard({ candidates, candidatesLoading: false });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('pve1')).toBeInTheDocument();
      expect(screen.getByText('pve2')).toBeInTheDocument();
      expect(screen.getByText('pve3')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.2')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.3')).toBeInTheDocument();
    });

    it('shows "--" when a node has no IP', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ ip: '' })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('--')).toBeInTheDocument();
    });

    it('shows "Already monitored" for nodes with agents', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ hasAgent: true })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Already monitored')).toBeInTheDocument();
    });

    it('shows "Available" for deployable nodes without agents', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ deployable: true, hasAgent: false })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Available')).toBeInTheDocument();
    });

    it('shows "Not deployable" for non-deployable nodes without a reason', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ deployable: false, hasAgent: false })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Not deployable')).toBeInTheDocument();
    });

    it('shows the custom reason for non-deployable nodes', () => {
      const wizard = createMockWizard({
        candidates: [
          makeCandidate({
            deployable: false,
            hasAgent: false,
            reason: 'Offline (last seen 2h ago)',
          }),
        ],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Offline (last seen 2h ago)')).toBeInTheDocument();
    });

    it('renders table headers', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Node')).toBeInTheDocument();
      expect(screen.getByText('IP')).toBeInTheDocument();
      expect(screen.getByText('Status')).toBeInTheDocument();
    });
  });

  /* ── Selection count display ────────────────────────────────── */

  describe('selection count', () => {
    it('shows "X of Y nodes selected"', () => {
      const candidates = [
        makeCandidate({ nodeId: 'n1', name: 'pve1' }),
        makeCandidate({ nodeId: 'n2', name: 'pve2' }),
        makeCandidate({ nodeId: 'n3', name: 'pve3' }),
      ];
      const wizard = createMockWizard({
        candidates,
        selectedNodeIds: new Set(['n1', 'n3']),
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('2 of 3 nodes selected')).toBeInTheDocument();
    });
  });

  /* ── Select All / Deselect All ──────────────────────────────── */

  describe('select/deselect all', () => {
    it('renders Select All and Deselect All buttons', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Select All')).toBeInTheDocument();
      expect(screen.getByText('Deselect All')).toBeInTheDocument();
    });

    it('calls selectAllNodes when Select All is clicked', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1' }), makeCandidate({ nodeId: 'n2' })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      fireEvent.click(screen.getByText('Select All'));
      expect(wizard.selectAllNodes).toHaveBeenCalledOnce();
    });

    it('calls deselectAllNodes when Deselect All is clicked', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        selectedNodeIds: new Set(['node-1']),
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      fireEvent.click(screen.getByText('Deselect All'));
      expect(wizard.deselectAllNodes).toHaveBeenCalledOnce();
    });

    it('disables Select All when all deployable nodes are selected', () => {
      const candidates = [makeCandidate({ nodeId: 'n1' })];
      const wizard = createMockWizard({
        candidates,
        selectedNodeIds: new Set(['n1']),
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const selectAllBtn = screen.getByText('Select All');
      expect(selectAllBtn).toBeDisabled();
    });

    it('disables Deselect All when no nodes are selected', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate()],
        selectedNodeIds: new Set<string>(),
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const deselectAllBtn = screen.getByText('Deselect All');
      expect(deselectAllBtn).toBeDisabled();
    });
  });

  /* ── Checkbox / row interaction ─────────────────────────────── */

  describe('node selection interaction', () => {
    it('calls toggleNodeSelection exactly once when a deployable row is clicked', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1', name: 'pve1' })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      fireEvent.click(screen.getByText('pve1'));
      expect(wizard.toggleNodeSelection).toHaveBeenCalledOnce();
      expect(wizard.toggleNodeSelection).toHaveBeenCalledWith('n1');
    });

    it('calls toggleNodeSelection exactly once when the checkbox itself is clicked (no double-toggle)', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1', name: 'pve1' })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);
      // The checkbox has stopPropagation + onChange, and the row has onClick.
      // Clicking the checkbox directly should trigger toggle exactly once, not twice.
      expect(wizard.toggleNodeSelection).toHaveBeenCalledOnce();
      expect(wizard.toggleNodeSelection).toHaveBeenCalledWith('n1');
    });

    it('does not call toggleNodeSelection when clicking a non-deployable row', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1', name: 'pve1', deployable: false })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      fireEvent.click(screen.getByText('pve1'));
      expect(wizard.toggleNodeSelection).not.toHaveBeenCalled();
    });

    it('does not call toggleNodeSelection when clicking a row that already has an agent', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1', name: 'pve1', hasAgent: true })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      fireEvent.click(screen.getByText('pve1'));
      expect(wizard.toggleNodeSelection).not.toHaveBeenCalled();
    });

    it('disables checkbox for nodes with agents', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ hasAgent: true })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeDisabled();
    });

    it('disables checkbox for non-deployable nodes', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ deployable: false })],
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeDisabled();
    });

    it('shows checkbox as checked for selected nodes', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1' })],
        selectedNodeIds: new Set(['n1']),
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeChecked();
    });

    it('shows checkbox as unchecked for unselected nodes', () => {
      const wizard = createMockWizard({
        candidates: [makeCandidate({ nodeId: 'n1' })],
        selectedNodeIds: new Set<string>(),
        candidatesLoading: false,
      });
      render(() => <CandidatesStep wizard={wizard} />);

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).not.toBeChecked();
    });
  });

  /* ── Mixed candidates scenario ──────────────────────────────── */

  describe('mixed candidates scenario', () => {
    it('renders correct status for each node type in a mixed list', () => {
      const candidates = [
        makeCandidate({ nodeId: 'n1', name: 'with-agent', hasAgent: true }),
        makeCandidate({ nodeId: 'n2', name: 'deployable', deployable: true, hasAgent: false }),
        makeCandidate({
          nodeId: 'n3',
          name: 'not-deployable',
          deployable: false,
          hasAgent: false,
          reason: 'Needs SSH key',
        }),
      ];
      const wizard = createMockWizard({
        candidates,
        candidatesLoading: false,
        // Only n2 is deployable
        deployableNodes: [candidates[1]],
        selectedNodeIds: new Set(['n2']),
      });
      render(() => <CandidatesStep wizard={wizard} />);

      expect(screen.getByText('Already monitored')).toBeInTheDocument();
      expect(screen.getByText('Available')).toBeInTheDocument();
      expect(screen.getByText('Needs SSH key')).toBeInTheDocument();

      // Selection count shows 1 of 1 (only 1 deployable node)
      expect(screen.getByText('1 of 1 nodes selected')).toBeInTheDocument();

      // Should have 3 checkboxes total
      const checkboxes = screen.getAllByRole('checkbox');
      expect(checkboxes).toHaveLength(3);

      // First checkbox (agent present) and third (not deployable) should be disabled
      expect(checkboxes[0]).toBeDisabled();
      expect(checkboxes[2]).toBeDisabled();

      // Second checkbox (deployable) should be enabled and checked
      expect(checkboxes[1]).not.toBeDisabled();
      expect(checkboxes[1]).toBeChecked();

      // Select All should be disabled since all deployable nodes are selected
      expect(screen.getByText('Select All')).toBeDisabled();
    });
  });
});
