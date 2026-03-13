import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@solidjs/testing-library';

// ── Hoisted mocks ──────────────────────────────────────────────────────

const { apiFetchJSONMock, navigateMock } = vi.hoisted(() => ({
  apiFetchJSONMock: vi.fn(),
  navigateMock: vi.fn(),
}));

// ── Module mocks ───────────────────────────────────────────────────────

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: apiFetchJSONMock,
}));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

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

import { K8sDeploymentsDrawer } from '../K8sDeploymentsDrawer';

// ── Helpers ─────────────────────────────────────────────────────────────

type K8sDeploymentResource = {
  id: string;
  name?: string;
  status?: string;
  kubernetes?: {
    namespace?: string;
    desiredReplicas?: number;
    updatedReplicas?: number;
    readyReplicas?: number;
    availableReplicas?: number;
  };
};

function makeDeployment(overrides: Partial<K8sDeploymentResource> = {}): K8sDeploymentResource {
  return {
    id: 'dep-1',
    name: 'my-deployment',
    status: 'running',
    kubernetes: {
      namespace: 'default',
      desiredReplicas: 3,
      updatedReplicas: 3,
      readyReplicas: 3,
      availableReplicas: 3,
    },
    ...overrides,
  };
}

function mockApiResponse(data: K8sDeploymentResource[], totalPages = 1) {
  apiFetchJSONMock.mockResolvedValueOnce({ data, meta: { totalPages } });
}

// ── Tests ───────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  vi.clearAllMocks();
});

describe('K8sDeploymentsDrawer', () => {
  describe('loading state', () => {
    it('shows loading message while deployments are being fetched', async () => {
      let resolveLoading!: (v: unknown) => void;
      apiFetchJSONMock.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveLoading = resolve;
        }),
      );

      try {
        render(() => <K8sDeploymentsDrawer cluster="c1" />);
        expect(screen.getByText('Loading deployments...')).toBeInTheDocument();
      } finally {
        resolveLoading({ data: [], meta: { totalPages: 1 } });
        await new Promise((r) => setTimeout(r, 0));
      }
    });
  });

  describe('empty states', () => {
    it('shows "No deployments found" when API returns empty data', async () => {
      mockApiResponse([]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      // Wait for loading to finish — the "No deployments found" title replaces "Loading deployments..."
      await waitFor(() => {
        expect(screen.getByTestId('empty-title')).toHaveTextContent('No deployments found');
      });

      expect(screen.getByTestId('empty-description')).toHaveTextContent(
        'Enable the Kubernetes agent deployment collection',
      );
    });

    it('shows "No deployments match your filters" when search filters everything', async () => {
      mockApiResponse([makeDeployment({ id: 'dep-1', name: 'nginx' })]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('nginx')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search deployments...');
      fireEvent.input(input, { target: { value: 'nonexistent-xyz' } });

      expect(screen.getByTestId('empty-title')).toHaveTextContent(
        'No deployments match your filters',
      );
      expect(screen.getByTestId('empty-description')).toHaveTextContent(
        'Try clearing the search or namespace filter.',
      );
    });
  });

  describe('deployments table', () => {
    it('renders deployment rows with correct data', async () => {
      const dep = makeDeployment({
        id: 'dep-1',
        name: 'web-frontend',
        status: 'running',
        kubernetes: {
          namespace: 'prod-ns',
          desiredReplicas: 5,
          updatedReplicas: 4,
          readyReplicas: 3,
          availableReplicas: 2,
        },
      });
      mockApiResponse([dep]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('web-frontend')).toBeInTheDocument();
      });

      // Namespace appears in both the dropdown option and the table cell — assert exactly 2
      const nsCells = screen.getAllByText('prod-ns');
      expect(nsCells.length).toBe(2); // 1 in dropdown + 1 in table row
      expect(screen.getByText('5')).toBeInTheDocument();
      expect(screen.getByText('4')).toBeInTheDocument();
      expect(screen.getByText('3')).toBeInTheDocument();
      expect(screen.getByText('2')).toBeInTheDocument();
    });

    it('uses deployment id as name when name is empty', async () => {
      const dep = makeDeployment({ id: 'dep-abc', name: '' });
      mockApiResponse([dep]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-abc')).toBeInTheDocument();
      });
    });

    it('shows dash for missing namespace', async () => {
      const dep = makeDeployment({
        id: 'dep-1',
        name: 'no-ns-dep',
        kubernetes: { namespace: undefined },
      });
      mockApiResponse([dep]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('no-ns-dep')).toBeInTheDocument();
      });

      const dashes = screen.getAllByText('—');
      expect(dashes.length).toBeGreaterThanOrEqual(1);
    });

    it('shows 0 for replica counts when missing', async () => {
      const dep = makeDeployment({
        id: 'dep-1',
        name: 'zero-dep',
        kubernetes: {
          namespace: 'default',
          desiredReplicas: undefined,
          updatedReplicas: undefined,
          readyReplicas: undefined,
          availableReplicas: undefined,
        },
      });
      mockApiResponse([dep]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('zero-dep')).toBeInTheDocument();
      });

      const zeros = screen.getAllByText('0');
      expect(zeros.length).toBeGreaterThanOrEqual(4);
    });

    it('renders all expected column headers', async () => {
      mockApiResponse([makeDeployment()]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('Deployment')).toBeInTheDocument();
      });

      // "Namespace" appears in both the label and table header — check via th elements
      const headers = Array.from(container.querySelectorAll('th')).map((th) => th.textContent);
      expect(headers).toContain('Namespace');
      expect(headers).toContain('Desired');
      expect(headers).toContain('Updated');
      expect(headers).toContain('Ready');
      expect(headers).toContain('Available');
      expect(headers).toContain('Actions');
    });
  });

  describe('search filtering', () => {
    it('filters by deployment name', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'nginx-proxy', kubernetes: { namespace: 'default' } }),
        makeDeployment({ id: '2', name: 'redis-cache', kubernetes: { namespace: 'default' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('nginx-proxy')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search deployments...');
      fireEvent.input(input, { target: { value: 'redis' } });

      expect(screen.queryByText('nginx-proxy')).not.toBeInTheDocument();
      expect(screen.getByText('redis-cache')).toBeInTheDocument();
    });

    it('search is case-insensitive', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'MyDeployment', kubernetes: { namespace: 'default' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('MyDeployment')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search deployments...');
      fireEvent.input(input, { target: { value: 'MYDEPLOYMENT' } });

      expect(screen.getByText('MyDeployment')).toBeInTheDocument();
    });

    it('shows all deployments when search is cleared', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'alpha', kubernetes: { namespace: 'default' } }),
        makeDeployment({ id: '2', name: 'beta', kubernetes: { namespace: 'default' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('alpha')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search deployments...');
      fireEvent.input(input, { target: { value: 'alpha' } });
      expect(screen.queryByText('beta')).not.toBeInTheDocument();

      fireEvent.input(input, { target: { value: '' } });
      expect(screen.getByText('alpha')).toBeInTheDocument();
      expect(screen.getByText('beta')).toBeInTheDocument();
    });

    it('falls back to id for search when name is empty', async () => {
      mockApiResponse([
        makeDeployment({ id: 'dep-id-abc', name: '', kubernetes: { namespace: 'default' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-id-abc')).toBeInTheDocument();
      });

      const input = screen.getByPlaceholderText('Search deployments...');
      fireEvent.input(input, { target: { value: 'dep-id' } });

      expect(screen.getByText('dep-id-abc')).toBeInTheDocument();
    });
  });

  describe('namespace filtering', () => {
    it('shows namespace dropdown when namespaces exist', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-a', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'dep-b', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-a')).toBeInTheDocument();
      });

      const select = screen.getByLabelText('Namespace') as HTMLSelectElement;
      expect(select).toBeInTheDocument();

      // Check "All namespaces" default option
      const options = select.querySelectorAll('option');
      expect(options[0]).toHaveTextContent('All namespaces');
      expect(options[1]).toHaveTextContent('production');
      expect(options[2]).toHaveTextContent('staging');
    });

    it('filters deployments by selected namespace', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'prod-dep', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'stage-dep', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('prod-dep')).toBeInTheDocument();
      });

      const select = screen.getByLabelText('Namespace');
      fireEvent.change(select, { target: { value: 'production' } });

      expect(screen.getByText('prod-dep')).toBeInTheDocument();
      expect(screen.queryByText('stage-dep')).not.toBeInTheDocument();
    });

    it('sorts namespace options alphabetically', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-1', kubernetes: { namespace: 'zebra' } }),
        makeDeployment({ id: '2', name: 'dep-2', kubernetes: { namespace: 'alpha' } }),
        makeDeployment({ id: '3', name: 'dep-3', kubernetes: { namespace: 'middle' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-1')).toBeInTheDocument();
      });

      const select = screen.getByLabelText('Namespace') as HTMLSelectElement;
      const options = Array.from(select.querySelectorAll('option')).map((o) => o.textContent);
      // First option is "All namespaces", then sorted
      expect(options).toEqual(['All namespaces', 'alpha', 'middle', 'zebra']);
    });

    it('deduplicates namespace options', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-1', kubernetes: { namespace: 'default' } }),
        makeDeployment({ id: '2', name: 'dep-2', kubernetes: { namespace: 'default' } }),
        makeDeployment({ id: '3', name: 'dep-3', kubernetes: { namespace: 'kube-system' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-1')).toBeInTheDocument();
      });

      const select = screen.getByLabelText('Namespace') as HTMLSelectElement;
      const options = Array.from(select.querySelectorAll('option')).map((o) => o.textContent);
      expect(options).toEqual(['All namespaces', 'default', 'kube-system']);
    });

    it('does not show namespace dropdown when no namespaces exist', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-1', kubernetes: { namespace: '' } })]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-1')).toBeInTheDocument();
      });

      expect(screen.queryByLabelText('Namespace')).not.toBeInTheDocument();
    });

    it('combines namespace and search filters', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'nginx-prod', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'nginx-stage', kubernetes: { namespace: 'staging' } }),
        makeDeployment({ id: '3', name: 'redis-prod', kubernetes: { namespace: 'production' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('nginx-prod')).toBeInTheDocument();
      });

      // Filter by namespace
      const select = screen.getByLabelText('Namespace');
      fireEvent.change(select, { target: { value: 'production' } });

      // Then also search
      const input = screen.getByPlaceholderText('Search deployments...');
      fireEvent.input(input, { target: { value: 'nginx' } });

      expect(screen.getByText('nginx-prod')).toBeInTheDocument();
      expect(screen.queryByText('nginx-stage')).not.toBeInTheDocument();
      expect(screen.queryByText('redis-prod')).not.toBeInTheDocument();
    });
  });

  describe('sorting', () => {
    it('sorts deployments alphabetically by name', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'zebra', kubernetes: { namespace: 'default' } }),
        makeDeployment({ id: '2', name: 'alpha', kubernetes: { namespace: 'default' } }),
        makeDeployment({ id: '3', name: 'middle', kubernetes: { namespace: 'default' } }),
      ]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('zebra')).toBeInTheDocument();
      });

      const rows = container.querySelectorAll('tr');
      const names = Array.from(rows)
        .map((row) => row.querySelector('.font-semibold')?.textContent)
        .filter(Boolean);

      expect(names).toEqual(['alpha', 'middle', 'zebra']);
    });

    it('uses id as tiebreaker when names are the same', async () => {
      // Invert markers: a-id gets higher replica count (99), z-id gets lower (11).
      // Only an id-based sort produces [a-id(99), z-id(11)]; namespace or replica
      // sorts would produce a different order, making the assertion uniquely prove
      // that id is the tiebreaker.
      mockApiResponse([
        makeDeployment({
          id: 'z-id',
          name: 'same',
          kubernetes: { namespace: 'ns-a', desiredReplicas: 11 },
        }),
        makeDeployment({
          id: 'a-id',
          name: 'same',
          kubernetes: { namespace: 'ns-z', desiredReplicas: 99 },
        }),
      ]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getAllByText('same').length).toBe(2);
      });

      // id asc → a-id (99) before z-id (11)
      const rows = container.querySelectorAll('tbody tr');
      expect(rows.length).toBe(2);
      expect(rows[0].textContent).toContain('99'); // a-id row (desiredReplicas=99)
      expect(rows[1].textContent).toContain('11'); // z-id row (desiredReplicas=11)
    });
  });

  describe('statusTone helper (via DOM classes)', () => {
    it('applies green class for running status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-running', status: 'running' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-running')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-emerald-500')).toBeInTheDocument();
    });

    it('applies green class for online status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-online', status: 'online' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-online')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-emerald-500')).toBeInTheDocument();
    });

    it('applies green class for healthy status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-healthy', status: 'healthy' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-healthy')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-emerald-500')).toBeInTheDocument();
    });

    it('applies amber class for warning status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-warn', status: 'warning' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-warn')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-amber-500')).toBeInTheDocument();
    });

    it('applies amber class for degraded status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-degraded', status: 'degraded' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-degraded')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-amber-500')).toBeInTheDocument();
    });

    it('applies red class for offline status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-offline', status: 'offline' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-offline')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-red-500')).toBeInTheDocument();
    });

    it('applies red class for stopped status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-stopped', status: 'stopped' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-stopped')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-red-500')).toBeInTheDocument();
    });

    it('applies slate class for unknown status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-unknown', status: 'something-else' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-unknown')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-slate-400')).toBeInTheDocument();
    });

    it('applies slate class for empty status', async () => {
      mockApiResponse([makeDeployment({ id: '1', name: 'dep-empty', status: '' })]);
      const { container } = render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-empty')).toBeInTheDocument();
      });

      expect(container.querySelector('.bg-slate-400')).toBeInTheDocument();
    });
  });

  describe('API interaction', () => {
    it('builds correct URL with cluster, page, and limit params', async () => {
      mockApiResponse([]);
      render(() => <K8sDeploymentsDrawer cluster="my-cluster" />);

      await waitFor(() => {
        expect(apiFetchJSONMock).toHaveBeenCalled();
      });

      const url = apiFetchJSONMock.mock.calls[0][0] as string;
      expect(url).toContain('type=k8s-deployment');
      expect(url).toContain('cluster=my-cluster');
      expect(url).toContain('page=1');
      expect(url).toContain('limit=100');
    });

    it('does not fetch when cluster is empty', async () => {
      render(() => <K8sDeploymentsDrawer cluster="" />);

      // Flush multiple microtask cycles to let SolidJS evaluate source signal
      await new Promise<void>((resolve) => queueMicrotask(() => resolve()));
      await new Promise<void>((resolve) => queueMicrotask(() => resolve()));

      expect(apiFetchJSONMock).not.toHaveBeenCalled();
    });

    it('fetches multiple pages when totalPages > 1', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-1', name: 'page1-dep' })],
        meta: { totalPages: 3 },
      });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-2', name: 'page2-dep' })],
        meta: { totalPages: 3 },
      });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-3', name: 'page3-dep' })],
        meta: { totalPages: 3 },
      });

      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('page1-dep')).toBeInTheDocument();
      });

      expect(screen.getByText('page2-dep')).toBeInTheDocument();
      expect(screen.getByText('page3-dep')).toBeInTheDocument();
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(3);
    });

    it('deduplicates deployments by id across pages', async () => {
      const dep = makeDeployment({ id: 'dup-dep', name: 'duplicated' });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [dep],
        meta: { totalPages: 2 },
      });
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [dep],
        meta: { totalPages: 2 },
      });

      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('duplicated')).toBeInTheDocument();
      });

      const matches = screen.getAllByText('duplicated');
      expect(matches).toHaveLength(1);
    });

    it('handles failed page fetches gracefully (Promise.allSettled)', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-ok', name: 'ok-dep' })],
        meta: { totalPages: 3 },
      });
      apiFetchJSONMock.mockRejectedValueOnce(new Error('network error'));
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-3', name: 'page3-dep' })],
        meta: { totalPages: 3 },
      });

      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('ok-dep')).toBeInTheDocument();
      });

      expect(screen.getByText('page3-dep')).toBeInTheDocument();
    });

    it('caps pagination at MAX_PAGES (20)', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-1', name: 'first-page' })],
        meta: { totalPages: 25 },
      });
      for (let i = 2; i <= 20; i++) {
        apiFetchJSONMock.mockResolvedValueOnce({
          data: [makeDeployment({ id: `dep-${i}`, name: `page${i}-dep` })],
          meta: { totalPages: 25 },
        });
      }

      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('first-page')).toBeInTheDocument();
      });

      // Should have fetched exactly 20 pages (1 initial + 19 additional), not 25
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(20);
    });

    it('handles non-array data gracefully', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        data: null,
        meta: { totalPages: 1 },
      });

      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByTestId('empty-title')).toHaveTextContent('No deployments found');
      });
    });

    it('handles missing meta.totalPages gracefully', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        data: [makeDeployment({ id: 'dep-1', name: 'solo-dep' })],
        meta: {},
      });

      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('solo-dep')).toBeInTheDocument();
      });

      // Should only fetch one page when totalPages is missing
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    });
  });

  describe('navigation', () => {
    it('"Open Pods" button navigates with cluster context', async () => {
      mockApiResponse([makeDeployment()]);
      render(() => <K8sDeploymentsDrawer cluster="my-cluster" />);

      await waitFor(() => {
        expect(screen.getByText('my-deployment')).toBeInTheDocument();
      });

      const openPodsBtn = screen.getByRole('button', { name: 'Open Pods' });
      fireEvent.click(openPodsBtn);

      expect(navigateMock).toHaveBeenCalledTimes(1);
      const path = navigateMock.mock.calls[0][0] as string;
      expect(path).toContain('/workloads');
      expect(path).toContain('type=pod');
      expect(path).toContain('context=my-cluster');
    });

    it('"Open Pods" passes selected namespace to navigation', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-a', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'dep-b', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-a')).toBeInTheDocument();
      });

      // Select a namespace first
      const select = screen.getByLabelText('Namespace');
      fireEvent.change(select, { target: { value: 'production' } });

      const openPodsBtn = screen.getByRole('button', { name: 'Open Pods' });
      fireEvent.click(openPodsBtn);

      expect(navigateMock).toHaveBeenCalledTimes(1);
      const path = navigateMock.mock.calls[0][0] as string;
      expect(path).toContain('namespace=production');
    });

    it('"View Pods" button on a row navigates with that deployment\'s namespace', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-a', kubernetes: { namespace: 'kube-system' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      await waitFor(() => {
        expect(screen.getByText('dep-a')).toBeInTheDocument();
      });

      const viewPodsBtn = screen.getByRole('button', { name: 'View Pods' });
      fireEvent.click(viewPodsBtn);

      expect(navigateMock).toHaveBeenCalledTimes(1);
      const path = navigateMock.mock.calls[0][0] as string;
      expect(path).toContain('namespace=kube-system');
    });

    it('does not navigate when cluster is empty', async () => {
      render(() => <K8sDeploymentsDrawer cluster="" />);

      const openPodsBtn = screen.getByRole('button', { name: 'Open Pods' });
      fireEvent.click(openPodsBtn);

      expect(navigateMock).not.toHaveBeenCalled();
    });
  });

  describe('initialNamespace prop', () => {
    it('prefills namespace filter when initialNamespace matches an existing namespace', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-prod', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'dep-stage', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" initialNamespace="production" />);

      await waitFor(() => {
        expect(screen.getByText('dep-prod')).toBeInTheDocument();
      });

      // Should filter to only production deployments
      expect(screen.queryByText('dep-stage')).not.toBeInTheDocument();
    });

    it('does not apply initialNamespace when it does not match any namespace', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-prod', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'dep-stage', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" initialNamespace="nonexistent" />);

      await waitFor(() => {
        expect(screen.getByText('dep-prod')).toBeInTheDocument();
      });

      // Both should be visible since nonexistent namespace is not applied
      expect(screen.getByText('dep-stage')).toBeInTheDocument();
    });

    it('case-mismatch initialNamespace is accepted but filters out all rows (documents bug)', async () => {
      // The component checks existence case-insensitively but filters case-sensitively.
      // When initialNamespace="production" but actual namespace is "Production",
      // the existence check passes and the filter value is set to "production",
      // but the row filter does an exact comparison and no rows match.
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-prod', kubernetes: { namespace: 'Production' } }),
        makeDeployment({ id: '2', name: 'dep-stage', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" initialNamespace="production" />);

      await waitFor(() => {
        expect(screen.getByTestId('empty-title')).toHaveTextContent(
          'No deployments match your filters',
        );
      });
    });

    it('does not apply initialNamespace when it is empty', async () => {
      mockApiResponse([
        makeDeployment({ id: '1', name: 'dep-prod', kubernetes: { namespace: 'production' } }),
        makeDeployment({ id: '2', name: 'dep-stage', kubernetes: { namespace: 'staging' } }),
      ]);
      render(() => <K8sDeploymentsDrawer cluster="c1" initialNamespace="" />);

      await waitFor(() => {
        expect(screen.getByText('dep-prod')).toBeInTheDocument();
      });

      expect(screen.getByText('dep-stage')).toBeInTheDocument();
    });
  });

  describe('header card', () => {
    it('renders title and subtitle', async () => {
      mockApiResponse([]);
      render(() => <K8sDeploymentsDrawer cluster="c1" />);

      expect(screen.getByText('Deployments')).toBeInTheDocument();
      expect(screen.getByText('Desired state controllers (not Pods)')).toBeInTheDocument();
      await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    });
  });
});
