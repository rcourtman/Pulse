import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { DockerHostsTable } from '../DockerHostsTable';
import dockerHostsTableSource from '../DockerHostsTable.tsx?raw';

const formatThresholds = (thresholds?: MetricDisplayThresholds | null): string =>
  thresholds ? `${thresholds.warning}/${thresholds.critical}` : '';

// Row bars must color from the alert-configured thresholds, not the
// hardcoded METRIC_THRESHOLDS display defaults (memory 75/85), so a host
// with a raised memory override stops showing red below its alert point.
const getMetricThresholdsMock = vi.hoisted(() =>
  vi.fn((_scope: string, metric: string) =>
    metric === 'memory' ? { warning: 90, critical: 95 } : { warning: 80, critical: 85 },
  ),
);
const updateDockerHostMetadataMock = vi.hoisted(() =>
  vi.fn(async (_runtimeId: string, metadata: { customUrl?: string }) => metadata),
);

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: (props: {
    type: string;
    isRunning?: boolean;
    resourceId?: string;
    thresholds?: MetricDisplayThresholds | null;
  }) => (
    <div
      data-testid={`responsive-${props.type}-metric`}
      data-resource-id={props.resourceId ?? ''}
      data-running={String(props.isRunning)}
      data-thresholds={formatThresholds(props.thresholds)}
    />
  ),
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} as Record<string, never> }),
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: getMetricThresholdsMock,
  }),
}));
vi.mock('@/api/dockerHostMetadata', () => ({
  DockerHostMetadataAPI: {
    getMetadata: vi.fn(async () => ({})),
    updateMetadata: updateDockerHostMetadataMock,
  },
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: {
    used: number;
    total: number;
    unavailable?: boolean;
    percentOnly?: number;
    thresholds?: MetricDisplayThresholds | null;
  }) => (
    <div
      data-testid="stacked-memory-bar"
      data-used={String(props.used)}
      data-total={String(props.total)}
      data-unavailable={String(props.unavailable === true)}
      data-percent-only={String(props.percentOnly ?? '')}
      data-thresholds={formatThresholds(props.thresholds)}
    />
  ),
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: (props: {
    disks?: Disk[];
    aggregateDisk?: Disk;
    mode?: string;
    thresholds?: MetricDisplayThresholds | null;
  }) => (
    <div
      data-testid="stacked-disk-bar"
      data-mode={props.mode ?? ''}
      data-disks={String(props.disks?.length ?? 0)}
      data-aggregate-usage={String(props.aggregateDisk?.usage ?? '')}
      data-thresholds={formatThresholds(props.thresholds)}
    />
  ),
}));

const makeDockerHost = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'agent:docker-01',
  name: 'docker-01',
  displayName: 'docker-01',
  platformId: 'homelab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'degraded',
  type: 'agent',
  lastSeen: 1_700_000_000_000,
  cpu: { current: 42 },
  memory: { total: 8_000, used: 3_200, free: 4_800, current: 40 },
  disk: { total: 20_000, used: 12_500, free: 7_500, current: 62.5 },
  agent: {
    disks: [
      { device: '/dev/sda1', mountpoint: '/', total: 10_000, used: 6_000, free: 4_000 },
      {
        device: '/dev/sdb1',
        mountpoint: '/var/lib/docker',
        total: 10_000,
        used: 6_500,
        free: 3_500,
      },
    ],
  },
  docker: {
    runtimeVersion: '27.5.1',
    containerCount: 12,
  } as NonNullable<Resource['docker']> & { runtimeVersion?: string; containerCount?: number },
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DockerHostsTable', () => {
  it('keeps five readable host fields below 360 pixels without wrapped metadata rows', () => {
    expect(dockerHostsTableSource).toContain('platform-table-mobile-w-30');
    expect(dockerHostsTableSource).toContain('platform-table-narrow-hidden');
    expect(dockerHostsTableSource).toContain('hidden min-[360px]:table-cell min-[360px]:w-[16%]');
    expect(dockerHostsTableSource).not.toContain('class="max-[359px]:hidden"');
    expect(dockerHostsTableSource).not.toContain('max-[359px]:[&>a]:hidden');
    expect(dockerHostsTableSource).not.toContain('md:hidden" title={badge().title');
  });

  it('renders Docker hosts with a single-line Version column and shared metric bars', () => {
    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost()]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Docker hosts')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Version' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'System' })).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Swarm role' })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Runtime' })).not.toBeInTheDocument();
    expect(screen.getByText('27.5.1')).toBeInTheDocument();
    expect(screen.queryByText('Docker')).not.toBeInTheDocument();
    expect(screen.getByTestId('responsive-cpu-metric')).toHaveAttribute('data-running', 'true');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-used', '3200');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-total', '8000');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-mode', 'vertical-bars');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-disks', '2');
  });

  it('colors row metric bars from alert-configured thresholds, not display defaults', () => {
    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost()]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByTestId('responsive-cpu-metric')).toHaveAttribute('data-thresholds', '80/85');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-thresholds', '90/95');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-thresholds', '80/85');
    expect(getMetricThresholdsMock).toHaveBeenCalledWith(
      'agent',
      'memory',
      expect.arrayContaining(['agent:docker-01']),
    );
  });

  it('opens host details inline without route navigation or submit-style drawer controls', () => {
    window.history.pushState({}, '', '/docker/overview');

    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost()]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    const hostRow = screen.getByText('docker-01').closest('tr');
    expect(hostRow).not.toBeNull();

    fireEvent.click(hostRow!);

    expect(hostRow).not.toHaveAttribute('aria-expanded');
    expect(hostRow?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(screen.getByTestId('docker-host-drawer')).toBeInTheDocument();
    expect(window.location.pathname).toBe('/docker/overview');
    expect(window.location.search).toBe('');
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('type', 'button');
    expect(screen.getByRole('tab', { name: 'History' })).toHaveAttribute('type', 'button');
  });

  it('opens a saved Docker host web interface without toggling the row', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            customUrl: 'https://docker-01.example:9443',
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('docker-01').closest('tr')!;
    const link = screen.getByRole('link', { name: 'Open web interface for docker-01' });
    expect(link).toHaveAttribute('href', 'https://docker-01.example:9443');

    fireEvent.click(link);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(screen.queryByTestId('docker-host-drawer')).not.toBeInTheDocument();
  });

  it('edits the Docker host access URL in the drawer and updates the row link', async () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              hostSourceId: 'runtime-stable-id',
            } as NonNullable<Resource['docker']>,
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);

    const drawer = screen.getByTestId('docker-host-drawer');
    fireEvent.click(within(drawer).getByRole('tab', { name: 'Manage' }));
    expect(within(drawer).getByText('Access')).toBeInTheDocument();
    const input = within(drawer).getByPlaceholderText('https://198.51.100.100:8080');
    fireEvent.input(input, { target: { value: 'https://portainer.example:9443' } });
    fireEvent.click(within(drawer).getByRole('button', { name: 'Save' }));

    await vi.waitFor(() => {
      expect(updateDockerHostMetadataMock).toHaveBeenCalledWith('runtime-stable-id', {
        customUrl: 'https://portainer.example:9443',
      });
      expect(
        screen.getByRole('link', { name: 'Open web interface for docker-01' }),
      ).toHaveAttribute('href', 'https://portainer.example:9443');
    });
  });

  it('keeps an attached availability check visible in the host detail', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            availability: {
              targetId: 'tower-api',
              address: '192.168.0.8',
              port: 8007,
              protocol: 'tcp',
              enabled: true,
              pollIntervalSeconds: 60,
              available: true,
              latencyMillis: 9,
              lastChecked: new Date().toISOString(),
              correlationState: 'attached',
            },
            availabilityChecks: [
              {
                targetId: 'tower-api',
                address: '192.168.0.8',
                port: 8007,
                protocol: 'tcp',
                enabled: true,
                pollIntervalSeconds: 60,
                available: true,
                latencyMillis: 9,
                lastChecked: new Date().toISOString(),
                correlationState: 'attached',
              },
              {
                targetId: 'tower-web',
                address: 'tower.example.test',
                protocol: 'https',
                path: '/health',
                enabled: true,
                pollIntervalSeconds: 60,
                available: true,
                latencyMillis: 14,
                lastChecked: new Date().toISOString(),
                correlationState: 'attached',
              },
            ],
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);

    const drawer = screen.getByTestId('docker-host-drawer');
    expect(within(drawer).getAllByTestId('availability-probe-status')).toHaveLength(2);
    expect(within(drawer).getByText('192.168.0.8:8007')).toBeInTheDocument();
    expect(within(drawer).getByText('tower.example.test/health')).toBeInTheDocument();
    expect(within(drawer).getAllByText('fresh')).toHaveLength(2);
  });

  it('colors drawer host temperatures from configured thresholds', () => {
    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost({ temperature: 76 })]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);

    const drawer = screen.getByTestId('docker-host-drawer');
    expect(within(drawer).getByText('76°C').closest('td')).toHaveClass(
      'text-emerald-700',
      'dark:text-emerald-300',
    );
  });

  it('surfaces container update actions in the host drawer', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              hostSourceId: 'docker-01',
              updatesAvailableCount: 3,
            } as NonNullable<Resource['docker']>,
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);
    const drawer = screen.getByTestId('docker-host-drawer');
    fireEvent.click(within(drawer).getByRole('tab', { name: 'Manage' }));

    expect(within(drawer).getByRole('button', { name: 'Check updates' })).toBeInTheDocument();
    expect(within(drawer).getByRole('button', { name: 'Update all (3)' })).toBeInTheDocument();
  });

  it('identifies the host system separately from the container runtime', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            name: 'tower',
            agent: {
              hostProfile: 'unraid',
              osName: 'Unraid OS',
              osVersion: '6.12.10',
            },
          }),
          makeDockerHost({
            id: 'agent:qnap-01',
            name: 'qnap-01',
            agent: {
              platform: 'linux',
              osName: 'QuTS hero',
              osVersion: '5.2',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getAllByText('Unraid 6.12.10').length).toBeGreaterThan(0);
    expect(screen.getAllByText('QNAP 5.2').length).toBeGreaterThan(0);
    expect(screen.queryByText('Docker / Podman')).not.toBeInTheDocument();
  });

  it('shows Swarm role only for hosts with active Swarm evidence', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              swarm: {
                nodeId: 'node-1',
                nodeRole: 'manager',
                localState: 'active',
              },
            } as NonNullable<Resource['docker']> & {
              runtimeVersion?: string;
              containerCount?: number;
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByRole('columnheader', { name: 'Swarm role' })).toBeInTheDocument();
    expect(screen.getByText('Manager')).toBeInTheDocument();
  });

  it('does not show inactive standalone Swarm metadata as a host role', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              swarm: {
                nodeRole: 'worker',
                localState: 'inactive',
                scope: 'node',
              },
            } as NonNullable<Resource['docker']> & {
              runtimeVersion?: string;
              containerCount?: number;
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.queryByRole('columnheader', { name: 'Swarm role' })).not.toBeInTheDocument();
    expect(screen.queryByText('Worker')).not.toBeInTheDocument();
  });

  it('uses percent-only memory and aggregate disk bars when capacity details are missing', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            status: 'online',
            memory: { current: 55 },
            disk: { current: 71 },
            agent: undefined,
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-total', '0');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-percent-only', '55');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-disks', '0');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-aggregate-usage', '71');
  });

  it('renders unavailable Docker memory honestly while retaining known capacity', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            status: 'online',
            memory: undefined,
            agent: undefined,
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              memory: { total: 8_000, usageUnavailable: true },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-total', '8000');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-unavailable', 'true');

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);
    const drawer = screen.getByTestId('docker-host-drawer');
    expect(within(drawer).getByText('Unavailable')).toBeInTheDocument();
    expect(within(drawer).getByText('7.81 KB')).toBeInTheDocument();
  });
});
