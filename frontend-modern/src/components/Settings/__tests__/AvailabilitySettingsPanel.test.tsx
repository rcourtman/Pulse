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

  it('lists monitor-only MQTT and HTTP endpoint checks in the monitoring home', async () => {
    render(() => <AvailabilitySettingsPanel />);

    await waitFor(() => expect(screen.getByText('MQTT broker')).toBeInTheDocument());
    expect(screen.getByText('HTTP health')).toBeInTheDocument();
    expect(screen.getByText('TCP 1883')).toBeInTheDocument();
    expect(screen.getByText('http://service.local/health')).toBeInTheDocument();
    expect(screen.getByText('Online · 8 ms')).toBeInTheDocument();
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
