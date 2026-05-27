import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentMetadataAPI } from '@/api/agentMetadata';
import type { Resource } from '@/types/resource';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { RESOURCE_METADATA_CHANGED_EVENT } from '@/utils/resourceMetadataEvents';
import { AgentsMachinesTable } from '../AgentsMachinesTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: { initialShowAccessContext?: boolean }) => (
    <div
      data-testid="resource-detail-drawer"
      data-initial-show-access-context={String(props.initialShowAccessContext ?? false)}
    />
  ),
}));

vi.mock('@/api/agentMetadata', () => ({
  AgentMetadataAPI: {
    getAllMetadata: vi.fn(async () => ({})),
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

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: {
    used: number;
    total: number;
    percentOnly?: number;
    balloon?: number;
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

const emptyIcon = <span data-testid="empty-icon" />;
const getAllAgentMetadataMock = vi.mocked(AgentMetadataAPI.getAllMetadata);

beforeEach(() => {
  getAllAgentMetadataMock.mockResolvedValue({});
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
  window.localStorage.clear();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('AgentsMachinesTable', () => {
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
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getByRole('button', { name: 'Sort by Net I/O' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sort by Disk I/O' })).toBeInTheDocument();
    expect(screen.getByText('1.00 KB/s')).toBeInTheDocument();
    expect(screen.getByText('8.00 KB/s')).toBeInTheDocument();

    expect(screen.queryByRole('button', { name: 'Sort by IP' })).not.toBeInTheDocument();
    expect(screen.getByText('192.168.0.10')).toBeInTheDocument();
    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
    await fireEvent.click(screen.getByLabelText('IP'));
    await fireEvent.click(screen.getByLabelText('RAID'));

    expect(screen.getByRole('button', { name: 'Sort by IP' })).toBeInTheDocument();
    expect(screen.getAllByText('192.168.0.10').length).toBeGreaterThan(0);
    expect(screen.getByText('1 clean')).toBeInTheDocument();
  });

  it('shows hostname and primary IP in the machine identity subtitle', () => {
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

    expect(screen.getByText('richard-mac-mini.local | 192.168.0.98')).toBeInTheDocument();
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
    expect(screen.getByRole('button', { name: 'macos' })).toBeInTheDocument();

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

  it('preserves last-seen context in the machine subtitle when that column is hidden', async () => {
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

    expect(screen.queryByText('192.168.0.21 | seen 5m')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
    await fireEvent.click(screen.getByLabelText('Last seen'));

    expect(screen.getByText('192.168.0.21 | seen 5m')).toBeInTheDocument();
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

    const icon = container.querySelector('[data-agent-machine-expand-icon]');
    expect(icon).not.toBeNull();
    expect(icon).not.toHaveClass('rotate-90');
    expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();

    const row = container.querySelector('[data-agents-machine-row="expandable-machine"]');
    expect(row).not.toBeNull();
    if (!row || !icon) return;

    await fireEvent.click(row);

    expect(icon).toHaveClass('rotate-90');
    expect(screen.getByTestId('resource-detail-drawer')).toBeInTheDocument();
  });

  it('opens saved agent web interface URLs from machine rows', async () => {
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
  });

  it('keeps machine web links in sync when detail metadata changes', async () => {
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

    await fireEvent.click(
      screen.getByRole('button', { name: 'Add web interface URL for Event Host' }),
    );
    expect(screen.getByTestId('resource-detail-drawer')).toBeInTheDocument();
    expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
      'data-initial-show-access-context',
      'true',
    );

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

    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
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

    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
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

    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
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
});
