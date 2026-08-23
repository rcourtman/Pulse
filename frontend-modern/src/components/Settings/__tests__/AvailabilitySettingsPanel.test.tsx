import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AvailabilityTargetsAPI, type AvailabilityTarget } from '@/api/availabilityTargets';
import availabilitySettingsPanelSource from '../AvailabilitySettingsPanel.tsx?raw';
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
    onSaved: () => void;
  }) => (
    <div
      data-testid="availability-target-slot"
      data-editing-target-id={props.editingTargetId ?? ''}
      data-initial-target-kind={props.initialTargetKind ?? ''}
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

  it('keeps large target inventories on the shared bounded list renderer', () => {
    expect(availabilitySettingsPanelSource).toContain('PlatformWindowedList');
    expect(availabilitySettingsPanelSource).toContain('estimatedItemHeight={92}');
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
});
