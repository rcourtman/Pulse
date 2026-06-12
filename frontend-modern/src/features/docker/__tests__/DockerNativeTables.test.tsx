import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { MonitoringAPI } from '@/api/monitoring';
import { ResourceActionsAPI } from '@/api/resourceActions';
import type { Resource } from '@/types/resource';
import { DockerContainersTable } from '../DockerContainersTable';
import { DockerConfigsTable } from '../DockerConfigsTable';
import { DockerImagesTable } from '../DockerImagesTable';
import { DockerNetworksTable } from '../DockerNetworksTable';
import { DockerSecretsTable } from '../DockerSecretsTable';
import { DockerServicesTable } from '../DockerServicesTable';
import { DockerStorageUsageTable } from '../DockerStorageUsageTable';
import { DockerSwarmNodesTable } from '../DockerSwarmNodesTable';
import { DockerTasksTable } from '../DockerTasksTable';
import { DockerVolumesTable } from '../DockerVolumesTable';

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    updateDockerContainer: vi.fn().mockResolvedValue({ success: true }),
  },
}));

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: {
    planAction: vi.fn().mockResolvedValue({
      actionId: 'action-1',
      requestId: 'request-1',
      allowed: true,
      requiresApproval: true,
      approvalPolicy: 'admin',
      rollbackAvailable: false,
      plannedAt: '2026-06-12T20:00:00Z',
      expiresAt: '2026-06-12T20:05:00Z',
      resourceVersion: 'resource-version',
      policyVersion: 'policy-version',
      planHash: 'plan-hash',
    }),
    decideAction: vi.fn().mockResolvedValue({
      actionId: 'action-1',
      state: 'approved',
      approval: {
        actor: 'operator',
        method: 'api',
        timestamp: '2026-06-12T20:00:01Z',
        outcome: 'approved',
      },
      audit: {},
    }),
    executeAction: vi.fn().mockResolvedValue({
      actionId: 'action-1',
      state: 'completed',
      result: { success: true },
      audit: {},
    }),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/stores/containerUpdates', () => ({
  clearContainerUpdateState: vi.fn(),
  getContainerUpdateState: vi.fn(() => undefined),
  markContainerQueued: vi.fn(),
  markContainerUpdateError: vi.fn(),
  markContainerUpdateSuccess: vi.fn(),
  updateStates: vi.fn(() => ({})),
}));

vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => false,
}));

// jsdom has no ResizeObserver; stub the shared metric cells like the
// DockerHostsTable tests do.
vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: (props: { type: string; isRunning?: boolean; resourceId?: string }) => (
    <div
      data-testid={`responsive-${props.type}-metric`}
      data-resource-id={props.resourceId ?? ''}
      data-running={String(props.isRunning)}
    />
  ),
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: { used: number; total: number; percentOnly?: number }) => (
    <div
      data-testid="stacked-memory-bar"
      data-used={String(props.used)}
      data-total={String(props.total)}
      data-percent-only={String(props.percentOnly ?? '')}
    />
  ),
}));

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

// DockerContainersTable URL-backs its host scope filter (useSearchParams), so
// it must render inside a Router context.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/" component={component} />
    </Router>
  ));

const setViewportWidth = (width: number) => {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  });
};

afterEach(() => {
  window.history.pushState({}, '', '/');
  setViewportWidth(1024);
  cleanup();
  vi.clearAllMocks();
});

describe('Docker native tables', () => {
  it('renders Docker container API fields', () => {
    setViewportWidth(1500);

    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            cpu: { current: 42 },
            memory: { current: 50, used: 512 * 1024 * 1024, total: 1024 * 1024 * 1024 },
            docker: {
              agentId: 'agent-1',
              hostname: 'edge-01',
              runtime: 'docker',
              runtimeVersion: '27.5.1',
              image: 'nginx:latest',
              containerState: 'running',
              health: 'healthy',
              restartCount: 2,
              ports: [{ ip: '0.0.0.0', publicPort: 8080, privatePort: 80, protocol: 'tcp' }],
              networks: [{ name: 'frontend', ipv4: '172.18.0.2' }],
              mounts: [
                {
                  type: 'volume',
                  source: 'nginx-html',
                  destination: '/usr/share/nginx/html',
                  mode: 'rw',
                  rw: true,
                },
              ],
              updateStatus: {
                updateAvailable: true,
                currentDigest: 'sha256:current',
                latestDigest: 'sha256:latest',
                lastChecked: '2026-05-24T13:00:00Z',
              },
            },
          }),
          makeResource({
            id: 'container-2',
            type: 'app-container',
            name: 'edge-cache',
            status: 'running',
            docker: {
              agentId: 'agent-2',
              hostname: 'edge-02',
              runtime: 'podman',
              runtimeVersion: '5.2.1',
              image: 'redis:7.4',
              containerState: 'running',
              health: 'healthy',
              restartCount: 7,
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Container')).toBeInTheDocument();
    expect(screen.getByText('Runtime')).toBeInTheDocument();
    expect(screen.getByText('CPU')).toBeInTheDocument();
    expect(screen.getByText('Memory')).toBeInTheDocument();
    expect(screen.getByText('Restarts')).toBeInTheDocument();
    expect(screen.getByText('Updates')).toBeInTheDocument();
    expect(screen.getByText('Actions')).toBeInTheDocument();
    expect(screen.queryByText('Health')).not.toBeInTheDocument();
    expect(screen.queryByText('State')).not.toBeInTheDocument();
    expect(screen.getByText('edge-web')).toBeInTheDocument();
    expect(screen.getByText('edge-01')).toBeInTheDocument();
    expect(screen.getByText('docker 27.5.1')).toBeInTheDocument();
    expect(screen.getByText('podman 5.2.1')).toBeInTheDocument();
    expect(screen.getByText('nginx:latest')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('0.0.0.0:8080->80/tcp')).toBeInTheDocument();
    expect(screen.getByText('frontend 172.18.0.2')).toBeInTheDocument();
    expect(screen.getByText('volume:/usr/share/nginx/html (rw)')).toBeInTheDocument();
    expect(screen.getByText('Available')).toBeInTheDocument();

    // A crash-looper (restarts above the v5 attention threshold) is flagged.
    expect(screen.getByText('7')).toHaveClass('text-red-600');
    expect(screen.getByText('2')).not.toHaveClass('text-red-600');

    // Both rows get a CPU cell; only the row with memory data gets a bar.
    expect(screen.getAllByTestId('responsive-cpu-metric').length).toBe(2);
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute(
      'data-used',
      String(512 * 1024 * 1024),
    );
  });

  it('hides the Runtime column when every container reports the same runtime', () => {
    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              hostname: 'edge-01',
              runtime: 'docker',
              runtimeVersion: '27.5.1',
              image: 'nginx:latest',
              containerState: 'running',
            },
          }),
          makeResource({
            id: 'container-2',
            type: 'app-container',
            name: 'edge-cache',
            status: 'running',
            docker: {
              hostname: 'edge-02',
              runtime: 'docker',
              runtimeVersion: '27.5.1',
              image: 'redis:7.4',
              containerState: 'running',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(screen.queryByText('Runtime')).not.toBeInTheDocument();
    expect(screen.queryByText('docker 27.5.1')).not.toBeInTheDocument();
  });

  it('applies the URL host scope to container rows', () => {
    window.history.pushState({}, '', '/?host=edge-02');

    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              hostname: 'edge-01',
              runtime: 'docker',
              image: 'nginx:latest',
              containerState: 'running',
            },
          }),
          makeResource({
            id: 'container-2',
            type: 'app-container',
            name: 'edge-cache',
            status: 'running',
            docker: {
              hostname: 'edge-02',
              runtime: 'docker',
              image: 'redis:7.4',
              containerState: 'running',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('edge-cache')).toBeInTheDocument();
    expect(screen.getByText('edge-02')).toBeInTheDocument();
    expect(screen.queryByText('edge-web')).not.toBeInTheDocument();
    expect(screen.queryByText('edge-01')).not.toBeInTheDocument();
  });

  it('hides the Restarts column when the current containers have no restart signal', () => {
    setViewportWidth(1500);

    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              hostname: 'edge-01',
              image: 'nginx:latest',
              containerState: 'running',
              restartCount: 0,
            },
          }),
          makeResource({
            id: 'container-2',
            type: 'app-container',
            name: 'edge-cache',
            status: 'running',
            docker: {
              hostname: 'edge-02',
              image: 'redis:7.4',
              containerState: 'running',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(screen.queryByText('Restarts')).not.toBeInTheDocument();
  });

  it('shows the State column when any current container is not running', () => {
    setViewportWidth(1500);

    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              hostname: 'edge-01',
              image: 'nginx:latest',
              containerState: 'running',
            },
          }),
          makeResource({
            id: 'container-2',
            type: 'app-container',
            name: 'edge-cache',
            status: 'offline',
            docker: {
              hostname: 'edge-02',
              image: 'redis:7.4',
              containerState: 'exited',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('State')).toBeInTheDocument();
    expect(screen.getByText('exited')).toBeInTheDocument();
  });

  it('renders actionable Docker container updates with native agent and container IDs', async () => {
    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              hostSourceId: 'agent-edge',
              containerId: 'native-container-1',
              hostname: 'edge-01',
              image: 'nginx:latest',
              containerState: 'running',
              updateStatus: {
                updateAvailable: true,
                currentDigest: 'sha256:current',
                latestDigest: 'sha256:latest',
                lastChecked: '2026-05-24T13:00:00Z',
              },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    const updateButton = screen.getByRole('button', { name: /click to update/i });
    fireEvent.click(updateButton);
    fireEvent.click(screen.getByRole('button', { name: /click again to confirm update/i }));

    await waitFor(() =>
      expect(MonitoringAPI.updateDockerContainer).toHaveBeenCalledWith(
        'agent-edge',
        'native-container-1',
        'edge-web',
      ),
    );
  });

  it('runs Docker lifecycle row actions through the governed action API', async () => {
    const onLifecycleActionSettled = vi.fn();

    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              agentId: 'agent-edge',
              hostSourceId: 'docker-host-edge',
              containerId: 'native-container-1',
              hostname: 'edge-01',
              image: 'nginx:latest',
              runtime: 'docker',
              containerState: 'running',
            },
            capabilities: [
              {
                name: 'restart',
                type: 'common',
                platform: 'docker',
                minimumApprovalLevel: 'admin',
              },
              {
                name: 'stop',
                type: 'common',
                platform: 'docker',
                minimumApprovalLevel: 'admin',
              },
            ],
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
        onLifecycleActionSettled={onLifecycleActionSettled}
      />
    ));

    const restartButton = screen.getByRole('button', {
      name: 'Restart edge-web through governed action',
    });
    fireEvent.click(restartButton);
    fireEvent.click(screen.getByRole('button', { name: 'Click again to restart edge-web' }));

    await waitFor(() =>
      expect(ResourceActionsAPI.planAction).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceId: 'container-1',
          capabilityName: 'restart',
          requestedBy: 'ui:docker-page',
        }),
      ),
    );
    await waitFor(() =>
      expect(ResourceActionsAPI.executeAction).toHaveBeenCalledWith(
        'action-1',
        expect.stringContaining('restart Docker container edge-web'),
      ),
    );
    expect(ResourceActionsAPI.decideAction).toHaveBeenCalledWith(
      'action-1',
      'approved',
      expect.stringContaining('restart Docker container edge-web'),
    );
    expect(onLifecycleActionSettled).toHaveBeenCalledTimes(1);
  });

  it('shows disabled Docker lifecycle buttons with explicit unavailable reasons', () => {
    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              agentId: 'agent-edge',
              containerId: 'native-container-1',
              hostname: 'edge-01',
              image: 'nginx:latest',
              runtime: 'docker',
              containerState: 'running',
            },
            sourceStatus: { docker: { status: 'stale' } },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(
      screen.getByRole('button', {
        name: 'Restart unavailable: Docker inventory is stale; refresh inventory before running lifecycle actions.',
      }),
    ).toBeDisabled();
  });

  it('renders container rows with status mapped from containerState + health + exitCode, attention rows first', () => {
    renderInRouter(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'happy',
            type: 'app-container',
            docker: { containerState: 'running', health: 'healthy' },
          }),
          makeResource({
            id: 'dead',
            type: 'app-container',
            docker: { containerState: 'dead' },
          }),
          makeResource({
            id: 'restart',
            type: 'app-container',
            docker: { containerState: 'restarting' },
          }),
          makeResource({
            id: 'exited',
            type: 'app-container',
            docker: { containerState: 'exited', exitCode: 137 },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    const rows = Array.from(document.querySelectorAll('[data-docker-container-row]')).map((row) =>
      row.getAttribute('data-docker-container-row'),
    );
    // Two danger rows tied -> name-sorted; then warning; then success.
    expect(rows).toEqual(['dead', 'exited', 'restart', 'happy']);
    expect(screen.getByText('State')).toBeInTheDocument();
    expect(screen.getByTitle('Dead')).toHaveClass('bg-red-500');
    expect(screen.getByTitle('Exited (137)')).toHaveClass('bg-red-500');
    expect(screen.getByTitle('Restarting')).toHaveClass('bg-amber-500');
    expect(screen.getByTitle('Healthy')).toHaveClass('bg-emerald-500');
  });

  it('renders Docker image API fields', () => {
    render(() => (
      <DockerImagesTable
        resources={[
          makeResource({
            id: 'image-1',
            type: 'docker-image',
            name: 'nginx:latest',
            docker: {
              hostname: 'edge-01',
              repoTags: ['nginx:latest', 'nginx:stable'],
              repoDigests: ['nginx@sha256:manifest'],
              sizeBytes: 805306368,
              imageContainers: 2,
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No images"
        emptyDescription="No images"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Tags')).toBeInTheDocument();
    expect(screen.getByText('Digests')).toBeInTheDocument();
    expect(screen.getByText('nginx:latest')).toBeInTheDocument();
    expect(screen.getByText('nginx:latest, nginx:stable')).toBeInTheDocument();
    expect(screen.getByText('nginx@sha256:manifest')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('edge-01')).toBeInTheDocument();
  });

  it('renders Docker volume API fields', () => {
    render(() => (
      <DockerVolumesTable
        resources={[
          makeResource({
            id: 'volume-1',
            type: 'docker-volume',
            name: 'app-data',
            docker: {
              driver: 'local',
              scope: 'global',
              sizeBytes: 2048,
              refCount: 3,
              createdAt: '2026-05-24T13:00:00Z',
              mountpoint: '/var/lib/docker/volumes/app-data/_data',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No volumes"
        emptyDescription="No volumes"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Created')).toBeInTheDocument();
    expect(screen.getByText('Refs')).toBeInTheDocument();
    expect(screen.getByText('app-data')).toBeInTheDocument();
    expect(screen.getByText('local')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    // Created renders as a relative time with the absolute timestamp in the
    // tooltip, not as a raw ISO string.
    expect(screen.queryByText('2026-05-24T13:00:00Z')).not.toBeInTheDocument();
    expect(screen.getByText(/ago$/)).toBeInTheDocument();
    expect(screen.getByText('/var/lib/docker/volumes/app-data/_data')).toBeInTheDocument();
  });

  it('keeps Docker volume tables filterable when rendered standalone', () => {
    render(() => (
      <DockerVolumesTable
        resources={[
          makeResource({
            id: 'volume-app',
            type: 'docker-volume',
            name: 'app-data',
            status: 'online',
            docker: { driver: 'local' },
          }),
          makeResource({
            id: 'volume-cache',
            type: 'docker-volume',
            name: 'cache-data',
            status: 'offline',
            docker: { driver: 'local' },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No volumes"
        emptyDescription="No volumes"
      />
    ));

    expect(document.querySelectorAll('[data-docker-volume-row]')).toHaveLength(2);

    fireEvent.click(screen.getByRole('button', { name: 'Offline' }));

    expect(screen.getByText('1 of 2 volumes')).toBeInTheDocument();
    expect(document.querySelector('[data-docker-volume-row="volume-cache"]')).not.toBeNull();
    expect(document.querySelector('[data-docker-volume-row="volume-app"]')).toBeNull();

    const search = screen.getByPlaceholderText('Search volumes');
    fireEvent.input(search, { target: { value: 'app' } });

    expect(screen.getByText('0 of 2 volumes')).toBeInTheDocument();
    expect(screen.getByText('No volumes match current filters')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));

    expect(search).toHaveValue('');
    expect(screen.getByText('2 volumes')).toBeInTheDocument();
    expect(document.querySelectorAll('[data-docker-volume-row]')).toHaveLength(2);
  });

  it('renders Docker network API fields', () => {
    const network = makeResource({
      id: 'network-1',
      type: 'docker-network',
      name: 'frontend',
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
        networkId: 'net-1',
        driver: 'overlay',
        scope: 'swarm',
        enableIpv4: true,
        attachable: true,
        ingress: true,
        subnets: [{ subnet: '10.88.0.0/24', gateway: '10.88.0.1' }],
      },
    });

    render(() => (
      <DockerNetworksTable
        resources={[network]}
        emptyIcon={<span />}
        emptyTitle="No networks"
        emptyDescription="No networks"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Attached workloads')).toBeInTheDocument();
    expect(screen.getByText('Attention')).toBeInTheDocument();
    expect(screen.getByText('frontend')).toBeInTheDocument();
    expect(screen.getByText('No containers')).toBeInTheDocument();
    expect(screen.getByText('Unused')).toBeInTheDocument();
    expect(screen.getByText('overlay')).toBeInTheDocument();
    expect(screen.getByText('10.88.0.0/24 via 10.88.0.1')).toBeInTheDocument();

    const row = document.querySelector('[data-docker-network-row="network-1"]');
    expect(row).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('Addressing')).toBeInTheDocument();
    expect(screen.getByText('Flags')).toBeInTheDocument();
    expect(screen.getByText('IPv4')).toBeInTheDocument();
    expect(screen.getByText('attachable, ingress')).toBeInTheDocument();
    expect(screen.getByText('net-1')).toBeInTheDocument();
  });

  it('nests Docker containers attached to a network and searches attachment fields', () => {
    const network = makeResource({
      id: 'network-1',
      type: 'docker-network',
      name: 'frontend',
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
        networkId: 'net-1',
        driver: 'bridge',
        scope: 'local',
        enableIpv4: true,
        subnets: [{ subnet: '10.88.0.0/24', gateway: '10.88.0.1' }],
      },
    });
    const container = makeResource({
      id: 'container-1',
      type: 'app-container',
      name: 'edge-web',
      displayName: 'edge-web',
      status: 'running',
      relationships: [
        {
          sourceId: 'container-1',
          targetId: 'network-1',
          type: 'attached_to',
          confidence: 1,
          active: true,
          discoverer: 'docker_adapter',
          observedAt: '2026-06-03T09:00:00Z',
          lastSeenAt: '2026-06-03T09:00:00Z',
        },
      ],
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
        image: 'nginx:latest',
        containerState: 'running',
        health: 'healthy',
        networks: [{ name: 'frontend', ipv4: '10.88.0.12' }],
        ports: [{ ip: '0.0.0.0', publicPort: 8080, privatePort: 80, protocol: 'tcp' }],
      },
    });

    render(() => (
      <DockerNetworksTable
        resources={[network]}
        relatedResources={[network, container]}
        emptyIcon={<span />}
        emptyTitle="No networks"
        emptyDescription="No networks"
      />
    ));

    expect(screen.getByText('1 attached container · edge-web 10.88.0.12')).toBeInTheDocument();
    expect(screen.getByText('1 running')).toBeInTheDocument();

    const search = screen.getByPlaceholderText('Search networks');
    fireEvent.input(search, { target: { value: '8080' } });

    expect(document.querySelector('[data-docker-network-row="network-1"]')).not.toBeNull();

    const row = document.querySelector('[data-docker-network-row="network-1"]');
    fireEvent.click(row!);

    const detail = within(document.querySelector('[data-docker-network-detail-row="network-1"]')!);
    expect(detail.getByText('Attached containers')).toBeInTheDocument();
    expect(detail.getByText('edge-web')).toBeInTheDocument();
    expect(detail.getByText('Healthy')).toBeInTheDocument();
    expect(detail.getByText('10.88.0.12')).toBeInTheDocument();
    expect(detail.getByText('0.0.0.0:8080->80/tcp')).toBeInTheDocument();
    expect(detail.getByText('nginx:latest')).toBeInTheDocument();
  });

  it('keeps dense Docker network attachment lists searchable and grouped', () => {
    const network = makeResource({
      id: 'network-dense',
      type: 'docker-network',
      name: 'frontend',
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
        networkId: 'net-dense',
        driver: 'bridge',
        scope: 'local',
        enableIpv4: true,
      },
    });
    const makeAttachedContainer = (
      index: number,
      overrides: Partial<Resource['docker']> = {},
    ): Resource => {
      const name =
        index === 0
          ? 'api-unhealthy'
          : index === 1
            ? 'api-restarting'
            : index === 2
              ? 'worker-stopped'
              : `worker-${String(index).padStart(2, '0')}`;
      return makeResource({
        id: `container-${index}`,
        type: 'app-container',
        name,
        displayName: name,
        status: 'running',
        relationships: [
          {
            sourceId: `container-${index}`,
            targetId: 'network-dense',
            type: 'attached_to',
            confidence: 1,
            active: true,
            discoverer: 'docker_adapter',
            observedAt: '2026-06-03T09:00:00Z',
            lastSeenAt: '2026-06-03T09:00:00Z',
          },
        ],
        docker: {
          hostname: 'edge-01',
          hostSourceId: 'docker-host-1',
          image: `repo/${name}:latest`,
          containerState: 'running',
          networks: [{ name: 'frontend', ipv4: `10.88.0.${index + 10}` }],
          ports: [{ ip: '0.0.0.0', publicPort: 8000 + index, privatePort: 80, protocol: 'tcp' }],
          ...overrides,
        },
      });
    };
    const containers = [
      makeAttachedContainer(0, { health: 'unhealthy' }),
      makeAttachedContainer(1, { containerState: 'restarting' }),
      makeAttachedContainer(2, { containerState: 'exited', exitCode: 0 }),
      ...Array.from({ length: 27 }, (_, offset) => makeAttachedContainer(offset + 3)),
    ];

    render(() => (
      <DockerNetworksTable
        resources={[network]}
        relatedResources={[network, ...containers]}
        emptyIcon={<span />}
        emptyTitle="No networks"
        emptyDescription="No networks"
      />
    ));

    expect(
      screen.getByText(
        '30 attached containers · api-unhealthy 10.88.0.10, api-restarting 10.88.0.11, worker-stopped 10.88.0.12 +27',
      ),
    ).toBeInTheDocument();

    fireEvent.click(document.querySelector('[data-docker-network-row="network-dense"]')!);

    const detail = within(
      document.querySelector('[data-docker-network-detail-row="network-dense"]')!,
    );
    expect(detail.getByPlaceholderText('Search attached containers')).toBeInTheDocument();
    expect(detail.getByText('Show all 30 containers')).toBeInTheDocument();
    expect(detail.getByText('api-unhealthy')).toBeInTheDocument();
    expect(detail.getByText('api-restarting')).toBeInTheDocument();
    expect(detail.queryByText('worker-29')).toBeNull();

    fireEvent.click(detail.getByRole('button', { name: 'Show all 30 containers' }));
    expect(detail.getByText('worker-29')).toBeInTheDocument();
    expect(detail.getByText('Show first 24')).toBeInTheDocument();

    fireEvent.click(detail.getByRole('button', { name: 'Attention' }));
    expect(detail.getByText('2 containers of 30 containers')).toBeInTheDocument();
    expect(detail.getByText('api-unhealthy')).toBeInTheDocument();
    expect(detail.getByText('api-restarting')).toBeInTheDocument();
    expect(detail.queryByText('worker-29')).toBeNull();

    fireEvent.click(detail.getByRole('button', { name: 'All' }));
    fireEvent.input(detail.getByPlaceholderText('Search attached containers'), {
      target: { value: 'worker-29' },
    });

    expect(detail.getByText('1 container of 30 containers')).toBeInTheDocument();
    expect(detail.getByText('worker-29')).toBeInTheDocument();
    expect(detail.getByText('0.0.0.0:8029->80/tcp')).toBeInTheDocument();
  });

  it('renders Docker Swarm node API fields', () => {
    render(() => (
      <DockerSwarmNodesTable
        resources={[
          makeResource({
            id: 'node-1',
            type: 'docker-swarm-node',
            name: 'worker-1',
            docker: {
              nodeRole: 'manager',
              availability: 'active',
              managerReachability: 'reachable',
              leader: true,
              engineVersion: '26.1.4',
              nanoCpus: 4_000_000_000,
              memoryBytes: 17179869184,
              address: '10.0.0.11',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Reachability')).toBeInTheDocument();
    expect(screen.getByText('worker-1')).toBeInTheDocument();
    expect(screen.getByText('manager')).toBeInTheDocument();
    expect(screen.getByText('leader')).toBeInTheDocument();
    expect(screen.getByText('26.1.4')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.11')).toBeInTheDocument();
  });

  it('renders Docker Swarm service API fields including update status', () => {
    render(() => (
      <DockerServicesTable
        resources={[
          makeResource({
            id: 'service-1',
            type: 'docker-service',
            name: 'checkout-api',
            docker: {
              hostname: 'manager-1',
              image: 'registry.example.com/checkout-api:2026.05',
              mode: 'replicated',
              desiredTasks: 4,
              runningTasks: 2,
              labels: { 'com.docker.stack.namespace': 'shop' },
              endpointPorts: [
                { protocol: 'tcp', targetPort: 8080, publishedPort: 18080, publishMode: 'ingress' },
              ],
              serviceUpdate: {
                state: 'rollback_started',
                message: 'Service replicas below desired',
              },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No services"
        emptyDescription="No services"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Update')).toBeInTheDocument();
    expect(screen.getByText('Stack')).toBeInTheDocument();
    expect(screen.getByText('shop')).toBeInTheDocument();
    expect(screen.getByText('checkout-api')).toBeInTheDocument();
    expect(screen.getByText('registry.example.com/checkout-api:2026.05')).toBeInTheDocument();
    expect(screen.getByText('replicated')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('rollback_started')).toBeInTheDocument();
    expect(screen.getByText('18080:8080/tcp')).toBeInTheDocument();
    expect(screen.getByText('manager-1')).toBeInTheDocument();
    expect(document.querySelector('[data-docker-service-row="service-1"]')).not.toBeNull();
  });

  it('renders Docker engine storage usage fields from the disk usage API shape', () => {
    render(() => (
      <DockerStorageUsageTable
        hosts={[
          makeResource({
            id: 'host-1',
            type: 'agent',
            name: 'edge-01',
            docker: {
              imagesUsage: {
                totalCount: 6,
                activeCount: 4,
                totalSizeBytes: 2 * 1024 * 1024 * 1024,
                reclaimableBytes: 512 * 1024 * 1024,
              },
              containersUsage: {
                totalCount: 8,
                activeCount: 5,
                totalSizeBytes: 3 * 1024 * 1024 * 1024,
                reclaimableBytes: 256 * 1024 * 1024,
              },
              volumesUsage: {
                totalCount: 3,
                activeCount: 2,
                totalSizeBytes: 12 * 1024 * 1024 * 1024,
                reclaimableBytes: 1024 * 1024 * 1024,
              },
              buildCacheUsage: {
                totalCount: 4,
                activeCount: 1,
                totalSizeBytes: 5 * 1024 * 1024 * 1024,
                reclaimableBytes: 4 * 1024 * 1024 * 1024,
              },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
      />
    ));

    expect(screen.getByText('Images')).toBeInTheDocument();
    expect(screen.getByText('Containers')).toBeInTheDocument();
    expect(screen.getByText('Volumes')).toBeInTheDocument();
    expect(screen.getByText('Build Cache')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search storage usage')).toBeInTheDocument();
    expect(screen.getByText('edge-01')).toBeInTheDocument();
    expect(screen.getByText('2.00 GB')).toBeInTheDocument();
    expect(screen.getByText('6 total, 4 active, 512 MB reclaimable')).toBeInTheDocument();
    expect(screen.getByText('5.00 GB')).toBeInTheDocument();
    expect(screen.getByText('4 total, 1 active, 4.00 GB reclaimable')).toBeInTheDocument();
    expect(document.querySelector('[data-docker-storage-row="host-1"]')).not.toBeNull();
  });

  it('keeps Docker engine storage usage tables filterable when rendered standalone', () => {
    const storageUsage = {
      totalCount: 1,
      activeCount: 1,
      totalSizeBytes: 1024,
      reclaimableBytes: 0,
    };

    render(() => (
      <DockerStorageUsageTable
        hosts={[
          makeResource({
            id: 'host-edge',
            type: 'agent',
            name: 'edge-01',
            status: 'online',
            docker: { imagesUsage: storageUsage },
          }),
          makeResource({
            id: 'host-archive',
            type: 'agent',
            name: 'archive-01',
            status: 'offline',
            docker: { imagesUsage: storageUsage },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
      />
    ));

    expect(document.querySelectorAll('[data-docker-storage-row]')).toHaveLength(2);

    const search = screen.getByPlaceholderText('Search storage usage');
    fireEvent.input(search, { target: { value: 'archive' } });

    expect(search).toHaveValue('archive');
    expect(screen.getByText('1 of 2 hosts')).toBeInTheDocument();
    expect(document.querySelector('[data-docker-storage-row="host-archive"]')).not.toBeNull();
    expect(document.querySelector('[data-docker-storage-row="host-edge"]')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));

    expect(search).toHaveValue('');
    expect(screen.getByText('2 hosts')).toBeInTheDocument();
    expect(document.querySelectorAll('[data-docker-storage-row]')).toHaveLength(2);
  });

  it('renders Docker Swarm task API fields', () => {
    render(() => (
      <DockerTasksTable
        resources={[
          makeResource({
            id: 'task-1',
            type: 'docker-task',
            name: 'web.2',
            docker: {
              serviceName: 'web',
              slot: 2,
              desiredState: 'running',
              currentState: 'running 2 minutes',
              nodeName: 'worker-1',
              startedAt: '2026-05-24T13:05:00Z',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No tasks"
        emptyDescription="No tasks"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Slot')).toBeInTheDocument();
    expect(screen.getByText('Desired')).toBeInTheDocument();
    expect(screen.getByText('Current')).toBeInTheDocument();
    expect(screen.getByText('web.2')).toBeInTheDocument();
    expect(screen.getByText('web')).toBeInTheDocument();
    expect(screen.getByText('running 2 minutes')).toBeInTheDocument();
    expect(screen.getByText('2026-05-24T13:05:00Z')).toBeInTheDocument();
  });

  it('renders Docker Swarm secret API metadata without secret data', () => {
    render(() => (
      <DockerSecretsTable
        resources={[
          makeResource({
            id: 'secret-1',
            type: 'docker-secret',
            name: 'api-token',
            docker: {
              hostname: 'manager-1',
              driver: 'vault',
              templatingDriver: 'golang',
              objectCreatedAt: '2026-05-24T13:10:00Z',
              labels: { stack: 'ops' },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No secrets"
        emptyDescription="No secrets"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Secret')).toBeInTheDocument();
    expect(screen.getByText('Template')).toBeInTheDocument();
    expect(screen.getByText('api-token')).toBeInTheDocument();
    expect(screen.getByText('vault')).toBeInTheDocument();
    expect(screen.getByText('golang')).toBeInTheDocument();
    expect(screen.getByText('stack=ops')).toBeInTheDocument();
    expect(screen.getByText('manager-1')).toBeInTheDocument();
  });

  it('renders Docker Swarm config API metadata', () => {
    render(() => (
      <DockerConfigsTable
        resources={[
          makeResource({
            id: 'config-1',
            type: 'docker-config',
            name: 'nginx-conf',
            docker: {
              hostname: 'manager-1',
              templatingDriver: 'golang',
              objectCreatedAt: '2026-05-24T13:15:00Z',
              labels: { stack: 'edge' },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No configs"
        emptyDescription="No configs"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Config')).toBeInTheDocument();
    expect(screen.getByText('Template')).toBeInTheDocument();
    expect(screen.getByText('nginx-conf')).toBeInTheDocument();
    expect(screen.getByText('golang')).toBeInTheDocument();
    expect(screen.getByText('stack=edge')).toBeInTheDocument();
    expect(screen.getByText('manager-1')).toBeInTheDocument();
  });
});
