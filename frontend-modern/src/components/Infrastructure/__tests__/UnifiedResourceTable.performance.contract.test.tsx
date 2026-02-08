import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({ isMobile: () => false }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="metric-cell">metric</div>,
}));

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div data-testid="resource-drawer">drawer</div>,
}));

const makeResource = (i: number, overrides?: Partial<Resource>): Resource => ({
  id: `resource-${i}`,
  type: i % 5 === 0 ? 'node' : i % 3 === 0 ? 'docker-host' : 'host',
  name: `host-${i}`,
  displayName: `Host ${i}`,
  platformId: `platform-${i}`,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: i % 10 === 0 ? 'offline' : 'online',
  cpu: { current: (i % 100) / 100 },
  memory: {
    current: (i % 80) / 100,
    total: 16 * 1024 * 1024 * 1024,
    used: ((i % 80) / 100) * 16 * 1024 * 1024 * 1024,
  },
  disk: { current: 0.3 },
  lastSeen: Date.now(),
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

const PROFILES = {
  S: 250,
  M: 1000,
  L: 3000,
} as const;

const makeResources = (
  count: number,
  overrides?: (i: number) => Partial<Resource>,
): Resource[] => Array.from({ length: count }, (_, i) => makeResource(i, overrides?.(i)));

const getTypeDistribution = (resources: Resource[]) => resources.reduce(
  (acc, resource) => {
    if (resource.type === 'node') acc.node += 1;
    if (resource.type === 'docker-host') acc.dockerHost += 1;
    if (resource.type === 'host') acc.host += 1;
    return acc;
  },
  { node: 0, dockerHost: 0, host: 0 },
);

const expectedTypeDistribution = (count: number) => {
  const node = Math.floor((count - 1) / 5) + 1;
  const multiplesOf3 = Math.floor((count - 1) / 3) + 1;
  const multiplesOf15 = Math.floor((count - 1) / 15) + 1;
  const dockerHost = multiplesOf3 - multiplesOf15;
  const host = count - node - dockerHost;
  return { node, dockerHost, host };
};

const getBodyRowCount = (container: HTMLElement) => container.querySelectorAll('tbody tr').length;

describe('UnifiedResourceTable performance contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  describe('Fixture profile validation', () => {
    for (const [profile, count] of Object.entries(PROFILES)) {
      it(`builds profile ${profile} with stable size and type distribution`, () => {
        const resources = makeResources(count);
        const distribution = getTypeDistribution(resources);
        const expectedDistribution = expectedTypeDistribution(count);

        expect(resources).toHaveLength(count);
        expect(distribution).toEqual(expectedDistribution);
      });
    }
  });

  describe('Baseline structural contracts', () => {
    it('renders Profile S table structure and all rows', async () => {
      const resources = makeResources(PROFILES.S);
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBeGreaterThan(0);
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S);
      });
    });
  });

  describe('Profile M baseline', () => {
    it('renders Profile M without crashing and keeps baseline row count', async () => {
      const resources = makeResources(PROFILES.M);
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.M);
      });
    });
  });

  describe('Profile L baseline', () => {
    it('renders Profile L without crashing as pre-windowing baseline', async () => {
      const resources = makeResources(PROFILES.L);
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(
        () => {
          expect(getBodyRowCount(container)).toBe(PROFILES.L);
        },
        { timeout: 15000 },
      );
    });
  });

  describe('Grouped vs flat mode contract', () => {
    it('renders grouped mode with profile S resources', async () => {
      const resources = makeResources(PROFILES.S, (i) => ({
        clusterId: i % 2 === 0 ? 'cluster-a' : 'cluster-b',
      }));
      const { container, getByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="grouped"
        />
      ));

      await waitFor(() => {
        expect(getByText('cluster-a')).toBeInTheDocument();
        expect(getByText('cluster-b')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S + 2);
      });
    });

    it('renders flat mode with profile S resources', async () => {
      const resources = makeResources(PROFILES.S, (i) => ({
        clusterId: i % 2 === 0 ? 'cluster-a' : 'cluster-b',
      }));
      const { container, queryByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S);
      });
      expect(queryByText('cluster-a')).not.toBeInTheDocument();
      expect(queryByText('cluster-b')).not.toBeInTheDocument();
    });
  });

  describe.todo('Transform budget placeholder');
});
