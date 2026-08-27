import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';

import { isAlertSnoozed, useAlertSnoozeState } from '../useAlertSnoozeState';

vi.mock('@/api/alerts', () => ({ AlertsAPI: { snooze: vi.fn(), unsnooze: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

function makeAlert(): Alert {
  return {
    id: 'cpu:vm/100',
    type: 'cpu',
    level: 'critical',
    resourceId: 'vm/100',
    resourceName: 'vm-100',
    message: 'CPU is critical',
    startTime: '2026-08-27T11:00:00Z',
    acknowledged: false,
  } as Alert;
}

describe('useAlertSnoozeState', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-27T12:00:00Z'));
    vi.clearAllMocks();
  });

  afterEach(() => vi.useRealTimers());

  it('optimistically snoozes and resumes the canonical operational record', async () => {
    const [alerts] = createSignal([makeAlert()]);
    const updateAlert = vi.fn();
    vi.mocked(AlertsAPI.snooze).mockResolvedValue({
      success: true,
      snoozedUntil: '2026-08-27T14:00:00Z',
    });
    vi.mocked(AlertsAPI.unsnooze).mockResolvedValue({ success: true });
    const { result } = renderHook(() => useAlertSnoozeState({ alerts, updateAlert }));

    await result.handleSnooze(alerts()[0], new Date('2026-08-27T14:00:00Z'));
    expect(isAlertSnoozed(result.effectiveAlerts()[0])).toBe(true);
    expect(AlertsAPI.snooze).toHaveBeenCalledWith('cpu:vm/100', '2026-08-27T14:00:00.000Z');
    expect(notificationStore.success).toHaveBeenCalledWith(
      expect.stringMatching(/^Alert snoozed until /),
    );

    await result.handleUnsnooze(result.effectiveAlerts()[0]);
    expect(isAlertSnoozed(result.effectiveAlerts()[0])).toBe(false);
    expect(result.effectiveAlerts()[0].operationalRecord?.state).toBe('open');
    expect(AlertsAPI.unsnooze).toHaveBeenCalledWith('cpu:vm/100');
  });

  it('restores the prior record when snooze persistence fails', async () => {
    const [alerts] = createSignal([makeAlert()]);
    const updateAlert = vi.fn();
    vi.mocked(AlertsAPI.snooze).mockRejectedValue(new Error('offline'));
    const { result } = renderHook(() => useAlertSnoozeState({ alerts, updateAlert }));

    await expect(
      result.handleSnooze(alerts()[0], new Date('2026-08-27T14:00:00Z')),
    ).rejects.toThrow('offline');
    expect(result.effectiveAlerts()[0].operationalRecord).toBeUndefined();
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to snooze alert');
  });
});
