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
const mockedTest = vi.mocked(AvailabilityTargetsAPI.test);

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

  it('enables HTTPS certificate monitoring with a 30-day warning by default', async () => {
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.change(screen.getByLabelText('Probe'), { target: { value: 'https' } });
    expect(screen.getByLabelText('Monitor TLS certificate validity')).toBeChecked();
    expect(screen.getByLabelText('Expiry warning (days)')).toHaveValue('30');

    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Pulse UI' } });
    fireEvent.input(screen.getByPlaceholderText('https://service.local/status'), {
      target: { value: 'pulse.example.test' },
    });
    fireEvent.input(screen.getByLabelText('Expiry warning (days)'), {
      target: { value: '45' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          protocol: 'https',
          certificateMonitoringDisabled: false,
          certificateExpiryWarningDays: 45,
        }),
      ),
    );
  });

  it('creates an HTTP application response contract from the proof question', async () => {
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.change(screen.getByLabelText('Probe'), { target: { value: 'https' } });
    expect(
      screen.getByRole('heading', { name: 'What proves this service is working?' }),
    ).toBeInTheDocument();
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Orders API' } });
    fireEvent.input(screen.getByLabelText(/^URL or host/), {
      target: { value: 'https://orders.example.test/health' },
    });
    fireEvent.change(screen.getByLabelText('Request method'), { target: { value: 'POST' } });
    fireEvent.input(screen.getByLabelText('Accepted status from'), { target: { value: '200' } });
    fireEvent.input(screen.getByLabelText('Accepted status to'), { target: { value: '204' } });
    fireEvent.input(screen.getByLabelText(/^Request body \(optional\)/), {
      target: { value: '{"operation":"health"}' },
    });
    fireEvent.change(screen.getByLabelText('Authentication'), { target: { value: 'bearer' } });
    fireEvent.input(screen.getByLabelText('Bearer token'), { target: { value: 'token-value' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add header' }));
    fireEvent.input(screen.getByLabelText('Header name'), { target: { value: 'X-Tenant' } });
    fireEvent.input(screen.getByLabelText('Header value'), { target: { value: 'tenant-a' } });
    fireEvent.input(screen.getByPlaceholderText('data.status'), {
      target: { value: 'data.status' },
    });
    fireEvent.input(screen.getByPlaceholderText('ok'), {
      target: { value: 'healthy' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

    await waitFor(() => expect(mockedCreate).toHaveBeenCalled());
    expect(mockedCreate.mock.calls.at(-1)?.[0]).toEqual(
      expect.objectContaining({
        protocol: 'https',
        http: expect.objectContaining({
          method: 'POST',
          body: '{"operation":"health"}',
          expectedStatusMin: 200,
          expectedStatusMax: 204,
          authentication: { type: 'bearer', bearerToken: 'token-value' },
          headers: [expect.objectContaining({ name: 'X-Tenant', value: 'tenant-a' })],
          jsonPath: 'data.status',
          jsonEquals: 'healthy',
        }),
      }),
    );
  });

  it('preserves write-only HTTP values when editing without re-entering them', async () => {
    mockedList.mockResolvedValue([
      {
        id: 'target-1',
        name: 'Orders API',
        address: 'https://orders.example.test/health',
        protocol: 'https',
        enabled: true,
        http: {
          method: 'POST',
          headers: [{ id: 'tenant-header', name: 'X-Tenant' }],
          authentication: { type: 'basic', username: 'pulse' },
          expectedStatusMin: 200,
          expectedStatusMax: 299,
          jsonPath: 'status',
          jsonEquals: 'healthy',
        },
        httpSecrets: {
          bodyConfigured: true,
          passwordConfigured: true,
          bearerTokenConfigured: false,
          headers: [{ id: 'tenant-header', valueConfigured: true }],
        },
      },
    ]);
    mockedUpdate.mockResolvedValue({
      id: 'target-1',
      name: 'Orders API',
      address: 'https://orders.example.test/health',
      protocol: 'https',
      enabled: true,
    });

    render(() => (
      <AvailabilityTargetSlot editingTargetId="target-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    ));
    await waitFor(() => expect(screen.getByLabelText('Request method')).toHaveValue('POST'));
    expect(screen.getByLabelText('Password')).toHaveAttribute(
      'placeholder',
      'Stored securely — leave blank to keep it',
    );
    expect(screen.getByLabelText(/^Request body \(optional\)/)).toHaveAttribute(
      'placeholder',
      'Stored securely — leave blank to keep it',
    );
    expect(screen.getByLabelText('Header value')).toHaveAttribute(
      'placeholder',
      'Stored securely — leave blank to keep it',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Save target' }));
    await waitFor(() => expect(mockedUpdate).toHaveBeenCalled());
    const [, payload] = mockedUpdate.mock.calls.at(-1)!;
    expect(payload.http?.body).toBeUndefined();
    expect(payload.http?.authentication.password).toBeUndefined();
    expect(payload.http?.headers).toEqual([
      { id: 'tenant-header', name: 'X-Tenant', value: undefined },
    ]);
  });

  it('can explicitly remove a stored POST body without changing the request method', async () => {
    mockedList.mockResolvedValue([
      {
        id: 'target-1',
        name: 'Orders API',
        address: 'https://orders.example.test/health',
        protocol: 'https',
        enabled: true,
        http: {
          method: 'POST',
          headers: [],
          authentication: { type: 'none' },
          expectedStatusMin: 200,
          expectedStatusMax: 299,
        },
        httpSecrets: {
          bodyConfigured: true,
          passwordConfigured: false,
          bearerTokenConfigured: false,
          headers: [],
        },
      },
    ]);
    mockedUpdate.mockResolvedValue({
      id: 'target-1',
      name: 'Orders API',
      address: 'https://orders.example.test/health',
      protocol: 'https',
      enabled: true,
    });

    render(() => (
      <AvailabilityTargetSlot editingTargetId="target-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    ));
    await waitFor(() => expect(screen.getByLabelText('Request method')).toHaveValue('POST'));
    fireEvent.click(screen.getByRole('button', { name: 'Remove stored body' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save target' }));

    await waitFor(() => expect(mockedUpdate).toHaveBeenCalled());
    const [, payload] = mockedUpdate.mock.calls.at(-1)!;
    expect(payload.http?.method).toBe('POST');
    expect(payload.http?.body).toBe('');
  });

  it('explains a reachable endpoint with a failing application contract', async () => {
    mockedTest.mockResolvedValue({
      success: false,
      latencyMillis: 18,
      outcome: 'unreachable',
      transportOutcome: 'reachable',
      application: { outcome: 'failed', statusCode: 503, failureCode: 'status_mismatch' },
      error: 'http response status 503 was outside the expected 200-299 range',
    });
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('Probe'), { target: { value: 'http' } });
    fireEvent.input(screen.getByLabelText(/^URL or host/), {
      target: { value: 'http://orders.example.test/health' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Test probe' }));

    expect(
      await screen.findByText(/Endpoint answered in 18 ms, but the application contract failed/),
    ).toBeInTheDocument();
    expect(screen.getByText(/HTTP 503/)).toBeInTheDocument();
  });

  describe('external probe assignment', () => {
    it('offers connected agent hosts and saves the assignment when licensed', async () => {
      resourceMocks.resources = [agentHostResource('host-edge-01', 'Edge 01')];
      render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

      const localLocation = screen.getByRole('checkbox', { name: /This Pulse server/ });
      const edgeLocation = screen.getByRole('checkbox', { name: /Edge 01/ });
      expect(localLocation).toBeChecked();
      expect(edgeLocation).not.toBeDisabled();
      expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();

      fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Remote MQTT' } });
      fireEvent.input(screen.getByPlaceholderText('service.local'), {
        target: { value: 'mqtt.remote.local' },
      });
      fireEvent.click(edgeLocation);
      fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

      await waitFor(() =>
        expect(mockedCreate).toHaveBeenCalledWith(
          expect.objectContaining({
            probeAgentId: '',
            observationLocationIds: ['pulse:local', 'agent:host-edge-01'],
          }),
        ),
      );
    });

    it('locks the control behind the canonical upgrade gate when unlicensed', () => {
      licenseMocks.features = new Set();
      resourceMocks.resources = [agentHostResource('host-edge-01', 'Edge 01')];
      render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />);

      // Discoverability is the point of the Pro gate: the control stays visible.
      const edgeLocation = screen.getByRole('checkbox', { name: /Edge 01/ });
      expect(edgeLocation).toBeDisabled();
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
        expect(mockedCreate).toHaveBeenCalledWith(
          expect.objectContaining({
            probeAgentId: '',
            observationLocationIds: ['pulse:local'],
          }),
        ),
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

      await waitFor(() => expect(screen.getByRole('checkbox', { name: /Edge 01/ })).toBeChecked());

      fireEvent.click(screen.getByRole('checkbox', { name: /This Pulse server/ }));
      fireEvent.click(screen.getByRole('checkbox', { name: /Edge 01/ }));
      fireEvent.click(screen.getByRole('button', { name: 'Save target' }));

      await waitFor(() => expect(mockedUpdate).toHaveBeenCalled());
      const [, payload] = mockedUpdate.mock.calls.at(-1)!;
      expect(payload.probeAgentId).toBe('');
      expect(payload.observationLocationIds).toEqual(['pulse:local']);
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
        expect(screen.getByRole('checkbox', { name: /host-gone/ })).toBeChecked(),
      );
      expect(screen.getByText('Not currently connected')).toBeInTheDocument();
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
      fireEvent.click(screen.getByRole('checkbox', { name: /Edge 01/ }));
      fireEvent.click(screen.getByRole('button', { name: 'Add service/device check' }));

      await waitFor(() =>
        expect(screen.getByRole('heading', { name: 'External Probes' })).toBeInTheDocument(),
      );
      expect(screen.getByRole('checkbox', { name: /Edge 01/ })).toBeDisabled();
      expect(screen.getByRole('link', { name: 'View plans' })).toBeInTheDocument();
    });
  });
});
