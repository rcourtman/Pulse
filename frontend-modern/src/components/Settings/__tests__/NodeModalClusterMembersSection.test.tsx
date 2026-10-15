import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal, type Setter } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ClusterEndpoint, NodeConfig } from '@/types/nodes';
import { NodeModalClusterMembersSection } from '../NodeModalClusterMembersSection';
import type { NodeModalProps } from '../nodeModalModel';
import { useNodeModalState } from '../useNodeModalState';

const trustReason =
  'TLS trust conflict - equivalent endpoint URLs have different saved fingerprints; independently verify and reconcile saved trust';

const member = (overrides: Partial<ClusterEndpoint> = {}): ClusterEndpoint => ({
  nodeId: 'node/member',
  nodeIdentity: 'member-identity',
  nodeName: 'member',
  host: 'https://member.example.invalid:8006',
  ip: '192.0.2.2',
  online: true,
  lastSeen: '2026-10-14T14:00:00Z',
  pulseReachable: false,
  pulseError: trustReason,
  ...overrides,
});

const cluster = (endpoints: ClusterEndpoint[]): NodeConfig => ({
  id: 'fixture-cluster',
  type: 'pve',
  name: 'Fixture cluster',
  host: 'https://primary.example.invalid:8006',
  user: 'fixture@pve',
  hasToken: true,
  tokenName: 'fixture@pve!monitoring',
  verifySSL: true,
  monitorVMs: true,
  monitorContainers: true,
  monitorStorage: true,
  monitorBackups: true,
  monitorPhysicalDisks: false,
  isCluster: true,
  clusterEndpoints: endpoints,
});

function renderMembers(endpoints = [member()]) {
  const onSave = vi.fn();
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  let setNode!: Setter<NodeConfig>;
  const Harness = () => {
    const [node, updateNode] = createSignal(cluster(endpoints));
    setNode = updateNode;
    const modalProps: NodeModalProps = {
      isOpen: true,
      nodeType: 'pve',
      get editingNode() {
        return node();
      },
      onClose: vi.fn(),
      onSave,
    };
    const state = useNodeModalState(modalProps);
    return (
      <form onSubmit={state.handleSubmit}>
        <NodeModalClusterMembersSection modalProps={modalProps} state={state} />
        <button type="submit">Save fixture</button>
      </form>
    );
  };
  const view = render(() => <Harness />);
  return { ...view, fetch, onSave, setNode };
}

describe('Cluster member connection diagnostics', () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('gives a refused online member a named native disclosure with its actual reason', () => {
    const { container, fetch, onSave } = renderMembers();
    const summary = container.querySelector('summary')!;
    expect(summary.tagName).toBe('SUMMARY');
    expect(summary.parentElement?.tagName).toBe('DETAILS');
    expect(summary).toHaveTextContent('Unreachable — details for member');
    expect(summary).not.toHaveAttribute('title');
    expect(screen.getByText(trustReason).tagName).toBe('P');
    expect(screen.getByText('Discovered: member.example.invalid:8006 (192.0.2.2)')).toBeVisible();
    // Native keyboard/touch activation and collapsed accessibility are checked
    // in real browsers, not emulated by jsdom.
    expect(summary.parentElement).not.toHaveAttribute('open');
    expect(fetch).not.toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it.each([undefined, null, true])(
    'does not mislabel reachability %s as a failed connection',
    (reachable) => {
      const { container } = renderMembers([member({ pulseReachable: reachable })]);
      expect(container.querySelector('details')).not.toBeInTheDocument();
      expect(screen.queryByText(trustReason)).not.toBeInTheDocument();
    },
  );

  it.each([undefined, ''])('keeps a usable generic explanation when the error is %s', (error) => {
    const { container } = renderMembers([member({ pulseError: error })]);
    expect(container.querySelector('summary')).toHaveTextContent(
      'Unreachable — details for member',
    );
    expect(screen.getByText('Pulse could not connect to this node')).toBeInTheDocument();
  });

  it('renders error markup literally rather than making it executable', () => {
    const error = '<img src="untrusted" onerror="alert(1)">';
    const { container } = renderMembers([member({ pulseError: error })]);
    expect(screen.getByText(error)).toBeInTheDocument();
    expect(container.querySelector('img')).not.toBeInTheDocument();
  });

  it('shows the complete saved member pin without treating it as a verified peer or an editable field', () => {
    const pin = Array.from({ length: 32 }, (_, i) => i.toString(16).padStart(2, '0')).join(':');
    const { fetch, onSave } = renderMembers([member({ fingerprint: pin })]);
    const trust = screen.getByRole('group', {
      name: 'Saved certificate trust for member',
      hidden: true,
    });
    expect(within(trust).getByText(pin).tagName).toBe('CODE');
    expect(trust).toHaveTextContent('Saved configuration, not a fresh certificate check');
    expect(trust).toHaveTextContent('Independently verify a replacement');
    expect(trust).toHaveTextContent('Do not clear a pin or disable verification');
    expect(trust).toHaveTextContent('A connection address override does not replace this pin');
    expect(trust.querySelector('input,button')).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it.each([undefined, '', '   '])(
    'does not invent member trust for an absent pin (%s)',
    (fingerprint) => {
      renderMembers([member({ fingerprint })]);
      expect(screen.getByText('No member certificate pin is recorded.')).toBeInTheDocument();
      expect(
        screen.queryByText(/Trusted certificate|Certificate verified/),
      ).not.toBeInTheDocument();
    },
  );

  it('keeps malformed pin text literal and separate for each member', () => {
    const invalid = '<img src="untrusted" onerror="alert(1)">';
    const otherPin = 'cd'.repeat(32);
    const { container } = renderMembers([
      member({ fingerprint: invalid }),
      member({ nodeId: 'node/other', nodeName: 'other', fingerprint: otherPin }),
    ]);
    const first = screen.getByRole('group', {
      name: 'Saved certificate trust for member',
      hidden: true,
    });
    const other = screen.getByRole('group', {
      name: 'Saved certificate trust for other',
      hidden: true,
    });
    expect(first).toHaveTextContent(invalid);
    expect(first).not.toHaveTextContent(otherPin);
    expect(other).toHaveTextContent(otherPin);
    expect(other).not.toHaveTextContent(invalid);
    expect(container.querySelector('img')).not.toBeInTheDocument();
  });

  it('adopts refreshed saved pins without discarding dirty edits or submitting member pins', () => {
    const oldPin = 'ab'.repeat(32);
    const newPin = 'cd'.repeat(32);
    const { setNode, fetch, onSave } = renderMembers([member({ fingerprint: oldPin })]);
    fireEvent.input(screen.getByLabelText('Display name for member'), {
      target: { value: 'Operator label' },
    });
    fireEvent.input(screen.getByLabelText('Connection address for member'), {
      target: { value: 'member.example.invalid:8006' },
    });
    setNode(cluster([member({ fingerprint: newPin })]));
    expect(screen.queryByText(oldPin)).not.toBeInTheDocument();
    expect(screen.getByText(newPin)).toBeInTheDocument();
    expect(screen.getByLabelText('Display name for member')).toHaveValue('Operator label');
    fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
    expect(onSave).toHaveBeenCalledOnce();
    const payload = onSave.mock.calls[0][0];
    expect(payload).toMatchObject({
      verifySSL: true,
      clusterEndpointOverrides: [{ nodeName: 'member', ipOverride: 'member.example.invalid:8006' }],
      clusterNodeDisplayNameOverrides: [
        { nodeIdentity: 'member-identity', displayName: 'Operator label' },
      ],
    });
    expect(payload).not.toHaveProperty('clusterEndpoints');
    expect(payload).not.toHaveProperty('tokenValue');
    expect(payload).not.toHaveProperty('password');
    expect(fetch).not.toHaveBeenCalled();
  });

  it('keeps each member reason separate, including members with the same native name', () => {
    const { container } = renderMembers([
      member(),
      member({
        nodeId: 'node/duplicate',
        nodeIdentity: 'duplicate-identity',
        pulseError: 'Connection timed out',
      }),
    ]);
    const details = container.querySelectorAll('details');
    expect(details).toHaveLength(2);
    expect(details[0]).toHaveTextContent(trustReason);
    expect(details[0]).not.toHaveTextContent('Connection timed out');
    expect(details[1]).toHaveTextContent('Connection timed out');
    expect(details[1]).not.toHaveTextContent(trustReason);
  });

  it('clears stale diagnostics on recovery without losing dirty names or address overrides', () => {
    const { container, setNode, fetch, onSave } = renderMembers();
    fireEvent.input(screen.getByLabelText('Display name for member'), {
      target: { value: 'Operator label' },
    });
    fireEvent.input(screen.getByLabelText('Connection address for member'), {
      target: { value: 'member.example.invalid:8006' },
    });
    setNode(cluster([member({ pulseReachable: true, pulseError: undefined })]));
    expect(container.querySelector('details')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Display name for member')).toHaveValue('Operator label');
    expect(screen.getByLabelText('Connection address for member')).toHaveValue(
      'member.example.invalid:8006',
    );
    expect(fetch).not.toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();

    // Saving remains a deliberate, separate action using the existing payload.
    fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
    expect(onSave).toHaveBeenCalledOnce();
    expect(onSave.mock.calls[0][0]).toMatchObject({
      type: 'pve',
      verifySSL: true,
      clusterNodeDisplayNameOverrides: [
        { nodeIdentity: 'member-identity', displayName: 'Operator label' },
      ],
      clusterEndpointOverrides: [{ nodeName: 'member', ipOverride: 'member.example.invalid:8006' }],
    });
    expect(onSave.mock.calls[0][0]).not.toHaveProperty('tokenValue');
    expect(onSave.mock.calls[0][0]).not.toHaveProperty('password');
    expect(fetch).not.toHaveBeenCalled();
  });
});
