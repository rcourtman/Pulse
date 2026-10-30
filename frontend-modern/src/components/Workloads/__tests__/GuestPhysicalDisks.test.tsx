import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { batch, createSignal } from 'solid-js';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { AlertsAPI } from '@/api/alerts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { eventBus } from '@/stores/events';
import type { AlertConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';
import { GuestPhysicalDisks } from '../GuestPhysicalDisks';

const queryState = vi.hoisted(() => ({
  resources: [] as Resource[],
  // Set to a signal accessor to drive live resource updates.
  accessor: undefined as (() => Resource[]) | undefined,
  error: undefined as unknown,
  errorAccessor: undefined as (() => unknown) | undefined,
  loading: () => false as boolean,
  refetch: vi.fn<() => Promise<Resource[]>>(),
  query: '',
  enabled: false,
  retainReadError: false,
  realtimeEnabled: undefined as (() => boolean) | undefined,
}));

// An opened disk shows the Storage disk drawer's Overview, whose charts read
// metrics history. Their rendering is covered by the GuestDrawerHistory and
// DiskDrawer suites; here they only report what they were asked to chart.
vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    enabled?: boolean;
    groups?: Array<{ label: string }>;
  }) => (
    <div
      data-testid="history-chart"
      data-enabled={String(props.enabled !== false)}
      data-range={props.range}
      data-target={props.target?.resourceId ?? ''}
    >
      {(props.groups ?? []).map((group) => group.label).join(', ')}
    </div>
  ),
}));

const activeAlertsMock = vi.hoisted(() => ({}) as Record<string, unknown>);
vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: activeAlertsMock }),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options: {
    query: string;
    enabled: () => boolean;
    realtimeEnabled?: () => boolean;
    retainReadError?: boolean;
  }) => {
    queryState.query = options.query;
    queryState.enabled = options.enabled();
    queryState.retainReadError = options.retainReadError === true;
    queryState.realtimeEnabled = options.realtimeEnabled;
    return {
      resources: () => (queryState.accessor ? queryState.accessor() : queryState.resources),
      error: () => queryState.errorAccessor?.() ?? queryState.error,
      loading: () => queryState.loading(),
      refetch: () => queryState.refetch(),
    };
  },
}));

const disk: Resource = {
  id: 'disk-101-a',
  parentId: 'vm-resource-101',
  type: 'physical_disk',
  name: 'disk-101-a',
  displayName: 'Guest SATA',
  platformId: 'cluster-a',
  platformType: 'proxmox-pve',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  physicalDisk: {
    devPath: '/dev/sda',
    model: 'Guest SATA',
    diskType: 'sata',
    sizeBytes: 1_000_000_000,
    health: 'PASSED',
    temperature: 45,
    smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 2 },
  },
};

describe('GuestPhysicalDisks', () => {
  // The first expansion lazy-loads the Storage disk Overview; a cold transform
  // of that module can outlast findByText's default wait on a loaded machine.
  beforeAll(async () => {
    await import('@/components/Storage/DiskDrawerOverview');
  }, 30_000);

  beforeEach(() => {
    queryState.resources = [];
    queryState.accessor = undefined;
    queryState.error = undefined;
    queryState.errorAccessor = undefined;
    queryState.loading = () => false;
    queryState.refetch.mockReset().mockResolvedValue([]);
    queryState.query = '';
    queryState.enabled = false;
    queryState.retainReadError = false;
    queryState.realtimeEnabled = undefined;
  });
  afterEach(() => {
    cleanup();
    for (const id of Object.keys(activeAlertsMock)) delete activeAlertsMock[id];
  });

  it('shows the linked guest disk and loads its existing SMART detail on expansion', async () => {
    queryState.resources = [disk, { ...disk, id: 'host-disk', parentId: 'pve-node-1' }];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    expect(queryState.query).toBe('type=physical_disk&parent=vm-resource-101');
    expect(queryState.enabled).toBe(true);
    expect(queryState.retainReadError).toBe(true);
    const card = screen.getByTestId('guest-physical-disks');
    expect(within(card).getByText('Physical Disks & SMART (1)')).toBeInTheDocument();
    expect(within(card).getByText('Guest SATA')).toBeInTheDocument();
    expect(within(card).getByText('Healthy')).toBeInTheDocument();
    expect(within(card).getByText('45°C')).toBeInTheDocument();
    expect(within(card).queryByText('Reallocated Sectors')).toBeNull();

    const row = within(card).getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));
    // The same Overview the Storage tab's disk drawer shows: alerts line,
    // charts, facts, SMART health card, then every attribute under Details.
    const overview = await within(card).findByTestId('disk-drawer-overview');
    expect(within(overview).getByTestId('drawer-no-attention')).toHaveTextContent(
      'No active alerts',
    );
    expect(await within(overview).findByTestId('history-chart')).toHaveAttribute(
      'data-range',
      '24h',
    );
    expect(within(overview).getByTestId('disk-drawer-health-card')).toBeInTheDocument();
    expect(within(overview).getByText('Reallocated Sectors')).toBeInTheDocument();
    expect(within(overview).getByText('Pending Sectors')).toBeInTheDocument();
    expect(within(overview).getByText('CRC Errors')).toBeInTheDocument();
    expect(within(card).getAllByTestId('guest-physical-disk')).toHaveLength(1);
  });

  it("lists the disk's own open alerts in its Overview", async () => {
    activeAlertsMock['disk-alert'] = {
      id: 'disk-alert',
      type: 'disk-health',
      level: 'critical',
      resourceId: 'disk-101-a',
      resourceName: 'Guest SATA',
      node: 'pve-node-1',
      instance: 'cluster-a',
      message: 'SMART health check failed',
      value: 0,
      threshold: 0,
      startTime: '2026-10-20T10:00:00Z',
      acknowledged: false,
    };
    activeAlertsMock['other-disk-alert'] = {
      ...(activeAlertsMock['disk-alert'] as object),
      id: 'other-disk-alert',
      resourceId: 'disk-202-b',
      message: 'Another disk failed',
    };
    queryState.resources = [disk];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const row = screen.getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));

    const overview = await screen.findByTestId('disk-drawer-overview');
    const attention = within(overview).getByTestId('drawer-attention-section');
    expect(attention).toHaveTextContent('SMART health check failed');
    expect(attention).not.toHaveTextContent('Another disk failed');
    expect(within(overview).queryByTestId('drawer-no-attention')).toBeNull();
  });

  it('pauses an expanded disk and the live list while its Overview is hidden', async () => {
    queryState.resources = [disk];
    const [shown, setShown] = createSignal(true);
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" historyEnabled={shown()} />);
    const row = screen.getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));
    await waitFor(() => expect(screen.getAllByTestId('history-chart').length).toBeGreaterThan(0));
    const enabledStates = () =>
      screen.getAllByTestId('history-chart').map((chart) => chart.dataset.enabled);
    expect(new Set(enabledStates())).toEqual(new Set(['true']));
    expect(queryState.realtimeEnabled?.()).toBe(true);
    setShown(false);
    expect(new Set(enabledStates())).toEqual(new Set(['false']));
    expect(queryState.realtimeEnabled?.()).toBe(false);
    // The initial read is not withheld: only background refresh pauses.
    expect(queryState.enabled).toBe(true);
    setShown(true);
    expect(new Set(enabledStates())).toEqual(new Set(['true']));
    expect(queryState.realtimeEnabled?.()).toBe(true);
  });

  it("judges the guest's disks by its own agent's Disk Temp override", async () => {
    const getConfig = vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue({
      enabled: true,
      activationState: 'active',
      agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
      diskTempByType: { sata: { trigger: 55, clear: 50 } },
      overrides: { 'host-guest-101': { diskTemperature: { trigger: 65, clear: 60 } } },
    } as unknown as AlertConfig);
    try {
      queryState.resources = [
        { ...disk, physicalDisk: { ...disk.physicalDisk!, temperature: 58 } } as Resource,
      ];
      await useAlertsActivation().refreshConfig();
      const guestCard = render(() => (
        <GuestPhysicalDisks parentId="vm-resource-101" alertResourceIds={['host-guest-101']} />
      ));
      // 58C passes the 55C SATA trigger but not the agent's 65C override.
      await waitFor(() =>
        expect(within(guestCard.container).getByText('Healthy')).toBeInTheDocument(),
      );
      guestCard.unmount();

      const unmatched = render(() => (
        <GuestPhysicalDisks parentId="vm-resource-101" alertResourceIds={['host-other']} />
      ));
      expect(within(unmatched.container).getByText('Running Hot')).toBeInTheDocument();
    } finally {
      getConfig.mockRestore();
      eventBus.emit('org_switched', 'default');
    }
  });

  it('does not add an empty storage card to an uninstrumented guest', () => {
    render(() => <GuestPhysicalDisks parentId="vm-resource-102" />);
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
  });

  it('encodes the parent value and never widens an empty parent to all disks', () => {
    render(() => <GuestPhysicalDisks parentId="vm&101" />);
    expect(queryState.query).toBe('type=physical_disk&parent=vm%26101');
    cleanup();
    render(() => <GuestPhysicalDisks parentId=" " />);
    expect(queryState.enabled).toBe(false);
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
  });

  it('marks a retained disk temperature as last known', () => {
    queryState.resources = [
      disk,
      {
        ...disk,
        id: 'disk-101-b',
        displayName: 'Guest NVMe',
        physicalDisk: {
          ...disk.physicalDisk!,
          devPath: '/dev/nvme0',
          model: 'Guest NVMe',
          temperature: 61,
          collection: {
            temperature: {
              state: 'unavailable',
              source: 'host_agent',
              reason: 'host agent stopped reporting',
            },
          },
        },
      },
    ];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);

    const [currentRow, retainedRow] = screen.getAllByTestId('guest-physical-disk');
    const current = within(currentRow).getByText('45°C');
    expect(current).toHaveAttribute('data-temperature-reading', 'current');
    expect(current).not.toHaveAttribute('title');

    const retained = retainedRow.querySelector('[data-temperature-reading="last-known"]');
    expect(retained).not.toBeNull();
    expect(retained).toHaveTextContent('61°C, last known');
    expect(retained).toHaveClass('underline', 'decoration-dotted', 'text-muted');
    expect(retained).toHaveAttribute(
      'title',
      'Last known reading, not current: host agent stopped reporting',
    );
  });

  it('follows a disk between current and last known as its collection state changes', () => {
    const withReading = (
      temperature: number,
      collection?: NonNullable<Resource['physicalDisk']>['collection'],
    ): Resource => ({
      ...disk,
      physicalDisk: { ...disk.physicalDisk!, temperature, collection },
    });
    const [resources, setResources] = createSignal<Resource[]>([disk]);
    queryState.accessor = resources;
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const reading = () =>
      screen.getByTestId('guest-physical-disk').querySelector('[data-temperature-reading]');

    expect(reading()).toHaveAttribute('data-temperature-reading', 'current');

    setResources([
      withReading(61, {
        temperature: {
          state: 'unavailable',
          source: 'host_agent',
          reason: 'host agent stopped reporting',
        },
      }),
    ]);
    expect(reading()).toHaveAttribute('data-temperature-reading', 'last-known');
    expect(reading()).toHaveTextContent('61°C, last known');
    expect(reading()).toHaveClass('decoration-dotted');

    setResources([withReading(47, { temperature: { state: 'available', source: 'smartctl' } })]);
    expect(reading()).toHaveAttribute('data-temperature-reading', 'current');
    expect(reading()).toHaveTextContent('47°C');
    expect(reading()).not.toHaveTextContent('last known');
    expect(reading()).not.toHaveAttribute('title');
    expect(reading()).not.toHaveClass('decoration-dotted');

    setResources([withReading(0)]);
    expect(reading()).toBeNull();
  });

  it('makes a failed child query visible rather than silently implying no disks', () => {
    queryState.error = new Error('unavailable');
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    expect(screen.getByRole('status')).toHaveTextContent('Physical disk details are unavailable.');
  });

  it('withholds cached health, temperatures and open charts after a scoped read fails', async () => {
    const [error, setError] = createSignal<unknown>();
    queryState.errorAccessor = error;
    queryState.resources = [disk];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const row = screen.getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));
    await screen.findByTestId('history-chart');
    row.querySelector('summary')!.focus();

    setError(new Error('private provider response must not render'));
    expect(screen.getByRole('status')).toHaveTextContent('Physical disk details are unavailable.');
    expect(screen.getByRole('status')).toHaveTextContent(
      'This does not establish that disks are absent or healthy.',
    );
    expect(screen.queryByTestId('guest-physical-disk')).toBeNull();
    expect(screen.queryByTestId('disk-drawer-overview')).toBeNull();
    expect(screen.queryByTestId('history-chart')).toBeNull();
    expect(screen.queryByText('Healthy')).toBeNull();
    expect(screen.queryByText('45°C')).toBeNull();
    expect(screen.queryByText('private provider response must not render')).toBeNull();
    expect(screen.getByRole('heading', { name: 'Physical Disks & SMART' })).not.toHaveTextContent(
      '(0)',
    );
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Physical Disks & SMART' })).toHaveFocus(),
    );
  });

  it('keeps a pending/failed retry focusable and restores only the successful scoped read', async () => {
    const [error, setError] = createSignal<unknown>(new Error('unavailable'));
    const [loading, setLoading] = createSignal(false);
    const [resources, setResources] = createSignal([disk]);
    queryState.errorAccessor = error;
    queryState.loading = loading;
    queryState.accessor = resources;
    let settle!: () => void;
    queryState.refetch.mockImplementation(() => {
      setLoading(true);
      return new Promise<Resource[]>((_resolve, reject) => {
        settle = () => {
          setLoading(false);
          reject(new Error('unavailable'));
        };
      });
    });
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const retry = screen.getByRole('button', { name: 'Retry physical disk read' });
    retry.focus();
    fireEvent.click(retry);
    expect(retry).toHaveTextContent('Retrying…');
    expect(retry).toHaveAttribute('aria-disabled', 'true');
    expect(retry).not.toBeDisabled();
    fireEvent.click(retry);
    expect(queryState.refetch).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId('guest-physical-disk')).toBeNull();
    settle();
    await waitFor(() => expect(retry).toHaveTextContent('Retry physical disk read'));
    expect(retry).toHaveFocus();
    expect(retry).toHaveAttribute('aria-disabled', 'false');

    queryState.refetch.mockImplementationOnce(async () => {
      batch(() => {
        setResources([{ ...disk, physicalDisk: { ...disk.physicalDisk!, temperature: 47 } }]);
        setError(undefined);
      });
      return resources();
    });
    fireEvent.click(retry);
    await waitFor(() => expect(screen.getByText('47°C')).toBeInTheDocument());
    expect(queryState.refetch).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole('button', { name: 'Retry physical disk read' })).toBeNull();
    expect(screen.queryByText('45°C')).toBeNull();
    expect(screen.getByTestId('guest-physical-disk')).not.toHaveAttribute('open');
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Physical Disks & SMART (1)' })).toHaveFocus(),
    );
  });

  it('distinguishes a successful empty retry from both failed reads and physical disk absence', async () => {
    const [error, setError] = createSignal<unknown>(new Error('unavailable'));
    const [resources, setResources] = createSignal([disk]);
    queryState.errorAccessor = error;
    queryState.accessor = resources;
    queryState.refetch.mockImplementationOnce(async () => {
      batch(() => {
        setResources([]);
        setError(undefined);
      });
      return [];
    });
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const retry = screen.getByRole('button', { name: 'Retry physical disk read' });
    retry.focus();
    fireEvent.click(retry);
    expect(screen.getByRole('status')).toHaveTextContent(
      'No physical disk records were returned for this guest. This does not mean the guest has no disks.',
    );
    expect(screen.queryByText('Physical disk details are unavailable.')).toBeNull();
    expect(screen.queryByTestId('guest-physical-disk')).toBeNull();
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Physical Disks & SMART (0)' })).toHaveFocus(),
    );
  });

  it.each(['elsewhere', 'hidden', 'disposed'] as const)(
    'does not reclaim focus on recovery when the inspection is %s',
    async (scenario) => {
      const [error, setError] = createSignal<unknown>(new Error('unavailable'));
      const [shown, setShown] = createSignal(true);
      queryState.errorAccessor = error;
      queryState.resources = [disk];
      const rendered = render(() => (
        <>
          <button type="button">Outside inspection</button>
          <GuestPhysicalDisks parentId="vm-resource-101" historyEnabled={shown()} />
        </>
      ));
      const retry = screen.getByRole('button', { name: 'Retry physical disk read' });
      retry.focus();
      if (scenario === 'elsewhere')
        screen.getByRole('button', { name: 'Outside inspection' }).focus();
      if (scenario === 'hidden') setShown(false);
      if (scenario === 'disposed') rendered.unmount();
      const active = document.activeElement;
      setError(undefined);
      await Promise.resolve();
      expect(document.activeElement).toBe(active);
    },
  );

  it('keeps admitted rows during an ordinary pending refresh and withdraws valid empty results', () => {
    const [loading, setLoading] = createSignal(false);
    const [resources, setResources] = createSignal([disk]);
    queryState.loading = loading;
    queryState.accessor = resources;
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const summary = screen.getByTestId('guest-physical-disk').querySelector('summary');
    setLoading(true);
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(screen.getByTestId('guest-physical-disk').querySelector('summary')).toBe(summary);
    setLoading(false);
    setResources([]);
    expect(screen.queryByTestId('guest-physical-disk')).toBeNull();
    expect(screen.getByRole('status')).toHaveTextContent('No physical disk records were returned');
  });
});
