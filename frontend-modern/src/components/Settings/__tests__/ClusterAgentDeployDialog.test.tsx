import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ClusterDeployCandidates, ClusterDeployJob } from '@/api/agentDeploy';
import {
  ClusterAgentDeployDialog,
  clusterDeployNodeStateFromTarget,
} from '../ClusterAgentDeployDialog';

const api = vi.hoisted(() => ({
  getCandidates: vi.fn(),
  createPreflight: vi.fn(),
  getPreflight: vi.fn(),
  createJob: vi.fn(),
  getJob: vi.fn(),
}));

vi.mock('@/api/agentDeploy', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/agentDeploy')>();
  return { ...actual, ClusterAgentDeployAPI: api };
});

const candidates = (overrides: Partial<ClusterDeployCandidates> = {}): ClusterDeployCandidates => ({
  clusterId: 'homelab',
  clusterName: 'homelab',
  sourceAgents: [{ agentId: 'agent-delly', nodeId: 'n-delly', online: true }],
  nodes: [
    {
      nodeId: 'n-delly',
      name: 'delly',
      ip: '192.168.0.5',
      hasAgent: true,
      deployable: false,
      reason: 'already_agent',
    },
    { nodeId: 'n-delly2', name: 'delly2', ip: '192.168.0.111', hasAgent: false, deployable: true },
    { nodeId: 'n-minipc', name: 'minipc', ip: '192.168.0.134', hasAgent: false, deployable: true },
    { nodeId: 'n-pi', name: 'pi', hasAgent: false, deployable: false, reason: 'no_address' },
  ],
  ...overrides,
});

const connectedCandidates = (): ClusterDeployCandidates =>
  candidates({
    sourceAgents: [
      ...candidates().sourceAgents!,
      { agentId: 'agent-delly2', nodeId: 'n-delly2', online: true },
    ],
    nodes: candidates().nodes!.map((node) =>
      node.nodeId === 'n-delly2' ? { ...node, hasAgent: true, deployable: false } : node,
    ),
  });

const job = (
  id: string,
  status: ClusterDeployJob['status'],
  targets: ClusterDeployJob['targets'],
): ClusterDeployJob => ({ id, clusterId: 'homelab', status, targets });

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((accept, fail) => {
    resolve = accept;
    reject = fail;
  });
  return { promise, resolve, reject };
};

const renderDialog = (props: Partial<Parameters<typeof ClusterAgentDeployDialog>[0]> = {}) => {
  const onClose = vi.fn();
  const onUseInstaller = vi.fn();
  const onInstalled = vi.fn();
  render(() => (
    <ClusterAgentDeployDialog
      isOpen={true}
      clusterName="homelab"
      onClose={onClose}
      onUseInstaller={onUseInstaller}
      onInstalled={onInstalled}
      pollIntervalMs={1}
      {...props}
    />
  ));
  return { onClose, onUseInstaller, onInstalled };
};

describe('ClusterAgentDeployDialog', () => {
  beforeEach(() => {
    Object.values(api).forEach((fn) => fn.mockReset());
  });
  afterEach(() => cleanup());

  it('keeps the focused Close control when loaded candidates change its label to Cancel', async () => {
    const load = deferred<ClusterDeployCandidates>();
    api.getCandidates.mockReturnValue(load.promise);
    const { onClose } = renderDialog();
    const dismiss = screen.getByRole('button', { name: 'Close' });
    dismiss.focus();
    load.resolve(candidates());

    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toBe(dismiss));
    expect(dismiss).toHaveFocus();
    fireEvent.click(dismiss);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(api.createPreflight).not.toHaveBeenCalled();
  });

  it.each(['offline', 'ambiguous'] as const)(
    'keeps dismissal focus when loading ends in the %s installer fallback',
    async (result) => {
      const load = deferred<ClusterDeployCandidates>();
      api.getCandidates.mockReturnValue(load.promise);
      const { onClose, onUseInstaller } = renderDialog();
      const dismiss = screen.getByRole('button', { name: 'Close' });
      dismiss.focus();
      if (result === 'offline') load.resolve(candidates({ sourceAgents: [] }));
      else load.reject(new Error('More than one Proxmox connection reports this cluster.'));

      const manual = await screen.findByRole('button', { name: 'Use the installer instead' });
      expect(screen.getByRole('button', { name: 'Close' })).toBe(dismiss);
      expect(dismiss).toHaveFocus();
      fireEvent.click(manual);
      expect(onClose).toHaveBeenCalledTimes(1);
      expect(onUseInstaller).toHaveBeenCalledTimes(1);
      expect(api.createPreflight).not.toHaveBeenCalled();
    },
  );

  it.each(['succeeded', 'failed'] as const)(
    'hands focused Install to stable dismissal through checking, installation and %s',
    async (outcome) => {
      const check = deferred<ClusterDeployJob>();
      const completion = deferred<ClusterDeployJob>();
      api.getCandidates
        .mockResolvedValueOnce(candidates())
        .mockResolvedValue(connectedCandidates());
      api.createPreflight.mockResolvedValue({ preflightId: 'focus-pf', status: 'running' });
      api.getPreflight.mockReturnValue(check.promise);
      api.createJob.mockResolvedValue({
        jobId: 'focus-job',
        acceptedTargets: ['n-delly2'],
        skippedTargets: [],
      });
      api.getJob.mockReturnValue(completion.promise);
      const { onInstalled } = renderDialog({ preselectNodeName: 'delly2' });
      const install = await screen.findByRole('button', { name: 'Install on 1 node' });
      const dismiss = screen.getByRole('button', { name: 'Cancel' });
      install.focus();
      fireEvent.click(install);
      expect(screen.getByRole('button', { name: 'Close' })).toBe(dismiss);
      expect(dismiss).toHaveFocus();

      await waitFor(() => expect(api.getPreflight).toHaveBeenCalledTimes(1));
      check.resolve(
        job('focus-pf', 'succeeded', [
          {
            id: 'focus-check',
            nodeId: 'n-delly2',
            nodeName: 'delly2',
            nodeIP: '192.168.0.111',
            status: 'ready',
          },
        ]),
      );
      await waitFor(() => expect(api.getJob).toHaveBeenCalledTimes(1));
      expect(dismiss).toHaveFocus();
      completion.resolve(
        job('focus-job', outcome, [
          {
            id: 'focus-target',
            nodeId: 'n-delly2',
            nodeName: 'delly2',
            nodeIP: '192.168.0.111',
            status: outcome === 'succeeded' ? 'succeeded' : 'failed_permanent',
          },
        ]),
      );
      await screen.findByText(outcome === 'succeeded' ? 'Connected to Pulse' : 'Failed');
      expect(screen.getByRole('button', { name: 'Close' })).toBe(dismiss);
      expect(dismiss).toHaveFocus();
      expect(api.createJob).toHaveBeenCalledTimes(1);
      expect(onInstalled).toHaveBeenCalledTimes(outcome === 'succeeded' ? 1 : 0);
    },
  );

  it('does not steal focus for an install action that was not focused', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    api.createPreflight.mockRejectedValue(new Error('Source agent is not connected'));
    renderDialog({ preselectNodeName: 'delly2' });
    const install = await screen.findByRole('button', { name: 'Install on 1 node' });
    const dialog = screen.getByRole('dialog');
    dialog.focus();
    fireEvent.click(install);
    await screen.findByRole('alert');
    expect(dialog).toHaveFocus();
    expect(api.createJob).not.toHaveBeenCalled();
  });

  it('lists uncovered nodes, preselects the installable ones, and explains the rest', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    renderDialog();

    const list = await screen.findByRole('list', { name: 'Cluster nodes without the Pulse Agent' });
    expect(within(list).queryByText('delly')).toBeNull();
    expect(within(list).getByRole('checkbox', { name: /delly2/ })).toBeChecked();
    expect(within(list).getByRole('checkbox', { name: /minipc/ })).toBeChecked();
    const pi = within(list).getByRole('checkbox', { name: /pi/ });
    expect(pi).toBeDisabled();
    expect(pi).not.toBeChecked();
    expect(screen.getByText(/Pulse has no IP address for this node/)).toBeInTheDocument();
    expect(screen.getByText(/install the agent from delly/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Install on 2 nodes' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Use the installer instead' })).toBeInTheDocument();
  });

  it('selects only the member the operator clicked install on', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    renderDialog({ preselectNodeName: 'delly2' });

    expect(await screen.findByRole('checkbox', { name: /delly2/ })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: /minipc/ })).not.toBeChecked();
    expect(screen.getByRole('button', { name: 'Install on 1 node' })).toBeEnabled();
  });

  it('checks, installs, and confirms the node has a connected agent before calling it done', async () => {
    api.getCandidates.mockResolvedValueOnce(candidates()).mockResolvedValue(connectedCandidates());
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_1', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_1', 'succeeded', [
        {
          id: 't1',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'ready',
        },
      ]),
    );
    api.createJob.mockResolvedValue({
      jobId: 'dep_1',
      acceptedTargets: ['n-delly2'],
      skippedTargets: [],
    });
    api.getJob.mockResolvedValue(
      job('dep_1', 'succeeded', [
        {
          id: 't2',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'enrolling',
        },
      ]),
    );
    const { onInstalled } = renderDialog({ preselectNodeName: 'delly2' });

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    await waitFor(() => expect(screen.getByText('Connected to Pulse')).toBeInTheDocument());
    expect(api.createPreflight).toHaveBeenCalledWith('homelab', 'agent-delly', ['n-delly2']);
    expect(api.createJob).toHaveBeenCalledWith('homelab', 'agent-delly', 'pf_1', ['n-delly2']);
    expect(onInstalled).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument();
  });

  it('closes out a node the job never reported on instead of leaving it busy', async () => {
    const reported = connectedCandidates();
    api.getCandidates.mockResolvedValueOnce(candidates()).mockResolvedValue(reported);
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_3', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_3', 'succeeded', [
        {
          id: 't1',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'ready',
        },
        {
          id: 't2',
          nodeId: 'n-minipc',
          nodeName: 'minipc',
          nodeIP: '192.168.0.134',
          status: 'ready',
        },
      ]),
    );
    api.createJob.mockResolvedValue({
      jobId: 'dep_3',
      acceptedTargets: ['n-delly2'],
      skippedTargets: [{ nodeId: 'n-minipc', reason: 'preflight_status_failed' }],
    });
    api.getJob.mockResolvedValue(
      job('dep_3', 'succeeded', [
        {
          id: 't3',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'succeeded',
        },
      ]),
    );
    renderDialog();

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 2 nodes' }));

    expect(
      await screen.findByText('Pulse could not finish installing on this node.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Connected to Pulse')).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Use the installer instead' })).toBeInTheDocument();
  });

  it('stops before installing when the check fails and offers the installer', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_2', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_2', 'failed', [
        {
          id: 't1',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'failed_permanent',
          errorMessage: 'SSH to 192.168.0.111 was refused',
        },
      ]),
    );
    const { onUseInstaller, onClose } = renderDialog({ preselectNodeName: 'delly2' });

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    expect(await screen.findByText('SSH to 192.168.0.111 was refused')).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(api.createJob).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Use the installer instead' }));
    expect(onClose).toHaveBeenCalled();
    expect(onUseInstaller).toHaveBeenCalled();
  });

  it('hands off to the installer when no cluster node has a connected agent', async () => {
    api.getCandidates.mockResolvedValue(candidates({ sourceAgents: [] }));
    const { onUseInstaller } = renderDialog();

    expect(
      await screen.findByText(/none of this cluster’s nodes has one connected/),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Install on/ })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Use the installer instead' }));
    expect(onUseInstaller).toHaveBeenCalled();
  });
});

describe('ClusterAgentDeployDialog edge cases', () => {
  beforeEach(() => {
    Object.values(api).forEach((fn) => fn.mockReset());
  });
  afterEach(() => cleanup());

  it('selects nothing when the clicked member cannot be installed from here', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    renderDialog({ preselectNodeName: 'pi' });

    expect(await screen.findByRole('checkbox', { name: /delly2/ })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: /minipc/ })).not.toBeChecked();
    expect(screen.getByRole('button', { name: 'Install on 0 nodes' })).toBeDisabled();
    expect(screen.getByText(/Pulse has no IP address for this node/)).toBeInTheDocument();
  });

  it('confirms a node that gained an agent during the check without installing it', async () => {
    const covered = connectedCandidates();
    api.getCandidates.mockResolvedValueOnce(candidates()).mockResolvedValue(covered);
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_4', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_4', 'succeeded', [
        {
          id: 't1',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'skipped_already_agent',
        },
      ]),
    );
    const { onInstalled } = renderDialog({ preselectNodeName: 'delly2' });

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    await waitFor(() => expect(screen.getByText('Connected to Pulse')).toBeInTheDocument());
    expect(api.createJob).not.toHaveBeenCalled();
    expect(onInstalled).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['missing connections', { sourceAgents: null }],
    ['empty connections', { sourceAgents: [] }],
    ['only a connected sibling', { sourceAgents: candidates().sourceAgents }],
    [
      'offline target',
      { sourceAgents: [{ agentId: 'agent-delly2', nodeId: 'n-delly2', online: false }] },
    ],
    [
      'empty agent identity',
      { sourceAgents: [{ agentId: ' ', nodeId: 'n-delly2', online: true }] },
    ],
    ['missing node inventory', { nodes: null }],
    ['no saved agent link', { nodes: candidates().nodes }],
  ] satisfies [string, Partial<ClusterDeployCandidates>][])(
    'does not confirm success from %s',
    async (_name, override) => {
      api.getCandidates.mockResolvedValueOnce(candidates()).mockResolvedValue({
        ...connectedCandidates(),
        ...override,
      });
      api.createPreflight.mockResolvedValue({ preflightId: 'pf_link', status: 'running' });
      api.getPreflight.mockResolvedValue(
        job('pf_link', 'succeeded', [
          {
            id: 't_link',
            nodeId: 'n-delly2',
            nodeName: 'delly2',
            nodeIP: '192.168.0.111',
            status: 'skipped_already_agent',
          },
        ]),
      );
      const { onInstalled } = renderDialog({
        preselectNodeName: 'delly2',
        connectionTimeoutMs: 100,
      });
      fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

      await screen.findByText('Pulse has not confirmed a connected agent on this node yet.');
      expect(screen.queryByText('Connected to Pulse')).toBeNull();
      expect(screen.getByText('Failed')).toBeInTheDocument();
      expect(onInstalled).not.toHaveBeenCalled();
      expect(api.createPreflight).toHaveBeenCalledTimes(1);
      expect(api.createJob).not.toHaveBeenCalled();
      expect(screen.getByRole('button', { name: 'Use the installer instead' })).toBeInTheDocument();
    },
  );

  it('waits for the installed node’s live connection, not just its saved link or successful job', async () => {
    const live = deferred<ClusterDeployCandidates>();
    api.getCandidates
      .mockResolvedValueOnce(candidates())
      .mockResolvedValueOnce({
        ...connectedCandidates(),
        sourceAgents: candidates().sourceAgents,
      })
      .mockReturnValue(live.promise);
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_live', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_live', 'succeeded', [
        {
          id: 't_check',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'ready',
        },
      ]),
    );
    api.createJob.mockResolvedValue({
      jobId: 'dep_live',
      acceptedTargets: ['n-delly2'],
      skippedTargets: [],
    });
    api.getJob.mockResolvedValue(
      job('dep_live', 'succeeded', [
        {
          id: 't_install',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'succeeded',
        },
      ]),
    );
    const { onInstalled } = renderDialog({ preselectNodeName: 'delly2' });
    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    await waitFor(() => expect(api.getCandidates).toHaveBeenCalledTimes(3));
    expect(screen.getByText('Connecting to Pulse')).toBeInTheDocument();
    expect(screen.queryByText('Connected to Pulse')).toBeNull();
    expect(onInstalled).not.toHaveBeenCalled();
    live.resolve(connectedCandidates());
    await screen.findByText('Connected to Pulse');
    expect(onInstalled).toHaveBeenCalledTimes(1);
    expect(api.createJob).toHaveBeenCalledTimes(1);
    expect(api.createPreflight).toHaveBeenCalledTimes(1);
  });

  it('refuses connection confirmation from a different cluster response', async () => {
    api.getCandidates.mockResolvedValueOnce(candidates()).mockResolvedValue({
      ...connectedCandidates(),
      clusterId: 'other-site',
    });
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_other', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_other', 'succeeded', [
        {
          id: 't_other',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'skipped_already_agent',
        },
      ]),
    );
    const { onInstalled } = renderDialog({ preselectNodeName: 'delly2' });
    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Pulse could not confirm this cluster’s agent connections.',
    );
    expect(screen.queryByText('Connected to Pulse')).toBeNull();
    expect(onInstalled).not.toHaveBeenCalled();
    expect(api.createJob).not.toHaveBeenCalled();
  });

  it('gives up on a stalled install with an explanation and the installer', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_5', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_5', 'succeeded', [
        {
          id: 't1',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'ready',
        },
      ]),
    );
    api.createJob.mockResolvedValue({
      jobId: 'dep_5',
      acceptedTargets: ['n-delly2'],
      skippedTargets: [],
    });
    api.getJob.mockResolvedValue(
      job('dep_5', 'running', [
        {
          id: 't2',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'installing',
        },
      ]),
    );
    renderDialog({ preselectNodeName: 'delly2', jobTimeoutMs: 20 });

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    expect(await screen.findByText(/Pulse stopped waiting after 20 minutes/)).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Use the installer instead' })).toBeInTheDocument();
  });

  it('gives up on a status request that never answers', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_7', status: 'running' });
    api.getPreflight.mockReturnValue(new Promise(() => {}));
    renderDialog({ preselectNodeName: 'delly2', requestTimeoutMs: 20 });

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));

    expect(await screen.findByText(/Pulse did not answer in time/)).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Use the installer instead' })).toBeInTheDocument();
  });

  it('keeps installing after the dialog is closed mid-run', async () => {
    api.getCandidates.mockResolvedValue(candidates());
    api.createPreflight.mockResolvedValue({ preflightId: 'pf_6', status: 'running' });
    api.getPreflight.mockResolvedValue(
      job('pf_6', 'succeeded', [
        {
          id: 't1',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'ready',
        },
      ]),
    );
    api.createJob.mockResolvedValue({
      jobId: 'dep_6',
      acceptedTargets: ['n-delly2'],
      skippedTargets: [],
    });
    // The job ends on its first poll so the orphaned run finishes instead of
    // polling on in the background after this test.
    api.getJob.mockResolvedValue(
      job('dep_6', 'failed', [
        {
          id: 't2',
          nodeId: 'n-delly2',
          nodeName: 'delly2',
          nodeIP: '192.168.0.111',
          status: 'failed_permanent',
        },
      ]),
    );
    renderDialog({ preselectNodeName: 'delly2' });

    fireEvent.click(await screen.findByRole('button', { name: 'Install on 1 node' }));
    cleanup();

    await waitFor(() =>
      expect(api.createJob).toHaveBeenCalledWith('homelab', 'agent-delly', 'pf_6', ['n-delly2']),
    );
  });

  it('shows the server refusal for an ambiguous cluster and offers the installer', async () => {
    api.getCandidates.mockRejectedValue(
      new Error('More than one Proxmox connection reports a cluster named "homelab".'),
    );
    const { onUseInstaller } = renderDialog();

    expect(await screen.findByRole('alert')).toHaveTextContent(/More than one Proxmox connection/);
    fireEvent.click(screen.getByRole('button', { name: 'Use the installer instead' }));
    expect(onUseInstaller).toHaveBeenCalled();
  });
});

describe('clusterDeployNodeStateFromTarget', () => {
  it('folds server target statuses into the steps an operator sees', () => {
    expect(clusterDeployNodeStateFromTarget('preflighting')).toBe('checking');
    expect(clusterDeployNodeStateFromTarget('ready')).toBe('checking');
    expect(clusterDeployNodeStateFromTarget('installing')).toBe('installing');
    expect(clusterDeployNodeStateFromTarget('enrolling')).toBe('connecting');
    expect(clusterDeployNodeStateFromTarget('succeeded')).toBe('connecting');
    expect(clusterDeployNodeStateFromTarget('failed_retryable')).toBe('failed');
    expect(clusterDeployNodeStateFromTarget('canceled')).toBe('failed');
  });
});
