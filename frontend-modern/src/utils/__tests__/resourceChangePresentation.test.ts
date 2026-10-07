import { describe, expect, it } from 'vitest';

import {
  formatResourceChangeHeadline,
  formatResourceChangeKind,
  getResourceChangeAlertResolution,
  getResourceChangeKindPresentation,
  getResourceChangePresentation,
  getResourceChangeSourceAdapterPresentation,
  getResourceChangeSourceTypePresentation,
  sortResourceChangesByObservedAt,
} from '@/utils/resourceChangePresentation';
import { formatConfidencePercentage } from '@/utils/confidencePresentation';

describe('resourceChangePresentation utils', () => {
  it('formats canonical resource change kinds', () => {
    expect(formatResourceChangeKind('activity')).toBe('Activity');
    expect(formatResourceChangeKind('config_update')).toBe('Config update');
    expect(formatResourceChangeKind('metric_anomaly')).toBe('Metric anomaly');
    expect(formatResourceChangeKind('relationship_change')).toBe('Relationship change');
    expect(formatResourceChangeKind('alert_fired')).toBe('Alert fired');
    expect(formatResourceChangeKind('alert_snoozed')).toBe('Alert snoozed');
    expect(formatResourceChangeKind('alert_unsnoozed')).toBe('Alert resumed');
    expect(formatResourceChangeKind('runbook_executed')).toBe('Runbook executed');
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

    expect(
      formatResourceChangeHeadline({
        id: 'change-3',
        resourceId: 'storage:pool:data',
        kind: 'alert_resolved',
        reason: "Alert resolved: ZFS pool 'data' has errors",
        observedAt: '2026-03-18T12:00:00Z',
      } as never),
    ).toBe("Alert resolved: ZFS pool 'data' has errors");
  });

  it('exposes canonical kind, source type, and adapter presentations', () => {
    expect(getResourceChangeKindPresentation('restart')).toMatchObject({
      label: 'Restart',
      plural: 'Restarts',
    });
    expect(getResourceChangeKindPresentation('activity')).toMatchObject({
      label: 'Activity',
      plural: 'Activities',
    });
    expect(getResourceChangeKindPresentation('alert_resolved')).toMatchObject({
      label: 'Alert resolved',
      plural: 'Alerts resolved',
    });
    expect(getResourceChangeKindPresentation('alert_snoozed')).toMatchObject({
      label: 'Alert snoozed',
      plural: 'Alerts snoozed',
    });
    expect(getResourceChangeKindPresentation('alert_unsnoozed')).toMatchObject({
      label: 'Alert resumed',
      plural: 'Alerts resumed',
    });
    expect(getResourceChangeSourceTypePresentation('platform_event')).toMatchObject({
      label: 'Platform event',
      plural: 'Platform events',
    });
    expect(getResourceChangeSourceAdapterPresentation('proxmox_adapter')).toMatchObject({
      label: 'Proxmox adapter',
      plural: 'Proxmox adapters',
    });
    expect(getResourceChangeSourceAdapterPresentation('vmware_adapter')).toMatchObject({
      label: 'VMware adapter',
      plural: 'VMware adapters',
    });
  });

  it('presents an alert close that was not a recovery as a move', () => {
    const summary =
      'Alert moved to pve1 (Host Agent). This is not a recovery: check the agent for the current reading.';
    const moved = {
      id: 'change-moved',
      resourceId: 'node:pve1',
      kind: 'alert_resolved' as const,
      observedAt: '2026-10-06T15:00:00Z',
      sourceType: 'heuristic' as const,
      confidence: 'high' as const,
      reason: summary,
      metadata: { alert_resolution: 'moved_to_agent', alert_type: 'memory' },
    };

    expect(getResourceChangeAlertResolution(moved)).toBe('moved_to_agent');
    expect(getResourceChangePresentation(moved)).toEqual({
      label: 'Alert moved',
      plural: 'Alerts moved',
      className: 'bg-blue-100 text-blue-700 dark:bg-blue-900/25 dark:text-blue-300',
    });
    // The engine's account is the whole headline, never "Alert resolved: ...".
    expect(formatResourceChangeHeadline(moved)).toBe(summary);
    expect(formatResourceChangeHeadline({ ...moved, reason: undefined })).toBe(
      'Alert moved: node:pve1',
    );

    // A reason code this build does not know still never reads as a recovery.
    expect(
      getResourceChangePresentation({
        kind: 'alert_resolved',
        metadata: { alert_resolution: 'some_future_reason' },
      }).label,
    ).toBe('Alert closed');

    // Ordinary recoveries, and other kinds, keep their kind presentation.
    const recovered = {
      ...moved,
      metadata: { alert_type: 'memory' },
      reason: 'Node memory at 95%',
    };
    expect(getResourceChangeAlertResolution(recovered)).toBeUndefined();
    expect(getResourceChangePresentation(recovered)).toEqual(
      getResourceChangeKindPresentation('alert_resolved'),
    );
    expect(formatResourceChangeHeadline(recovered)).toBe('Alert resolved: Node memory at 95%');
    expect(
      getResourceChangePresentation({
        kind: 'alert_fired',
        metadata: { alert_resolution: 'moved_to_agent' },
      }).label,
    ).toBe('Alert fired');
  });

  it('formats shared confidence percentages', () => {
    expect(formatConfidencePercentage(0.5)).toBe('50%');
    expect(formatConfidencePercentage(0.875)).toBe('88%');
    expect(formatConfidencePercentage(0)).toBe('0%');
  });

  it('sorts recent changes canonically', () => {
    const sorted = sortResourceChangesByObservedAt([
      {
        id: 'change-b',
        resourceId: 'vm-42',
        kind: 'config_update',
        observedAt: '2026-03-18T12:00:00Z',
      } as never,
      {
        id: 'change-a',
        resourceId: 'vm-42',
        kind: 'config_update',
        observedAt: '2026-03-18T12:05:00Z',
      } as never,
      {
        id: 'change-c',
        resourceId: 'vm-42',
        kind: 'config_update',
        observedAt: '2026-03-18T12:05:00Z',
      } as never,
    ]);

    expect(sorted.map((change) => change.id)).toEqual(['change-c', 'change-a', 'change-b']);
  });
});
