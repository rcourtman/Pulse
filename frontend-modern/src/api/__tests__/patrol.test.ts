import { beforeEach, describe, expect, it, vi } from 'vitest';
import { triggerPatrolRun } from '../patrol';

const fetchMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => fetchMock(...(args as [unknown, unknown])),
}));

describe('triggerPatrolRun scope body', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({ success: true, message: 'Triggered targeted Patrol check' });
  });

  it('sends no body for a fleet-wide run', async () => {
    await triggerPatrolRun();
    expect(fetchMock).toHaveBeenCalledWith('/api/ai/patrol/run', { method: 'POST' });
  });

  it('sends a JSON scope body for a targeted check', async () => {
    const scope = {
      resource_ids: ['vm-101'],
      alert_identifier: 'alert-1',
      alert_type: 'cpu',
      context: 'Manual targeted check from alert: cpu',
    };
    await triggerPatrolRun(scope);
    expect(fetchMock).toHaveBeenCalledWith('/api/ai/patrol/run', {
      method: 'POST',
      body: JSON.stringify(scope),
      headers: { 'Content-Type': 'application/json' },
    });
  });

  it('treats a scope with no real ids as a fleet-wide run', async () => {
    await triggerPatrolRun({ resource_ids: [], resource_types: [] });
    expect(fetchMock).toHaveBeenCalledWith('/api/ai/patrol/run', { method: 'POST' });
  });
});
