import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AvailabilityTargetsAPI, type AvailabilityTarget } from '@/api/availabilityTargets';
import { AvailabilitySettingsPanel } from '../AvailabilitySettingsPanel';

const routeState = vi.hoisted(() => ({
  pathname: '/settings/monitoring/availability',
  search: '',
}));
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => routeState,
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/api/availabilityTargets', () => ({
  AvailabilityTargetsAPI: {
    create: vi.fn(),
    list: vi.fn(),
    remove: vi.fn(),
    test: vi.fn(),
    testSaved: vi.fn(),
    update: vi.fn(),
  },
}));

const resourceMocks = vi.hoisted(() => ({
  resources: [] as Array<Record<string, unknown>>,
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => resourceMocks.resources,
  }),
}));

vi.mock('../ConnectionEditor/CredentialSlots/AvailabilityTargetSlot', () => ({
  AvailabilityTargetSlot: (props: {
    editingTargetId?: string | null;
    initialTargetKind?: string;
    initialName?: string;
    initialAddress?: string;
    onSaved: () => void;
  }) => (
    <div
      data-testid="availability-target-slot"
      data-editing-target-id={props.editingTargetId ?? ''}
      data-initial-target-kind={props.initialTargetKind ?? ''}
      data-initial-name={props.initialName ?? ''}
      data-initial-address={props.initialAddress ?? ''}
    >
      availability target slot
      <button type="button" onClick={props.onSaved}>
        Mock save
      </button>
    </div>
  ),
}));

const targets: AvailabilityTarget[] = [
  {
    id: 'mqtt-broker',
    name: 'MQTT broker',
    address: 'mqtt.local',
    protocol: 'tcp',
    port: 1883,
    enabled: true,
    status: {
      targetId: 'mqtt-broker',
      name: 'MQTT broker',
      address: 'mqtt.local',
      protocol: 'tcp',
      enabled: true,
      available: true,
      latencyMillis: 8,
    },
  },
  {
    id: 'http-health',
    name: 'HTTP health',
    address: 'http://service.local',
    protocol: 'http',
    path: '/health',
    enabled: false,
  },
];

describe('AvailabilitySettingsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resourceMocks.resources = [];
    routeState.pathname = '/settings/monitoring/availability';
    routeState.search = '';
    vi.mocked(AvailabilityTargetsAPI.list).mockResolvedValue(targets);
    vi.mocked(AvailabilityTargetsAPI.update).mockResolvedValue(targets[0]);
    vi.mocked(AvailabilityTargetsAPI.testSaved).mockResolvedValue({
      success: true,
      latencyMillis: 8,
    });
  });

  afterEach(() => cleanup());

  it('mounts a bounded window of a large target inventory', async () => {
    vi.mocked(AvailabilityTargetsAPI.list).mockResolvedValue(
      Array.from({ length: 80 }, (_, index) => ({
        id: `target-${index}`,
        name: `Target ${index}`,
        address: `host-${index}.local`,
        protocol: 'tcp' as const,
        port: 1000 + index,
        enabled: true,
      })),
    );

    const { container } = render(() => <AvailabilitySettingsPanel />);

    const rows = await screen.findAllByText(/^Target \d+$/);
    expect(rows.length).toBeLessThan(80);
    expect(container.querySelectorAll('[data-platform-window-spacer]')).toHaveLength(2);
  });

  it('lists monitor-only MQTT and HTTP endpoint checks in the monitoring home', async () => {
    render(() => <AvailabilitySettingsPanel />);

    await waitFor(() => expect(screen.getByText('MQTT broker')).toBeInTheDocument());
    expect(screen.getByText('HTTP health')).toBeInTheDocument();
    expect(screen.getByText('TCP 1883')).toBeInTheDocument();
    expect(screen.getByText('http://service.local/health')).toBeInTheDocument();
    expect(screen.getByText('Online · 8 ms')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Test' })[0]).toHaveClass(
      'min-h-11',
      'sm:min-h-8',
    );
    expect(screen.getAllByRole('button', { name: 'Pause' })[0]).toHaveClass(
      'min-h-11',
      'sm:min-h-8',
    );
    expect(screen.getAllByRole('button', { name: 'Manage' })[0]).toHaveClass(
      'min-h-11',
      'sm:min-h-8',
    );
  });

  it('attributes probe-reported checks to the agent host that ran them', async () => {
    resourceMocks.resources = [
      {
        id: 'agent:edge-01',
        type: 'agent',
        name: 'edge-01',
        displayName: 'Edge 01',
        platformId: 'edge-01',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        status: 'online',
        lastSeen: 1_700_000_000_000,
        agent: { agentId: 'host-edge-01' },
      },
    ];
    vi.mocked(AvailabilityTargetsAPI.list).mockResolvedValue([
      {
        ...targets[0],
        probeAgentId: 'host-edge-01',
        status: { ...targets[0].status!, probeAgentId: 'host-edge-01' },
      },
      {
        ...targets[0],
        id: 'stale-probe',
        name: 'Stale probe',
        probeAgentId: 'host-gone',
        status: {
          ...targets[0].status!,
          targetId: 'stale-probe',
          available: false,
          outcome: 'indeterminate',
          lastError: 'no recent report from probe agent',
          probeAgentId: 'host-gone',
        },
      },
    ]);

    render(() => <AvailabilitySettingsPanel />);

    await waitFor(() => expect(screen.getByText('via Edge 01')).toBeInTheDocument());
    // Unknown host ids still get attributed, by raw id.
    expect(screen.getByText('via host-gone')).toBeInTheDocument();
    // Stale probe reports reuse the existing indeterminate warning treatment.
    expect(screen.getByText('No recent probe report')).toBeInTheDocument();
  });

  it('counts a check as offline only after its failure threshold, like Machines', async () => {
    vi.mocked(AvailabilityTargetsAPI.list).mockResolvedValue([
      {
        ...targets[0],
        id: 'solar',
        name: 'Solar inverter',
        failureThreshold: 2,
        status: {
          ...targets[0].status!,
          targetId: 'solar',
          available: false,
          lastChecked: '2026-10-05T00:00:00Z',
          consecutiveFailures: 1,
          failureThreshold: 2,
          lastError: 'http probe returned 503 Service Unavailable',
        },
      },
      {
        ...targets[0],
        id: 'door',
        name: 'Door controller',
        failureThreshold: 2,
        status: {
          ...targets[0].status!,
          targetId: 'door',
          available: false,
          lastChecked: '2026-10-05T00:00:00Z',
          consecutiveFailures: 3,
          failureThreshold: 2,
          lastError: 'icmp probe timed out',
        },
      },
    ]);

    render(() => <AvailabilitySettingsPanel />);

    await waitFor(() =>
      expect(screen.getByText('2 need attention · 1 offline · 2 enabled')).toBeInTheDocument(),
    );
    const failing = screen.getByText('http probe returned 503 Service Unavailable');
    expect(failing.className).toContain('bg-amber-100');
    expect(failing).toHaveAttribute(
      'title',
      '1 failed check in a row. It counts as offline after 2.',
    );
    expect(screen.getByText('icmp probe timed out').className).toContain('bg-rose-100');
  });

  it('opens add and edit dialogs from the canonical availability route', async () => {
    render(() => <AvailabilitySettingsPanel />);

    await waitFor(() => expect(screen.getByText('MQTT broker')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /^Add service\/device check$/i }));
    expect(navigateSpy).toHaveBeenLastCalledWith(
      '/settings/monitoring/availability?add=target&targetKind=service',
      {
        scroll: false,
      },
    );

    routeState.search = '?add=target&targetKind=service';
    cleanup();
    render(() => <AvailabilitySettingsPanel />);
    await waitFor(() => expect(screen.getByTestId('availability-target-slot')).toBeInTheDocument());
    expect(screen.getByRole('heading', { name: 'Add service/device check' })).toBeInTheDocument();
    expect(screen.getByTestId('availability-target-slot')).toHaveAttribute(
      'data-initial-target-kind',
      'service',
    );

    // A link from another page can name the thing to check.
    expect(screen.getByTestId('availability-target-slot')).toHaveAttribute(
      'data-initial-address',
      '',
    );
    cleanup();
    routeState.search = '?add=target&targetKind=device&name=Gateway&address=192.168.0.1';
    render(() => <AvailabilitySettingsPanel />);
    await waitFor(() => expect(screen.getByTestId('availability-target-slot')).toBeInTheDocument());
    const prefilled = screen.getByTestId('availability-target-slot');
    expect(prefilled).toHaveAttribute('data-initial-target-kind', 'device');
    expect(prefilled).toHaveAttribute('data-initial-name', 'Gateway');
    expect(prefilled).toHaveAttribute('data-initial-address', '192.168.0.1');

    // The map's link: a router, and closing the form goes back to the map.
    cleanup();
    navigateSpy.mockClear();
    routeState.search = '?add=target&targetKind=router&name=Gateway&address=192.168.0.1&from=map';
    render(() => <AvailabilitySettingsPanel />);
    await waitFor(() => expect(screen.getByTestId('availability-target-slot')).toBeInTheDocument());
    expect(screen.getByRole('heading', { name: 'Add router check' })).toBeInTheDocument();
    expect(screen.getByText(/draws it on the map as the device it is/)).toBeInTheDocument();
    expect(screen.getByTestId('availability-target-slot')).toHaveAttribute(
      'data-initial-target-kind',
      'router',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Mock save' }));
    expect(navigateSpy).toHaveBeenLastCalledWith('/map', { replace: true });

    cleanup();
    routeState.search = '?add=target&targetKind=machine';
    render(() => <AvailabilitySettingsPanel />);
    await waitFor(() => expect(screen.getByTestId('availability-target-slot')).toBeInTheDocument());
    expect(screen.getByRole('heading', { name: 'Add machine check' })).toBeInTheDocument();
    expect(screen.getByTestId('availability-target-slot')).toHaveAttribute(
      'data-initial-target-kind',
      'machine',
    );

    cleanup();
    routeState.search = '';
    render(() => <AvailabilitySettingsPanel />);
    await waitFor(() => expect(screen.getByText('MQTT broker')).toBeInTheDocument());
    fireEvent.click(screen.getAllByRole('button', { name: /^Manage$/i })[1]);
    expect(screen.getByTestId('availability-target-slot')).toHaveAttribute(
      'data-editing-target-id',
      'mqtt-broker',
    );
  });

  it('keeps loading and empty states mutually exclusive and renders a single Add button when empty', async () => {
    let resolveList!: (value: AvailabilityTarget[]) => void;
    vi.mocked(AvailabilityTargetsAPI.list).mockReturnValue(
      new Promise<AvailabilityTarget[]>((resolve) => {
        resolveList = resolve;
      }),
    );

    render(() => <AvailabilitySettingsPanel />);
    expect(screen.getByText('Loading availability checks...')).toBeInTheDocument();
    expect(screen.queryByText('No availability checks yet')).not.toBeInTheDocument();

    resolveList([]);
    expect(await screen.findByText('No availability checks yet')).toBeInTheDocument();
    expect(screen.queryByText('Loading availability checks...')).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /^Add service\/device check$/i })).toHaveLength(1);
  });
});

it('ages loaded availability counts, copy and colour without another target read', async () => {
  const start = Date.parse('2026-10-10T12:00:00Z');
  vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
  const availability = {
    targetId: 'clock-check',
    name: 'Clock check',
    protocol: 'tcp',
    address: 'clock.example.test',
    enabled: true,
    available: true,
    lastChecked: new Date(start).toISOString(),
    pollIntervalSeconds: 30,
    evidence: { validUntil: new Date(start + 10_000).toISOString() },
  };
  resourceMocks.resources = [
    {
      id: 'clock-check',
      type: 'network-endpoint',
      platformType: 'availability',
      sources: ['availability'],
      name: 'Clock check',
      lastSeen: start,
      status: 'online',
      availability,
    },
  ];
  vi.mocked(AvailabilityTargetsAPI.list)
    .mockClear()
    .mockResolvedValue([
      {
        id: 'clock-check',
        name: 'Clock check',
        address: 'clock.example.test',
        protocol: 'tcp',
        enabled: true,
        status: availability,
      },
    ]);
  try {
    const { container } = render(() => <AvailabilitySettingsPanel />);
    expect(await screen.findByText('1 enabled · 1 total')).toBeInTheDocument();
    expect(container.querySelector('span.bg-emerald-100')).not.toBeNull();
    expect(screen.getByText('Online')).toBeInTheDocument();
    vi.advanceTimersByTime(30_000);
    expect(screen.getByText('1 needs attention · 1 enabled')).toBeInTheDocument();
    expect(container.querySelector('span.bg-amber-100')).not.toBeNull();
    expect(screen.getByText('Stale')).toHaveAttribute(
      'title',
      'The last check is stale. It does not establish current availability.',
    );
    expect(screen.queryByText('Online')).not.toBeInTheDocument();
    expect(AvailabilityTargetsAPI.list).toHaveBeenCalledOnce();
  } finally {
    cleanup();
    vi.useRealTimers();
  }
});
