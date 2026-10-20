import { createRoot, createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Alert, State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { useStorageAlertState } from '@/components/Storage/useStorageAlertState';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';
import { getStorageRecordAlertResourceIds } from '@/features/storageBackups/storageAlertState';
import capturedFrames from '../../../../browser-tests/unraid-storage-frames.json';

// Unmodified synthetic CheckHost -> registry -> broadcast output retained by
// source-o5sso26j. This binds the consumer to the producer's actual key, not
// to another hand-written spelling of the alert identity.
const frames = capturedFrames as unknown as Record<string, State>;
const frame = frames['assigned-missing'];
const storage = (frame.resources as Resource[]).find((resource) => resource.type === 'storage')!;
const map = (resource: Resource) => buildStorageRecords({ state: frame, resources: [resource] })[0];
const makeControls = () => [
  map({
    ...storage,
    id: 'other-host',
    metricsTarget: { resourceType: 'storage', resourceId: 'other-machine/storage:unraid-array' },
  }),
  map({
    ...storage,
    id: 'other-array',
    metricsTarget: { resourceType: 'storage', resourceId: 'unraid-machine/storage:other-array' },
  }),
  map({ ...storage, id: 'api-lookalike', sourceType: 'api' }),
];

describe('agent storage alert identity', () => {
  it('carries the exact producer identity separately from metrics and registry ids', () => {
    const record = map(storage);
    const alert = frame.activeAlerts[0];
    expect(record.id).toBe('storage-384661c4e7c2f845');
    expect(record.refs?.resourceId).toBe('unraid-machine/storage:unraid-array');
    expect(record.metricsTarget).toEqual(storage.metricsTarget);
    expect(record.refs?.alertResourceId).toBe(alert.resourceId);
    expect(getStorageRecordAlertResourceIds(record)).toContain(alert.resourceId);
  });

  it.each(Object.entries(frames))(
    'consumes captured %s without hiding real failures',
    (mode, state) => {
      createRoot((dispose) => {
        try {
          const records = buildStorageRecords({ state, resources: state.resources as Resource[] });
          const { getRecordAlertState } = useStorageAlertState({
            records: () => records,
            activeAlerts: () => state.activeAlerts,
            alertsEnabled: () => true,
          });
          const row = records.find((record) => record.details?.type === 'unraid-array')!;
          const expectedAlert = state.activeAlerts.find(
            (alert) => alert.type === 'storage-topology',
          );
          expect(getRecordAlertState(row.id)).toMatchObject({
            hasAlert: Boolean(expectedAlert),
            severity: expectedAlert?.level ?? null,
            alertCount: expectedAlert ? 1 : 0,
            hasUnacknowledgedAlert: Boolean(expectedAlert),
          });
          if (expectedAlert) expect(getRecordAlertState(row.id).headline).toBeTruthy();
          else expect(getRecordAlertState(row.id).headline).toBeNull();
          // The fixture's fourteen healthy/recovery consumer states are not
          // allowed to become a blanket Unraid alarm suppression rule.
          expect(Boolean(expectedAlert)).toBe(
            mode !== 'legacy-17-empty' && mode !== 'fresh-agent' && !mode.endsWith('-recovered'),
          );
        } finally {
          dispose();
        }
      });
    },
  );

  it('does not borrow matching display names, node names, host roots or other array keys', () => {
    createRoot((dispose) => {
      try {
        const controls = makeControls();
        const row = map(storage);
        expect(controls.every((control) => control.name === row.name)).toBe(true);
        expect(controls.every((control) => control.location.label === row.location.label)).toBe(
          true,
        );
        const unrelated = [
          { ...frame.activeAlerts[0], id: 'machine', resourceId: 'agent:unraid-machine' },
          {
            ...frame.activeAlerts[0],
            id: 'component',
            resourceId: 'agent:unraid-machine/raid:md0',
          },
        ];
        const [alerts, setAlerts] = createSignal<Alert[]>(frame.activeAlerts);
        const { getRecordAlertState } = useStorageAlertState({
          records: () => [row, ...controls],
          activeAlerts: alerts,
          alertsEnabled: () => true,
        });
        expect(getRecordAlertState(row.id).alertCount).toBe(1);
        for (const control of controls)
          expect(getRecordAlertState(control.id).hasAlert).toBe(false);
        setAlerts(unrelated);
        for (const record of [row, ...controls]) {
          expect(getRecordAlertState(record.id).hasAlert).toBe(false);
          expect(getRecordAlertState(record.id).headline).toBeNull();
        }
      } finally {
        dispose();
      }
    });
  });

  it('preserves acknowledgement, detection-off, and ordinary replacement/recovery', () => {
    createRoot((dispose) => {
      try {
        const row = map(storage);
        const [alerts, setAlerts] = createSignal<Alert[]>(frame.activeAlerts);
        const [enabled, setEnabled] = createSignal(true);
        const { getRecordAlertState } = useStorageAlertState({
          records: () => [row],
          activeAlerts: alerts,
          alertsEnabled: enabled,
        });
        expect(getRecordAlertState(row.id)).toMatchObject({
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        });
        setAlerts(frame.activeAlerts.map((alert) => ({ ...alert, acknowledged: true })));
        expect(getRecordAlertState(row.id)).toMatchObject({
          hasAlert: true,
          alertCount: 1,
          hasAcknowledgedOnlyAlert: true,
          acknowledgedCount: 1,
          unacknowledgedCount: 0,
          // Acknowledged incidents remain counted, but do not tint the row.
          severity: null,
        });
        // Existing acknowledgement semantics retire the unacknowledged reason.
        expect(getRecordAlertState(row.id).headline).toBeNull();
        setAlerts(frame.activeAlerts);
        expect(getRecordAlertState(row.id)).toMatchObject({
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          hasAcknowledgedOnlyAlert: false,
          acknowledgedCount: 0,
        });
        expect(getRecordAlertState(row.id).headline).toBeTruthy();
        setEnabled(false);
        expect(getRecordAlertState(row.id)).toMatchObject({ hasAlert: false, headline: null });
        setEnabled(true);
        expect(getRecordAlertState(row.id)).toMatchObject({
          hasAlert: true,
          severity: 'critical',
        });
        setAlerts([]);
        expect(getRecordAlertState(row.id)).toMatchObject({
          hasAlert: false,
          alertCount: 0,
          headline: null,
          severity: null,
        });
      } finally {
        dispose();
      }
    });
  });

  it.each([
    { sourceType: 'api' as const },
    { metricsTarget: undefined },
    { metricsTarget: { resourceType: 'agent' as const, resourceId: 'unraid-machine' } },
    { metricsTarget: { resourceType: 'storage' as const, resourceId: '   ' } },
    { storage: { type: 'zfspool', platform: 'truenas' }, technology: 'truenas', platformData: {} },
  ])('does not invent an agent assessment identity for %j', (overrides) => {
    const record = map({ ...storage, ...overrides } as Resource);
    expect(record.refs?.alertResourceId).toBeUndefined();
    expect(getStorageRecordAlertResourceIds(record)).not.toContain(
      frame.activeAlerts[0].resourceId,
    );
  });

  it('retains the assessment identity when duplicate observations are folded', () => {
    const records = buildStorageRecords({ state: frame, resources: [storage, { ...storage }] });
    expect(records).toHaveLength(1);
    expect(getStorageRecordAlertResourceIds(records[0])).toContain(
      frame.activeAlerts[0].resourceId,
    );
  });
});
