import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    fallbackMetrics?: Record<string, number | undefined>;
  }) => (
    <div
      data-testid="machine-history"
      data-resource-type={props.target?.resourceType}
      data-resource-id={props.target?.resourceId}
      data-range={props.range}
      data-cpu={props.fallbackMetrics?.cpu}
      data-netin={props.fallbackMetrics?.netin}
    />
  ),
  GuestDrawerHistoryRangeSelect: (props: {
    range: string;
    onRangeChange: (range: string) => void;
  }) => (
    <select
      aria-label="History range"
      value={props.range}
      onChange={(event) => props.onRangeChange(event.currentTarget.value)}
    >
      <option value="24h">24 hours</option>
      <option value="7d">7 days</option>
    </select>
  ),
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: (props: {
    resourceType: string;
    agentId?: string;
    resourceId: string;
    hostname: string;
    showManualRunAction?: boolean;
  }) => (
    <div
      data-testid="machine-discovery"
      data-resource-type={props.resourceType}
      data-agent-id={props.agentId}
      data-resource-id={props.resourceId}
      data-hostname={props.hostname}
      data-manual-run={String(props.showManualRunAction === true)}
    />
  ),
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: (props: { mode?: string }) => (
    <div data-testid="stacked-disk-bar" data-mode={props.mode} />
  ),
}));

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'agent-1',
    name: overrides.name ?? overrides.id ?? 'agent-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'agent-1',
    type: overrides.type ?? 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('ResourceDetailDrawer machine metrics history', () => {
  it('adds a metrics history tab for Pulse Agent machines', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'agent-mac-mini',
          metricsTarget: { resourceType: 'agent', resourceId: 'agent-mac-mini' },
          cpu: { current: 42 },
          network: { rxBytes: 1024, txBytes: 2048 },
        })}
      />
    ));

    await fireEvent.click(screen.getByRole('tab', { name: 'History' }));

    const history = screen.getByTestId('machine-history');
    expect(history).toHaveAttribute('data-resource-type', 'agent');
    expect(history).toHaveAttribute('data-resource-id', 'agent-mac-mini');
    expect(history).toHaveAttribute('data-range', '24h');
    expect(history).toHaveAttribute('data-cpu', '42');
    expect(history).toHaveAttribute('data-netin', '1024');
  });

  it('does not add machine history to non-agent availability resources', () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'ping-mac-mini',
          type: 'network-endpoint',
          platformType: 'availability',
          sourceType: 'api',
        })}
      />
    ));

    expect(screen.queryByRole('tab', { name: 'History' })).not.toBeInTheDocument();
  });

  it('adds a first-class discovery tab for Pulse Agent machines', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'agent-mac-mini',
          displayName: 'Mac Mini',
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-mac-mini',
            resourceId: 'agent-mac-mini',
            hostname: 'richard-mac-mini.local',
          },
        })}
      />
    ));

    expect(screen.getByRole('tab', { name: 'Discovery' })).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('tab', { name: 'Discovery' }));

    const discovery = screen.getByTestId('machine-discovery');
    expect(discovery).toHaveAttribute('data-resource-type', 'agent');
    expect(discovery).toHaveAttribute('data-agent-id', 'agent-mac-mini');
    expect(discovery).toHaveAttribute('data-resource-id', 'agent-mac-mini');
    expect(discovery).toHaveAttribute('data-hostname', 'richard-mac-mini.local');
    expect(discovery).toHaveAttribute('data-manual-run', 'true');
  });

  it('opens agent machine facts by default in table-row presentation', () => {
    render(() => (
      <ResourceDetailDrawer
        presentation="table-row"
        resource={resource({
          id: 'agent-mac-mini',
          displayName: 'Mac Mini',
          agent: {
            hostname: 'richard-mac-mini.local',
            osName: 'macOS',
            osVersion: '15.5',
            kernelVersion: '24.5.0',
            architecture: 'arm64',
            cpuCount: 10,
            memory: {
              total: 24 * 1024 * 1024 * 1024,
              used: 12 * 1024 * 1024 * 1024,
              free: 12 * 1024 * 1024 * 1024,
              usage: 0.5,
            },
            disks: [
              {
                mountpoint: '/',
                total: 994 * 1024 * 1024 * 1024,
                used: 512 * 1024 * 1024 * 1024,
                free: 482 * 1024 * 1024 * 1024,
              },
            ],
            networkInterfaces: [
              {
                name: 'en0',
                mac: '00:11:22:33:44:55',
                addresses: ['192.168.0.42'],
              },
            ],
            agentVersion: '6.0.0',
            uptimeSeconds: 3600,
          },
        })}
      />
    ));

    const machineSection = screen.getByTestId('resource-host-details-section');
    expect(within(machineSection).getByText('Machine')).toBeInTheDocument();
    expect(
      within(machineSection).getByRole('button', { name: 'Hide details' }),
    ).toBeInTheDocument();
    expect(within(machineSection).queryByRole('button', { name: 'Show details' })).toBeNull();
    expect(within(machineSection).getByText('richard-mac-mini.local')).toBeInTheDocument();
    expect(within(machineSection).getByText('Network')).toBeInTheDocument();
    expect(within(machineSection).getByText('192.168.0.42')).toBeInTheDocument();
    expect(within(machineSection).getByText('Disks')).toBeInTheDocument();
  });

  it('keeps provider-owned host facts collapsed in table-row presentation', () => {
    render(() => (
      <ResourceDetailDrawer
        presentation="table-row"
        resource={resource({
          id: 'pve-node-1',
          displayName: 'pve-node-1',
          platformType: 'proxmox-pve',
          sourceType: 'api',
          sources: ['proxmox'],
          platformData: {
            sources: ['proxmox'],
            proxmox: {
              nodeName: 'pve-node-1',
              pveVersion: '8.2.7',
              kernelVersion: '6.8.12',
              cpuInfo: {
                cores: 8,
              },
            },
          },
        })}
      />
    ));

    const hostSection = screen.getByTestId('resource-host-details-section');
    expect(within(hostSection).getByText('Host')).toBeInTheDocument();
    expect(within(hostSection).getByRole('button', { name: 'Show host' })).toBeInTheDocument();
    expect(within(hostSection).queryByRole('button', { name: 'Hide host' })).toBeNull();
    expect(within(hostSection).queryByText('Hardware')).toBeNull();
  });
});
