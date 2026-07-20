import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { DiskDetail } from '@/components/Storage/DiskDetail';
import type { Resource } from '@/types/resource';

vi.mock('@/components/shared/HistoryChart', () => ({
  HistoryChart: (props: {
    resourceType: string;
    resourceId: string;
    metric: string;
    range?: string;
  }) => (
    <div data-testid="history-chart">
      {props.resourceType}:{props.resourceId}:{props.metric}:{props.range}
    </div>
  ),
}));

const buildDisk = (): Resource =>
  ({
    id: 'disk-1',
    type: 'physical_disk',
    name: 'disk-1',
    displayName: 'Archive HDD',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    metricsTarget: { resourceType: 'disk', resourceId: 'agent-tower:sda' },
    identity: { hostname: 'tower' },
    canonicalIdentity: { hostname: 'tower' },
    platformData: {
      proxmox: { nodeName: 'tower', instance: 'cluster-main' },
    },
    physicalDisk: {
      devPath: '/dev/sda',
      model: 'Archive HDD',
      serial: 'SERIAL-1',
      diskType: 'hdd',
      temperature: 42,
      smart: {
        powerOnHours: 100,
        reallocatedSectors: 0,
      },
    },
  }) as unknown as Resource;

describe('DiskDetail', () => {
  it('keeps disk detail range choices inside the current history entitlement', () => {
    render(() => <DiskDetail disk={buildDisk()} nodes={[]} />);

    const rangeSelector = screen.getByRole('combobox', {
      name: 'Disk history range',
    }) as HTMLSelectElement;

    expect(Array.from(rangeSelector.options).map((option) => option.value)).toEqual([
      '1h',
      '6h',
      '12h',
      '24h',
      '7d',
    ]);
    expect(Array.from(rangeSelector.options).map((option) => option.value)).not.toContain('14d');
    expect(Array.from(rangeSelector.options).map((option) => option.value)).not.toContain('90d');
  });
});
