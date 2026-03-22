import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import { useAlertScheduleState } from '../useAlertScheduleState';
import {
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  createDefaultResolveNotifications,
} from '../helpers';

describe('useAlertScheduleState', () => {
  it('owns schedule reset and update control flow outside the tab shell', () => {
    const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
    const [quietHours, setQuietHours] = createSignal(createDefaultQuietHours());
    const [cooldown, setCooldown] = createSignal(createDefaultCooldown());
    const [grouping, setGrouping] = createSignal(createDefaultGrouping());
    const [notifyOnResolve, setNotifyOnResolve] = createSignal(false);
    const [escalation, setEscalation] = createSignal(createDefaultEscalation());

    const { result } = renderHook(() =>
      useAlertScheduleState({
        setHasUnsavedChanges,
        quietHours,
        setQuietHours,
        cooldown,
        setCooldown,
        grouping,
        setGrouping,
        notifyOnResolve,
        setNotifyOnResolve,
        escalation,
        setEscalation,
      }),
    );

    result.setQuietHoursEnabled(true);
    result.setQuietHoursStart('21:30');
    result.toggleQuietDay('saturday');
    result.setQuietSuppressCategory('storage', true);
    result.setCooldownEnabled(true);
    result.setCooldownMinutes('45');
    result.setCooldownMaxAlerts('5');
    result.setGroupingEnabled(true);
    result.setGroupingWindow('8');
    result.setGroupingByNode(true);
    result.setNotifyOnResolveEnabled(true);
    result.setEscalationEnabled(true);
    result.addEscalationLevel();
    result.setEscalationAfter(0, '30');
    result.setEscalationNotify(0, 'webhook');

    expect(hasUnsavedChanges()).toBe(true);
    expect(quietHours()).toMatchObject({
      enabled: true,
      start: '21:30',
      suppress: expect.objectContaining({ storage: true }),
      days: expect.objectContaining({ saturday: true }),
    });
    expect(result.weekendsOnly()).toBe(false);
    expect(cooldown()).toMatchObject({ enabled: true, minutes: 45, maxAlerts: 5 });
    expect(grouping()).toMatchObject({ enabled: true, window: 8, byNode: true });
    expect(notifyOnResolve()).toBe(true);
    expect(escalation()).toMatchObject({
      enabled: true,
      levels: [expect.objectContaining({ after: 30, notify: 'webhook' })],
    });

    result.removeEscalationLevel(0);
    expect(escalation().levels).toHaveLength(0);

    result.resetToDefaults();

    expect(quietHours()).toEqual(createDefaultQuietHours());
    expect(cooldown()).toEqual(createDefaultCooldown());
    expect(grouping()).toEqual(createDefaultGrouping());
    expect(notifyOnResolve()).toBe(createDefaultResolveNotifications());
    expect(escalation()).toEqual(createDefaultEscalation());
  });
});
