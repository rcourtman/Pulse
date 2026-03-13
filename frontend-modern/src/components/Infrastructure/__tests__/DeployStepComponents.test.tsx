import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import type { Accessor } from 'solid-js';
import type { DeployTarget } from '@/types/agentDeploy';

/* ── hoisted mocks ────────────────────────────────────────────── */

vi.mock('../deploy/DeployStatusBadge', () => ({
  DeployStatusBadge: (props: { status: string }) => (
    <span data-testid={`badge-${props.status}`}>{props.status}</span>
  ),
}));

vi.mock('lucide-solid/icons/check', () => ({ default: () => <span /> }));
vi.mock('lucide-solid/icons/alert-circle', () => ({ default: () => <span /> }));
vi.mock('lucide-solid/icons/check-circle-2', () => ({ default: () => <span /> }));
vi.mock('lucide-solid/icons/x-circle', () => ({ default: () => <span /> }));
vi.mock('lucide-solid/icons/loader-2', () => ({ default: () => <span /> }));
vi.mock('lucide-solid/icons/chevron-down', () => ({ default: () => <span /> }));
vi.mock('lucide-solid/icons/chevron-right', () => ({ default: () => <span /> }));

/* ── component imports (after mocks) ──────────────────────────── */

import { CandidatesStep } from '../deploy/CandidatesStep';
import { PreflightStep } from '../deploy/PreflightStep';
import { ConfirmStep } from '../deploy/ConfirmStep';
import { DeployingStep } from '../deploy/DeployingStep';
import { ResultsStep } from '../deploy/ResultsStep';

/* ── helpers ──────────────────────────────────────────────────── */

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

/** Creates a minimal mock wizard with only the signals needed by the component. */
function createMockWizard(overrides: Record<string, unknown> = {}) {
  const sig = <T,>(key: string, def: T): Accessor<T> => {
    const val = overrides[key] as T | undefined;
    return () => val ?? def;
  };

  return {
    // Candidates
    candidates: sig('candidates', []),
    candidatesLoading: sig('candidatesLoading', false),
    candidatesError: sig('candidatesError', ''),
    sourceAgents: sig('sourceAgents', []),
    onlineSourceAgents: sig('onlineSourceAgents', []),
    selectedSourceAgent: sig('selectedSourceAgent', ''),
    setSelectedSourceAgent: (overrides.setSelectedSourceAgent as (v: string) => void) ?? vi.fn(),
    selectedNodeIds: sig('selectedNodeIds', new Set<string>()),
    toggleNodeSelection: (overrides.toggleNodeSelection as (id: string) => void) ?? vi.fn(),
    selectAllNodes: (overrides.selectAllNodes as () => void) ?? vi.fn(),
    deselectAllNodes: (overrides.deselectAllNodes as () => void) ?? vi.fn(),
    deployableNodes: sig('deployableNodes', []),
    // Preflight
    preflightTargets: sig('preflightTargets', []),
    preflightError: sig('preflightError', ''),
    readyNodes: sig('readyNodes', []),
    failedPreflightNodes: sig('failedPreflightNodes', []),
    // Confirm
    confirmSelectedNodeIds: sig('confirmSelectedNodeIds', new Set<string>()),
    toggleConfirmNode: (overrides.toggleConfirmNode as (id: string) => void) ?? vi.fn(),
    maxAgentSlots: sig('maxAgentSlots', 0),
    // Deploy
    jobTargets: sig('jobTargets', []),
    deployError: sig('deployError', ''),
    // Results
    succeededTargets: sig('succeededTargets', []),
    failedTargets: sig('failedTargets', []),
    retryableTargets: sig('retryableTargets', []),
    skippedTargets: sig('skippedTargets', []),
    canceledTargets: sig('canceledTargets', []),
  } as never; // Cast to avoid full DeployWizardState typing
}

/* ── lifecycle ────────────────────────────────────────────────── */

beforeEach(() => vi.clearAllMocks());
afterEach(() => cleanup());

/* ================================================================
   CandidatesStep
   ================================================================ */

describe('CandidatesStep', () => {
  it('shows loading state', () => {
    const wizard = createMockWizard({ candidatesLoading: true });
    render(() => <CandidatesStep wizard={wizard} />);
    expect(screen.getByText('Loading cluster nodes...')).toBeInTheDocument();
  });

  it('shows error message', () => {
    const wizard = createMockWizard({ candidatesError: 'Network failure' });
    render(() => <CandidatesStep wizard={wizard} />);
    expect(screen.getByText('Network failure')).toBeInTheDocument();
  });

  it('shows empty state when no candidates', () => {
    const wizard = createMockWizard({ candidates: [] });
    render(() => <CandidatesStep wizard={wizard} />);
    expect(screen.getByText('No nodes found in this cluster.')).toBeInTheDocument();
  });

  it('renders candidate nodes in table', () => {
    const wizard = createMockWizard({
      candidates: [
        { nodeId: 'n1', name: 'pve-node1', ip: '10.0.0.1', hasAgent: false, deployable: true },
        { nodeId: 'n2', name: 'pve-node2', ip: '10.0.0.2', hasAgent: true, deployable: false },
      ],
      deployableNodes: [{ nodeId: 'n1' }],
      selectedNodeIds: new Set(['n1']),
    });
    render(() => <CandidatesStep wizard={wizard} />);

    expect(screen.getByText('pve-node1')).toBeInTheDocument();
    expect(screen.getByText('pve-node2')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
    expect(screen.getByText('Already monitored')).toBeInTheDocument();
  });

  it('shows selection count', () => {
    const wizard = createMockWizard({
      candidates: [{ nodeId: 'n1', name: 'a', ip: '1', hasAgent: false, deployable: true }],
      deployableNodes: [{ nodeId: 'n1' }],
      selectedNodeIds: new Set(['n1']),
    });
    render(() => <CandidatesStep wizard={wizard} />);
    expect(screen.getByText('1 of 1 nodes selected')).toBeInTheDocument();
  });

  it('shows source agent warning when none online', () => {
    const wizard = createMockWizard({
      candidates: [{ nodeId: 'n1', name: 'a', ip: '1', hasAgent: false, deployable: true }],
      onlineSourceAgents: [],
    });
    render(() => <CandidatesStep wizard={wizard} />);
    expect(screen.getByText(/No online source agents found/)).toBeInTheDocument();
  });

  it('shows source agent dropdown when multiple online', () => {
    const wizard = createMockWizard({
      candidates: [{ nodeId: 'n1', name: 'a', ip: '1', hasAgent: false, deployable: true }],
      onlineSourceAgents: [
        { agentId: 'a1', nodeId: 'n1', online: true },
        { agentId: 'a2', nodeId: 'n2', online: true },
      ],
      deployableNodes: [{ nodeId: 'n1' }],
      selectedNodeIds: new Set<string>(),
    });
    render(() => <CandidatesStep wizard={wizard} />);
    expect(screen.getByText('Source Agent')).toBeInTheDocument();
    expect(screen.getByText('Select a source agent...')).toBeInTheDocument();
  });

  it('calls toggleNodeSelection on row click', () => {
    const toggleMock = vi.fn();
    const wizard = createMockWizard({
      candidates: [{ nodeId: 'n1', name: 'clickable', ip: '1', hasAgent: false, deployable: true }],
      deployableNodes: [{ nodeId: 'n1' }],
      selectedNodeIds: new Set<string>(),
      toggleNodeSelection: toggleMock,
    });
    render(() => <CandidatesStep wizard={wizard} />);
    fireEvent.click(screen.getByText('clickable'));
    expect(toggleMock).toHaveBeenCalledWith('n1');
  });
});

/* ================================================================
   PreflightStep
   ================================================================ */

describe('PreflightStep', () => {
  it('shows progress when checks are ongoing', () => {
    const wizard = createMockWizard({
      preflightTargets: [
        makeTarget({ id: 't1', status: 'ready' }),
        makeTarget({ id: 't2', nodeName: 'node2', status: 'preflighting' }),
      ],
    });
    render(() => <PreflightStep wizard={wizard} />);
    expect(screen.getByText(/Checking 1 of 2 nodes/)).toBeInTheDocument();
  });

  it('shows completion when all checks done', () => {
    const wizard = createMockWizard({
      preflightTargets: [
        makeTarget({ id: 't1', status: 'ready' }),
        makeTarget({ id: 't2', status: 'failed_permanent' }),
      ],
    });
    render(() => <PreflightStep wizard={wizard} />);
    expect(screen.getByText('Preflight checks complete')).toBeInTheDocument();
  });

  it('renders target rows with status badges', () => {
    const wizard = createMockWizard({
      preflightTargets: [
        makeTarget({ id: 't1', nodeName: 'node-a', nodeIP: '10.0.0.1', status: 'ready' }),
      ],
    });
    render(() => <PreflightStep wizard={wizard} />);
    expect(screen.getByText('node-a')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
    expect(screen.getByTestId('badge-ready')).toBeInTheDocument();
  });

  it('shows error message for failed targets', () => {
    const wizard = createMockWizard({
      preflightTargets: [
        makeTarget({ status: 'failed_permanent', errorMessage: 'SSH unreachable' }),
      ],
    });
    render(() => <PreflightStep wizard={wizard} />);
    expect(screen.getByText('SSH unreachable')).toBeInTheDocument();
  });

  it('shows preflight error banner', () => {
    const wizard = createMockWizard({
      preflightError: 'Connection lost',
      preflightTargets: [],
    });
    render(() => <PreflightStep wizard={wizard} />);
    expect(screen.getByText('Connection lost')).toBeInTheDocument();
  });
});

/* ================================================================
   ConfirmStep
   ================================================================ */

describe('ConfirmStep', () => {
  it('renders ready nodes with checkboxes', () => {
    const wizard = createMockWizard({
      readyNodes: [
        makeTarget({ id: 't1', nodeId: 'n1', nodeName: 'node-1', nodeIP: '10.0.0.1', status: 'ready' }),
      ],
      confirmSelectedNodeIds: new Set(['n1']),
    });
    render(() => <ConfirmStep wizard={wizard} />);
    expect(screen.getByText('node-1')).toBeInTheDocument();
    expect(screen.getByText('Ready to deploy (1)')).toBeInTheDocument();
  });

  it('renders failed preflight nodes', () => {
    const wizard = createMockWizard({
      failedPreflightNodes: [
        makeTarget({ id: 't1', nodeName: 'bad-node', status: 'failed_permanent', errorMessage: 'SSH failed' }),
      ],
    });
    render(() => <ConfirmStep wizard={wizard} />);
    expect(screen.getByText('bad-node')).toBeInTheDocument();
    expect(screen.getByText('Cannot deploy (1)')).toBeInTheDocument();
    expect(screen.getByText('SSH failed')).toBeInTheDocument();
  });

  it('shows license slot info when limit exists', () => {
    const wizard = createMockWizard({
      maxAgentSlots: 5,
      confirmSelectedNodeIds: new Set(['n1', 'n2']),
      readyNodes: [],
    });
    render(() => <ConfirmStep wizard={wizard} />);
    expect(screen.getByText(/5 license slots available, 2 nodes selected/)).toBeInTheDocument();
  });

  it('shows license warning when exceeding limit', () => {
    const wizard = createMockWizard({
      maxAgentSlots: 2,
      confirmSelectedNodeIds: new Set(['n1', 'n2', 'n3']),
      readyNodes: [],
    });
    render(() => <ConfirmStep wizard={wizard} />);
    expect(screen.getByText(/Only 2 nodes can be deployed/)).toBeInTheDocument();
  });

  it('calls toggleConfirmNode on row click', () => {
    const toggleMock = vi.fn();
    const wizard = createMockWizard({
      readyNodes: [
        makeTarget({ id: 't1', nodeId: 'n1', nodeName: 'click-me', status: 'ready' }),
      ],
      confirmSelectedNodeIds: new Set<string>(),
      toggleConfirmNode: toggleMock,
    });
    render(() => <ConfirmStep wizard={wizard} />);
    fireEvent.click(screen.getByText('click-me'));
    expect(toggleMock).toHaveBeenCalledWith('n1');
  });
});

/* ================================================================
   DeployingStep
   ================================================================ */

describe('DeployingStep', () => {
  it('shows progress count', () => {
    const wizard = createMockWizard({
      jobTargets: [
        makeTarget({ id: 't1', status: 'succeeded' }),
        makeTarget({ id: 't2', nodeName: 'node2', status: 'installing' }),
        makeTarget({ id: 't3', nodeName: 'node3', status: 'pending' }),
      ],
    });
    render(() => <DeployingStep wizard={wizard} />);
    expect(screen.getByText(/Installing 1 of 3 nodes/)).toBeInTheDocument();
    expect(screen.getByText(/1 in progress/)).toBeInTheDocument();
  });

  it('renders target rows with status badges', () => {
    const wizard = createMockWizard({
      jobTargets: [
        makeTarget({ id: 't1', nodeName: 'node-x', nodeIP: '10.1.1.1', status: 'installing' }),
      ],
    });
    render(() => <DeployingStep wizard={wizard} />);
    expect(screen.getByText('node-x')).toBeInTheDocument();
    expect(screen.getByText('10.1.1.1')).toBeInTheDocument();
    expect(screen.getByTestId('badge-installing')).toBeInTheDocument();
  });

  it('shows deploy error banner', () => {
    const wizard = createMockWizard({
      deployError: 'SSE connection lost',
      jobTargets: [],
    });
    render(() => <DeployingStep wizard={wizard} />);
    expect(screen.getByText('SSE connection lost')).toBeInTheDocument();
  });

  it('shows error message for failed targets', () => {
    const wizard = createMockWizard({
      jobTargets: [
        makeTarget({ status: 'failed_retryable', errorMessage: 'Exit 1: permission denied' }),
      ],
    });
    render(() => <DeployingStep wizard={wizard} />);
    expect(screen.getByText('Exit 1: permission denied')).toBeInTheDocument();
  });
});

/* ================================================================
   ResultsStep
   ================================================================ */

describe('ResultsStep', () => {
  it('shows succeeded section with count', () => {
    const wizard = createMockWizard({
      succeededTargets: [
        makeTarget({ id: 't1', nodeName: 'good-node', status: 'succeeded' }),
      ],
    });
    render(() => <ResultsStep wizard={wizard} />);
    expect(screen.getByText('Deployed (1)')).toBeInTheDocument();
    expect(screen.getByText('good-node')).toBeInTheDocument();
  });

  it('shows failed section with error messages', () => {
    const wizard = createMockWizard({
      failedTargets: [
        makeTarget({ id: 't1', nodeName: 'bad-node', status: 'failed_retryable', errorMessage: 'Install error' }),
      ],
    });
    render(() => <ResultsStep wizard={wizard} />);
    expect(screen.getByText('Failed (1)')).toBeInTheDocument();
    expect(screen.getByText('bad-node')).toBeInTheDocument();
    expect(screen.getByText('Install error')).toBeInTheDocument();
  });

  it('shows skipped section', () => {
    const wizard = createMockWizard({
      skippedTargets: [
        makeTarget({ id: 't1', nodeName: 'skip-node', status: 'skipped_already_agent' }),
      ],
    });
    render(() => <ResultsStep wizard={wizard} />);
    expect(screen.getByText('Skipped (1)')).toBeInTheDocument();
    expect(screen.getByText('skip-node')).toBeInTheDocument();
  });

  it('shows canceled section', () => {
    const wizard = createMockWizard({
      canceledTargets: [
        makeTarget({ id: 't1', nodeName: 'cancel-node', status: 'canceled' }),
      ],
    });
    render(() => <ResultsStep wizard={wizard} />);
    expect(screen.getByText('Canceled (1)')).toBeInTheDocument();
    expect(screen.getByText('cancel-node')).toBeInTheDocument();
  });

  it('hides sections with zero targets', () => {
    const wizard = createMockWizard({
      succeededTargets: [],
      failedTargets: [],
      skippedTargets: [],
      canceledTargets: [],
    });
    render(() => <ResultsStep wizard={wizard} />);
    expect(screen.queryByText(/Deployed/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Failed/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Skipped/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Canceled/)).not.toBeInTheDocument();
  });

  it('toggles manual install instructions on click', () => {
    const wizard = createMockWizard({
      failedTargets: [makeTarget({ status: 'failed_retryable' })],
    });
    render(() => <ResultsStep wizard={wizard} />);

    // Initially hidden
    expect(screen.queryByText(/SSHing into the node/)).not.toBeInTheDocument();

    // Click to expand
    fireEvent.click(screen.getByText('Manual install instructions'));
    expect(screen.getByText(/SSHing into the node/)).toBeInTheDocument();

    // Click to collapse
    fireEvent.click(screen.getByText('Manual install instructions'));
    expect(screen.queryByText(/SSHing into the node/)).not.toBeInTheDocument();
  });
});
