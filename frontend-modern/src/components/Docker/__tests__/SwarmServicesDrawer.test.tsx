import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@solidjs/testing-library';

// ── Hoisted mocks ──────────────────────────────────────────────────────

const { apiFetchJSONMock } = vi.hoisted(() => ({
  apiFetchJSONMock: vi.fn(),
}));

// ── Module mocks ───────────────────────────────────────────────────────

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: apiFetchJSONMock,
}));

vi.mock('@/components/shared/Card', () => ({
  Card: (props: any) => (
    <div data-testid="card" class={props.class}>
      {props.children}
    </div>
  ),
}));

vi.mock('@/components/shared/Table', () => ({
  Table: (props: any) => (
    <table data-testid="table" class={props.class}>
      {props.children}
    </table>
  ),
  TableHeader: (props: any) => <thead>{props.children}</thead>,
  TableBody: (props: any) => <tbody class={props.class}>{props.children}</tbody>,
  TableRow: (props: any) => <tr class={props.class}>{props.children}</tr>,
  TableHead: (props: any) => <th class={props.class}>{props.children}</th>,
  TableCell: (props: any) => (
    <td class={props.class} title={props.title}>
      {props.children}
    </td>
  ),
}));

vi.mock('@/components/shared/EmptyState', () => ({
  EmptyState: (props: any) => (
    <div data-testid="empty-state">
      <span data-testid="empty-title">{props.title}</span>
      <span data-testid="empty-description">{props.description}</span>
    </div>
  ),
}));

// ── Import after mocks ─────────────────────────────────────────────────

import { SwarmServicesDrawer } from '../SwarmServicesDrawer';

// ── Helpers ─────────────────────────────────────────────────────────────

type DockerServiceResource = {
  id: string;
  name?: string;
  status?: string;
  docker?: {
    serviceId?: string;
    stack?: string;
    image?: string;
    mode?: string;
    desiredTasks?: number;
    runningTasks?: number;
    completedTasks?: number;
    serviceUpdate?: { state?: string; message?: string; completedAt?: string };
    endpointPorts?: Array<{
      name?: string;
      protocol?: string;
      targetPort?: number;
      publishedPort?: number;
      publishMode?: string;
    }>;
  };
};

function makeService(overrides: Partial<DockerServiceResource> = {}): DockerServiceResource {
  return {
    id: 'svc-1',
    name: 'my-service',
    status: 'running',
    docker: {
      serviceId: 'svc-1',
      stack: 'my-stack',
      image: 'nginx:latest',
      mode: 'replicated',
      desiredTasks: 3,
      runningTasks: 3,
      completedTasks: 0,
      serviceUpdate: undefined,
      endpointPorts: [],
    },
    ...overrides,
  };
}

function mockApiResponse(data: DockerServiceResource[], totalPages = 1) {
  apiFetchJSONMock.mockResolvedValueOnce({ data, meta: { totalPages } });
}

// ── Tests ───────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  vi.clearAllMocks();
});

describe('SwarmServicesDrawer', () => {
  describe('swarm info card', () => {
    it('shows cluster name when swarm info is provided', async () => {
      mockApiResponse([]);
      render(() => (
        <SwarmServicesDrawer
          cluster="cluster-1"
          swarm={{ clusterName: 'Production Swarm', clusterId: 'abc123' }}
        />
      ));

      expect(screen.getByText('Cluster: Production Swarm')).toBeInTheDocument();
      // Flush pending createResource microtask
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('falls back to clusterId when clusterName is empty', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="cluster-1" swarm={{ clusterId: 'abc123' }} />);

      expect(screen.getByText('Cluster: abc123')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('falls back to cluster prop when swarm has no name or id', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="my-cluster" swarm={{}} />);

      expect(screen.getByText('Cluster: my-cluster')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('shows "No Swarm cluster detected" when cluster prop is empty', () => {
      // Empty cluster = falsy source = resource not fetched
      render(() => <SwarmServicesDrawer cluster="" />);

      expect(screen.getByText('No Swarm cluster detected')).toBeInTheDocument();
    });

    it('displays cluster ID when available', async () => {
      mockApiResponse([]);
      render(() => (
        <SwarmServicesDrawer
          cluster="c1"
          swarm={{ clusterId: 'swarm-id-xyz', clusterName: 'Prod' }}
        />
      ));

      expect(screen.getByText(/Cluster ID:/)).toBeInTheDocument();
      expect(screen.getByText(/swarm-id-xyz/)).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('does not display cluster ID section when clusterId is empty', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ clusterName: 'Prod' }} />);

      expect(screen.queryByText(/Cluster ID:/)).not.toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('shows node role badge', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ nodeRole: 'manager' }} />);

      expect(screen.getByText('Role: manager')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('shows local state badge', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ localState: 'active' }} />);

      expect(screen.getByText('State: active')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('shows control available badge', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ controlAvailable: true }} />);

      expect(screen.getByText('Control: available')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('shows control unavailable badge', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ controlAvailable: false }} />);

      expect(screen.getByText('Control: unavailable')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('shows swarm error message', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ error: 'Node is draining' }} />);

      expect(screen.getByText('Node is draining')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });

    it('does not show error section when error is empty', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" swarm={{ error: '' }} />);

      // No amber error box rendered
      const errorBoxes = document.querySelectorAll('.bg-amber-50');
      expect(errorBoxes.length).toBe(0);
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });
  });

  describe('loading state', () => {
    it('shows loading message while services are being fetched', async () => {
      // Use a controllable promise — resolve before cleanup to prevent SolidJS scheduler corruption
      let resolveLoading!: (v: unknown) => void;
      apiFetchJSONMock.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveLoading = resolve;
        }),
      );

      try {
        render(() => <SwarmServicesDrawer cluster="c1" />);
        expect(screen.getByText('Loading Swarm services...')).toBeInTheDocument();
      } finally {
        // Always resolve before cleanup to prevent SolidJS scheduler corruption
        resolveLoading({ data: [], meta: { totalPages: 1 } });
        await new Promise((r) => setTimeout(r, 0));
      }
    });
  });

  describe('empty states', () => {
    it('shows "No Swarm services found" when API returns empty data', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      // Wait for resource to resolve
      await waitFor(() => {
        expect(screen.getByTestId('empty-state')).toBeInTheDocument();
      });

      expect(screen.getByTestId('empty-title')).toHaveTextContent('No Swarm services found');
    });

    it('shows "No services match your filters" when search filters everything out', async () => {
      mockApiResponse([makeService({ id: 'svc-1', name: 'nginx' })]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('nginx')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search services...');
      fireEvent.input(input, { target: { value: 'nonexistent-service-xyz' } });

      expect(screen.getByTestId('empty-title')).toHaveTextContent('No services match your filters');
    });
  });

  describe('services table', () => {
    it('renders service rows with correct data', async () => {
      const svc = makeService({
        id: 'svc-1',
        name: 'web-frontend',
        status: 'running',
        docker: {
          stack: 'prod',
          image: 'nginx:1.25',
          mode: 'replicated',
          desiredTasks: 3,
          runningTasks: 2,
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('web-frontend')).toBeInTheDocument();
      });

      expect(screen.getByText('prod')).toBeInTheDocument();
      expect(screen.getByText('nginx:1.25')).toBeInTheDocument();
      expect(screen.getByText('replicated')).toBeInTheDocument();
      expect(screen.getByText('3')).toBeInTheDocument();
      expect(screen.getByText('2')).toBeInTheDocument();
    });

    it('uses service id as name when name is empty', async () => {
      const svc = makeService({ id: 'svc-abc', name: '' });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-abc')).toBeInTheDocument();
      });
    });

    it('shows dash for missing optional fields', async () => {
      const svc = makeService({
        id: 'svc-1',
        name: 'bare-svc',
        docker: {
          stack: undefined,
          image: undefined,
          mode: undefined,
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('bare-svc')).toBeInTheDocument();
      });

      // Stack, image, mode should all show '—'
      const dashes = screen.getAllByText('—');
      expect(dashes.length).toBeGreaterThanOrEqual(3);
    });

    it('shows 0 for desiredTasks and runningTasks when missing', async () => {
      const svc = makeService({
        id: 'svc-1',
        name: 'zero-svc',
        docker: { desiredTasks: undefined, runningTasks: undefined },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('zero-svc')).toBeInTheDocument();
      });

      const zeros = screen.getAllByText('0');
      expect(zeros.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('search filtering', () => {
    it('filters by service name', async () => {
      mockApiResponse([
        makeService({ id: '1', name: 'nginx-proxy' }),
        makeService({ id: '2', name: 'redis-cache' }),
      ]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('nginx-proxy')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search services...');
      fireEvent.input(input, { target: { value: 'redis' } });

      expect(screen.queryByText('nginx-proxy')).not.toBeInTheDocument();
      expect(screen.getByText('redis-cache')).toBeInTheDocument();
    });

    it('filters by stack name', async () => {
      mockApiResponse([
        makeService({ id: '1', name: 'svc-a', docker: { stack: 'monitoring' } }),
        makeService({ id: '2', name: 'svc-b', docker: { stack: 'app' } }),
      ]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-a')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search services...');
      fireEvent.input(input, { target: { value: 'monitoring' } });

      expect(screen.getByText('svc-a')).toBeInTheDocument();
      expect(screen.queryByText('svc-b')).not.toBeInTheDocument();
    });

    it('filters by image name', async () => {
      mockApiResponse([
        makeService({ id: '1', name: 'svc-a', docker: { image: 'postgres:15' } }),
        makeService({ id: '2', name: 'svc-b', docker: { image: 'nginx:latest' } }),
      ]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-a')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search services...');
      fireEvent.input(input, { target: { value: 'postgres' } });

      expect(screen.getByText('svc-a')).toBeInTheDocument();
      expect(screen.queryByText('svc-b')).not.toBeInTheDocument();
    });

    it('search is case-insensitive', async () => {
      mockApiResponse([makeService({ id: '1', name: 'MyService' })]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('MyService')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search services...');
      fireEvent.input(input, { target: { value: 'MYSERVICE' } });

      expect(screen.getByText('MyService')).toBeInTheDocument();
    });

    it('shows all services when search is cleared', async () => {
      mockApiResponse([
        makeService({ id: '1', name: 'alpha' }),
        makeService({ id: '2', name: 'beta' }),
      ]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('alpha')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search services...');
      fireEvent.input(input, { target: { value: 'alpha' } });
      expect(screen.queryByText('beta')).not.toBeInTheDocument();

      fireEvent.input(input, { target: { value: '' } });
      expect(screen.getByText('alpha')).toBeInTheDocument();
      expect(screen.getByText('beta')).toBeInTheDocument();
    });
  });

  describe('sorting', () => {
    it('sorts services alphabetically by name', async () => {
      mockApiResponse([
        makeService({ id: '1', name: 'zebra' }),
        makeService({ id: '2', name: 'alpha' }),
        makeService({ id: '3', name: 'middle' }),
      ]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('zebra')).toBeInTheDocument();
      });

      // Check DOM order: alpha < middle < zebra
      const rows = container.querySelectorAll('tr');
      const names = Array.from(rows)
        .map((row) => row.querySelector('.font-semibold')?.textContent)
        .filter(Boolean);

      expect(names).toEqual(['alpha', 'middle', 'zebra']);
    });
  });

  describe('formatUpdate helper', () => {
    it('shows dash when no service update', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-no-update',
        docker: { serviceUpdate: undefined },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-no-update')).toBeInTheDocument();
      });

      // Update column should have '—'
      const dashes = screen.getAllByText('—');
      expect(dashes.length).toBeGreaterThanOrEqual(1);
    });

    it('shows "state: message" when both are present', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-updating',
        docker: {
          serviceUpdate: { state: 'updating', message: 'rolling back' },
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('updating: rolling back')).toBeInTheDocument();
      });
    });

    it('shows only state when message is empty', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-state-only',
        docker: {
          serviceUpdate: { state: 'completed', message: '' },
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('completed')).toBeInTheDocument();
      });
    });

    it('shows only message when state is empty', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-msg-only',
        docker: {
          serviceUpdate: { state: '', message: 'rollback complete' },
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('rollback complete')).toBeInTheDocument();
      });
    });
  });

  describe('formatPorts helper', () => {
    it('shows dash when no ports', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-no-ports',
        docker: { endpointPorts: [] },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-no-ports')).toBeInTheDocument();
      });

      // Ports column should show '—'
      const dashes = screen.getAllByText('—');
      expect(dashes.length).toBeGreaterThanOrEqual(1);
    });

    it('formats published->target/protocol for a single port', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-port',
        docker: {
          endpointPorts: [{ publishedPort: 8080, targetPort: 80, protocol: 'tcp' }],
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('8080->80/tcp')).toBeInTheDocument();
      });
    });

    it('formats multiple ports comma-separated', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-multi-ports',
        docker: {
          endpointPorts: [
            { publishedPort: 8080, targetPort: 80, protocol: 'tcp' },
            { publishedPort: 8443, targetPort: 443, protocol: 'tcp' },
          ],
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('8080->80/tcp, 8443->443/tcp')).toBeInTheDocument();
      });
    });

    it('shows only target/protocol when publishedPort is missing', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-target-only',
        docker: {
          endpointPorts: [{ targetPort: 3000 }],
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('3000/tcp')).toBeInTheDocument();
      });
    });

    it('defaults protocol to tcp when not specified', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-default-proto',
        docker: {
          endpointPorts: [{ publishedPort: 80, targetPort: 80 }],
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('80->80/tcp')).toBeInTheDocument();
      });
    });

    it('uses specified protocol (udp)', async () => {
      const svc = makeService({
        id: '1',
        name: 'svc-udp',
        docker: {
          endpointPorts: [{ publishedPort: 53, targetPort: 53, protocol: 'udp' }],
        },
      });
      mockApiResponse([svc]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('53->53/udp')).toBeInTheDocument();
      });
    });
  });

  describe('statusTone helper (via DOM classes)', () => {
    it('applies green class for running status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-running', status: 'running' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-running')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-emerald-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies green class for online status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-online', status: 'online' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-online')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-emerald-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies green class for healthy status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-healthy', status: 'healthy' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-healthy')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-emerald-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies amber class for warning status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-warn', status: 'warning' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-warn')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-amber-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies amber class for degraded status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-degraded', status: 'degraded' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-degraded')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-amber-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies red class for offline status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-offline', status: 'offline' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-offline')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-red-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies red class for stopped status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-stopped', status: 'stopped' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-stopped')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-red-500');
      expect(dot).toBeInTheDocument();
    });

    it('applies slate class for unknown status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-unknown', status: 'something-else' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-unknown')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-slate-400');
      expect(dot).toBeInTheDocument();
    });

    it('applies slate class for empty status', async () => {
      mockApiResponse([makeService({ id: '1', name: 'svc-empty', status: '' })]);
      const { container } = render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('svc-empty')).toBeInTheDocument();
      });

      const dot = container.querySelector('.bg-slate-400');
      expect(dot).toBeInTheDocument();
    });
  });

  describe('API interaction', () => {
    it('builds correct URL with cluster, page, and limit params', async () => {
      mockApiResponse([]);
      render(() => <SwarmServicesDrawer cluster="my-cluster" />);

      await waitFor(() => {
        expect(apiFetchJSONMock).toHaveBeenCalled();
      });

      const url = apiFetchJSONMock.mock.calls[0][0] as string;
      expect(url).toContain('type=docker-service');
      expect(url).toContain('cluster=my-cluster');
      expect(url).toContain('page=1');
      expect(url).toContain('limit=100');
    });

    it('does not fetch when cluster is empty', async () => {
      render(() => <SwarmServicesDrawer cluster="" />);

      // Flush multiple microtask cycles to let SolidJS evaluate source signal
      await new Promise<void>((resolve) => queueMicrotask(() => resolve()));
      await new Promise<void>((resolve) => queueMicrotask(() => resolve()));

      // Empty cluster = falsy source signal = fetcher not called
      expect(apiFetchJSONMock).not.toHaveBeenCalled();
    });

    it('fetches multiple pages when totalPages > 1', async () => {
      // First page response
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeService({ id: 'svc-1', name: 'page1-svc' })],
        meta: { totalPages: 3 },
      });
      // Pages 2 and 3
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeService({ id: 'svc-2', name: 'page2-svc' })],
        meta: { totalPages: 3 },
      });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeService({ id: 'svc-3', name: 'page3-svc' })],
        meta: { totalPages: 3 },
      });

      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('page1-svc')).toBeInTheDocument();
      });

      expect(screen.getByText('page2-svc')).toBeInTheDocument();
      expect(screen.getByText('page3-svc')).toBeInTheDocument();
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(3);
    });

    it('deduplicates services by id across pages', async () => {
      const svc = makeService({ id: 'dup-svc', name: 'duplicated' });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [svc],
        meta: { totalPages: 2 },
      });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [svc], // same id
        meta: { totalPages: 2 },
      });

      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('duplicated')).toBeInTheDocument();
      });

      // Only one row should appear (deduped)
      const matches = screen.getAllByText('duplicated');
      expect(matches).toHaveLength(1);
    });

    it('handles failed page fetches gracefully (Promise.allSettled)', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeService({ id: 'svc-ok', name: 'ok-svc' })],
        meta: { totalPages: 3 },
      });
      // Page 2 fails
      apiFetchJSONMock.mockRejectedValueOnce(new Error('network error'));
      // Page 3 succeeds
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeService({ id: 'svc-3', name: 'page3-svc' })],
        meta: { totalPages: 3 },
      });

      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('ok-svc')).toBeInTheDocument();
      });

      expect(screen.getByText('page3-svc')).toBeInTheDocument();
    });

    it('caps pagination at MAX_PAGES (20)', async () => {
      // First page says there are 25 total pages, but component caps at 20
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeService({ id: 'svc-1', name: 'first-page' })],
        meta: { totalPages: 25 },
      });
      // Mock pages 2-20 (19 additional pages)
      for (let i = 2; i <= 20; i++) {
        apiFetchJSONMock.mockResolvedValueOnce({
          data: [makeService({ id: `svc-${i}`, name: `page${i}-svc` })],
          meta: { totalPages: 25 },
        });
      }

      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('first-page')).toBeInTheDocument();
      });

      // Should have fetched exactly 20 pages (1 initial + 19 additional), not 25
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(20);
    });
  });

  describe('table headers', () => {
    it('renders all expected column headers', async () => {
      mockApiResponse([makeService()]);
      render(() => <SwarmServicesDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('Service')).toBeInTheDocument();
      });

      expect(screen.getByText('Stack')).toBeInTheDocument();
      expect(screen.getByText('Image')).toBeInTheDocument();
      expect(screen.getByText('Mode')).toBeInTheDocument();
      expect(screen.getByText('Desired')).toBeInTheDocument();
      expect(screen.getByText('Running')).toBeInTheDocument();
      expect(screen.getByText('Update')).toBeInTheDocument();
      expect(screen.getByText('Ports')).toBeInTheDocument();
    });
  });
});
