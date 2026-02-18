import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@solidjs/testing-library';
import { createSignal, onCleanup, onMount } from 'solid-js';
import { Dashboard } from '../Dashboard';
import {
  filterWorkloads,
  createWorkloadSortComparator,
  groupWorkloads,
  computeWorkloadStats,
} from '../workloadSelectors';

let mockLocationSearch = '';
let mockWorkloads: Array<Record<string, unknown>> = [];
let setMockWorkloadsSignal: ((next: Array<Record<string, unknown>>) => void) | null = null;
let guestRowMountCount = 0;
let guestRowUnmountCount = 0;

const pushMockWorkloads = (next: Array<Record<string, unknown>>) => {
  mockWorkloads = next;
  setMockWorkloadsSignal?.(next);
};

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/workloads', search: mockLocationSearch }),
    useNavigate: () => vi.fn(),
  };
});

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    connected: () => true,
    activeAlerts: () => ({}),
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
    state: { kubernetesClusters: [], temperatureMonitoringEnabled: false },
  }),
}));

vi.mock('@/hooks/useWorkloads', () => ({
  useWorkloads: () => {
    const [workloads, setWorkloads] = createSignal(mockWorkloads as any);
    setMockWorkloadsSignal = (next) => setWorkloads(next as any);
    return {
      workloads,
      refetch: vi.fn(),
      mutate: vi.fn(),
      loading: () => false,
      error: () => null,
    };
  },
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: { getAllMetadata: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ activationState: () => 'active' }),
}));

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: { focusInput: () => false },
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/url', () => ({
  isKioskMode: () => false,
  subscribeToKioskMode: () => () => undefined,
}));

vi.mock('@/components/Workloads/WorkloadsSummary', () => ({
  WorkloadsSummary: () => <div data-testid="workloads-summary">summary</div>,
}));

vi.mock('@/components/shared/UnifiedNodeSelector', () => ({
  UnifiedNodeSelector: () => <div data-testid="node-selector">node-selector</div>,
}));

vi.mock('../DashboardFilter', () => ({
  DashboardFilter: () => <div data-testid="dashboard-filter">filter</div>,
}));

vi.mock('../GuestDrawer', () => ({
  GuestDrawer: () => <div data-testid="guest-drawer">drawer</div>,
}));

vi.mock('../GuestRow', () => {
  const columns = [
    { id: 'name', label: 'Name', toggleable: false, sortKey: 'name' },
    { id: 'status', label: 'Status', toggleable: false, sortKey: 'status' },
  ];
  return {
    GUEST_COLUMNS: columns,
    VIEW_MODE_COLUMNS: {
      all: new Set(['name', 'status']),
      vm: new Set(['name', 'status']),
      lxc: new Set(['name', 'status']),
      docker: new Set(['name', 'status']),
      k8s: new Set(['name', 'status']),
    },
    GuestRow: (props: { guest: { name: string } }) => {
      onMount(() => {
        guestRowMountCount += 1;
      });
      onCleanup(() => {
        guestRowUnmountCount += 1;
      });
      return (
        <tr data-testid={`guest-row-${props.guest.name}`}>
          <td>{props.guest.name}</td>
          <td>running</td>
        </tr>
      );
    },
  };
});

const makeGuest = (i: number, overrides?: Record<string, unknown>) => ({
  id: `guest-${i}`,
  vmid: 100 + i,
  name: `workload-${i}`,
  node: `node-${i % 5}`,
  instance: `cluster-${i % 3}`,
  status: i % 7 === 0 ? 'stopped' : 'running',
  type: i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'docker' : 'vm',
  cpu: (i % 100) / 100,
  cpus: 2,
  memory: { total: 4 * 1024, used: ((i % 80) / 100) * 4 * 1024, free: 0, usage: (i % 80) / 100 },
  disk: { total: 100 * 1024, used: ((i % 60) / 100) * 100 * 1024, free: 0, usage: (i % 60) / 100 },
  networkIn: i * 100,
  networkOut: i * 50,
  diskRead: i * 10,
  diskWrite: i * 5,
  uptime: i * 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: new Date().toISOString(),
  workloadType: i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'docker' : 'vm',
  ...overrides,
});

const PROFILES = {
  S: 400,
  M: 1500,
  L: 5000,
} as const;

const makeGuests = (
  count: number,
  overrides?: (i: number) => Record<string, unknown>,
) => Array.from({ length: count }, (_, i) => makeGuest(i, overrides?.(i)));

const getTypeDistribution = (guests: Array<Record<string, unknown>>) => guests.reduce<{
  vm: number;
  lxc: number;
  docker: number;
}>(
  (acc, guest) => {
    const type = guest.type;
    if (type === 'vm') acc.vm += 1;
    if (type === 'lxc') acc.lxc += 1;
    if (type === 'docker') acc.docker += 1;
    return acc;
  },
  { vm: 0, lxc: 0, docker: 0 },
);

const expectedTypeDistribution = (count: number) => {
  const lxc = Math.floor((count - 1) / 4) + 1;
  const multiplesOf3 = Math.floor((count - 1) / 3) + 1;
  const multiplesOf12 = Math.floor((count - 1) / 12) + 1;
  const docker = multiplesOf3 - multiplesOf12;
  const vm = count - lxc - docker;
  return { vm, lxc, docker };
};

const getGuestRowCount = (container: HTMLElement) =>
  container.querySelectorAll('tr[data-testid^="guest-row-"]').length;

describe('Dashboard performance contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockLocationSearch = '';
    mockWorkloads = [];
    setMockWorkloadsSignal = null;
    guestRowMountCount = 0;
    guestRowUnmountCount = 0;
  });

  describe('Fixture profile validation', () => {
    for (const [profile, count] of Object.entries(PROFILES)) {
      it(`builds profile ${profile} with stable size and type distribution`, () => {
        const guests = makeGuests(count);
        const distribution = getTypeDistribution(guests);
        const expectedDistribution = expectedTypeDistribution(count);

        expect(guests).toHaveLength(count);
        expect(distribution).toEqual(expectedDistribution);
      });
    }
  });

  describe('Baseline structural contracts', () => {
    it('renders Profile S dashboard table and guest rows', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.S);

      const { container, getByTestId } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getByTestId('workloads-summary')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(PROFILES.S);
      });
    });
  });

  describe('Workload windowing contracts', () => {
    it('Profile M: caps mounted guest rows when windowing is active', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.M);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        const rowCount = getGuestRowCount(container);
        expect(rowCount).toBeGreaterThan(0);
        expect(rowCount).toBeLessThanOrEqual(140);
      });
    });

    it('Profile L: keeps mounted guest rows capped under large load', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.L);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(
        () => {
          const rowCount = getGuestRowCount(container);
          expect(rowCount).toBeGreaterThan(0);
          expect(rowCount).toBeLessThanOrEqual(140);
        },
        { timeout: 20000 },
      );
    });

    it('Profile S: renders all guest rows without windowing', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.S);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(PROFILES.S);
      });
    });

    it('unchanged poll-like updates do not remount table rows', async () => {
      mockLocationSearch = '?type=all';
      const guests = makeGuests(40);
      mockWorkloads = guests;

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(40);
      });

      const mountsAfterInitialRender = guestRowMountCount;
      const unmountsAfterInitialRender = guestRowUnmountCount;
      expect(mountsAfterInitialRender).toBe(40);

      pushMockWorkloads(guests.map((guest) => ({ ...guest })));

      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(40);
      });

      expect(guestRowMountCount).toBe(mountsAfterInitialRender);
      expect(guestRowUnmountCount).toBe(unmountsAfterInitialRender);
    });
  });

  describe('Filter mode contract', () => {
    it('applies all/vm/lxc/docker type filters deterministically for Profile S', async () => {
      const profileGuests = makeGuests(PROFILES.S);
      const expectedByMode = {
        all: PROFILES.S,
        vm: profileGuests.filter((guest) => guest.type === 'vm').length,
        lxc: profileGuests.filter((guest) => guest.type === 'lxc').length,
        docker: profileGuests.filter((guest) => guest.type === 'docker').length,
      };

      for (const mode of ['all', 'vm', 'lxc', 'docker'] as const) {
        mockLocationSearch = `?type=${mode}`;
        mockWorkloads = profileGuests;

        const { container, unmount } = render(() => (
          <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
        ));

        await waitFor(() => {
          expect(getGuestRowCount(container)).toBe(expectedByMode[mode]);
        });

        unmount();
      }
    });
  });

  describe('Workload derivation contracts', () => {
    it('filterWorkloads returns all guests when no filters active', () => {
      const guests = makeGuests(PROFILES.S);
      const result = filterWorkloads({
        guests: guests as any,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
        selectedNamespace: null,
      });
      expect(result).toHaveLength(PROFILES.S);
    });

    it('filterWorkloads correctly filters by viewMode', () => {
      const guests = makeGuests(PROFILES.S);
      const vmCount = guests.filter((g) => g.type === 'vm').length;
      const result = filterWorkloads({
        guests: guests as any,
        viewMode: 'vm',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
        selectedNamespace: null,
      });
      expect(result).toHaveLength(vmCount);
    });

    it('createWorkloadSortComparator preserves array length', () => {
      const guests = makeGuests(PROFILES.M);
      const comparator = createWorkloadSortComparator('cpu', 'asc');
      expect(comparator).not.toBeNull();
      const sorted = [...(guests as any)].sort(comparator!);
      expect(sorted).toHaveLength(PROFILES.M);
    });

    it('groupWorkloads produces correct total count in grouped mode', () => {
      const guests = makeGuests(PROFILES.S);
      const groups = groupWorkloads(guests as any, 'grouped', null);
      const total = Object.values(groups).reduce((sum, g) => sum + g.length, 0);
      expect(total).toBe(PROFILES.S);
    });

    it('computeWorkloadStats counts match total', () => {
      const guests = makeGuests(PROFILES.S);
      const stats = computeWorkloadStats(guests as any);
      expect(stats.total).toBe(PROFILES.S);
      expect(stats.vms + stats.containers + stats.docker + stats.k8s).toBe(PROFILES.S);
      expect(stats.running + stats.degraded + stats.stopped).toBe(PROFILES.S);
    });
  });
});
