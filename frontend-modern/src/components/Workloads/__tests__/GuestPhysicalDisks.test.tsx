import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal, type JSX } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
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
  loading: false,
  refetch: vi.fn(),
  query: '',
  enabled: false,
  realtimeEnabled: undefined as (() => boolean) | undefined,
}));

// The expanded disk detail now shows its history charts in view; the chart
// measures itself with ResizeObserver, which jsdom lacks. Its rendering is
// covered by the HistoryChart and DiskDetail suites.
vi.mock('@/components/shared/HistoryChart', () => ({
  HistoryChartHoverGroup: (props: { children: JSX.Element }) => <div>{props.children}</div>,
  HistoryChart: (props: { label: string; enabled?: boolean }) => (
    <div data-testid="history-chart" data-enabled={String(props.enabled !== false)}>
      {props.label}
    </div>
  ),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options: {
    query: string;
    enabled: () => boolean;
    realtimeEnabled?: () => boolean;
    reportBackgroundErrors?: boolean;
    readTimeoutMs?: number;
  }) => {
    expect(options.reportBackgroundErrors).toBe(true);
    expect(options.readTimeoutMs).toBe(20_000);
    queryState.query = options.query;
    queryState.enabled = options.enabled();
    queryState.realtimeEnabled = options.realtimeEnabled;
    return {
      resources: () => (queryState.accessor ? queryState.accessor() : queryState.resources),
      error: () => queryState.errorAccessor?.() ?? queryState.error,
      loading: () => queryState.loading,
      refetch: queryState.refetch,
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
  // The first expansion lazy-loads the Storage disk detail; a cold transform of
  // that module can outlast findByText's default wait on a loaded machine.
  beforeAll(async () => {
    await import('@/components/Storage/DiskDetail');
  }, 30_000);

  beforeEach(() => {
    queryState.resources = [];
    queryState.accessor = undefined;
    queryState.error = undefined;
    queryState.errorAccessor = undefined;
    queryState.loading = false;
    queryState.refetch.mockReset();
    queryState.query = '';
    queryState.enabled = false;
    queryState.realtimeEnabled = undefined;
  });
  afterEach(cleanup);

  it('shows the linked guest disk and loads its existing SMART detail on expansion', async () => {
    queryState.resources = [disk, { ...disk, id: 'host-disk', parentId: 'pve-node-1' }];
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    expect(queryState.query).toBe('type=physical_disk&parent=vm-resource-101');
    expect(queryState.enabled).toBe(true);
    const card = screen.getByTestId('guest-physical-disks');
    expect(within(card).getByText('Physical Disks & SMART (1)')).toBeInTheDocument();
    expect(within(card).getByText('Guest SATA')).toBeInTheDocument();
    expect(within(card).getByText('Healthy')).toBeInTheDocument();
    expect(within(card).getByText('45°C')).toBeInTheDocument();
    expect(within(card).queryByText('Reallocated Sectors')).toBeNull();

    const row = within(card).getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));
    // The SMART card and the disk's Reallocated Sectors history chart both
    // name the attribute once the detail is open.
    expect((await within(card).findAllByText('Reallocated Sectors')).length).toBeGreaterThan(0);
    expect(within(card).getByText('Pending Sectors')).toBeInTheDocument();
    expect(within(card).getByText('CRC Errors')).toBeInTheDocument();
    expect(within(card).getAllByTestId('guest-physical-disk')).toHaveLength(1);
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

  it('keeps an expired successful inventory report qualified in the row and open SMART view', async () => {
    // The actual resource owner reconciles by canonical ID, rather than
    // replacing the disk owner (which would correctly close a different row).
    const [state, setState] = createStore<{ resources: Resource[] }>({
      resources: [structuredClone(disk)],
    });
    queryState.accessor = () => state.resources;
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const row = screen.getByTestId('guest-physical-disk') as HTMLDetailsElement;
    row.open = true;
    fireEvent(row, new Event('toggle'));
    await screen.findByTestId('disk-detail-health');
    setState(
      'resources',
      reconcile(
        [{ ...disk, health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale' }] } }],
        { key: 'id' },
      ),
    );
    await waitFor(() => expect(screen.getAllByText('Last known')).toHaveLength(2));
    expect(row.open).toBe(true);
    expect(within(row).queryByText('Healthy')).toBeNull();
    expect(screen.getByTestId('disk-detail-health')).toHaveTextContent(
      'current disk health is unconfirmed',
    );
    expect(row.querySelector('[data-temperature-reading]')).toHaveAttribute(
      'data-temperature-reading',
      'last-known',
    );
    expect(row.querySelector('[data-temperature-reading]')).toHaveTextContent('45°C, last known');
    setState(
      'resources',
      reconcile([{ ...disk, health: { verdict: 'ok', reasons: [] } }], { key: 'id' }),
    );
    await waitFor(() => expect(screen.getAllByText('Healthy')).toHaveLength(2));
    expect(row.open).toBe(true);
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

  it.each([401, 403, 503])(
    'withdraws previously healthy disk and SMART details after HTTP %s',
    async (status) => {
      const [error, setError] = createSignal<unknown>();
      queryState.errorAccessor = error;
      queryState.resources = [disk];
      render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
      const row = screen.getByTestId('guest-physical-disk') as HTMLDetailsElement;
      row.open = true;
      fireEvent(row, new Event('toggle'));
      await screen.findAllByText('Reallocated Sectors');
      expect(screen.getAllByText('Healthy').length).toBeGreaterThan(0);

      setError(Object.assign(new Error('PRIVATE_PROVIDER_TEXT'), { status }));
      expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
      expect(screen.queryByText('45°C')).toBeNull();
      expect(screen.queryByTestId('history-chart')).toBeNull();
      expect(screen.getByRole('status')).toHaveTextContent(
        'Physical disk details are unavailable.',
      );
      expect(screen.getByRole('status')).toHaveTextContent('not a guest probe');
      expect(screen.queryByText(/PRIVATE_PROVIDER_TEXT/)).toBeNull();
      expect(screen.getByRole('button', { name: 'Retry physical disk details' })).toBeEnabled();
      expect(queryState.refetch).not.toHaveBeenCalled();
    },
  );

  it('keeps failed readings withdrawn throughout a single explicit retry, then renders its new result collapsed', async () => {
    const [error, setError] = createSignal<unknown>(new Error('unavailable'));
    const [resources, setResources] = createSignal<Resource[]>([disk]);
    queryState.errorAccessor = error;
    queryState.accessor = resources;
    let finish!: () => void;
    queryState.refetch.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const button = screen.getByRole('button', { name: 'Retry physical disk details' });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(queryState.refetch).toHaveBeenCalledTimes(1);
    expect(button).toBeDisabled();
    expect(button).toHaveTextContent('Retrying');
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();

    setResources([{ ...disk, physicalDisk: { ...disk.physicalDisk!, temperature: 48 } }]);
    setError(undefined);
    finish();
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
    expect(screen.getByText('48°C')).toBeInTheDocument();
    expect(screen.queryByText('45°C')).toBeNull();
    expect((screen.getByTestId('guest-physical-disk') as HTMLDetailsElement).open).toBe(false);
    expect(screen.queryByTestId('history-chart')).toBeNull();
  });

  it('contains a rejected retry without exposing its error or losing the next explicit attempt', async () => {
    queryState.resources = [disk];
    queryState.error = new Error('unavailable');
    queryState.refetch.mockRejectedValue(new Error('PRIVATE_RETRY_PAYLOAD'));
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const button = screen.getByRole('button', { name: 'Retry physical disk details' });
    fireEvent.click(button);
    await waitFor(() => expect(button).toBeEnabled());
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
    expect(screen.queryByText(/PRIVATE_RETRY_PAYLOAD/)).toBeNull();
    fireEvent.click(button);
    await waitFor(() => expect(queryState.refetch).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(button).toBeEnabled());
  });

  it('does not turn a successful empty retry into unavailable disks or a healthy phantom disk', async () => {
    const [error, setError] = createSignal<unknown>(new Error('unavailable'));
    const [resources, setResources] = createSignal<Resource[]>([disk]);
    queryState.errorAccessor = error;
    queryState.accessor = resources;
    queryState.refetch.mockImplementation(async () => {
      setResources([]);
      setError(undefined);
    });
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    fireEvent.click(screen.getByRole('button', { name: 'Retry physical disk details' }));
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
    expect(screen.queryByTestId('guest-physical-disks')).toBeNull();
    expect(screen.queryByText('Healthy')).toBeNull();
    expect(queryState.refetch).toHaveBeenCalledTimes(1);
  });

  it('does not let an unanswered old-organisation retry disable or settle a new-organisation retry', async () => {
    queryState.error = new Error('unavailable');
    let finishOld!: () => void;
    let finishNew!: () => void;
    queryState.refetch
      .mockImplementationOnce(
        () =>
          new Promise<void>((resolve) => {
            finishOld = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise<void>((resolve) => {
            finishNew = resolve;
          }),
      );
    render(() => <GuestPhysicalDisks parentId="vm-resource-101" />);
    const button = screen.getByRole('button', { name: 'Retry physical disk details' });
    fireEvent.click(button);
    expect(button).toBeDisabled();
    eventBus.emit('org_switched', 'other-org');
    expect(button).toBeEnabled();
    fireEvent.click(button);
    expect(queryState.refetch).toHaveBeenCalledTimes(2);
    expect(button).toBeDisabled();
    finishOld();
    await Promise.resolve();
    await Promise.resolve();
    expect(button).toBeDisabled();
    finishNew();
    await waitFor(() => expect(button).toBeEnabled());
  });
});
