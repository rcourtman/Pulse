import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AvailabilityTargetsAPI, type AvailabilityTarget } from '@/api/availabilityTargets';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('AvailabilityTargetsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists, creates, updates, removes, and tests through canonical routes', async () => {
    const target: AvailabilityTarget = {
      id: 'sensor-1',
      name: 'Energy monitor',
      targetKind: 'device',
      address: '192.0.2.10',
      protocol: 'icmp',
      enabled: true,
    };

    mockedApiFetchJSON.mockResolvedValueOnce([target]);
    await expect(AvailabilityTargetsAPI.list()).resolves.toEqual([target]);
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/availability-targets');

    mockedApiFetchJSON.mockResolvedValueOnce(target);
    await AvailabilityTargetsAPI.create(target);
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/availability-targets', {
      method: 'POST',
      body: JSON.stringify(target),
    });

    mockedApiFetchJSON.mockResolvedValueOnce({ ...target, enabled: false });
    await AvailabilityTargetsAPI.update('sensor/1', { enabled: false });
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/availability-targets/sensor%2F1', {
      method: 'PUT',
      body: JSON.stringify({ enabled: false }),
    });

    mockedApiFetchJSON.mockResolvedValueOnce({ success: true });
    await AvailabilityTargetsAPI.remove('sensor/1');
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/availability-targets/sensor%2F1', {
      method: 'DELETE',
    });

    mockedApiFetchJSON.mockResolvedValueOnce({ success: true, latencyMillis: 5 });
    await AvailabilityTargetsAPI.test(target);
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/availability-targets/test', {
      method: 'POST',
      body: JSON.stringify(target),
    });

    mockedApiFetchJSON.mockResolvedValueOnce({ success: true, latencyMillis: 5 });
    await AvailabilityTargetsAPI.testSaved('sensor/1');
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith(
      '/api/availability-targets/sensor%2F1/test',
      {
        method: 'POST',
      },
    );
  });

  it('round-trips a probe agent assignment through list and create', async () => {
    const target: AvailabilityTarget = {
      id: 'sensor-1',
      name: 'Remote MQTT',
      address: 'mqtt.remote.local',
      protocol: 'tcp',
      port: 1883,
      enabled: true,
      probeAgentId: 'host-agent-7',
      status: {
        targetId: 'sensor-1',
        name: 'Remote MQTT',
        address: 'mqtt.remote.local',
        protocol: 'tcp',
        enabled: true,
        available: true,
        outcome: 'reachable',
        probeAgentId: 'host-agent-7',
      },
    };

    mockedApiFetchJSON.mockResolvedValueOnce([target]);
    const listed = await AvailabilityTargetsAPI.list();
    expect(listed[0].probeAgentId).toBe('host-agent-7');
    expect(listed[0].status?.probeAgentId).toBe('host-agent-7');

    mockedApiFetchJSON.mockResolvedValueOnce(target);
    await AvailabilityTargetsAPI.create(target);
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/availability-targets', {
      method: 'POST',
      body: JSON.stringify(target),
    });
  });

  it('sends an explicit empty probe agent id when clearing an assignment', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({});
    await AvailabilityTargetsAPI.update('sensor-1', { probeAgentId: '' });

    const [, init] = mockedApiFetchJSON.mock.calls.at(-1) as [string, { body: string }];
    // The server decodes onto the existing record, so an omitted key would keep
    // the stored assignment. The empty string has to survive serialization.
    expect(JSON.parse(init.body)).toEqual({ probeAgentId: '' });
    expect(init.body).toContain('"probeAgentId":""');
  });

  it('accepts https protocol for secure web services', async () => {
    const target: AvailabilityTarget = {
      id: '',
      name: 'Proxmox VE',
      address: '192.0.2.5',
      protocol: 'https',
      port: 8006,
      enabled: true,
    };
    mockedApiFetchJSON.mockResolvedValueOnce(target);
    await AvailabilityTargetsAPI.create(target);
    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/availability-targets', {
      method: 'POST',
      body: JSON.stringify(target),
    });
  });
});
