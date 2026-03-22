import { describe, expect, it } from 'vitest';

import type { AlertConfig } from '@/types/alerts';

import {
  ALERT_DOCKER_GAP_VALIDATION_ERROR,
  buildAlertsConfigurationPayload,
  createDefaultAlertsConfigurationSnapshot,
  readAlertsConfigurationSnapshot,
} from '../alertsConfigurationModel';

describe('alertsConfigurationModel', () => {
  it('creates the canonical default snapshot', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();

    expect(snapshot.guestDefaults.cpu).toBe(80);
    expect(snapshot.dockerDefaults.serviceWarnGapPercent).toBe(10);
    expect(snapshot.timeThresholds.guest).toBe(5);
    expect(snapshot.scheduleCooldown.enabled).toBe(true);
    expect(snapshot.backupDefaults.ignoreVMIDs).toEqual([]);
  });

  it('normalizes alert config into the runtime snapshot', () => {
    const config = {
      enabled: true,
      guestDefaults: {
        cpu: { trigger: 91, clear: 86 },
        disableConnectivity: true,
        poweredOffSeverity: 'critical',
      },
      nodeDefaults: {},
      agentDefaults: {},
      storageDefault: { trigger: 88, clear: 83 },
      overrides: {},
      dockerDefaults: {
        serviceWarnGapPercent: 40,
        serviceCriticalGapPercent: 10,
        statePoweredOffSeverity: 'critical',
      },
      backupDefaults: {
        enabled: true,
        warningDays: 30,
        criticalDays: 10,
        ignoreVMIDs: [' 101 ', '101', ''],
      },
      snapshotDefaults: {
        enabled: true,
        warningDays: 25,
        criticalDays: 5,
      },
      schedule: {
        quietHours: {
          enabled: true,
          start: '21:00',
          end: '06:00',
          days: [1, 3, 5],
        },
        cooldown: 45,
        maxAlertsHour: 7,
      },
    } as AlertConfig;

    const snapshot = readAlertsConfigurationSnapshot(config);

    expect(snapshot.guestDefaults.cpu).toBe(91);
    expect(snapshot.guestDisableConnectivity).toBe(true);
    expect(snapshot.guestPoweredOffSeverity).toBe('critical');
    expect(snapshot.dockerDefaults.serviceWarnGapPercent).toBe(40);
    expect(snapshot.dockerDefaults.serviceCriticalGapPercent).toBe(40);
    expect(snapshot.backupDefaults.warningDays).toBe(10);
    expect(snapshot.backupDefaults.criticalDays).toBe(10);
    expect(snapshot.backupDefaults.ignoreVMIDs).toEqual(['101']);
    expect(snapshot.snapshotDefaults.warningDays).toBe(5);
    expect(snapshot.snapshotDefaults.criticalDays).toBe(5);
    expect(snapshot.scheduleQuietHours.days).toEqual({
      sunday: false,
      monday: true,
      tuesday: false,
      wednesday: true,
      thursday: false,
      friday: true,
      saturday: false,
    });
    expect(snapshot.scheduleCooldown.maxAlerts).toBe(7);
  });

  it('builds the canonical save payload from the runtime snapshot', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.guestDefaults.cpu = 92;
    snapshot.dockerIgnoredPrefixes = [' web ', ''];
    snapshot.guestTagWhitelist = [' prod ', ''];
    snapshot.metricTimeThresholds = {
      Guest: { CPU: 17 },
    } as Record<string, Record<string, number>>;
    snapshot.scheduleCooldown = {
      enabled: true,
      minutes: 47,
      maxAlerts: 99,
    };
    snapshot.scheduleGrouping = {
      enabled: true,
      window: 2,
      byNode: true,
      byGuest: false,
    };
    snapshot.backupDefaults = {
      enabled: true,
      warningDays: 30,
      criticalDays: 10,
      freshHours: 12,
      staleHours: 48,
      alertOrphaned: true,
      ignoreVMIDs: [' 101 ', ''],
    };

    const result = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: { 'guest-1': { disabled: true } },
      alertsActivationState: 'active',
      alertsActivationConfig: {
        enabled: true,
        activationTime: '2026-03-22T10:00:00Z',
        observationWindowHours: 24,
      },
    });

    expect(result.dockerValidationError).toBeUndefined();
    expect(result.alertConfig).toBeDefined();
    expect(result.alertConfig?.guestDefaults.cpu).toEqual({ trigger: 92, clear: 87 });
    expect(result.alertConfig?.dockerIgnoredContainerPrefixes).toEqual(['web']);
    expect(result.alertConfig?.guestTagWhitelist).toEqual(['prod']);
    expect(result.alertConfig?.metricTimeThresholds).toEqual({ guest: { cpu: 17 } });
    expect(result.alertConfig?.schedule?.cooldown).toBe(47);
    expect(result.alertConfig?.schedule?.maxAlertsHour).toBe(10);
    expect(result.alertConfig?.schedule?.grouping).toEqual({
      enabled: true,
      window: 120,
      byNode: true,
      byGuest: false,
    });
    expect(result.alertConfig?.backupDefaults).toMatchObject({
      warningDays: 10,
      criticalDays: 10,
      ignoreVMIDs: ['101'],
    });
    expect(result.alertConfig?.overrides).toEqual({ 'guest-1': { disabled: true } });
  });

  it('returns the canonical docker-gap validation error before save', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.dockerDefaults.serviceWarnGapPercent = 60;
    snapshot.dockerDefaults.serviceCriticalGapPercent = 40;

    const result = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(result.alertConfig).toBeUndefined();
    expect(result.dockerValidationError).toBe(ALERT_DOCKER_GAP_VALIDATION_ERROR);
  });
});
