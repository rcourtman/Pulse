import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@solidjs/testing-library';

/* ── hoisted mocks ────────────────────────────────────────────── */
vi.mock('lucide-solid/icons/info', () => ({
  default: () => <span data-testid="info-icon" />,
}));

vi.mock('lucide-solid/icons/rocket', () => ({
  default: () => <span data-testid="rocket-icon" />,
}));

/* ── component import (after mocks) ──────────────────────────── */
import { ClusterDeployBanner } from '../ClusterDeployBanner';
import type { ResourceGroup } from '../infrastructureSelectors';
import type { Resource } from '@/types/resource';

/* ── helpers ──────────────────────────────────────────────────── */

/** Minimal PVE node resource with optional agent */
function makePveNode(id: string, agentId?: string): Resource {
  return {
    id,
    type: 'agent',
    name: id,
    displayName: id,
    platformId: 'pve1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    agent: agentId ? { agentId } : undefined,
  } as Resource;
}

/** Non-PVE resource (e.g. a VM) */
function makeVm(id: string): Resource {
  return {
    id,
    type: 'vm',
    name: id,
    displayName: id,
    platformId: 'pve1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
  } as Resource;
}

/** Non-PVE platform node (e.g. k8s) with optional agent */
function makeK8sNode(id: string, agentId?: string): Resource {
  return {
    id,
    type: 'k8s-node',
    name: id,
    displayName: id,
    platformId: 'k8s1',
    platformType: 'kubernetes',
    sourceType: 'agent',
    status: 'online',
    agent: agentId ? { agentId } : undefined,
  } as Resource;
}

function makeGroup(cluster: string, resources: Resource[]): ResourceGroup {
  return { cluster, resources };
}

/* ── lifecycle ────────────────────────────────────────────────── */
beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  cleanup();
});

/* ── tests ────────────────────────────────────────────────────── */
describe('ClusterDeployBanner', () => {
  describe('visibility conditions', () => {
    it('renders nothing when group has no cluster name', () => {
      const group = makeGroup('', [makePveNode('node1', 'agent-1'), makePveNode('node2')]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.queryByText('Review & Deploy')).not.toBeInTheDocument();
    });

    it('renders nothing when there are no PVE nodes', () => {
      const group = makeGroup('my-cluster', [makeVm('vm1'), makeK8sNode('k8s-node1')]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.queryByText('Review & Deploy')).not.toBeInTheDocument();
    });

    it('renders nothing when no PVE node has a source agent', () => {
      const group = makeGroup('my-cluster', [
        makePveNode('node1'), // no agent
        makePveNode('node2'), // no agent
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.queryByText('Review & Deploy')).not.toBeInTheDocument();
    });

    it('renders nothing when all PVE nodes already have agents', () => {
      const group = makeGroup('my-cluster', [
        makePveNode('node1', 'agent-1'),
        makePveNode('node2', 'agent-2'),
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.queryByText('Review & Deploy')).not.toBeInTheDocument();
    });

    it('renders when there is a mix of monitored and unmonitored PVE nodes', () => {
      const group = makeGroup('my-cluster', [
        makePveNode('node1', 'agent-1'), // source agent
        makePveNode('node2'), // unmonitored
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.getByText('1 node unmonitored')).toBeInTheDocument();
      expect(screen.getByText('Review & Deploy')).toBeInTheDocument();
    });
  });

  describe('unmonitored count display', () => {
    it('shows singular "node" for 1 unmonitored', () => {
      const group = makeGroup('cluster-a', [makePveNode('node1', 'agent-1'), makePveNode('node2')]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.getByText('1 node unmonitored')).toBeInTheDocument();
    });

    it('shows plural "nodes" for multiple unmonitored', () => {
      const group = makeGroup('cluster-a', [
        makePveNode('node1', 'agent-1'),
        makePveNode('node2'),
        makePveNode('node3'),
        makePveNode('node4'),
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.getByText('3 nodes unmonitored')).toBeInTheDocument();
    });
  });

  describe('deploy button', () => {
    it('calls onDeploy with cluster name when clicked', () => {
      const onDeploy = vi.fn();
      const group = makeGroup('prod-cluster', [
        makePveNode('node1', 'agent-1'),
        makePveNode('node2'),
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={onDeploy} />);
      fireEvent.click(screen.getByText('Review & Deploy'));
      expect(onDeploy).toHaveBeenCalledTimes(1);
      expect(onDeploy).toHaveBeenCalledWith('prod-cluster', 'prod-cluster');
    });

    it('stops event propagation on click', () => {
      const parentHandler = vi.fn();
      const group = makeGroup('my-cluster', [
        makePveNode('node1', 'agent-1'),
        makePveNode('node2'),
      ]);
      render(() => (
        <div onClick={parentHandler}>
          <ClusterDeployBanner group={group} onDeploy={vi.fn()} />
        </div>
      ));
      fireEvent.click(screen.getByText('Review & Deploy'));
      expect(parentHandler).not.toHaveBeenCalled();
    });
  });

  describe('edge cases', () => {
    it('ignores VMs when computing PVE node counts', () => {
      const group = makeGroup('my-cluster', [
        makePveNode('node1', 'agent-1'),
        makeVm('vm1'),
        makePveNode('node2'),
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.getByText('1 node unmonitored')).toBeInTheDocument();
    });

    it('ignores non-PVE platform nodes', () => {
      const group = makeGroup('my-cluster', [
        makePveNode('pve-node1', 'agent-1'),
        makeK8sNode('k8s-node1'),
        makePveNode('pve-node2'),
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.getByText('1 node unmonitored')).toBeInTheDocument();
    });

    it('does not treat non-PVE agent as a PVE source agent', () => {
      // A k8s node with an agent should NOT satisfy the source-agent requirement for PVE
      const group = makeGroup('my-cluster', [
        makePveNode('pve-node1'), // PVE node, no agent
        makePveNode('pve-node2'), // PVE node, no agent
        makeK8sNode('k8s-node1', 'k8s-agent-1'), // non-PVE node with agent
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.queryByText('Review & Deploy')).not.toBeInTheDocument();
    });

    it('renders icons', () => {
      const group = makeGroup('my-cluster', [
        makePveNode('node1', 'agent-1'),
        makePveNode('node2'),
      ]);
      render(() => <ClusterDeployBanner group={group} onDeploy={vi.fn()} />);
      expect(screen.getByTestId('info-icon')).toBeInTheDocument();
      expect(screen.getByTestId('rocket-icon')).toBeInTheDocument();
    });
  });
});
