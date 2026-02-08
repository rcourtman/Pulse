import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@solidjs/testing-library';
import { Dashboard } from '../Dashboard';

let mockLocationSearch = '';
let mockV2Workloads: Array<Record<string, unknown>> = [];

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

vi.mock('@/hooks/useV2Workloads', () => ({
  useV2Workloads: () => ({ workloads: () => mockV2Workloads as any, refetch: vi.fn() }),
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
    GuestRow: (props: { guest: { name: string } }) => (
      <tr data-testid={`guest-row-${props.guest.name}`}>
        <td>{props.guest.name}</td>
        <td>running</td>
      </tr>
    ),
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
    mockV2Workloads = [];
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
      mockV2Workloads = makeGuests(PROFILES.S);

      const { container, getByTestId } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useV2Workloads />
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

  describe('Profile M baseline', () => {
    it('renders Profile M without crashing and keeps baseline row count', async () => {
      mockLocationSearch = '?type=all';
      mockV2Workloads = makeGuests(PROFILES.M);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useV2Workloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(PROFILES.M);
      });
    });
  });

  describe('Profile L baseline', () => {
    it('renders Profile L without crashing as pre-windowing baseline', async () => {
      mockLocationSearch = '?type=all';
      mockV2Workloads = makeGuests(PROFILES.L);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useV2Workloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(
        () => {
          expect(getGuestRowCount(container)).toBe(PROFILES.L);
        },
        { timeout: 20000 },
      );
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
        mockV2Workloads = profileGuests;

        const { container, unmount } = render(() => (
          <Dashboard vms={[]} containers={[]} nodes={[]} useV2Workloads />
        ));

        await waitFor(() => {
          expect(getGuestRowCount(container)).toBe(expectedByMode[mode]);
        });

        unmount();
      }
    });
  });

  describe.todo('Transform budget placeholder (IWP-02)');
  describe.todo('Windowing budget placeholder (IWP-04)');
});
