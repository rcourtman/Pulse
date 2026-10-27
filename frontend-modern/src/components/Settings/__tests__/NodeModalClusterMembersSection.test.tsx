import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, type Setter } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { NodesAPI } from '@/api/nodes';
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
    vi.restoreAllMocks();
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
  it('edits only the displayed peer TLS endpoint and leaves the primary settings separate', () => {
    const { onSave } = renderMembers([
      member({
        nodeName: 'primary',
        tlsEndpoint: 'https://primary.test:8006',
        usesPrimaryTLS: true,
        fingerprint: 'primary-pin',
      }),
      member({ tlsEndpoint: 'https://member.test:8006', fingerprint: 'member-pin' }),
    ]);
    expect(
      screen.queryByRole('textbox', { name: 'Certificate fingerprint for primary' }),
    ).toBeNull();
    const pin = screen.getByRole('textbox', { name: 'Certificate fingerprint for member' });
    fireEvent.input(pin, { target: { value: '' } });
    expect(
      screen.getByText('No pin: a trusted, hostname-matching certificate is required.'),
    ).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        clusterEndpointTrustOverrides: [{ endpoint: 'https://member.test:8006', fingerprint: '' }],
      }),
    );
  });
  it('requires a deliberate CA choice for an unknown first-use member', () => {
    const { onSave } = renderMembers([
      member({ tlsEndpoint: 'https://member.test:8006', tlsFirstUse: true, fingerprint: '' }),
    ]);
    expect(screen.getByText(/No saved member pin/)).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Use CA validation for member' }));
    expect(
      screen.getByText('No pin: a trusted, hostname-matching certificate is required.'),
    ).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        clusterEndpointTrustOverrides: [{ endpoint: 'https://member.test:8006', fingerprint: '' }],
      }),
    );
  });
  it('shows a member test result beside that member without requiring a trip to the footer', async () => {
    vi.spyOn(NodesAPI, 'testExistingNode').mockResolvedValueOnce({
      status: 'success',
      message:
        'Connected to this PVE endpoint. Other endpoints and background collection were not tested.',
    });
    renderMembers([
      member({ tlsEndpoint: 'https://member.test:8006', fingerprint: 'verified-pin' }),
    ]);
    fireEvent.click(screen.getByRole('button', { name: 'Test TLS for member' }));
    await waitFor(() =>
      expect(screen.getByRole('status', { name: 'TLS test result for member' })).toHaveTextContent(
        'member: Connected to this PVE endpoint.',
      ),
    );
    expect(screen.getByRole('status', { name: 'TLS test result for member' })).toHaveTextContent(
      'Other endpoints and background collection were not tested.',
    );
  });
});
