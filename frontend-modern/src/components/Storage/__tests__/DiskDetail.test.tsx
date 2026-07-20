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

  it('shows explicit collection status and does not render misleading live I/O', () => {
    const disk = buildDisk();
    disk.physicalDisk!.collection = {
      serial: { state: 'available', source: 'smartctl' },
      temperature: {
        state: 'unavailable',
        source: 'smartctl',
        reason: 'collection deadline exceeded',
      },
      io: {
        state: 'unsupported',
        source: 'controller',
        reason: 'per-member counters unavailable',
      },
      controller: { state: 'available', source: 'linux-sysfs' },
      pool: { state: 'available', source: 'zpool-status' },
    };

    render(() => <DiskDetail disk={disk} nodes={[]} />);

    expect(
      screen.getByText('Temperature is temporarily unavailable: collection deadline exceeded'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Disk I/O is unsupported: per-member counters unavailable'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Live I/O (30m)')).not.toBeInTheDocument();
    expect(screen.queryByText(/:diskread:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/:diskwrite:/)).not.toBeInTheDocument();
  });
});
