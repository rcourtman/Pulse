import { describe, expect, it, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { ConfirmStep } from '../ConfirmStep';

// ---------- helpers ----------

interface MockTarget {
  nodeId: string;
  nodeName: string;
  nodeIP: string;
  arch?: string;
  status?: string;
  errorMessage?: string;
}

function makeTarget(overrides: Partial<MockTarget> = {}): MockTarget {
  return {
    nodeId: overrides.nodeId ?? 'node-1',
    nodeName: overrides.nodeName ?? 'pve-node1',
    nodeIP: overrides.nodeIP ?? '192.168.1.10',
    arch: overrides.arch,
    status: overrides.status ?? 'ready',
    errorMessage: overrides.errorMessage,
  };
}

interface WizardOverrides {
  confirmSelectedNodeIds?: Set<string>;
  maxAgentSlots?: number;
  readyNodes?: MockTarget[];
  failedPreflightNodes?: MockTarget[];
  toggleConfirmNode?: ReturnType<typeof vi.fn>;
}

function createMockWizard(overrides: WizardOverrides = {}) {
  const selectedIds = overrides.confirmSelectedNodeIds ?? new Set<string>();
  const slots = overrides.maxAgentSlots ?? 0;
  const ready = overrides.readyNodes ?? [];
  const failed = overrides.failedPreflightNodes ?? [];

  const [confirmSelectedNodeIds] = createSignal(selectedIds);
  const [maxAgentSlots] = createSignal(slots);
  const [readyNodes] = createSignal(ready);
  const [failedPreflightNodes] = createSignal(failed);

  return {
    confirmSelectedNodeIds,
    maxAgentSlots,
    readyNodes,
    failedPreflightNodes,
    toggleConfirmNode: overrides.toggleConfirmNode ?? vi.fn(),
  };
}

function renderConfirm(overrides: WizardOverrides = {}) {
  const wizard = createMockWizard(overrides);
  const result = render(() => <ConfirmStep wizard={wizard as any} />);
  return { ...result, wizard };
}

// ---------- tests ----------

describe('ConfirmStep', () => {
  afterEach(() => {
    cleanup();
  });

  // --- License slot summary ---

  describe('license slot summary', () => {
    it('does not show license info when maxAgentSlots is 0', () => {
      renderConfirm({ maxAgentSlots: 0, readyNodes: [makeTarget()] });
      expect(screen.queryByText(/license slots available/)).not.toBeInTheDocument();
    });

    it('shows slot count and selected count when maxSlots > 0', () => {
      renderConfirm({
        maxAgentSlots: 5,
        confirmSelectedNodeIds: new Set(['a', 'b']),
        readyNodes: [makeTarget({ nodeId: 'a' }), makeTarget({ nodeId: 'b' })],
      });
      expect(screen.getByText(/5 license slots available, 2 nodes selected/)).toBeInTheDocument();
    });

    it('shows warning when selection exceeds license slots', () => {
      renderConfirm({
        maxAgentSlots: 2,
        confirmSelectedNodeIds: new Set(['a', 'b', 'c']),
        readyNodes: [
          makeTarget({ nodeId: 'a' }),
          makeTarget({ nodeId: 'b' }),
          makeTarget({ nodeId: 'c' }),
        ],
      });
      expect(screen.getByText(/Only 2 nodes can be deployed/)).toBeInTheDocument();
      expect(screen.getByText(/Remove 1 nodes/)).toBeInTheDocument();
    });

    it('does not show exceeds warning when selection is within limit', () => {
      renderConfirm({
        maxAgentSlots: 5,
        confirmSelectedNodeIds: new Set(['a', 'b']),
        readyNodes: [makeTarget({ nodeId: 'a' }), makeTarget({ nodeId: 'b' })],
      });
      expect(screen.queryByText(/Only \d+ nodes can be deployed/)).not.toBeInTheDocument();
    });

    it('does not show exceeds warning when selection equals limit exactly', () => {
      renderConfirm({
        maxAgentSlots: 2,
        confirmSelectedNodeIds: new Set(['a', 'b']),
        readyNodes: [makeTarget({ nodeId: 'a' }), makeTarget({ nodeId: 'b' })],
      });
      expect(screen.queryByText(/Only \d+ nodes can be deployed/)).not.toBeInTheDocument();
    });

    it('shows license banner with 0 selected when maxSlots > 0 and nothing selected', () => {
      renderConfirm({
        maxAgentSlots: 10,
        confirmSelectedNodeIds: new Set(),
        readyNodes: [],
      });
      expect(screen.getByText(/10 license slots available, 0 nodes selected/)).toBeInTheDocument();
      expect(screen.queryByText(/Only \d+ nodes can be deployed/)).not.toBeInTheDocument();
    });

    it('applies amber styling when exceeding license slots', () => {
      const { container } = renderConfirm({
        maxAgentSlots: 1,
        confirmSelectedNodeIds: new Set(['a', 'b']),
        readyNodes: [makeTarget({ nodeId: 'a' }), makeTarget({ nodeId: 'b' })],
      });
      const banner = container.querySelector('.bg-amber-50');
      expect(banner).toBeInTheDocument();
    });

    it('applies blue styling when within license slots', () => {
      const { container } = renderConfirm({
        maxAgentSlots: 5,
        confirmSelectedNodeIds: new Set(['a']),
        readyNodes: [makeTarget({ nodeId: 'a' })],
      });
      const banner = container.querySelector('.bg-blue-50');
      expect(banner).toBeInTheDocument();
    });
  });

  // --- Ready nodes table ---

  describe('ready nodes table', () => {
    it('does not show ready section when there are no ready nodes', () => {
      renderConfirm({ readyNodes: [] });
      expect(screen.queryByText(/Ready to deploy/)).not.toBeInTheDocument();
    });

    it('shows ready nodes heading with count', () => {
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'a' }), makeTarget({ nodeId: 'b' })],
      });
      expect(screen.getByText(/Ready to deploy \(2\)/)).toBeInTheDocument();
    });

    it('renders node name, IP, and default arch', () => {
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'n1', nodeName: 'myhost', nodeIP: '10.0.0.1' })],
      });
      expect(screen.getByText('myhost')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
      expect(screen.getByText('amd64')).toBeInTheDocument();
    });

    it('renders explicit arch when provided', () => {
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'n1', arch: 'arm64' })],
      });
      expect(screen.getByText('arm64')).toBeInTheDocument();
      expect(screen.queryByText('amd64')).not.toBeInTheDocument();
    });

    it('shows checkbox as checked for selected nodes', () => {
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'x' })],
        confirmSelectedNodeIds: new Set(['x']),
      });
      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeChecked();
    });

    it('shows checkbox as unchecked for unselected nodes', () => {
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'x' })],
        confirmSelectedNodeIds: new Set(),
      });
      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).not.toBeChecked();
    });

    it('calls toggleConfirmNode when row is clicked', async () => {
      const toggle = vi.fn();
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'abc', nodeName: 'clickable-host' })],
        toggleConfirmNode: toggle,
      });
      await fireEvent.click(screen.getByText('clickable-host'));
      expect(toggle).toHaveBeenCalledWith('abc');
    });

    it('calls toggleConfirmNode when checkbox onChange fires', async () => {
      const toggle = vi.fn();
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'def' })],
        toggleConfirmNode: toggle,
      });
      const checkbox = screen.getByRole('checkbox');
      await fireEvent.click(checkbox);
      expect(toggle).toHaveBeenCalledWith('def');
    });

    it('stops propagation on checkbox click so row handler is not triggered twice', async () => {
      const toggle = vi.fn();
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'dup' })],
        toggleConfirmNode: toggle,
      });
      const checkbox = screen.getByRole('checkbox');
      await fireEvent.click(checkbox);
      // Checkbox click has stopPropagation, so toggle should be called
      // from the checkbox onChange but NOT from the row onClick
      // The click event on the checkbox triggers onChange but not the row
      expect(toggle).toHaveBeenCalledTimes(1);
    });

    it('toggles the correct node when clicking a specific row in a multi-row table', async () => {
      const toggle = vi.fn();
      renderConfirm({
        readyNodes: [
          makeTarget({ nodeId: 'first', nodeName: 'host-alpha' }),
          makeTarget({ nodeId: 'second', nodeName: 'host-beta' }),
          makeTarget({ nodeId: 'third', nodeName: 'host-gamma' }),
        ],
        toggleConfirmNode: toggle,
      });
      await fireEvent.click(screen.getByText('host-beta'));
      expect(toggle).toHaveBeenCalledWith('second');
      expect(toggle).toHaveBeenCalledTimes(1);
    });

    it('renders multiple ready nodes', () => {
      renderConfirm({
        readyNodes: [
          makeTarget({ nodeId: '1', nodeName: 'host-a', nodeIP: '10.0.0.1' }),
          makeTarget({ nodeId: '2', nodeName: 'host-b', nodeIP: '10.0.0.2' }),
          makeTarget({ nodeId: '3', nodeName: 'host-c', nodeIP: '10.0.0.3' }),
        ],
      });
      expect(screen.getByText('host-a')).toBeInTheDocument();
      expect(screen.getByText('host-b')).toBeInTheDocument();
      expect(screen.getByText('host-c')).toBeInTheDocument();
      expect(screen.getAllByRole('checkbox')).toHaveLength(3);
    });
  });

  // --- Failed preflight nodes ---

  describe('failed preflight nodes', () => {
    it('does not show failed section when there are no failed nodes', () => {
      renderConfirm({ failedPreflightNodes: [] });
      expect(screen.queryByText(/Cannot deploy/)).not.toBeInTheDocument();
    });

    it('shows failed nodes heading with count', () => {
      renderConfirm({
        failedPreflightNodes: [
          makeTarget({ nodeId: 'f1', errorMessage: 'timeout' }),
          makeTarget({ nodeId: 'f2', errorMessage: 'auth fail' }),
        ],
      });
      expect(screen.getByText(/Cannot deploy \(2\)/)).toBeInTheDocument();
    });

    it('renders node name, IP, and error message', () => {
      renderConfirm({
        failedPreflightNodes: [
          makeTarget({
            nodeId: 'bad',
            nodeName: 'broken-host',
            nodeIP: '10.0.0.99',
            errorMessage: 'SSH connection refused',
          }),
        ],
      });
      expect(screen.getByText('broken-host')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.99')).toBeInTheDocument();
      expect(screen.getByText('SSH connection refused')).toBeInTheDocument();
    });

    it('shows default "Preflight failed" when errorMessage is empty', () => {
      renderConfirm({
        failedPreflightNodes: [makeTarget({ nodeId: 'no-err', errorMessage: '' })],
      });
      expect(screen.getByText('Preflight failed')).toBeInTheDocument();
    });

    it('shows default "Preflight failed" when errorMessage is undefined', () => {
      renderConfirm({
        failedPreflightNodes: [makeTarget({ nodeId: 'undef' })],
      });
      expect(screen.getByText('Preflight failed')).toBeInTheDocument();
    });

    it('does not render checkboxes for failed nodes', () => {
      renderConfirm({
        readyNodes: [],
        failedPreflightNodes: [makeTarget({ nodeId: 'f1' })],
      });
      expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    });
  });

  // --- Combined states ---

  describe('combined ready and failed nodes', () => {
    it('shows both sections when there are ready and failed nodes', () => {
      renderConfirm({
        readyNodes: [makeTarget({ nodeId: 'r1', nodeName: 'good-host' })],
        failedPreflightNodes: [
          makeTarget({ nodeId: 'f1', nodeName: 'bad-host', errorMessage: 'denied' }),
        ],
      });
      expect(screen.getByText(/Ready to deploy \(1\)/)).toBeInTheDocument();
      expect(screen.getByText(/Cannot deploy \(1\)/)).toBeInTheDocument();
      expect(screen.getByText('good-host')).toBeInTheDocument();
      expect(screen.getByText('bad-host')).toBeInTheDocument();
    });

    it('renders nothing when both lists are empty and no license', () => {
      renderConfirm({
        readyNodes: [],
        failedPreflightNodes: [],
        maxAgentSlots: 0,
      });
      // Only the outer wrapper div exists, with no visible child content
      expect(screen.queryByText(/Ready to deploy/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Cannot deploy/)).not.toBeInTheDocument();
      expect(screen.queryByText(/license slots/)).not.toBeInTheDocument();
    });
  });

  // --- Table headers ---

  describe('table headers', () => {
    it('shows correct headers for ready nodes table', () => {
      renderConfirm({ readyNodes: [makeTarget()] });
      expect(screen.getByText('Node')).toBeInTheDocument();
      expect(screen.getByText('IP')).toBeInTheDocument();
      expect(screen.getByText('Arch')).toBeInTheDocument();
    });

    it('shows correct headers for failed nodes table', () => {
      renderConfirm({
        failedPreflightNodes: [makeTarget({ nodeId: 'f', errorMessage: 'err' })],
      });
      // Failed table has Node, IP, Reason columns
      expect(screen.getByText('Reason')).toBeInTheDocument();
    });
  });
});
