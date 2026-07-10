import { describe, expect, it } from 'vitest';

import type { AlertConfig } from '@/types/alerts';

import {
  buildAlertsConfigurationPayload,
  createDefaultAlertsConfigurationSnapshot,
  FACTORY_BACKUP_DEFAULTS,
  FACTORY_DOCKER_DEFAULTS,
  readAlertsConfigurationSnapshot,
} from '../alertsConfigurationModel';

// The functions under test (createHysteresisThreshold, normalizeGap,
// normalizeWarningCriticalPair, normalizeStringList, cloneDays, cloneBackupDefaults)
// are all module-private. Each is exercised below through its public entry point:
// createDefaultAlertsConfigurationSnapshot / readAlertsConfigurationSnapshot /
// buildAlertsConfigurationPayload.
const buildFromSnapshot = (snapshot: ReturnType<typeof createDefaultAlertsConfigurationSnapshot>) =>
  buildAlertsConfigurationPayload({
    snapshot,
    rawOverridesConfig: {},
    alertsActivationState: null,
    alertsActivationConfig: null,
  });

describe('createHysteresisThreshold (via buildAlertsConfigurationPayload)', () => {
  it('coerces a non-number trigger to 0 and clamps the clear floor at 0', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.guestDefaults.cpu = undefined; // non-number branch -> normalized 0; clear 0-5 -> clamped 0
    snapshot.guestDefaults.memory = 3; // number below default margin -> clear clamps to 0
    snapshot.guestDefaults.disk = 5; // exactly at margin -> clear is exactly 0
    snapshot.guestDefaults.diskRead = 6; // just above margin -> clear is 1

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(alertConfig?.guestDefaults?.cpu).toEqual({ trigger: 0, clear: 0 });
    expect(alertConfig?.guestDefaults?.memory).toEqual({ trigger: 3, clear: 0 });
    expect(alertConfig?.guestDefaults?.disk).toEqual({ trigger: 5, clear: 0 });
    expect(alertConfig?.guestDefaults?.diskRead).toEqual({ trigger: 6, clear: 1 });
  });

  it('applies the threshold factory to each diskTempByType entry', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.diskTempByType = { hot: 7, sub: 2 };

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(alertConfig?.diskTempByType?.hot).toEqual({ trigger: 7, clear: 2 });
    expect(alertConfig?.diskTempByType?.sub).toEqual({ trigger: 2, clear: 0 });
  });
});

describe('normalizeGap (via readAlertsConfigurationSnapshot dockerDefaults)', () => {
  // serviceWarnGapPercent flows 1:1 into snapshot.dockerDefaults.serviceWarnGapPercent, so it
  // exposes normalizeGap's output directly (independent of the warn>critical clamp downstream).

  it('falls back to the factory default for non-finite-coercing inputs', () => {
    const fallback = FACTORY_DOCKER_DEFAULTS.serviceWarnGapPercent;
    const badValues: unknown[] = ['abc', Infinity, -Infinity, undefined, {}, NaN];

    badValues.forEach((bad) => {
      const config = {
        overrides: {},
        dockerDefaults: { serviceWarnGapPercent: bad },
      } as unknown as AlertConfig;
      const snapshot = readAlertsConfigurationSnapshot(config);
      expect(snapshot.dockerDefaults.serviceWarnGapPercent).toBe(fallback);
    });
  });

  it('clamps values below 0 up to 0 and above 100 down to 100', () => {
    const low = {
      overrides: {},
      dockerDefaults: { serviceWarnGapPercent: -5 },
    } as AlertConfig;
    expect(readAlertsConfigurationSnapshot(low).dockerDefaults.serviceWarnGapPercent).toBe(0);

    const high = {
      overrides: {},
      dockerDefaults: { serviceWarnGapPercent: 150 },
    } as AlertConfig;
    expect(readAlertsConfigurationSnapshot(high).dockerDefaults.serviceWarnGapPercent).toBe(100);
  });

  it('keeps in-range numeric values (including numeric strings) and boundaries', () => {
    expect(
      readAlertsConfigurationSnapshot({
        overrides: {},
        dockerDefaults: { serviceWarnGapPercent: 0 },
      } as AlertConfig).dockerDefaults.serviceWarnGapPercent,
    ).toBe(0);

    expect(
      readAlertsConfigurationSnapshot({
        overrides: {},
        dockerDefaults: { serviceWarnGapPercent: 100 },
      } as AlertConfig).dockerDefaults.serviceWarnGapPercent,
    ).toBe(100);

    expect(
      readAlertsConfigurationSnapshot({
        overrides: {},
        dockerDefaults: { serviceWarnGapPercent: 42 },
      } as AlertConfig).dockerDefaults.serviceWarnGapPercent,
    ).toBe(42);

    // Number('24') === 24 -> finite, in range, so the string is coerced (not fallback).
    expect(
      readAlertsConfigurationSnapshot({
        overrides: {},
        dockerDefaults: { serviceWarnGapPercent: '24' },
      } as unknown as AlertConfig).dockerDefaults.serviceWarnGapPercent,
    ).toBe(24);
  });
});

describe('normalizeWarningCriticalPair', () => {
  it('keeps warning unchanged when warning is strictly less than critical', () => {
    const snapshot = readAlertsConfigurationSnapshot({
      overrides: {},
      backupDefaults: { enabled: true, warningDays: 5, criticalDays: 10 },
    } as AlertConfig);

    expect(snapshot.backupDefaults.warningDays).toBe(5);
    expect(snapshot.backupDefaults.criticalDays).toBe(10);
  });

  it('treats equal warning/critical as not strictly greater (no clamp)', () => {
    const snapshot = readAlertsConfigurationSnapshot({
      overrides: {},
      backupDefaults: { enabled: true, warningDays: 7, criticalDays: 7 },
    } as AlertConfig);

    expect(snapshot.backupDefaults.warningDays).toBe(7);
    expect(snapshot.backupDefaults.criticalDays).toBe(7);
  });

  it('clamps negative warning and critical inputs to 0 and collapses critical to warning', () => {
    const snapshot = readAlertsConfigurationSnapshot({
      overrides: {},
      backupDefaults: { enabled: true, warningDays: -3, criticalDays: -9 },
    } as AlertConfig);

    expect(snapshot.backupDefaults.warningDays).toBe(0);
    expect(snapshot.backupDefaults.criticalDays).toBe(0);
  });

  it('defaults an undefined warning to 0 while preserving a positive critical', () => {
    const snapshot = readAlertsConfigurationSnapshot({
      overrides: {},
      backupDefaults: { enabled: true, warningDays: undefined, criticalDays: 14 },
    } as unknown as AlertConfig);

    expect(snapshot.backupDefaults.warningDays).toBe(0);
    expect(snapshot.backupDefaults.criticalDays).toBe(14);
  });

  it('inherits critical from warning when critical is 0 (build snapshotDefaults path)', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.snapshotDefaults = { enabled: true, warningDays: 15, criticalDays: 0 };

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(alertConfig?.snapshotDefaults?.warningDays).toBe(15);
    expect(alertConfig?.snapshotDefaults?.criticalDays).toBe(15);
  });

  it('defaults an undefined critical to 0 and inherits the warning value', () => {
    const snapshot = readAlertsConfigurationSnapshot({
      overrides: {},
      backupDefaults: { enabled: true, warningDays: 4, criticalDays: undefined },
    } as unknown as AlertConfig);

    expect(snapshot.backupDefaults.warningDays).toBe(4);
    expect(snapshot.backupDefaults.criticalDays).toBe(4);
  });
});

describe('normalizeStringList', () => {
  it('returns an empty array for undefined input on the build path', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.dockerIgnoredPrefixes = undefined as unknown as string[];

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(alertConfig?.dockerIgnoredContainerPrefixes).toEqual([]);
  });

  it('trims entries, drops empty/whitespace-only items, and preserves internal spacing', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.guestTagWhitelist = ['  prod  ', '\t', '', '   ', 'web app', 'db'];

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(alertConfig?.guestTagWhitelist).toEqual(['prod', 'web app', 'db']);
  });

  it('does NOT dedupe on the build path (duplicates survive, only trimmed/filtered)', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.backupDefaults = {
      ...snapshot.backupDefaults,
      ignoreVMIDs: ['1', '  1 ', '2', '1'],
    };

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(alertConfig?.backupDefaults?.ignoreVMIDs).toEqual(['1', '1', '2', '1']);
  });

  it('dedupes (after trim) on the read path for backup ignoreVMIDs', () => {
    const snapshot = readAlertsConfigurationSnapshot({
      overrides: {},
      backupDefaults: {
        enabled: true,
        warningDays: 7,
        criticalDays: 14,
        ignoreVMIDs: ['  1 ', '1', '2', '2', ''],
      },
    } as AlertConfig);

    expect(snapshot.backupDefaults.ignoreVMIDs).toEqual(['1', '2']);
  });
});

describe('cloneDays (via buildAlertsConfigurationPayload schedule.quietHours.days)', () => {
  it('produces a shallow copy that equals but is not the source reference', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    const sourceDays = snapshot.scheduleQuietHours.days;

    const { alertConfig } = buildFromSnapshot(snapshot);
    const payloadDays = alertConfig?.schedule?.quietHours?.days;

    expect(payloadDays).toEqual(sourceDays);
    expect(payloadDays).not.toBe(sourceDays);
  });

  it('isolates payload-side mutations from the snapshot', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    const beforeFriday = snapshot.scheduleQuietHours.days.friday;

    const { alertConfig } = buildFromSnapshot(snapshot);
    const payloadDays = alertConfig?.schedule?.quietHours?.days as Record<string, boolean>;
    payloadDays.friday = !beforeFriday;

    expect(snapshot.scheduleQuietHours.days.friday).toBe(beforeFriday);
  });

  it('spreads arbitrary day keys through the clone', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.scheduleQuietHours.days = {
      ...snapshot.scheduleQuietHours.days,
      customHoliday: true,
    };

    const { alertConfig } = buildFromSnapshot(snapshot);

    expect(
      (alertConfig?.schedule?.quietHours?.days as Record<string, boolean>)?.customHoliday,
    ).toBe(true);
  });
});

describe('cloneBackupDefaults (via createDefaultAlertsConfigurationSnapshot)', () => {
  it('returns a backupDefaults object that is a value-equal but distinct clone', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();

    expect(snapshot.backupDefaults).not.toBe(FACTORY_BACKUP_DEFAULTS);
    expect(snapshot.backupDefaults.enabled).toBe(FACTORY_BACKUP_DEFAULTS.enabled);
    expect(snapshot.backupDefaults.warningDays).toBe(FACTORY_BACKUP_DEFAULTS.warningDays);
    expect(snapshot.backupDefaults.ignoreVMIDs).not.toBe(FACTORY_BACKUP_DEFAULTS.ignoreVMIDs);
    expect(snapshot.backupDefaults.ignoreVMIDs).toEqual([]);
  });

  it('isolates ignoreVMIDs mutations from the factory default', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    // cloneBackupDefaults always yields a defined array (FACTORY_BACKUP_DEFAULTS ships []).
    const clonedIgnoreVMIDs = snapshot.backupDefaults.ignoreVMIDs as string[];
    clonedIgnoreVMIDs.push('999');

    expect(FACTORY_BACKUP_DEFAULTS.ignoreVMIDs).toEqual([]);
  });
});
