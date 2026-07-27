import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AvailabilityTargetsAPI } from '@/api/availabilityTargets';
import { AvailabilityTargetSlot } from '../CredentialSlots/AvailabilityTargetSlot';

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

const licenseMocks = vi.hoisted(() => ({
  features: new Set<string>(),
  loaded: true,
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (feature: string) => licenseMocks.features.has(feature),
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
  runtimeCapabilitiesLoaded: () => licenseMocks.loaded,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: (feature: string) => ({
    href: `https://example.test/upgrade?feature=${feature}`,
    external: true,
  }),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => false,
}));

const agentHostResource = (agentId: string, displayName: string) => ({
  id: `agent:${agentId}`,
  type: 'agent',
  name: displayName,
  displayName,
  platformId: agentId,
  platformType: 'agent',
  sourceType: 'agent',
  sources: ['agent'],
  status: 'online',
  lastSeen: 1_700_000_000_000,
  agent: { agentId },
});

const mockedCreate = vi.mocked(AvailabilityTargetsAPI.create);
const mockedList = vi.mocked(AvailabilityTargetsAPI.list);
const mockedUpdate = vi.mocked(AvailabilityTargetsAPI.update);

describe('AvailabilityTargetSlot', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resourceMocks.resources = [];
    licenseMocks.features = new Set(['external_probe']);
    licenseMocks.loaded = true;
    mockedCreate.mockResolvedValue({
      id: 'target-1',
      name: 'Rack sensor',
      address: 'rack-sensor.local',
      targetKind: 'device',
      protocol: 'tcp',
      port: 6053,
      enabled: true,
    });
  });

  afterEach(() => cleanup());

  it('prefills ESPHome devices as TCP availability targets', async () => {
    const onSaved = vi.fn();
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={onSaved} />);

    fireEvent.change(screen.getByLabelText('Preset'), {
      target: { value: 'esphome-device' },
    });

    await waitFor(() => expect(screen.getByLabelText('Probe')).toHaveValue('tcp'));
    expect(screen.getByLabelText('Target type')).toHaveValue('device');
    expect(screen.getByLabelText('Port')).toHaveValue('6053');

    fireEvent.input(screen.getByLabelText('Name'), {
      target: { value: 'Rack sensor' },
    });
    fireEvent.input(screen.getByPlaceholderText('sensor.local'), {
      target: { value: 'rack-sensor.local' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Rack sensor',
          targetKind: 'device',
          address: 'rack-sensor.local',
          protocol: 'tcp',
          port: 6053,
          enabled: true,
        }),
      ),
    );
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  it('starts machine add routes as machine reachability checks', async () => {
    const onSaved = vi.fn();
    render(() => (
      <AvailabilityTargetSlot initialTargetKind="machine" onCancel={vi.fn()} onSaved={onSaved} />
    ));

    expect(screen.getByLabelText('Preset')).toHaveValue('ping-machine');
    expect(screen.getByLabelText('Target type')).toHaveValue('machine');
    expect(screen.getByPlaceholderText('mac-mini')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('server.local')).toBeInTheDocument();

    fireEvent.input(screen.getByLabelText('Name'), {
      target: { value: 'mac-mini' },
    });
    fireEvent.input(screen.getByPlaceholderText('server.local'), {
      target: { value: 'mac-mini.local' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add machine check' }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'mac-mini',
          targetKind: 'machine',
          address: 'mac-mini.local',
          protocol: 'icmp',
          enabled: true,
        }),
      ),
    );
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  it('binds an explicit canonical resource id in the saved target', async () => {
    resourceMocks.resources = [
      {
        id: 'docker-service:api',
        type: 'docker-service',
        name: 'api',
        displayName: 'Customer API',
        platformId: 'docker-main',
        platformType: 'docker',
        sourceType: 'agent',
        status: 'online',
        lastSeen: Date.now(),
      },
    ];
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.input(screen.getByLabelText('Name'), {
      target: { value: 'Customer API' },
    });
    fireEvent.input(screen.getByPlaceholderText('service.local'), {
      target: { value: 'api.example.test' },
    });
    fireEvent.change(screen.getByLabelText('Link to resource (optional)'), {
      target: { value: 'docker-service:api' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          linkedResourceId: 'docker-service:api',
        }),
      ),
    );
  });

  it('creates response-validated UDP checks', async () => {
    const onSaved = vi.fn();
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={onSaved} />);

    fireEvent.change(screen.getByLabelText('Probe'), { target: { value: 'udp' } });
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'DNS health' } });
    fireEvent.input(screen.getByLabelText(/^Address/), { target: { value: 'dns.internal' } });
    fireEvent.input(screen.getByLabelText('Port'), { target: { value: '53' } });
    fireEvent.input(screen.getByLabelText(/^Request payload/), { target: { value: 'PING' } });
    fireEvent.input(screen.getByLabelText(/^Expected response \(optional\)/), {
      target: { value: 'PONG' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'DNS health',
          address: 'dns.internal',
          protocol: 'udp',
          port: 53,
          udpMode: 'response_required',
          udpRequest: 'PING',
          udpExpectedResponse: 'PONG',
        }),
      ),
    );
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  describe('external probe assignment', () => {
    it('offers connected agent hosts and saves the assignment when licensed', async () => {
      resourceMocks.resources = [agentHostResource('host-edge-01', 'Edge 01')];
      render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

      const runFrom = screen.getByLabelText('Run from') as HTMLSelectElement;
      expect(runFrom).not.toBeDisabled();
      expect(screen.getByRole('option', { name: 'This Pulse server' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'Edge 01' })).toBeInTheDocument();
      expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();

      fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Remote MQTT' } });
      fireEvent.input(screen.getByPlaceholderText('service.local'), {
        target: { value: 'mqtt.remote.local' },
      });
      fireEvent.change(runFrom, { target: { value: 'host-edge-01' } });
      fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

      await waitFor(() =>
        expect(mockedCreate).toHaveBeenCalledWith(
          expect.objectContaining({ probeAgentId: 'host-edge-01' }),
        ),
      );
    });

    it('locks the control behind the canonical upgrade gate when unlicensed', () => {
      licenseMocks.features = new Set();
      resourceMocks.resources = [agentHostResource('host-edge-01', 'Edge 01')];
      render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

      // Discoverability is the point of the Pro gate: the control stays visible.
      const runFrom = screen.getByLabelText('Run from') as HTMLSelectElement;
      expect(runFrom).toBeDisabled();
      expect(screen.getByRole('option', { name: 'Edge 01' })).toBeInTheDocument();
      expect(screen.getByRole('heading', { name: 'External Probes' })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'View plans' })).toHaveAttribute(
        'href',
        'https://example.test/upgrade?feature=external_probe',
      );
    });

    it('saves a local target unlicensed without any license friction', async () => {
      licenseMocks.features = new Set();
      render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

      fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Local ping' } });
      fireEvent.input(screen.getByPlaceholderText('service.local'), {
        target: { value: 'printer.local' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

      await waitFor(() =>
        expect(mockedCreate).toHaveBeenCalledWith(expect.objectContaining({ probeAgentId: '' })),
      );
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('clears an assignment with an explicit empty string', async () => {
      resourceMocks.resources = [agentHostResource('host-edge-01', 'Edge 01')];
      mockedList.mockResolvedValue([
        {
          id: 'target-1',
          name: 'Remote MQTT',
          address: 'mqtt.remote.local',
          protocol: 'tcp',
          port: 1883,
          enabled: true,
          probeAgentId: 'host-edge-01',
        },
      ]);
      mockedUpdate.mockResolvedValue({
        id: 'target-1',
        name: 'Remote MQTT',
        address: 'mqtt.remote.local',
        protocol: 'tcp',
        enabled: true,
      });

      render(() => (
        <AvailabilityTargetSlot editingTargetId="target-1" onCancel={vi.fn()} onSaved={vi.fn()} />
      ));

      await waitFor(() => expect(screen.getByLabelText('Run from')).toHaveValue('host-edge-01'));

      fireEvent.change(screen.getByLabelText('Run from'), { target: { value: '' } });
      fireEvent.click(screen.getByRole('button', { name: 'Save target' }));

      await waitFor(() => expect(mockedUpdate).toHaveBeenCalled());
      const [, payload] = mockedUpdate.mock.calls.at(-1)!;
      expect(payload.probeAgentId).toBe('');
      expect(Object.prototype.hasOwnProperty.call(payload, 'probeAgentId')).toBe(true);
    });

    it('keeps a saved assignment selectable when its host is not currently connected', async () => {
      mockedList.mockResolvedValue([
        {
          id: 'target-1',
          name: 'Remote MQTT',
          address: 'mqtt.remote.local',
          protocol: 'tcp',
          enabled: true,
          probeAgentId: 'host-gone',
        },
      ]);

      render(() => (
        <AvailabilityTargetSlot editingTargetId="target-1" onCancel={vi.fn()} onSaved={vi.fn()} />
      ));

      await waitFor(() =>
        expect(
          screen.getByRole('option', { name: 'host-gone (not currently connected)' }),
        ).toBeInTheDocument(),
      );
      expect(screen.getByLabelText('Run from')).toHaveValue('host-gone');
    });

    it('falls back to the upgrade gate when the server answers 402 license_required', async () => {
      resourceMocks.resources = [agentHostResource('host-edge-01', 'Edge 01')];
      mockedCreate.mockRejectedValueOnce(
        Object.assign(new Error('external probe requires a paid feature'), {
          status: 402,
          feature: 'external_probe',
        }),
      );

      render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

      fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Remote MQTT' } });
      fireEvent.input(screen.getByPlaceholderText('service.local'), {
        target: { value: 'mqtt.remote.local' },
      });
      fireEvent.change(screen.getByLabelText('Run from'), { target: { value: 'host-edge-01' } });
      fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

      await waitFor(() =>
        expect(screen.getByRole('heading', { name: 'External Probes' })).toBeInTheDocument(),
      );
      expect(screen.getByLabelText('Run from')).toBeDisabled();
      expect(screen.getByRole('link', { name: 'View plans' })).toBeInTheDocument();
    });
  });
});
