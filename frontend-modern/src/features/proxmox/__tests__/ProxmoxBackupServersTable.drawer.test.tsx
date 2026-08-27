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
});
