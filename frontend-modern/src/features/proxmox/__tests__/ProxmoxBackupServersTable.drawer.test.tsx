import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: {
    resource: Resource;
    initialShowHostDetails?: boolean;
    onClose?: () => void;
  }) => (
    <div
      data-testid="pbs-resource-detail"
      data-resource-id={props.resource.id}
      data-host-details-open={String(props.initialShowHostDetails === true)}
      data-agent-id={props.resource.agent?.agentId}
      data-metrics-resource-id={props.resource.metricsTarget?.resourceId}
    >
      <button type="button" onClick={props.onClose}>
        Close details
      </button>
    </div>
  ),
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
  it('opens the canonical resource drawer with merged host details expanded', () => {
    render(() => <ProxmoxBackupServersTable servers={[makePbsResource()]} />);

    fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));

    const detail = screen.getByTestId('pbs-resource-detail');
    expect(detail).toHaveAttribute('data-resource-id', 'pbs-1');
    expect(detail).toHaveAttribute('data-host-details-open', 'true');

    fireEvent.click(screen.getByRole('button', { name: 'Close details' }));
    expect(screen.queryByTestId('pbs-resource-detail')).not.toBeInTheDocument();
  });

  it('uses the uniquely correlated agent resource for host details and metrics history', () => {
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
      type: 'agent',
      name: 'pbs-main.local',
      displayName: 'PBS host',
      platformId: 'agent-host-1',
      platformType: 'proxmox-pbs',
      sourceType: 'hybrid',
      sources: ['agent', 'pbs'],
      status: 'online',
      lastSeen: pbs.lastSeen + 1_000,
      agent: { agentId: 'agent-pbs-1', hostname: 'pbs-main.local', osName: 'Debian GNU/Linux' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-pbs-1' },
      platformData: {
        sources: ['agent', 'pbs'],
        agent: { agentId: 'agent-pbs-1', hostname: 'pbs-main.local' },
      },
    } as Resource;

    render(() => <ProxmoxBackupServersTable servers={[pbs, agent]} />);
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for pbs-main' }));

    const detail = screen.getByTestId('pbs-resource-detail');
    expect(detail).toHaveAttribute('data-resource-id', 'pbs-1');
    expect(detail).toHaveAttribute('data-agent-id', 'agent-pbs-1');
    expect(detail).toHaveAttribute('data-metrics-resource-id', 'agent-pbs-1');
  });

  it('does not guess when two agent resources share the PBS hostname', () => {
    const pbs = makePbsResource();
    pbs.sources = ['pbs'];
    pbs.agent = undefined;
    pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-main' };
    const candidate = (id: string): Resource =>
      ({
        id,
        type: 'agent',
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
  });
});
