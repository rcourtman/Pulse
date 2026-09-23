import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

const resourceDetailDrawerMount = vi.hoisted(() => vi.fn());

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: {
    resource: Resource;
    initialShowHostDetails?: boolean;
    onClose?: () => void;
  }) => {
    resourceDetailDrawerMount(props.resource.id);
    return (
      <div
        data-testid="pbs-resource-detail"
        data-resource-id={props.resource.id}
        data-host-details-open={String(props.initialShowHostDetails === true)}
        data-agent-id={props.resource.agent?.agentId}
        data-metrics-resource-id={props.resource.metricsTarget?.resourceId}
        data-metrics-resource-type={props.resource.metricsTarget?.resourceType}
      >
        <button type="button" onClick={props.onClose}>
          Close details
        </button>
      </div>
    );
  },
}));

const makePbsResource = (): Resource =>
  ({
    id: 'pbs-1',
    type: 'pbs',
    name: 'pbs-main',
    displayName: 'PBS Main',
    platformId: 'pbs-main',
    platformType: 'proxmox-pbs',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    cpu: { current: 12.4 },
    memory: { current: 40, total: 8_000, used: 3_200, free: 4_800 },
    pbs: {
      instanceId: 'pbs-main',
      version: '3.2.1',
      connectionHealth: 'healthy',
      datastores: [{ name: 'tank', total: 1_000, used: 400, available: 600, usagePercent: 40 }],
    },
    platformData: {
      sources: ['pbs', 'agent'],
      agent: {
        agentId: 'agent-pbs-1',
        hostname: 'pbs-main',
        osName: 'Debian GNU/Linux',
        disks: [{ mountpoint: '/', total: 10_000, used: 4_000 }],
      },
      pbs: { instanceId: 'pbs-main', hostname: 'pbs-main', datastoreCount: 1 },
    },
  }) as Resource;

describe('ProxmoxBackupServersTable details', () => {
  beforeEach(() => {
    resourceDetailDrawerMount.mockClear();
  });

  it('keeps the open detail drawer mounted across refreshed PBS snapshots', async () => {
    const [servers, setServers] = createSignal<Resource[]>([makePbsResource()]);
    render(() => <ProxmoxBackupServersTable servers={servers()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));
    expect(resourceDetailDrawerMount).toHaveBeenCalledTimes(1);

    const refreshed = makePbsResource();
    refreshed.pbs = { ...refreshed.pbs!, version: '3.2.2' };
    setServers([refreshed]);

    await waitFor(() => expect(screen.getByText('3.2.2')).toBeInTheDocument());
    expect(resourceDetailDrawerMount).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('pbs-resource-detail')).toBeInTheDocument();
  });

  it('keeps each datastore identity and open drawer through reordered snapshots', async () => {
    const makeServers = (reverse = false) => {
      const pbs = makePbsResource();
      pbs.pbs!.datastores!.push({
        name: 'archive',
        total: 2000,
        used: 600,
        available: 1400,
        usagePercent: 30,
      });
      if (reverse) pbs.pbs!.datastores!.reverse();
      return [pbs];
    };
    const [servers, setServers] = createSignal(makeServers());
    const { container } = render(() => <ProxmoxBackupServersTable servers={servers()} />);
    const datastoreNames = () =>
      Array.from(container.querySelectorAll('td[title]')).map((cell) => cell.getAttribute('title'));
    fireEvent.click(screen.getAllByRole('button', { name: 'Expand details for pbs-main' })[1]);
    const detail = screen.getByTestId('pbs-resource-detail');
    expect(datastoreNames()).toEqual(['pbs-main · archive', 'pbs-main · tank']);

    setServers(makeServers(true));
    await waitFor(() =>
      expect(datastoreNames()).toEqual(['pbs-main · archive', 'pbs-main · tank']),
    );
    expect(screen.getByTestId('pbs-resource-detail')).toBe(detail);
    expect(resourceDetailDrawerMount).toHaveBeenCalledTimes(1);
  });

  it('opens the canonical resource drawer with merged host details expanded', () => {
    render(() => <ProxmoxBackupServersTable servers={[makePbsResource()]} />);

    fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));

    const detail = screen.getByTestId('pbs-resource-detail');
    expect(detail).toHaveAttribute('data-resource-id', 'pbs-1');
    expect(detail).toHaveAttribute('data-host-details-open', 'true');

    fireEvent.click(screen.getByRole('button', { name: 'Close details' }));
    expect(screen.queryByTestId('pbs-resource-detail')).not.toBeInTheDocument();
  });

  it.each(['agent', 'vm', 'system-container'] as const)(
    'uses the uniquely correlated %s resource for host details and metrics history',
    (type) => {
      const pbs = makePbsResource();
      pbs.sources = ['pbs'];
      pbs.agent = undefined;
      pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-main' };
      pbs.platformData = {
        sources: ['pbs'],
        pbs: { instanceId: 'pbs-main', hostname: 'pbs-main', datastoreCount: 1 },
      };
      const agent = {
        id: 'agent-host-1',
        type,
        name: 'pbs-main.local',
        displayName: 'PBS host',
        platformId: 'agent-host-1',
        platformType: 'proxmox-pbs',
        sourceType: 'hybrid',
        sources: ['agent', 'pbs'],
        status: 'online',
        lastSeen: pbs.lastSeen + 1_000,
        agent: { agentId: 'agent-pbs-1', hostname: 'pbs-main.local', osName: 'Debian GNU/Linux' },
        metricsTarget: { resourceType: type, resourceId: 'agent-pbs-1' },
        platformData: {
          sources: ['agent', 'pbs'],
          agent: { agentId: 'agent-pbs-1', hostname: 'pbs-main.local' },
        },
      } as Resource;

      expect(agent.disk).toBeUndefined();
      render(() => <ProxmoxBackupServersTable servers={[pbs, agent]} />);
      fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));

      const detail = screen.getByTestId('pbs-resource-detail');
      expect(detail).toHaveAttribute('data-resource-id', 'pbs-1');
      expect(detail).toHaveAttribute('data-agent-id', 'agent-pbs-1');
      expect(detail).toHaveAttribute('data-metrics-resource-id', 'agent-pbs-1');
      expect(detail).toHaveAttribute('data-metrics-resource-type', type);
    },
  );

  it('correlates a PBS connection configured by IP to its agent via the reported node name', () => {
    const pbs = makePbsResource();
    pbs.sources = ['pbs'];
    pbs.agent = undefined;
    pbs.name = 'backup-connection';
    pbs.displayName = 'Backup connection';
    pbs.platformId = 'pbs-1';
    pbs.pbs = {
      ...pbs.pbs!,
      instanceId: 'pbs-1',
      hostname: '10.0.0.5',
      nodeName: 'pbs-one.local',
    };
    pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-1' };
    pbs.platformData = {
      sources: ['pbs'],
      pbs: {
        instanceId: 'pbs-1',
        hostname: '10.0.0.5',
        nodeName: 'pbs-one.local',
        datastoreCount: 1,
      },
    };
    const agent = {
      id: 'agent-host-1',
      type: 'agent',
      name: 'pbs-one.local',
      displayName: 'PBS host',
      platformId: 'agent-host-1',
      platformType: 'proxmox-pbs',
      sourceType: 'hybrid',
      sources: ['agent', 'pbs'],
      status: 'online',
      lastSeen: pbs.lastSeen + 1_000,
      agent: { agentId: 'agent-pbs-1', hostname: 'pbs-one.local', osName: 'Debian GNU/Linux' },
      identity: { hostname: 'pbs-one.local', ips: ['10.9.9.9'] },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-pbs-1' },
      platformData: {
        sources: ['agent', 'pbs'],
        agent: { agentId: 'agent-pbs-1', hostname: 'pbs-one.local' },
      },
    } as Resource;

    render(() => <ProxmoxBackupServersTable servers={[pbs, agent]} />);
    fireEvent.click(screen.getByRole('row', { name: /backup-connection/ }));

    const detail = screen.getByTestId('pbs-resource-detail');
    expect(detail).toHaveAttribute('data-agent-id', 'agent-pbs-1');
    expect(detail).toHaveAttribute('data-metrics-resource-id', 'agent-pbs-1');
    expect(detail).toHaveAttribute('data-metrics-resource-type', 'agent');
  });

  it('does not correlate a guest without host telemetry just by name', () => {
    const pbs = makePbsResource();
    pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-main' };
    const guest = {
      ...pbs,
      id: 'vm-unrelated',
      type: 'vm',
      agent: undefined,
      platformData: {},
      pbs: undefined,
      metricsTarget: { resourceType: 'vm', resourceId: 'unrelated' },
    } as Resource;
    render(() => <ProxmoxBackupServersTable servers={[pbs, guest]} />);
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));
    expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
      'data-metrics-resource-id',
      'pbs-main',
    );
  });

  it.each(['agent', 'vm'] as const)(
    'does not guess when an agent and %s share the PBS hostname',
    (type) => {
      const pbs = makePbsResource();
      pbs.sources = ['pbs'];
      pbs.agent = undefined;
      pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-main' };
      const candidate = (id: string): Resource =>
        ({
          id,
          type: id === 'agent-pbs-b' ? type : 'agent',
          name: 'pbs-main.local',
          displayName: id,
          platformId: id,
          platformType: 'proxmox-pbs',
          sourceType: 'hybrid',
          sources: ['agent', 'pbs'],
          status: 'online',
          lastSeen: pbs.lastSeen,
          agent: { agentId: id, hostname: 'pbs-main.local' },
          metricsTarget: { resourceType: 'agent', resourceId: id },
        }) as Resource;

      render(() => (
        <ProxmoxBackupServersTable
          servers={[pbs, candidate('agent-pbs-a'), candidate('agent-pbs-b')]}
        />
      ));
      fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));

      expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
        'data-metrics-resource-id',
        'pbs-main',
      );
    },
  );

  it('reuses the PVE guest when the same agent is listed as both guest and standalone host', () => {
    const pbs = makePbsResource();
    pbs.sources = ['pbs'];
    pbs.agent = undefined;
    pbs.name = 'proxback';
    pbs.displayName = 'proxback';
    pbs.platformId = 'pbs-1';
    pbs.pbs = { ...pbs.pbs!, instanceId: 'proxback', hostname: 'proxback-vm' };
    pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-1' };
    pbs.platformData = {
      sources: ['pbs'],
      pbs: { instanceId: 'proxback', hostname: 'proxback-vm', datastoreCount: 1 },
    };
    const sharedAgent = { agentId: 'agent-proxback', hostname: 'proxback-vm' };
    const guest = {
      id: 'vm-100',
      type: 'vm',
      name: 'proxback-vm',
      displayName: 'proxback-vm',
      platformId: 'proxmox:100',
      platformType: 'proxmox-pve',
      sourceType: 'hybrid',
      sources: ['proxmox', 'agent'],
      status: 'online',
      lastSeen: pbs.lastSeen,
      agent: sharedAgent,
      metricsTarget: { resourceType: 'vm', resourceId: 'proxmox:100' },
      platformData: { sources: ['proxmox', 'agent'], agent: sharedAgent },
    } as Resource;
    const standalone = {
      id: 'agent-proxback',
      type: 'agent',
      name: 'proxback-vm',
      displayName: 'proxback-vm',
      platformId: 'agent-proxback',
      platformType: 'proxmox-pbs',
      sourceType: 'hybrid',
      sources: ['agent', 'pbs'],
      status: 'online',
      lastSeen: pbs.lastSeen,
      agent: sharedAgent,
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
    } as Resource;

    render(() => <ProxmoxBackupServersTable servers={[pbs, guest, standalone]} />);
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for proxback' }));

    expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
      'data-metrics-resource-type',
      'vm',
    );
    expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
      'data-metrics-resource-id',
      'proxmox:100',
    );
  });

  it('retains the correlated host target when a refreshed snapshot omits the agent row', async () => {
    const makeServers = (): [Resource, Resource] => {
      const pbs = makePbsResource();
      pbs.id = 'pbs-1';
      pbs.name = 'proxback';
      pbs.displayName = 'proxback';
      pbs.platformId = 'pbs-1';
      pbs.sources = ['pbs'];
      pbs.agent = undefined;
      // The PBS service target names the service key, not the host series.
      pbs.metricsTarget = { resourceType: 'agent', resourceId: 'proxback' };
      pbs.pbs = {
        instanceId: 'proxback',
        hostname: 'proxback-vm',
        version: '3.2.1',
        connectionHealth: 'healthy',
        datastores: [{ name: 'tank', total: 1000, used: 400, available: 600, usagePercent: 40 }],
      };
      pbs.platformData = {
        sources: ['pbs'],
        pbs: { instanceId: 'proxback', hostname: 'proxback-vm', datastoreCount: 1 },
      };
      const agent = {
        id: 'agent-proxback',
        type: 'agent',
        name: 'proxback-vm',
        displayName: 'proxback-vm',
        platformId: 'agent-proxback',
        platformType: 'proxmox-pbs',
        sourceType: 'hybrid',
        sources: ['agent', 'pbs'],
        status: 'online',
        lastSeen: pbs.lastSeen + 1000,
        agent: { agentId: 'agent-proxback', hostname: 'proxback-vm' },
        metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
        platformData: {
          sources: ['agent', 'pbs'],
          agent: { agentId: 'agent-proxback', hostname: 'proxback-vm' },
        },
      } as Resource;
      return [pbs, agent];
    };
    const [pbs, agent] = makeServers();
    const [servers, setServers] = createSignal<Resource[]>([pbs, agent]);
    render(() => <ProxmoxBackupServersTable servers={servers()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Expand details for proxback' }));
    expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
      'data-metrics-resource-id',
      'agent-proxback',
    );

    // A live refresh snapshot can transiently omit the correlated agent row.
    // The drawer target must not fall back to the PBS service target.
    setServers([pbs]);
    await waitFor(() =>
      expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
        'data-metrics-resource-id',
        'agent-proxback',
      ),
    );

    // When the agent returns, the retained correlation is confirmed, not re-guessed.
    setServers([pbs, agent]);
    await waitFor(() =>
      expect(screen.getByTestId('pbs-resource-detail')).toHaveAttribute(
        'data-metrics-resource-id',
        'agent-proxback',
      ),
    );
  });
});
