import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createStore, reconcile } from 'solid-js/store';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

const expandPlatformDetails = (): void => {
  // Platform context sits inside the Overview's one Details disclosure.
  const details = screen.getByTestId('resource-technical-details') as HTMLDetailsElement;
  details.open = true;
  fireEvent(details, new Event('toggle'));
};

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    currentMetrics?: Record<string, number | undefined>;
    groups?: Array<{ id: string; series: Array<{ metric: string }> }>;
  }) => (
    <div
      data-testid="machine-history"
      data-resource-type={props.target?.resourceType}
      data-resource-id={props.target?.resourceId}
      data-range={props.range}
      data-cpu={props.currentMetrics?.cpu}
      data-netin={props.currentMetrics?.netin}
      data-groups={props.groups?.map((group) => group.id).join(',')}
      data-metrics={props.groups
        ?.flatMap((group) => group.series.map((series) => series.metric))
        .join(',')}
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
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('ResourceDetailDrawer machine inventory', () => {
  const gib = 1024 ** 3;
  it('keeps startup waiting informational and withholds the current history fallback', () => {
    const [machine, setMachine] = createStore(
      resource({
        status: 'unknown',
        sourceStatus: { agent: { status: 'pending' } },
        cpu: { current: 12 },
      }),
    );
    render(() => <ResourceDetailDrawer resource={machine} />);
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for first report');
    expect(screen.queryByTestId('resource-drawer-exceptions')).toBeNull();
    expect(screen.getByTestId('machine-history')).not.toHaveAttribute('data-cpu');
    setMachine({ status: 'online', sourceStatus: { agent: { status: 'online' } } });
    expect(screen.queryByRole('status')).toBeNull();
    expect(screen.getByTestId('machine-history')).toHaveAttribute('data-cpu', '12');
  });
  it('states one facts line and keeps the inventory under a closed Details', () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'agent-facts',
          memory: { current: 50, total: 16 * gib, used: 8 * gib },
          disk: { current: 25, total: 200 * gib, used: 50 * gib },
          agent: { agentId: 'agent-facts', cpuCount: 8 } as Resource['agent'],
        })}
      />
    ));
    expect(screen.getByTestId('drawer-no-attention')).toHaveTextContent('No active alerts');
    expect(screen.queryByTestId('resource-drawer-exceptions')).toBeNull();
    const facts = screen.getByTestId('resource-drawer-facts');
    expect(facts).toHaveTextContent('8 logical CPUs');
    expect(facts).toHaveTextContent('8.00 GB of 16.0 GB RAM');
    expect(facts).toHaveTextContent('50.0 GB of 200 GB disk');
    const details = screen.getByTestId('resource-technical-details') as HTMLDetailsElement;
    expect(details.open).toBe(false);
    expect(within(details).getByTestId('resource-runtime-context-section')).toBeInTheDocument();
  });

  it('lifts an offline machine in front and states its sizes, not stale used amounts', () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'agent-offline',
          status: 'offline',
          lastSeen: Date.now() - 14 * 60_000,
          memory: { current: 50, total: 16 * gib, used: 8 * gib },
        })}
      />
    ));
    const lifted = screen.getByTestId('resource-drawer-exceptions');
    expect(within(lifted).getByText('Observed state')).toBeInTheDocument();
    expect(within(lifted).getByText(/^Offline · last seen/)).toBeInTheDocument();
    const facts = screen.getByTestId('resource-drawer-facts');
    expect(facts).toHaveTextContent('16.0 GB RAM');
    expect(facts).not.toHaveTextContent('of 16.0 GB');
  });

  it('keeps a passing check under Details for a machine but in front for an endpoint', () => {
    const passing = {
      available: true,
      aggregateState: 'healthy',
      pollIntervalSeconds: 30,
      lastChecked: new Date(Date.now() - 10_000).toISOString(),
    } as Resource['availability'];
    const machine = render(() => (
      <ResourceDetailDrawer resource={resource({ id: 'agent-checked', availability: passing })} />
    ));
    expect(screen.getByTestId('drawer-no-attention')).toHaveTextContent(
      'availability checks passing',
    );
    const details = screen.getByTestId('resource-technical-details');
    expect(details.contains(screen.getByTestId('availability-probe-status'))).toBe(true);
    machine.unmount();

    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'endpoint-a',
          type: 'network-endpoint',
          platformType: 'availability',
          availability: passing,
        })}
      />
    ));
    // The check is the endpoint's whole reading, so it never folds away.
    const card = screen.getByTestId('availability-probe-status');
    expect(card.closest('details')).toBeNull();
  });

  it('states sizes only while the memory reading is retained rather than current', () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'agent-retained',
          memory: {
            current: 50,
            total: 16 * gib,
            used: 8 * gib,
            observation: { state: 'last-known', source: 'agent' },
          },
        })}
      />
    ));
    const facts = screen.getByTestId('resource-drawer-facts');
    expect(facts).toHaveTextContent('16.0 GB RAM');
    expect(facts).not.toHaveTextContent('of 16.0 GB');
  });
});

describe('ResourceDetailDrawer machine metrics history', () => {
  it('charts metrics history on Overview for Pulse Agent machines, with no History tab', async () => {
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

    expect(screen.queryByRole('tab', { name: 'History' })).not.toBeInTheDocument();
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
    expect(screen.queryByTestId('machine-history')).not.toBeInTheDocument();
  });

  it('renders storage history with the capacity catalog instead of workload metrics', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'storage-pool-tank',
          type: 'storage',
          metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
          disk: { current: 0, used: 30, total: 100 },
        })}
      />
    ));

    const history = screen.getByTestId('machine-history');
    expect(history).toHaveAttribute('data-resource-type', 'storage');
    expect(history).toHaveAttribute('data-groups', 'capacity');
    expect(history).toHaveAttribute('data-metrics', 'usage');
  });

  it('renders physical-disk history with only canonical disk metrics', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'disk-sdc',
          type: 'storage',
          metricsTarget: { resourceType: 'disk', resourceId: 'WD-WX12A3456' },
          disk: { current: 42 },
          temperature: 39,
        })}
      />
    ));

    const history = screen.getByTestId('machine-history');
    expect(history).toHaveAttribute('data-resource-type', 'disk');
    expect(history).toHaveAttribute('data-groups', 'disk-activity,disk-io,disk-thermal');
    expect(history).toHaveAttribute('data-metrics', 'disk,diskread,diskwrite,smart_temp');
  });

  it('renders explicit Docker host history with the canonical host catalog', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'nebula-1-mock',
          type: 'docker-host',
          metricsTarget: { resourceType: 'docker-host', resourceId: 'nebula-1-mock' },
          temperature: 58,
        })}
      />
    ));

    const history = screen.getByTestId('machine-history');
    expect(history).toHaveAttribute('data-resource-type', 'docker-host');
    expect(history).toHaveAttribute('data-groups', 'utilization,network,disk-io,thermals');
    expect(history).toHaveAttribute(
      'data-metrics',
      'cpu,memory,disk,netin,netout,diskread,diskwrite,temperature',
    );
  });

  it('keeps agent machine facts behind the shared nested detail disclosures', () => {
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

    // Provider host facts wait inside the closed Overview Details disclosure.
    expect(
      screen.getByTestId('resource-host-details-section').closest('details'),
    ).not.toHaveAttribute('open');
    expandPlatformDetails();
    const machineSection = screen.getByTestId('resource-host-details-section');
    expect(within(machineSection).getByText('Machine')).toBeInTheDocument();
    expect(
      within(machineSection).getByRole('button', { name: 'Show details' }),
    ).toBeInTheDocument();
    expect(within(machineSection).queryByRole('button', { name: 'Hide details' })).toBeNull();
    fireEvent.click(within(machineSection).getByRole('button', { name: 'Show details' }));
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

    // Provider host facts wait inside the closed Overview Details disclosure.
    expect(
      screen.getByTestId('resource-host-details-section').closest('details'),
    ).not.toHaveAttribute('open');
    expandPlatformDetails();
    const hostSection = screen.getByTestId('resource-host-details-section');
    expect(within(hostSection).getByText('Host')).toBeInTheDocument();
    expect(within(hostSection).getByRole('button', { name: 'Show host' })).toBeInTheDocument();
    expect(within(hostSection).queryByRole('button', { name: 'Hide host' })).toBeNull();
    expect(within(hostSection).queryByText('Hardware')).toBeNull();
  });

  it('withdraws and restores the Overview charts when a snapshot briefly drops the metrics target', async () => {
    const initial = resource({
      id: 'pbs-1',
      type: 'pbs',
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-pbs-1' },
      cpu: { current: 12 },
    });
    const [pbs, setPbs] = createStore<Resource>(initial);
    render(() => <ResourceDetailDrawer resource={pbs} />);
    expect(screen.getByTestId('machine-history')).toHaveAttribute(
      'data-resource-id',
      'agent-pbs-1',
    );

    // A snapshot refresh arrives without the merged metrics target. Overview
    // stays selected without charts, and they return when the target does.
    setPbs(reconcile({ ...initial, metricsTarget: undefined }, { merge: false }));
    expect(screen.queryByTestId('machine-history')).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');

    setPbs(
      reconcile(
        resource({
          id: 'pbs-1',
          type: 'pbs',
          metricsTarget: { resourceType: 'agent', resourceId: 'agent-pbs-1' },
          cpu: { current: 12 },
        }),
        { merge: false },
      ),
    );
    expect(screen.getByTestId('machine-history')).toHaveAttribute(
      'data-resource-id',
      'agent-pbs-1',
    );
  });
});
