import { describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildAlertHistoryItems } from '../alertHistoryModel';
import { makeSystemAlert, SYSTEM_ALERT_TYPES } from '../__fixtures__/systemAlerts';

const pulseVM = { id: 'vm-pulse', name: 'Pulse', type: 'vm' } as Resource;

describe('Pulse system-alert history scope', () => {
  it.each(SYSTEM_ALERT_TYPES)(
    'keeps active and resolved %s separate from a resource named Pulse',
    (type) => {
      const getResource = vi.fn(() => pulseVM);
      const active = makeSystemAlert(type);
      const resolved = makeSystemAlert(type, { id: `${active.id}-older`, acknowledged: true });
      const items = buildAlertHistoryItems({
        activeAlerts: { [active.id]: active },
        alertHistory: [active, resolved],
        getResource,
        allResources: [pulseVM],
      });
      expect(items).toHaveLength(2);
      for (const item of items) {
        expect(item.resourceType).toBe('Pulse');
        expect(item).toHaveProperty('systemAlert', true);
        expect(item.resourceId).toBe('');
      }
      expect(getResource).not.toHaveBeenCalled();
    },
  );

  it('prioritises explicit system metadata over conflicting resource hints', () => {
    const alert = makeSystemAlert('future-system-condition', {
      id: 'legacy-system-id',
      resourceId: pulseVM.id,
      metadata: { systemAlert: true, resourceType: 'vm' },
    });
    const [item] = buildAlertHistoryItems({
      activeAlerts: { [alert.id]: alert },
      alertHistory: [],
      getResource: () => pulseVM,
      allResources: [pulseVM],
    });
    expect(item.resourceType).toBe('Pulse');
    expect(item.resourceId).toBe('');
    expect(item).toHaveProperty('systemAlert', true);
  });

  it('does not turn a monitored VM called Pulse into a system alert', () => {
    const alert = makeSystemAlert('cpu', {
      id: 'resource-cpu',
      resourceId: pulseVM.id,
      metadata: { systemAlert: false },
    });
    const [item] = buildAlertHistoryItems({
      activeAlerts: { [alert.id]: alert },
      alertHistory: [],
      getResource: () => pulseVM,
      allResources: [pulseVM],
    });
    expect(item.resourceType).toBe('VM');
    expect(item.resourceId).toBe(pulseVM.id);
    expect(item.systemAlert).not.toBe(true);
  });
});
