import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';

const queryState = vi.hoisted(() => ({
  calls: [] as Array<{ query: string; cacheKey?: string; enabled: boolean }>,
  resources: [] as Resource[],
  error: undefined as unknown,
  refetch: vi.fn<() => Promise<Resource[]>>(),
}));

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ state: { pmg: [] }, activeAlerts: {}, connected: () => true }),
  useDarkMode: () => () => false,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn(async () => ({ capabilities: [], relationships: [], recentChanges: [] })),
  },
}));
vi.mock('@/api/ai', () => ({
  AIAPI: { getResourceIntelligence: vi.fn(async () => null) },
}));
vi.mock('@/api/actionAudit', () => ({
  ActionAuditAPI: {
    listActionAudits: vi.fn(async () => ({ audits: [], count: 0, available: false })),
  },
}));
vi.mock('@/api/resourceOperatorState', () => ({
  getResourceOperatorState: vi.fn(async () => null),
}));

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    enabled?: boolean;
  }) => (
    <div
      data-testid="history-chart"
      data-enabled={String(props.enabled !== false)}
      data-target={props.target?.resourceId ?? ''}
    />
  ),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options: { query: string; cacheKey?: string; enabled?: () => boolean }) => {
    queryState.calls.push({
      query: options.query,
      cacheKey: options.cacheKey,
      enabled: options.enabled ? options.enabled() : true,
    });
    return {
      resources: () => queryState.resources,
      error: () => queryState.error,
      loading: () => false,
      refetch: () => queryState.refetch(),
    };
  },
}));

const makeMachine = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'machine-san-01',
    name: 'debian-san',
    displayName: 'debian-san',
    type: 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_790_982_000_000,
    agent: {
      agentId: 'agent-host-san-01',
      osName: 'Debian GNU/Linux',
      osVersion: '12',
      sensors: {
        smart: [
          {
            device: '/dev/sda',
            model: 'WDC WD80EFZX-68UW8N0',
            serial: 'VK123456',
            type: 'sata',
            temperature: 38,
            health: 'PASSED',
          },
          {
            device: '/dev/nvme0n1',
            model: 'Samsung SSD 980 PRO 1TB',
            serial: 'S5GXNX0R123456',
            type: 'nvme',
            temperature: 44,
            health: 'PASSED',
          },
        ],
      },
    },
    ...overrides,
  }) as Resource;

const makePhysicalDisk = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'disk-san-sda',
    parentId: 'machine-san-01',
    type: 'physical_disk',
    name: 'WDC WD80EFZX-68UW8N0',
    displayName: 'WDC WD80EFZX-68UW8N0',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_790_982_000_000,
    physicalDisk: {
      devPath: '/dev/sda',
      model: 'WDC WD80EFZX-68UW8N0',
      serial: 'VK123456',
      diskType: 'sata',
      sizeBytes: 8_001_563_222_016,
      health: 'PASSED',
      temperature: 38,
      smart: {
        powerOnHours: 14_320,
        powerCycles: 42,
        reallocatedSectors: 0,
        pendingSectors: 0,
        udmaCrcErrors: 1,
      },
    },
    ...overrides,
  }) as Resource;

beforeEach(() => {
  queryState.calls = [];
  queryState.resources = [];
  queryState.error = undefined;
  queryState.refetch.mockReset().mockResolvedValue([]);
});

afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('ResourceDetailDrawer machine physical disks & SMART view', () => {
  it('renders Physical Disks & SMART directly on Overview for standalone machines with SMART telemetry and expands SMART attributes', async () => {
    queryState.resources = [
      makePhysicalDisk(),
      makePhysicalDisk({
        id: 'disk-san-nvme0',
        name: 'Samsung SSD 980 PRO 1TB',
        displayName: 'Samsung SSD 980 PRO 1TB',
        physicalDisk: {
          devPath: '/dev/nvme0n1',
          model: 'Samsung SSD 980 PRO 1TB',
          serial: 'S5GXNX0R123456',
          diskType: 'nvme',
          sizeBytes: 1_000_204_886_016,
          health: 'PASSED',
          temperature: 44,
          wearout: 97,
          smart: {
            powerOnHours: 6_200,
            percentageUsed: 3,
            availableSpare: 100,
          },
        },
      }),
    ];

    render(() => <ResourceDetailDrawer resource={makeMachine()} />);

    const card = await screen.findByTestId('resource-physical-disks');
    expect(within(card).getByText(/Physical Disks & SMART/i)).toBeInTheDocument();
    expect(within(card).getByText(/\(2\)/)).toBeInTheDocument();

    expect(queryState.calls).toContainEqual({
      query: 'type=physical_disk&parent=machine-san-01',
      cacheKey: 'resource-physical-disks:machine-san-01',
      enabled: true,
    });

    const rows = within(card).getAllByTestId('resource-physical-disk') as HTMLDetailsElement[];
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText('WDC WD80EFZX-68UW8N0')).toBeInTheDocument();
    expect(within(rows[0]).getByText('/dev/sda')).toBeInTheDocument();
    expect(within(rows[0]).getByText('38°C')).toBeInTheDocument();

    // Expanding a disk row lazy-loads DiskDrawerOverview with full SMART detail.
    rows[0].open = true;
    fireEvent(rows[0], new Event('toggle'));

    await waitFor(() => {
      expect(within(rows[0]).getByText(/SMART health/i)).toBeInTheDocument();
    });
  }, 15_000);

  it('does not fetch or render physical disks on standalone machines without SMART/RAID/Unraid telemetry', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={makeMachine({
          id: 'machine-vps-01',
          agent: {
            agentId: 'agent-vps-01',
            osName: 'Ubuntu',
            osVersion: '24.04',
          },
        })}
      />
    ));

    await Promise.resolve();
    await Promise.resolve();

    expect(screen.queryByTestId('resource-physical-disks')).toBeNull();
    expect(queryState.calls).toEqual([]);
  });

  it('uses machine wording when a previously shown physical-disk read later recovers empty', async () => {
    queryState.error = new Error('transient backend error');
    render(() => <ResourceDetailDrawer resource={makeMachine()} />);

    const card = await screen.findByTestId('resource-physical-disks');
    expect(within(card).getByText('Physical disk details are unavailable.')).toBeInTheDocument();
    expect(
      within(card).getByRole('button', { name: 'Retry physical disk read' }),
    ).toBeInTheDocument();
  });

  it('skips the agent physical-disk panel for TrueNAS systems that own their own Disks view', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={makeMachine({
          id: 'truenas-01',
          platformId: 'truenas-01',
          platformType: 'truenas',
          sourceType: 'api',
        })}
      />
    ));

    await Promise.resolve();
    await Promise.resolve();

    expect(screen.queryByTestId('resource-physical-disks')).toBeNull();
    expect(queryState.calls).toEqual([]);
  });
});
