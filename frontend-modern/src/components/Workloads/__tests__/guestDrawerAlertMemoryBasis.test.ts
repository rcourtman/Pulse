import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { getAlertAttentionCopy } from '@/features/alerts/metricAlertPresentation';
import { getGuestDrawerAlertAttention } from '../guestDrawerModel';

const at = '2026-10-12T05:00:00Z';
const now = () => Date.parse(at);
const note = 'Alert percentages use guest allocation. The table row uses host capacity.';
const alert: Alert = {
  id: 'memory-alert',
  resourceId: 'lab:pve:101',
  resourceName: 'memory-guest',
  node: 'pve',
  instance: 'lab',
  type: 'memory',
  level: 'warning',
  message: 'VM memory at 86.6%',
  value: 86.6,
  threshold: 85,
  startTime: at,
  acknowledged: false,
};
const guest = (state?: string): WorkloadGuest => ({
  id: 'lab:pve:101',
  vmid: 101,
  name: 'memory-guest',
  node: 'pve',
  instance: 'lab',
  status: 'running',
  type: 'qemu',
  cpu: 0.1,
  cpus: 2,
  memory: {
    total: 4 * 1024 ** 3,
    used: 1024 ** 3,
    free: 3 * 1024 ** 3,
    usage: 25,
    ...(state ? { observation: { state, source: 'agent', observedAt: at } } : {}),
  },
  disk: { total: 1024, used: 512, usage: 50 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: at,
});

describe('guest drawer alert memory basis', () => {
  it.each(['current', 'last-known', 'unavailable', 'future-state', undefined])(
    'does not derive an alert comparison from a %s guest observation',
    (state) => {
      const value = guest(state);
      value.memory.usageUnavailable = state === 'unavailable';
      const copy = getGuestDrawerAlertAttention(
        alert,
        {
          guest: value,
          memoryDisplayBasis: 'host',
          parentMemoryTotal: 8 * 1024 ** 3,
        },
        now,
      );
      expect(copy).toEqual({ message: alert.message, detail: note });
      // A refreshed poll and different host capacity cannot rewrite the alert's evidence.
      value.lastSeen = '2026-10-12T05:30:00Z';
      value.memory.used = 3 * 1024 ** 3;
      expect(
        getGuestDrawerAlertAttention(
          alert,
          {
            guest: value,
            memoryDisplayBasis: 'host',
            parentMemoryTotal: 16 * 1024 ** 3,
          },
          now,
        ),
      ).toEqual(copy);
    },
  );

  it.each([0, undefined, NaN, -1, Infinity])(
    'explains the selected basis even when host capacity is %s',
    (parentMemoryTotal) => {
      const value = guest('current');
      value.memory.used = 0;
      expect(
        getGuestDrawerAlertAttention(
          alert,
          {
            guest: value,
            memoryDisplayBasis: 'host',
            parentMemoryTotal,
          },
          now,
        ),
      ).toEqual({ message: alert.message, detail: note });
    },
  );

  it.each(['breaching', 'latched', 'recovering'] as const)(
    'preserves alert-owned %s values, times, clear rules and hover evidence',
    (phase) => {
      const evaluated: Alert = {
        ...alert,
        metricStatus: {
          phase,
          value: 72,
          unit: '%',
          observedAt: at,
          trigger: 85,
          recovery: 70,
          lastBreachAt: '2026-10-12T04:40:00Z',
          recoveryDelaySeconds: 300,
        },
      };
      const original = getAlertAttentionCopy(evaluated, now);
      const copy = getGuestDrawerAlertAttention(
        evaluated,
        {
          guest: guest('unavailable'),
          memoryDisplayBasis: 'host',
          parentMemoryTotal: 8 * 1024 ** 3,
        },
        now,
      );
      expect(copy.message).toBe(original.message);
      expect(copy.title).toBe(original.title);
      expect(copy.detail).toBe(`${original.detail} ${note}`);
      expect(copy.message).not.toContain('13%');
      expect(evaluated.value).toBe(86.6);
      expect(evaluated.metricStatus?.value).toBe(72);
    },
  );

  it.each(['guest', undefined] as const)(
    'keeps %s-basis attention unchanged',
    (memoryDisplayBasis) => {
      expect(
        getGuestDrawerAlertAttention(alert, { guest: guest(), memoryDisplayBasis }, now),
      ).toEqual(getAlertAttentionCopy(alert, now));
    },
  );

  it.each(['cpu', 'disk', 'temperature'])('does not annotate a %s alert', (type) => {
    const independent = { ...alert, type };
    expect(
      getGuestDrawerAlertAttention(
        independent,
        {
          guest: guest('last-known'),
          memoryDisplayBasis: 'host',
          parentMemoryTotal: 8 * 1024 ** 3,
        },
        now,
      ),
    ).toEqual(getAlertAttentionCopy(independent, now));
  });
});
