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
