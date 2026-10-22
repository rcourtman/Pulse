import { createSignal } from 'solid-js';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { DiskDrawer } from '@/components/Storage/DiskDrawer';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

const historySpy = vi.fn();

// The drawer charts with the shared drawer history component; its own suite
// covers the charts. This stub records what the disk asks it to draw.
vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    enabled?: boolean;
    groups?: Array<{ id: string; series: Array<{ metric: string }> }>;
  }) => {
    historySpy(props);
    return (
      <div
        data-testid="drawer-history"
        data-enabled={String(props.enabled !== false)}
        data-range={props.range}
        data-metrics={props.groups
          ?.flatMap((group) => group.series.map((series) => series.metric))
          .join(',')}
      >
        {props.target?.resourceType}:{props.target?.resourceId}
      </div>
    );
  },
}));

vi.mock('@/components/Infrastructure/ResourceOperatorStateSection', () => ({
  ResourceOperatorStateSection: (props: { resourceId: string; resourceType?: string }) => (
    <div data-testid="operator-state">
      {props.resourceType}:{props.resourceId}
    </div>
  ),
}));

const buildDisk = (overrides: Partial<NonNullable<Resource['physicalDisk']>> = {}): Resource =>
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
      serial: 'SERIAL-WITH-A-VERY-LONG-IDENTIFIER-0123456789',
      diskType: 'hdd',
      health: 'PASSED',
      temperature: 42,
      sizeBytes: 4_000_000_000_000,
      smart: { powerOnHours: 100, reallocatedSectors: 0, pendingSectors: 0 },
      ...overrides,
    },
  }) as unknown as Resource;

const alert = (overrides: Partial<Alert> = {}): Alert => ({
  id: 'alert-1',
  type: 'disk-health',
  level: 'critical',
  resourceId: 'cluster-main:tower:disk:dev-sda',
  resourceName: 'Archive HDD (/dev/sda)',
  node: 'tower',
  instance: 'cluster-main',
  message: 'Disk health check failed',
  value: 0,
  threshold: 0,
  startTime: '2026-10-20T00:00:00Z',
  acknowledged: false,
  ...overrides,
});

const renderDrawer = (
  disk: () => Resource,
  props: { alerts?: Alert[]; onClose?: () => void } = {},
) =>
  render(() => (
    <DiskDrawer disk={disk()} alerts={props.alerts} onClose={props.onClose ?? (() => {})} />
  ));

describe('DiskDrawer', () => {
  it('heads the drawer with the disk model and the Health cell verdict, and collapses', () => {
    const onClose = vi.fn();
    renderDrawer(() => buildDisk({ health: 'FAILED' }), { onClose });

    expect(screen.getByRole('heading', { level: 2, name: 'Archive HDD' })).toBeInTheDocument();
    const dot = screen.getByLabelText('Replace Now');
    expect(dot.className).toContain('red');

    fireEvent.click(screen.getByRole('button', { name: 'Collapse Archive HDD details' }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('says there are no active alerts, or lists the disk alerts first', () => {
    const [alerts, setAlerts] = createSignal<Alert[]>([]);
    render(() => <DiskDrawer disk={buildDisk()} alerts={alerts()} onClose={() => {}} />);
    expect(screen.getByTestId('drawer-no-attention')).toHaveTextContent('No active alerts');

    setAlerts([alert()]);
    expect(screen.queryByTestId('drawer-no-attention')).toBeNull();
    expect(screen.getByTestId('drawer-attention-section')).toHaveTextContent(
      'Disk health check failed',
    );
  });

  it('lifts the health verdict and out-of-range SMART attributes into Worth checking', () => {
    renderDrawer(() =>
      buildDisk({
        health: 'FAILED',
        smart: { powerOnHours: 100, reallocatedSectors: 8, pendingSectors: 2 },
      }),
    );

    const worth = screen.getByTestId('disk-drawer-exceptions');
    expect(worth).toHaveTextContent('Replace Now: Disk health has degraded to a critical state.');
    expect(within(worth).getByText('Reallocated Sectors')).toBeInTheDocument();
    expect(within(worth).getByText('Pending Sectors')).toBeInTheDocument();
    // Lifted facts move rather than repeat in Details.
    const details = screen.getByTestId('disk-technical-details');
    expect(within(details).queryByText('Reallocated Sectors')).toBeNull();
    expect(within(details).getByText('Power-On Time')).toBeInTheDocument();
  });

  it('does not restate a health verdict that one of the disk alerts already states', () => {
    renderDrawer(() => buildDisk({ health: 'FAILED' }), { alerts: [alert()] });

    expect(screen.getByTestId('drawer-attention-section')).toHaveTextContent(
      'Disk health check failed',
    );
    expect(screen.queryByTestId('disk-drawer-exceptions')).toBeNull();
    // The verdict stays in the SMART health card instead.
    expect(screen.getByTestId('disk-drawer-health')).toHaveTextContent('Replace Now');
  });

  it('shows no Worth checking for a healthy disk and keeps the health card in front', () => {
    renderDrawer(() => buildDisk());
    expect(screen.queryByTestId('disk-drawer-exceptions')).toBeNull();
    const card = screen.getByTestId('disk-drawer-health-card');
    expect(within(card).getByText('Healthy')).toBeInTheDocument();
    expect(within(card).getByText('42°C')).toBeInTheDocument();
  });

  it('charts temperature, I/O and wear for the disk at the drawer default range', async () => {
    historySpy.mockClear();
    renderDrawer(() => buildDisk());
    const chart = await screen.findByTestId('drawer-history');
    expect(chart).toHaveTextContent('disk:agent-tower:sda');
    expect(chart).toHaveAttribute('data-range', '24h');
    expect(chart).toHaveAttribute(
      'data-metrics',
      'smart_temp,diskread,diskwrite,disk,smart_reallocated_sectors',
    );
    expect(historySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        currentMetrics: expect.objectContaining({ smart_temp: 42, smart_reallocated_sectors: 0 }),
      }),
    );
    // No section of its own called History, and no Live I/O section.
    expect(screen.queryByText('History')).toBeNull();
    expect(screen.queryByText('Live I/O (30m)')).toBeNull();
    expect(screen.queryByRole('combobox', { name: 'Disk history range' })).toBeNull();
  });

  it('leaves I/O panels out, and says why, where the collector cannot read per-disk I/O', async () => {
    renderDrawer(() =>
      buildDisk({
        diskType: 'nvme',
        smart: { percentageUsed: 12, availableSpare: 97 },
        collection: {
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
        },
      }),
    );
    expect(await screen.findByTestId('drawer-history')).toHaveAttribute(
      'data-metrics',
      'smart_temp,smart_percentage_used,smart_available_spare',
    );
    const notices = screen.getByTestId('disk-drawer-notices');
    expect(notices).toHaveTextContent(
      'Temperature is temporarily unavailable: collection deadline exceeded',
    );
    expect(notices).toHaveTextContent('Disk I/O is unsupported: per-member counters unavailable');
    // A retained reading is labelled last known, with no health tone.
    const card = screen.getByTestId('disk-drawer-health-card');
    expect(within(card).getByText('Last known temperature')).toBeInTheDocument();
    expect(within(card).getByText('42°C').closest('td')).toHaveClass('text-muted');
    expect(historySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        deferredMetrics: { smart_temp: expect.objectContaining({ lastKnownValue: 42 }) },
      }),
    );
  });

  it('states plainly when a disk reports no SMART detail at all', () => {
    renderDrawer(() => buildDisk({ smart: undefined, temperature: undefined }));
    expect(screen.getByTestId('disk-drawer-notices')).toHaveTextContent(
      'Detailed SMART attributes are not available for this disk.',
    );
  });

  it('keeps identity in a closed Details disclosure, wrapping the values operators copy', () => {
    renderDrawer(() => buildDisk());
    const details = screen.getByTestId('disk-technical-details') as HTMLDetailsElement;
    expect(details.tagName).toBe('DETAILS');
    expect(details.open).toBe(false);
    const serial = within(details).getByText('SERIAL-WITH-A-VERY-LONG-IDENTIFIER-0123456789');
    expect(serial).toHaveClass('whitespace-normal');
    expect(serial).not.toHaveClass('truncate');
    expect(within(details).getByText('/dev/sda')).toBeInTheDocument();
    expect(screen.getByTestId('disk-drawer-facts')).toHaveTextContent('HDD · 3.64 TB');
  });

  it('pauses the charts while Manage shows the disk operator state', async () => {
    renderDrawer(() => buildDisk());
    expect(await screen.findByTestId('drawer-history')).toHaveAttribute('data-enabled', 'true');
    expect(screen.queryByRole('tab', { name: 'History' })).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(screen.getByTestId('operator-state')).toHaveTextContent('physical_disk:disk-1');
    expect(screen.getByTestId('drawer-history')).toHaveAttribute('data-enabled', 'false');
  });
});
