import { describe, expect, it } from 'vitest';

import {
  formatResourceChangeHeadline,
  formatResourceChangeKind,
  getResourceChangeKindPresentation,
  getResourceChangeSourceAdapterPresentation,
  getResourceChangeSourceTypePresentation,
} from '@/utils/resourceChangePresentation';

describe('resourceChangePresentation utils', () => {
  it('formats canonical resource change kinds', () => {
    expect(formatResourceChangeKind('config_update')).toBe('Config update');
    expect(formatResourceChangeKind('metric_anomaly')).toBe('Metric anomaly');
    expect(formatResourceChangeKind('relationship_change')).toBe('Relationship change');
  });

  it('formats canonical resource change headlines', () => {
    expect(
      formatResourceChangeHeadline({
        id: 'change-1',
        resourceId: 'vm:42',
        kind: 'state_transition',
        from: 'running',
        to: 'restarting',
        observedAt: '2026-03-18T12:00:00Z',
      } as never),
    ).toBe('State transition: running → restarting');

    expect(
      formatResourceChangeHeadline({
        id: 'change-2',
        resourceId: 'vm:42',
        kind: 'config_update',
        reason: 'Updated canonical config',
        observedAt: '2026-03-18T12:00:00Z',
      } as never),
    ).toBe('Config update: Updated canonical config');
  });

  it('exposes canonical kind, source type, and adapter presentations', () => {
    expect(getResourceChangeKindPresentation('restart')).toMatchObject({
      label: 'Restart',
      plural: 'Restarts',
    });
    expect(getResourceChangeSourceTypePresentation('platform_event')).toMatchObject({
      label: 'Platform event',
      plural: 'Platform events',
    });
    expect(getResourceChangeSourceAdapterPresentation('proxmox_adapter')).toMatchObject({
      label: 'Proxmox adapter',
      plural: 'Proxmox adapters',
    });
  });
});
