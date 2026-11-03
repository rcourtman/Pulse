import { describe, expect, it } from 'vitest';

import { getAlertsForResource, getAlertStyles } from '@/utils/alerts';
import type { Alert } from '@/types/api';

const alert = (id: string, overrides: Partial<Alert> = {}): Alert => ({
  id,
  resourceId: `${id}-resource`,
  resourceName: id,
  node: 'pve1',
  instance: 'north',
  type: 'memory',
  level: 'warning',
  message: id,
  value: 90,
  threshold: 85,
  startTime: '2026-11-03T12:00:00Z',
  acknowledged: false,
  ...overrides,
});
const byId = (...alerts: Alert[]): Record<string, Alert> =>
  Object.fromEntries(alerts.map((entry) => [entry.id, entry]));

describe('instance-scoped node alert fallback', () => {
  const scope = { name: 'pve1', instance: 'north' };

  it('keeps the node and its local guest, disk and storage alerts without crossing estates', () => {
    const active = byId(
      alert('own', { resourceId: 'north-node' }),
      alert('guest', { resourceId: 'north-pve1-101' }),
      alert('disk', { resourceId: 'north:pve1:disk:/dev/sda' }),
      alert('storage', { resourceId: 'north-pve1-local-zfs', type: 'usage' }),
      alert('other-estate-guest', { resourceId: 'south-pve1-101', instance: 'south' }),
      alert('other-estate-disk', { instance: 'south' }),
      alert('other-node', { node: 'pve10' }),
      alert('shared-storage', { node: 'cluster' }),
    );

    expect(getAlertsForResource(['north-node'], active, true, scope).map((a) => a.id)).toEqual([
      'own',
      'guest',
      'disk',
      'storage',
    ]);
    expect(getAlertStyles('north-node', active, true, scope).alertCount).toBe(4);
    expect(getAlertsForResource(['north-node'], active, false, scope)).toEqual([]);
  });

  it('uses exact resource identities even when old alert metadata has no node or instance', () => {
    const active = byId(
      alert('own', { resourceId: 'north-node', node: '', instance: '' }),
      alert('unscoped-child', { instance: '' }),
    );

    expect(getAlertsForResource(['north-node'], active, true, scope).map((a) => a.id)).toEqual([
      'own',
    ]);
    // An ambiguous child is not assigned to a host, but still matches its own
    // resource. This selector does not remove or acknowledge the active alert.
    expect(
      getAlertsForResource(['unscoped-child-resource'], active, true).map((a) => a.id),
    ).toEqual(['unscoped-child']);
    expect(Object.keys(active)).toEqual(['own', 'unscoped-child']);
  });

  it.each([
    { name: 'pve1', instance: '' },
    { name: '', instance: 'north' },
    { name: ' ', instance: 'north' },
  ])('does not guess child attribution for an incomplete scope %j', (incomplete) => {
    const active = byId(alert('own', { resourceId: 'north-node' }), alert('guest'));
    expect(getAlertsForResource(['north-node'], active, true, incomplete).map((a) => a.id)).toEqual(
      ['own'],
    );
  });

  it('keeps the existing name-only caller contract outside the scoped Proxmox path', () => {
    const active = byId(alert('north'), alert('south', { instance: 'south' }));
    expect(getAlertsForResource([], active, true, 'pve1').map((a) => a.id)).toEqual([
      'north',
      'south',
    ]);
  });
});
