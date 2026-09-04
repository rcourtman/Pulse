import { describe, expect, it, vi } from 'vitest';
import { createRoot, createSignal, type Setter } from 'solid-js';
import type { NodeConfig } from '@/types/nodes';
import { useNodeModalState, type NodeModalState } from '../useNodeModalState';
import type { NodeModalProps } from '../nodeModalModel';

vi.mock('@/api/nodes', () => ({ NodesAPI: {} }));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: vi.fn() }));

const buildPveNode = (overrides: Partial<NodeConfig> = {}): NodeConfig =>
  ({
    id: 'pve-enacon',
    type: 'pve',
    name: 'enacon',
    host: 'https://192.168.16.70:8006',
    user: '',
    fingerprint: 'AA:BB:CC:DD',
    verifySSL: true,
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    monitorPhysicalDisks: true,
    isCluster: true,
    clusterName: 'enacon',
    clusterEndpoints: [
      {
        NodeID: 'node/pve01',
        nodeIdentity: 'enacon-pve01',
        nodeName: 'pve01',
        displayName: '',
        ipOverride: '',
      },
    ],
    ...overrides,
  }) as unknown as NodeConfig;

const buildProps = (editingNode: () => NodeConfig | undefined): NodeModalProps =>
  ({
    isOpen: true,
    nodeType: 'pve',
    get editingNode() {
      return editingNode();
    },
    onClose: () => {},
    onSave: () => {},
  }) as NodeModalProps;

const mountHook = () => {
  let state!: NodeModalState;
  let setNode!: Setter<NodeConfig | undefined>;
  const dispose = createRoot((d) => {
    const [node, setNodeSignal] = createSignal<NodeConfig | undefined>(buildPveNode());
    setNode = setNodeSignal;
    state = useNodeModalState(buildProps(node));
    return d;
  });
  return { state, setNode, dispose };
};

describe('useNodeModalState poll refresh vs unsaved edits', () => {
  it('keeps unsaved edits when a poll refreshes the same editing target', () => {
    const { state, setNode, dispose } = mountHook();

    expect(state.formData().name).toBe('enacon');

    state.updateField('name', 'renamed-cluster');
    state.updateField('setupMode', 'agent');
    state.updateField('verifySSL', false);
    state.updateField('fingerprint', '');
    state.updateClusterNodeDisplayName('enacon-pve01', 'steinboeck-pve01');

    // Adoption progress: the poll delivers a refreshed snapshot of the SAME
    // connection with a new cluster member. Before the fix this rebuilt the
    // form and wiped these edits.
    setNode(
      buildPveNode({
        fingerprint: '11:22:33:44',
        clusterEndpoints: [
          {
            NodeID: 'node/pve01',
            nodeIdentity: 'enacon-pve01',
            nodeName: 'pve01',
            displayName: '',
            ipOverride: '',
          },
          {
            NodeID: 'node/pve02',
            nodeIdentity: 'enacon-pve02',
            nodeName: 'pve02',
            displayName: '',
            ipOverride: '',
          },
        ],
      } as Partial<NodeConfig>),
    );

    expect(state.formData().name).toBe('renamed-cluster');
    expect(state.formData().setupMode).toBe('agent');
    expect(state.formData().verifySSL).toBe(false);
    expect(state.formData().fingerprint).toBe('');
    expect(state.formData().clusterNodeDisplayNames['enacon-pve01']).toBe('steinboeck-pve01');

    dispose();
  });

  it('still syncs poll refreshes while the form is untouched', () => {
    const { state, setNode, dispose } = mountHook();

    setNode(buildPveNode({ host: 'https://192.168.16.71:8006' } as Partial<NodeConfig>));

    expect(state.formData().host).toBe('https://192.168.16.71:8006');

    dispose();
  });

  it('resyncs and drops stale edits when the editing target changes identity', () => {
    const { state, setNode, dispose } = mountHook();

    state.updateField('name', 'renamed-cluster');

    setNode(
      buildPveNode({
        id: 'pve-rewo',
        name: 'rewo',
        host: 'https://10.0.0.5:8006',
      } as Partial<NodeConfig>),
    );

    expect(state.formData().name).toBe('rewo');
    expect(state.formData().host).toBe('https://10.0.0.5:8006');

    dispose();
  });
});
