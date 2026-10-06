import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentMetadataAPI } from '@/api/agentMetadata';
import { MonitoringAPI } from '@/api/monitoring';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { RESOURCE_METADATA_CHANGED_EVENT } from '@/utils/resourceMetadataEvents';
import { AgentsMachinesTable } from '../AgentsMachinesTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: {
    initialShowAccessContext?: boolean;
    initialShowHostDetails?: boolean;
  }) => (
    <div
      data-testid="resource-detail-drawer"
      data-initial-show-access-context={String(props.initialShowAccessContext ?? false)}
      data-initial-show-host-details={String(props.initialShowHostDetails ?? true)}
    />
  ),
}));

vi.mock('@/api/agentMetadata', () => ({
  AgentMetadataAPI: {
    getAllMetadata: vi.fn(async () => ({})),
    deleteMetadata: vi.fn(async () => undefined),
  },
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    deleteAgent: vi.fn(async () => undefined),
  },
}));

// The websocket's activeAlerts is a Solid store, so rows must follow it live.
const activeAlertsRef = vi.hoisted(() => ({ current: {} as Record<string, Alert> }));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: activeAlertsRef.current }),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  },
}));

vi.mock('@/components/Workloads/EnhancedCPUBar', () => ({
  EnhancedCPUBar: (props: {
    usage: number;
    loadAverage?: number;
    cores?: number;
    resourceId?: string;
  }) => (
    <div
      data-testid="agent-machine-cpu-bar"
      data-usage={props.usage}
      data-load-average={props.loadAverage}
      data-cores={props.cores}
      data-resource-id={props.resourceId}
    />
  ),
}));

vi.mock('@/components/Workloads/MetricBar', () => ({
  MetricBar: (props: { value: number; label: string; resourceId?: string }) => (
    <div
      data-testid="agent-machine-gpu-bar"
      data-value={props.value}
      data-label={props.label}
      data-resource-id={props.resourceId}
    />
  ),
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: {
    used: number;
    total: number;
    percentOnly?: number;
    balloon?: number;
    cache?: number;
    cacheInclusiveLabel?: string;
    swapUsed?: number;
    swapTotal?: number;
    resourceId?: string;
  }) => (
    <div
      data-testid="agent-machine-memory-bar"
      data-used={props.used}
      data-total={props.total}
      data-percent-only={props.percentOnly}
      data-balloon={props.balloon}
      data-cache={props.cache}
      data-cache-inclusive-label={props.cacheInclusiveLabel}
      data-swap-used={props.swapUsed}
      data-swap-total={props.swapTotal}
      data-resource-id={props.resourceId}
    />
  ),
}));

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'machine-1',
    name: overrides.name ?? overrides.id ?? 'machine-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'machine-1',
    type: 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
activeAlertsRef.current = activeAlerts;

const emptyIcon = <span data-testid="empty-icon" />;
const getAllAgentMetadataMock = vi.mocked(AgentMetadataAPI.getAllMetadata);
const deleteAgentMetadataMock = vi.mocked(AgentMetadataAPI.deleteMetadata);
const deleteAgentMock = vi.mocked(MonitoringAPI.deleteAgent);

const openMachineColumnPicker = async () => {
  const viewTrigger = screen.getByRole('button', { name: 'View' });
  await fireEvent.click(viewTrigger);
  const viewDialog = screen.getByRole('region', { name: 'View preferences' });
  await fireEvent.click(within(viewDialog).getByTitle('Choose which columns to display'));
  return viewDialog;
};

beforeEach(() => {
  getAllAgentMetadataMock.mockResolvedValue({});
  deleteAgentMetadataMock.mockResolvedValue(undefined);
  deleteAgentMock.mockResolvedValue(undefined);
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
    },
  );
});

afterEach(() => {
  cleanup();
  setActiveAlerts(reconcile({}));
  window.localStorage.clear();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('AgentsMachinesTable', () => {
  it('keeps disk and last-seen telemetry in the phone column set', () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[resource({ id: 'tower', name: 'Tower' })]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const headers = [...container.querySelectorAll('thead th')];
    expect(headers.find((header) => header.textContent?.includes('Machine'))).toHaveClass(
      'w-[30%]',
    );
    expect(headers.find((header) => header.textContent?.includes('Disk'))).toHaveClass('w-[15%]');
    expect(headers.find((header) => header.textContent?.includes('Seen'))).toHaveClass('w-[15%]');
  });

  it('delegates controlled filter resets once to the route owner', () => {
    const onExternalSearchChange = vi.fn();
    const onExternalStatusChange = vi.fn();
    const onResetFilters = vi.fn();

    render(() => (
      <AgentsMachinesTable
        resources={[resource({ id: 'tower', name: 'Tower' })]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
        externalSearch={() => 'Tower'}
        onExternalSearchChange={onExternalSearchChange}
        externalStatus={() => 'online'}
        onExternalStatusChange={onExternalStatusChange}
        onResetFilters={onResetFilters}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));

    expect(onResetFilters).toHaveBeenCalledTimes(1);
    expect(onExternalSearchChange).not.toHaveBeenCalled();
    expect(onExternalStatusChange).not.toHaveBeenCalled();
  });

  it('tints a machine row for the open alerts its drawer lists', () => {
    const agentAlert = (id: string, resourceId: string, overrides: Partial<Alert> = {}): Alert => ({
      id,
      type: 'memory',
      level: 'warning',
      resourceId,
      resourceName: id,
      node: id,
      instance: '',
      message: id,
      value: 91,
      threshold: 85,
      startTime: '2026-10-06T10:00:00Z',
      acknowledged: false,
      ...overrides,
    });
    // Agent alerts are keyed "agent:<agentId>", never the row's resource id.
    setActiveAlerts({
      memory: agentAlert('memory', 'agent:host-tower'),
      disk: agentAlert('disk', 'agent:host-nas/disk:data', { type: 'disk', level: 'critical' }),
      acked: agentAlert('acked', 'agent:host-pi', { acknowledged: true }),
    });
    const machine = (id: string, agentId: string) =>
      resource({ id, name: id, agent: { agentId } } as Partial<Resource>);

    render(() => (
      <AgentsMachinesTable
        resources={[
          machine('tower', 'host-tower'),
          machine('nas', 'host-nas'),
          machine('pi', 'host-pi'),
          machine('laptop', 'host-laptop'),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const row = (id: string) => document.querySelector(`[data-agents-machine-row="${id}"]`)!;
    expect(row('tower')).toHaveClass('bg-yellow-50');
    expect(row('nas')).toHaveClass('bg-red-50');
    // An acknowledged alert stays in the drawer but no longer tints the row.
    for (const id of ['pi', 'laptop']) {
      expect(row(id)).not.toHaveClass('bg-yellow-50');
      expect(row(id)).not.toHaveClass('bg-red-50');
    }

    // The tint follows the live alert store without a re-render: acknowledging
    // clears it, escalation turns it red, and resolution clears it.
    setActiveAlerts('memory', 'acknowledged', true);
    expect(row('tower')).not.toHaveClass('bg-yellow-50');
    setActiveAlerts('memory', { acknowledged: false, level: 'critical' });
    expect(row('tower')).toHaveClass('bg-red-50');
    setActiveAlerts(reconcile({}));
    expect(row('tower')).not.toHaveClass('bg-red-50');
    expect(row('nas')).not.toHaveClass('bg-red-50');
  });

  it('keeps the column picker inside the shared View preferences disclosure', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[resource({ id: 'tower', name: 'Tower' })]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const search = screen.getByPlaceholderText('Search machines');
    const viewTrigger = screen.getByRole('button', { name: 'View' });
    const filterBar = viewTrigger.closest('.filter-bar');

    expect(filterBar).not.toBeNull();
    expect(filterBar).toContainElement(search);
    expect(screen.queryByTitle('Choose which columns to display')).not.toBeInTheDocument();

    const viewDialog = await openMachineColumnPicker();
    expect(within(viewDialog).getByText('Table')).toBeInTheDocument();
    expect(within(viewDialog).getByLabelText('IP')).toBeInTheDocument();
    expect(within(viewDialog).queryByLabelText('GPU')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sort by GPU' })).not.toBeInTheDocument();
  });

  it('keeps identity primary while exposing disk and recency on narrow screens', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[resource({ id: 'tower', name: 'Tower' })]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const machineHead = screen.getByRole('button', { name: 'Sort by Machine' }).closest('th');
    const diskHead = screen.getByRole('button', { name: 'Sort by Disk' }).closest('th');

    expect(machineHead).toHaveClass('platform-table-mobile-w-30', 'w-[30%]', 'sm:w-[28%]');
    expect(diskHead).toHaveClass('w-[15%]', 'sm:w-[20%]');
    expect(diskHead).not.toHaveClass('hidden');
    expect(screen.getByRole('button', { name: 'Sort by CPU' }).closest('th')).toHaveClass(
      'w-[15%]',
      'sm:w-[20%]',
    );
  });

  it('surfaces machine-native monitoring columns for agent machines', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'tower',
            name: 'Tower',
            cpu: { current: 12 },
            memory: { current: 32, total: 100, used: 32, free: 68 },
            disk: { current: 45, total: 100, used: 45, free: 55 },
            network: { rxBytes: 1024, txBytes: 2048 },
            diskIO: { readRate: 4096, writeRate: 8192 },
            temperature: 52,
            uptime: 86_400,
            identity: { ips: ['192.168.0.10'] },
            agent: {
              agentVersion: '6.0.0',
              osName: 'Unraid',
              osVersion: '7.2.2',
              architecture: 'x86_64',
              kernelVersion: '6.12.24',
              memory: {
                total: 100,
                used: 32,
                cache: 10,
              },
              networkInterfaces: [
                {
                  name: 'br0',
                  mac: 'aa:bb:cc:dd:ee:ff',
                  addresses: ['192.168.0.10'],
                  rxBytes: 1024,
                  txBytes: 2048,
                  speedMbps: 1000,
                },
              ],
              raid: [
                {
                  device: '/dev/md0',
                  level: 'raid1',
                  state: 'clean',
                  totalDevices: 2,
                  activeDevices: 2,
                  workingDevices: 2,
                  failedDevices: 0,
                  spareDevices: 0,
                  devices: [],
                  rebuildPercent: 0,
                },
              ],
              sensors: {
                gpu: [
                  {
                    id: '0',
                    name: 'NVIDIA RTX A6000',
                    utilizationPercent: 67,
                    memoryUsedBytes: 12 * 1024 ** 3,
                    memoryTotalBytes: 48 * 1024 ** 3,
                    temperatureCelsius: 71,
                  },
                ],
              },
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.queryByRole('button', { name: 'Sort by Net I/O' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sort by Disk I/O' })).not.toBeInTheDocument();
    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Net I/O'));
    await fireEvent.click(screen.getByLabelText('Disk I/O'));

    expect(screen.getByRole('button', { name: 'Sort by Net I/O' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sort by Disk I/O' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sort by GPU' })).toBeInTheDocument();
    expect(screen.getByText('1.00 KB/s')).toBeInTheDocument();
    expect(screen.getByText('8.00 KB/s')).toBeInTheDocument();
    expect(screen.getByTestId('agent-machine-gpu-bar')).toHaveAttribute('data-value', '67');
    expect(
      screen.getByTitle('NVIDIA RTX A6000: 67% load, 12.0 GB / 48.0 GB VRAM, 71 °C'),
    ).toBeInTheDocument();

    expect(screen.queryByRole('button', { name: 'Sort by IP' })).not.toBeInTheDocument();
    expect(screen.getByTitle('Tower · 192.168.0.10')).toBeInTheDocument();
    await fireEvent.click(screen.getByLabelText('IP'));
    await fireEvent.click(screen.getByLabelText('RAID'));

    expect(screen.getByRole('button', { name: 'Sort by IP' })).toBeInTheDocument();
    // The compact identity stays one line; the selected IP column owns the
    // visible value while the identity tooltip retains the same context.
    expect(screen.getAllByText('192.168.0.10').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('1 clean')).toBeInTheDocument();
    const memoryBar = screen.getByTestId('agent-machine-memory-bar');
    expect(memoryBar).toHaveAttribute('data-cache', '10');
    expect(memoryBar).not.toHaveAttribute('data-cache-inclusive-label', 'Shown in Proxmox');
  });

  it('keeps hostname and primary IP in the single-line machine identity context', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'mac-mini',
            name: 'Mac Mini',
            identity: { hostname: 'richard-mac-mini.local', ips: ['192.168.0.98'] },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(
      screen.getByTitle('Mac Mini · richard-mac-mini.local | 192.168.0.98'),
    ).toBeInTheDocument();
  });

  it('normalizes canonical OS labels from platform fallback values', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'mac-mini',
            name: 'Mac Mini',
            agent: {
              agentVersion: '6.0.0',
              platform: 'macos',
            },
          }),
          resource({
            id: 'linux-server',
            name: 'Linux Server',
            agent: {
              agentVersion: '6.0.0',
              osName: 'Proxmox VE',
              osVersion: '8.3.3',
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getAllByText('macOS').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Proxmox VE 8.3.3').length).toBeGreaterThanOrEqual(1);
  });

  it('explains missing telemetry when the agent version is behind the server', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'old-agent',
            name: 'Old Agent',
            cpu: { current: 3 },
            agent: {
              agentVersion: 'v6.0.0-rc.5',
              platform: 'macos',
            },
          }),
        ]}
        targetAgentVersion="v6.0.0-rc.6"
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getAllByText('old agent').length).toBeGreaterThanOrEqual(2);
    expect(
      screen.getAllByTitle(
        'Update this agent from v6.0.0-rc.5 to v6.0.0-rc.6 for full machine telemetry.',
      ).length,
    ).toBeGreaterThanOrEqual(2);
  });

  it('flags a non-reporting agent version as stale even when the row stays online', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'omv',
            name: 'omv',
            status: 'online',
            agent: {
              agentVersion: '6.0.2',
              stale: true,
              lastReportAt: '2026-07-08T09:00:00Z',
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getByText('(stale)')).toBeInTheDocument();
    expect(screen.getByTitle(/Agent has stopped reporting/)).toBeInTheDocument();
  });

  it('renders multi-disk machine usage as vertical mini-bars', () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'multi-disk',
            name: 'Multi Disk',
            agent: {
              agentVersion: '6.0.0',
              osName: 'Linux',
              disks: [
                {
                  device: '/dev/sda1',
                  mountpoint: '/',
                  type: 'ext4',
                  total: 1000,
                  used: 250,
                  free: 750,
                },
                {
                  device: '/dev/sdb1',
                  mountpoint: '/data',
                  type: 'xfs',
                  total: 1000,
                  used: 910,
                  free: 90,
                },
              ],
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(container.querySelectorAll('[data-stacked-disk-fill="vertical"]')).toHaveLength(2);
    expect(screen.queryByText('max')).not.toBeInTheDocument();
    expect(screen.queryByText('2 disks')).not.toBeInTheDocument();
  });

  it('searches machine-native fields and exposes host-style search affordances', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'mac-mini',
            name: 'Mac Mini',
            identity: { hostname: 'richard-mac-mini.local', ips: ['192.168.0.98'] },
            agent: {
              osName: 'macOS',
              osVersion: '15.5',
              architecture: 'arm64',
              kernelVersion: 'Darwin 24.5.0',
              networkInterfaces: [
                {
                  name: 'en0',
                  mac: '10:20:30:40:50:60',
                  addresses: ['10.0.0.98'],
                },
              ],
            },
          }),
          resource({
            id: 'windows-runner',
            name: 'Windows Runner',
            identity: { ips: ['192.168.0.49'] },
            agent: {
              osName: 'Windows',
              osVersion: '11 Pro',
              architecture: 'x86_64',
              kernelVersion: '10.0.22631',
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const search = screen.getByPlaceholderText('Search machines');

    await fireEvent.input(search, { target: { value: 'macos' } });
    expect(screen.getByText('Mac Mini')).toBeInTheDocument();
    expect(screen.queryByText('Windows Runner')).not.toBeInTheDocument();

    await fireEvent.keyDown(search, { key: 'Enter' });
    expect(window.localStorage.getItem(STORAGE_KEYS.MACHINES_SEARCH_HISTORY)).toContain('macos');

    await fireEvent.input(search, { target: { value: '10.0.0.98' } });
    expect(screen.getByText('Mac Mini')).toBeInTheDocument();
    expect(screen.queryByText('Windows Runner')).not.toBeInTheDocument();

    const historyToggle = screen.getByTitle('Show recent searches');
    await fireEvent.click(historyToggle);
    expect(screen.getByRole('menuitem', { name: 'macos' })).toBeInTheDocument();

    const tipsButton = screen.getByRole('button', { name: 'Search tips' });
    await fireEvent.click(tipsButton);
    expect(screen.getByRole('dialog', { name: 'Search tips' })).toBeInTheDocument();
    expect(screen.getByText('arm64')).toBeInTheDocument();
    expect(
      screen.getByText('Hidden columns such as IP, RAID, Arch, and Kernel are still searchable.'),
    ).toBeInTheDocument();
  });

  it('resets active filters from the Machines empty state', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({ id: 'mac-mini', name: 'Mac Mini', status: 'online' }),
          resource({ id: 'windows-runner', name: 'Windows Runner', status: 'offline' }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const search = screen.getByPlaceholderText('Search machines') as HTMLInputElement;

    await fireEvent.input(search, { target: { value: 'does-not-exist' } });

    expect(screen.getByText('No machines match current filters')).toBeInTheDocument();
    expect(screen.queryByText('Mac Mini')).not.toBeInTheDocument();
    expect(screen.queryByText('Windows Runner')).not.toBeInTheDocument();

    await fireEvent.click(screen.getAllByRole('button', { name: 'Reset filters' })[0]);

    expect(search.value).toBe('');
    expect(screen.queryByText('No machines match current filters')).not.toBeInTheDocument();
    expect(screen.getByText('Mac Mini')).toBeInTheDocument();
    expect(screen.getByText('Windows Runner')).toBeInTheDocument();
  });

  it('preserves last-seen context in the single-line machine identity when that column is hidden', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_700_000_600_000);

    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'recent-machine',
            name: 'Recent Machine',
            lastSeen: 1_700_000_300_000,
            identity: { ips: ['192.168.0.21'] },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.queryByText('192.168.0.21 | seen 5m ago')).not.toBeInTheDocument();

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Last seen'));

    expect(screen.getByTitle('Recent Machine · 192.168.0.21 | seen 5m ago')).toBeInTheDocument();
  });

  it('shows a row expansion affordance for machine details', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[resource({ id: 'expandable-machine', name: 'Expandable Machine' })]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(
      screen.getByRole('button', { name: 'Expand details for Expandable Machine' }),
    ).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();

    const row = container.querySelector('[data-agents-machine-row="expandable-machine"]');
    expect(row).not.toBeNull();
    if (!row) return;

    await fireEvent.click(row);

    expect(
      screen.getByRole('button', { name: 'Collapse details for Expandable Machine' }),
    ).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
      'data-initial-show-host-details',
      'false',
    );
  });

  it('removes agent machines through row actions with confirmation', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'removable-row',
            name: 'Removable Machine',
            availability: { targetKind: 'machine', protocol: 'icmp' },
            agent: { agentVersion: '6.0.0' },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await fireEvent.click(
      screen.getByRole('button', { name: 'Machine actions for Removable Machine' }),
    );

    await fireEvent.click(
      screen.getByRole('menuitem', { name: 'Remove Removable Machine from Pulse' }),
    );

    expect(deleteAgentMock).not.toHaveBeenCalled();
    expect(screen.getByText('Click again to confirm')).toBeInTheDocument();
    expect(screen.getByText(/Pulse will forget this machine/)).toBeInTheDocument();

    await fireEvent.click(
      screen.getByRole('menuitem', { name: 'Confirm remove Removable Machine from Pulse' }),
    );

    await waitFor(() => {
      expect(deleteAgentMock).toHaveBeenCalledWith('removable-row');
    });
    expect(deleteAgentMetadataMock).toHaveBeenCalledWith('removable-row');
    await waitFor(() => {
      expect(screen.queryByText('Removable Machine')).not.toBeInTheDocument();
    });
  });

  it('opens saved agent web interface URLs beside the machine name', async () => {
    getAllAgentMetadataMock.mockResolvedValueOnce({
      'web-host': { id: 'web-host', customUrl: 'https://web-host.local:9443' },
    });

    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'web-host-row',
            name: 'Web Host',
            agent: { agentId: 'web-host' },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    const link = await screen.findByRole('link', { name: 'Open web interface for Web Host' });
    expect(link).toHaveAttribute('href', 'https://web-host.local:9443');
    expect(link).toHaveTextContent('');
    expect(screen.getByText('Web Host').closest('a')).toBeNull();
    expect(screen.queryByRole('columnheader', { name: 'Web' })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Add web interface URL for Web Host' }),
    ).not.toBeInTheDocument();
  });

  it('keeps machine name web links in sync when detail metadata changes', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'event-host-row',
            name: 'Event Host',
            agent: { agentId: 'event-host' },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    window.dispatchEvent(
      new CustomEvent(RESOURCE_METADATA_CHANGED_EVENT, {
        detail: {
          metadataKind: 'agent',
          metadataId: 'event-host',
          customUrl: 'http://event-host.local:8080',
        },
      }),
    );

    const link = await screen.findByRole('link', { name: 'Open web interface for Event Host' });
    expect(link).toHaveAttribute('href', 'http://event-host.local:8080');
  });

  it('shows structured agent network interface details from the Net I/O value', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'network-host',
            name: 'Network Host',
            network: { rxBytes: 1024, txBytes: 2048 },
            agent: {
              networkInterfaces: [
                {
                  name: 'en0',
                  mac: '10:20:30:40:50:60',
                  addresses: ['192.168.0.20', 'fe80::1'],
                  rxBytes: 1024,
                  txBytes: 2048,
                  speedMbps: 1000,
                },
              ],
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Net I/O'));
    const trigger = container.querySelector('[data-agent-machine-network-trigger="true"]');
    expect(trigger).not.toBeNull();
    if (!trigger) return;

    await fireEvent.mouseEnter(trigger);

    expect(await screen.findByText('Network Interfaces')).toBeInTheDocument();
    expect(screen.getByText('en0')).toBeInTheDocument();
    expect(screen.getByText('10:20:30:40:50:60')).toBeInTheDocument();
    expect(screen.getAllByText('192.168.0.20').length).toBeGreaterThan(0);
    expect(screen.getByText('fe80::1')).toBeInTheDocument();
    expect(screen.getByText('RX')).toBeInTheDocument();
    expect(screen.getByText('TX')).toBeInTheDocument();
    expect(screen.getAllByText('1.00 KB/s').length).toBeGreaterThan(0);
    expect(screen.getAllByText('2.00 KB/s').length).toBeGreaterThan(0);
    expect(screen.getByText('1000 Mbps')).toBeInTheDocument();
  });

  it('shows structured agent IP details from the IP value', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'ip-host',
            name: 'IP Host',
            identity: { ips: ['192.168.0.20', '10.0.0.20'] },
            agent: {
              networkInterfaces: [
                {
                  name: 'en0',
                  mac: '10:20:30:40:50:60',
                  addresses: ['192.168.0.20', 'fe80::1'],
                },
              ],
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('IP'));

    const trigger = container.querySelector('[data-agent-machine-ip-trigger="true"]');
    expect(trigger).not.toBeNull();
    if (!trigger) return;

    expect(trigger).toHaveTextContent('192.168.0.20');
    expect(trigger).toHaveTextContent('+2');

    await fireEvent.mouseEnter(trigger);

    expect(await screen.findByText('IP Addresses')).toBeInTheDocument();
    expect(screen.getAllByText('192.168.0.20').length).toBeGreaterThan(0);
    expect(screen.getByText('10.0.0.20')).toBeInTheDocument();
    expect(screen.getAllByText('fe80::1').length).toBeGreaterThan(0);
    expect(screen.getByText('Network Interfaces')).toBeInTheDocument();
    expect(screen.getByText('en0')).toBeInTheDocument();
    expect(screen.getByText('10:20:30:40:50:60')).toBeInTheDocument();
  });

  it('shows structured agent disk I/O details from the Disk I/O value', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'diskio-host',
            name: 'Disk I/O Host',
            diskIO: { readRate: 4096, writeRate: 8192 },
            agent: {
              diskIO: [
                {
                  device: '/dev/sda',
                  readBytes: 1_048_576,
                  writeBytes: 2_097_152,
                  readOps: 120,
                  writeOps: 240,
                  ioTimeMs: 360,
                },
              ],
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Disk I/O'));
    const trigger = container.querySelector('[data-agent-machine-diskio-trigger="true"]');
    expect(trigger).not.toBeNull();
    if (!trigger) return;

    await fireEvent.mouseEnter(trigger);

    expect((await screen.findAllByText('Disk I/O')).length).toBeGreaterThan(1);
    expect(screen.getByText('/dev/sda')).toBeInTheDocument();
    expect(screen.getAllByText('4.00 KB/s').length).toBeGreaterThan(0);
    expect(screen.getAllByText('8.00 KB/s').length).toBeGreaterThan(0);
    expect(screen.getByText('1.00 MB')).toBeInTheDocument();
    expect(screen.getByText('2.00 MB')).toBeInTheDocument();
    expect(screen.getByText('120')).toBeInTheDocument();
    expect(screen.getByText('240')).toBeInTheDocument();
    expect(screen.getByText('360 ms')).toBeInTheDocument();
  });

  it('shows structured agent RAID array details from the RAID value', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'raid-host',
            name: 'RAID Host',
            agent: {
              raid: [
                {
                  device: '/dev/md0',
                  name: 'media',
                  level: 'raid6',
                  state: 'recovering',
                  totalDevices: 6,
                  activeDevices: 5,
                  workingDevices: 5,
                  failedDevices: 1,
                  spareDevices: 1,
                  devices: [
                    { device: '/dev/sda', state: 'active', slot: 0 },
                    { device: '/dev/sdb', state: 'faulty', slot: 1 },
                    { device: '/dev/sdc', state: 'spare', slot: 2 },
                  ],
                  rebuildPercent: 37.2,
                  rebuildSpeed: '120 MB/s',
                },
              ],
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('RAID'));

    const trigger = container.querySelector('[data-agent-machine-raid-trigger="true"]');
    expect(trigger).not.toBeNull();
    if (!trigger) return;

    await fireEvent.mouseEnter(trigger);

    expect(await screen.findByText('RAID Arrays')).toBeInTheDocument();
    expect(screen.getByText('media')).toBeInTheDocument();
    expect(screen.getByText('raid6')).toBeInTheDocument();
    expect(screen.getByText('recovering')).toBeInTheDocument();
    expect(screen.getByText('/dev/md0')).toBeInTheDocument();
    expect(screen.getByText('/dev/sda')).toBeInTheDocument();
    expect(screen.getByText('/dev/sdb')).toBeInTheDocument();
    expect(screen.getByText('/dev/sdc')).toBeInTheDocument();
    expect(screen.getByText('37%')).toBeInTheDocument();
    expect(screen.getByText('120 MB/s')).toBeInTheDocument();
  });

  it('sorts machines by operational metrics from the column headers', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({ id: 'alpha', name: 'Alpha', cpu: { current: 3 } }),
          resource({ id: 'zulu', name: 'Zulu', cpu: { current: 80 } }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(
      Array.from(container.querySelectorAll('[data-agents-machine-row]')).map((row) =>
        row.getAttribute('data-agents-machine-row'),
      ),
    ).toEqual(['alpha', 'zulu']);

    await fireEvent.click(screen.getByRole('button', { name: 'Sort by CPU' }));

    expect(
      Array.from(container.querySelectorAll('[data-agents-machine-row]')).map((row) =>
        row.getAttribute('data-agents-machine-row'),
      ),
    ).toEqual(['zulu', 'alpha']);
  });

  it('keeps deep identity columns available through the column picker', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'mac-mini',
            name: 'Mac Mini',
            agent: {
              architecture: 'arm64',
              kernelVersion: 'Darwin 25.5.0',
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.queryByText('arm64')).not.toBeInTheDocument();

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Arch'));

    expect(screen.getByText('arm64')).toBeInTheDocument();
  });

  it('preserves agent CPU load and memory swap details in row metrics', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'workstation',
            name: 'Workstation',
            cpu: { current: 48 },
            memory: { current: 61 },
            agent: {
              cpuCount: 12,
              loadAverage: [2.45, 2.1, 1.9],
              memory: {
                used: 6_000,
                total: 12_000,
                free: 6_000,
                usage: 50,
                balloon: 8_000,
                swapUsed: 1_024,
                swapTotal: 2_048,
              },
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getByTestId('agent-machine-cpu-bar')).toHaveAttribute(
      'data-load-average',
      '2.45',
    );
    expect(screen.getByTestId('agent-machine-cpu-bar')).toHaveAttribute('data-cores', '12');
    expect(screen.getByTestId('agent-machine-memory-bar')).toHaveAttribute(
      'data-swap-used',
      '1024',
    );
    expect(screen.getByTestId('agent-machine-memory-bar')).toHaveAttribute(
      'data-swap-total',
      '2048',
    );
    expect(screen.getByTestId('agent-machine-memory-bar')).toHaveAttribute('data-balloon', '8000');
  });

  it('shows structured agent temperature details from the table value', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'thermal-host',
            name: 'Thermal Host',
            agent: {
              sensors: {
                temperatureCelsius: { 'cpu.package': 61 },
                fanRpm: { cpu_fan: 1_400 },
                additional: { nvme0: 42 },
                smart: [
                  { device: '/dev/sda', model: 'Cold Standby', temperature: 33, standby: true },
                  { device: '/dev/sdb', model: 'Archive HDD', temperature: 38 },
                ],
              },
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Temp'));
    const trigger = container.querySelector('[data-agent-machine-temperature-trigger="true"]');
    expect(trigger).not.toBeNull();
    if (!trigger) return;

    await fireEvent.mouseEnter(trigger);

    expect(await screen.findByText('Temperatures')).toBeInTheDocument();
    expect(screen.getByText('Disk Temperatures')).toBeInTheDocument();
    expect(screen.getByText('Fan Speeds')).toBeInTheDocument();
    expect(screen.getByText('Other Sensors')).toBeInTheDocument();
    expect(screen.getByText('Disk /dev/sdb Archive HDD')).toBeInTheDocument();
    expect(screen.getByText('Disk /dev/sda Cold Standby')).toBeInTheDocument();
    expect(screen.getByText('standby')).toBeInTheDocument();
    expect(screen.getByText('1400 RPM')).toBeInTheDocument();
  });

  it('shows a silent agent disk temperature as last known, never as a current reading', async () => {
    // A host agent past its reporting lease is stale and keeps its last SMART
    // temperatures with a non-available collection state.
    const stoppedReporting = {
      temperature: {
        state: 'unavailable' as const,
        source: 'smartctl',
        reason: 'host agent stopped reporting',
      },
    };
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'silent-nas',
            name: 'Silent NAS',
            status: 'warning',
            agent: {
              stale: true,
              sensors: {
                smart: [
                  { device: '/dev/sda', model: 'Archive HDD', temperature: 71 },
                  { device: '/dev/sdb', model: 'Parity HDD', temperature: 66 },
                ].map((disk) => ({ ...disk, collection: stoppedReporting })),
              },
            },
          }),
          resource({
            id: 'mixed-host',
            name: 'Mixed Host',
            agent: {
              sensors: {
                smart: [
                  {
                    device: '/dev/sda',
                    model: 'Retained HDD',
                    temperature: 69,
                    collection: stoppedReporting,
                  },
                  { device: '/dev/nvme0', model: 'Fast SSD', temperature: 44 },
                ],
              },
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Temp'));

    const triggerFor = (id: string) =>
      container.querySelector(
        `[data-agents-machine-row="${id}"] [data-agent-machine-temperature-trigger="true"]`,
      ) as HTMLElement | null;

    // Nothing is collected now, so the cell keeps the hottest retained value
    // without a threshold colour and says it is last known.
    const silent = triggerFor('silent-nas');
    expect(silent).not.toBeNull();
    if (!silent) return;
    const retained = silent.querySelector('[data-temperature-reading="last-known"]');
    expect(retained).not.toBeNull();
    expect(retained).toHaveTextContent('71°C, last known');
    expect(retained).toHaveClass('underline', 'decoration-dotted', 'text-muted');
    expect(retained?.className).not.toMatch(/text-(green|yellow|red)-/);

    await fireEvent.mouseEnter(silent);
    const tooltip = await waitFor(() => {
      const element = document.querySelector('[data-agent-machine-temperature-tooltip="true"]');
      expect(element).not.toBeNull();
      return element as HTMLElement;
    });
    expect(within(tooltip).getByText('71°C (last known)')).toHaveClass('text-muted');
    expect(within(tooltip).getByText('66°C (last known)')).toHaveClass('text-muted');
    await fireEvent.mouseLeave(silent);

    // A disk collected now leads the cell even when a retained one is hotter.
    const mixed = triggerFor('mixed-host');
    expect(mixed).not.toBeNull();
    if (!mixed) return;
    expect(mixed).toHaveTextContent('44°C');
    expect(mixed.querySelector('[data-temperature-reading="last-known"]')).toBeNull();
    expect(within(mixed).queryByText('69°C')).toBeNull();
  });

  it('moves a machine temperature between current and last known as its disk reports change', async () => {
    // Fresh objects per update, like parsed API payloads: the store reconciles
    // into the objects it was given, so a reused fixture would be rewritten.
    const collected = () => ({ temperature: { state: 'available' as const, source: 'smartctl' } });
    const stoppedReporting = () => ({
      temperature: {
        state: 'unavailable' as const,
        source: 'smartctl',
        reason: 'host agent stopped reporting',
      },
    });
    const nas = (
      temperature: number,
      collection?: ReturnType<typeof collected> | ReturnType<typeof stoppedReporting>,
    ) =>
      resource({
        id: 'nas',
        name: 'NAS',
        agent: {
          sensors: { smart: [{ device: '/dev/sda', type: 'sata', temperature, collection }] },
        },
      });
    const [resources, setResources] = createSignal<Resource[]>([nas(52, collected())]);
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={resources()}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Temp'));
    const cell = () =>
      container.querySelector(
        '[data-agents-machine-row="nas"] [data-agent-machine-temperature-trigger="true"]',
      ) as HTMLElement;
    const lastKnown = () => cell().querySelector('[data-temperature-reading="last-known"]');

    expect(cell()).toHaveTextContent('52°C');
    expect(lastKnown()).toBeNull();

    setResources([nas(52, stoppedReporting())]);
    await waitFor(() => expect(lastKnown()).not.toBeNull());
    expect(cell()).toHaveTextContent('52°C, last known');

    setResources([nas(47, collected())]);
    await waitFor(() => expect(lastKnown()).toBeNull());
    expect(cell()).toHaveTextContent('47°C');
    expect(cell().querySelector('.decoration-dotted')).toBeNull();
    expect(cell()).not.toHaveTextContent('last known');

    setResources([nas(0)]);
    await waitFor(() => expect(cell()).toHaveTextContent('—'));
    expect(lastKnown()).toBeNull();
  });

  it('shows thermal pressure when macOS reports no Celsius temperature', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'mac-agent',
            name: 'RICHARD-MAC-MINI.local',
            temperature: undefined,
            agent: {
              agentVersion: 'v6.0.0',
              sensors: {
                thermalState: {
                  source: 'pmset',
                  pressure: 'nominal',
                },
              },
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    await openMachineColumnPicker();
    await fireEvent.click(screen.getByLabelText('Temp'));
    const pressure = await screen.findByText('Nominal');

    expect(pressure).toBeInTheDocument();
    expect(pressure.closest('[data-agent-machine-temperature-trigger="true"]')).toHaveAttribute(
      'title',
      'Thermal pressure nominal via pmset',
    );
  });

  it("keeps a silent machine's seen and last-report ages moving while its data does not change", async () => {
    const start = Date.parse('2026-07-08T09:05:00Z');
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });

    try {
      render(() => (
        <AgentsMachinesTable
          resources={[
            resource({
              id: 'omv',
              name: 'omv',
              lastSeen: Date.parse('2026-07-08T09:00:00Z'),
              identity: { ips: ['192.168.0.21'] },
              agent: {
                agentVersion: '6.0.2',
                stale: true,
                lastReportAt: '2026-07-08T09:00:00Z',
              },
            }),
          ]}
          emptyIcon={emptyIcon}
          emptyTitle="No machines"
          emptyDescription="Install Pulse Agent."
        />
      ));
      await openMachineColumnPicker();
      await fireEvent.click(screen.getByLabelText('Last seen'));

      expect(screen.getByTitle('omv · 192.168.0.21 | seen 5m ago')).toBeInTheDocument();
      expect(
        screen.getByTitle(/Agent has stopped reporting\. Last report 5m ago\./),
      ).toBeInTheDocument();

      // The agent stays silent: no new data arrives and only the clock moves.
      vi.setSystemTime(start + 3 * 60 * 60 * 1000);
      vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);

      expect(screen.getByTitle('omv · 192.168.0.21 | seen 3h ago')).toBeInTheDocument();
      expect(
        screen.getByTitle(/Agent has stopped reporting\. Last report 3h ago\./),
      ).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });
});
