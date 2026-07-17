import { describe, expect, it } from 'vitest';

import type { AlertConfig } from '@/types/alerts';

import {
  buildAlertsConfigurationPayload,
  createDefaultAlertsConfigurationSnapshot,
  readAlertsConfigurationSnapshot,
} from '../alertsConfigurationModel';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
/** Build a minimal AlertConfig from extra fields, using `as unknown as` so
 *  we can omit required-but-irrelevant props and inject wrong-typed values. */
const cfg = (extra: Record<string, unknown>): AlertConfig =>
  ({ overrides: {}, ...extra } as unknown as AlertConfig);

const localTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

/** A hysteresis object whose `trigger` key exists but the value is undefined.
 *  getTriggerValue returns `undefined` for this, which makes the
 *  `getTriggerValue(x) ?? FACTORY` chains in readAlertsConfigurationSnapshot
 *  actually evaluate their right-hand fallback — the only way that branch fires. */
const UNDEF_TRIGGER = { trigger: undefined };

// ===========================================================================
// readAlertsConfigurationSnapshot
// ===========================================================================

describe('readAlertsConfigurationSnapshot — absent optional sections', () => {
  const snapshot = readAlertsConfigurationSnapshot(cfg({}));

  it('keeps factory guestDefaults / nodeDefaults / agentDefaults', () => {
    expect(snapshot.guestDefaults.cpu).toBe(80);
    expect(snapshot.guestDefaults.memory).toBe(85);
    expect(snapshot.guestDefaults.disk).toBe(90);
    expect(snapshot.guestDefaults.diskRead).toBe(-1);
    expect(snapshot.nodeDefaults.temperature).toBe(80);
    expect(snapshot.agentDefaults.diskTemperature).toBe(55);
  });

  it('keeps factory pbsDefaults / kubernetesDefaults', () => {
    expect(snapshot.pbsDefaults.cpu).toBe(80);
    expect(snapshot.pbsDefaults.memory).toBe(85);
    expect(snapshot.kubernetesDefaults.cpu).toBe(80);
    expect(snapshot.kubernetesDefaults.disk).toBe(90);
    expect(snapshot.kubernetesDefaults.diskRead).toBe(-1);
  });

  it('keeps factory trueNASDefaults / trueNASDiskDefaults / vmwareDefaults', () => {
    expect(snapshot.trueNASDefaults.cpu).toBe(80);
    expect(snapshot.trueNASDefaults.usage).toBe(85);
    expect(snapshot.trueNASDefaults.diskRead).toBe(-1);
    expect(snapshot.trueNASDiskDefaults.temperature).toBe(55);
    expect(snapshot.vmwareDefaults.cpu).toBe(80);
    expect(snapshot.vmwareDefaults.usage).toBe(85);
  });

  it('keeps factory diskTempByType and empty diskFillByType', () => {
    expect(snapshot.diskTempByType).toEqual({ nvme: 70, sas: 65, sata: 55 });
    expect(snapshot.diskFillByType).toEqual({});
  });

  it('keeps factory dockerDefaults / storageDefault', () => {
    expect(snapshot.dockerDefaults.cpu).toBe(80);
    expect(snapshot.dockerDefaults.serviceWarnGapPercent).toBe(10);
    expect(snapshot.dockerDisableConnectivity).toBe(false);
    expect(snapshot.dockerPoweredOffSeverity).toBe('warning');
    expect(snapshot.storageDefault).toBe(85);
  });

  it('keeps factory backupDefaults / snapshotDefaults / pmgThresholds', () => {
    expect(snapshot.backupDefaults.enabled).toBe(false);
    expect(snapshot.backupDefaults.freshHours).toBe(24);
    expect(snapshot.snapshotDefaults.enabled).toBe(false);
    expect(snapshot.snapshotDefaults.warningDays).toBe(30);
    expect(snapshot.pmgThresholds.queueTotalWarning).toBe(500);
    expect(snapshot.pmgThresholds.quarantineGrowthCritMin).toBe(500);
  });

  it('keeps default timeThresholds / metricTimeThresholds', () => {
    expect(snapshot.timeThresholds.guest).toBe(5);
    expect(snapshot.timeThresholds.pod).toBe(5);
    expect(snapshot.timeThresholds['vmware-network']).toBe(5);
    expect(snapshot.metricTimeThresholds).toEqual({});
  });

  it('keeps default schedule sections when schedule is absent', () => {
    expect(snapshot.scheduleQuietHours.enabled).toBe(false);
    expect(snapshot.scheduleCooldown.enabled).toBe(true);
    expect(snapshot.scheduleGrouping.enabled).toBe(true);
    expect(snapshot.scheduleEscalation.enabled).toBe(false);
    expect(snapshot.notifyOnResolve).toBe(true);
  });

  it('defaults all disableAll flags to false', () => {
    expect(snapshot.disableAllNodes).toBe(false);
    expect(snapshot.disableAllGuests).toBe(false);
    expect(snapshot.disableAllAgents).toBe(false);
    expect(snapshot.disableAllStorage).toBe(false);
    expect(snapshot.disableAllPBS).toBe(false);
    expect(snapshot.disableAllPMG).toBe(false);
    expect(snapshot.disableAllDockerHosts).toBe(false);
    expect(snapshot.disableAllDockerServices).toBe(false);
    expect(snapshot.disableAllDockerContainers).toBe(false);
    expect(snapshot.disableAllKubernetes).toBe(false);
    expect(snapshot.disableAllTrueNAS).toBe(false);
    expect(snapshot.disableAllVMware).toBe(false);
    expect(snapshot.disableAllNodesOffline).toBe(false);
    expect(snapshot.disableAllGuestsOffline).toBe(false);
    expect(snapshot.disableAllAgentsOffline).toBe(false);
    expect(snapshot.disableAllPBSOffline).toBe(false);
    expect(snapshot.disableAllPMGOffline).toBe(false);
    expect(snapshot.disableAllDockerHostsOffline).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Factory-fallback via hysteresis with undefined trigger
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — factory fallback (undefined-trigger hysteresis)', () => {
  // Every field below is a hysteresis object whose `trigger` key exists but is
  // undefined. getTriggerValue returns `undefined`, so the `?? FACTORY` chain
  // evaluates its right-hand side and the factory default is used.
  const snapshot = readAlertsConfigurationSnapshot(
    cfg({
      guestDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
        diskRead: UNDEF_TRIGGER,
        diskWrite: UNDEF_TRIGGER,
        networkIn: UNDEF_TRIGGER,
        networkOut: UNDEF_TRIGGER,
      },
      nodeDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
        temperature: UNDEF_TRIGGER,
      },
      pbsDefaults: { cpu: UNDEF_TRIGGER, memory: UNDEF_TRIGGER },
      kubernetesDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
        diskRead: UNDEF_TRIGGER,
        diskWrite: UNDEF_TRIGGER,
        networkIn: UNDEF_TRIGGER,
        networkOut: UNDEF_TRIGGER,
      },
      truenasDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
        usage: UNDEF_TRIGGER,
        temperature: UNDEF_TRIGGER,
        diskRead: UNDEF_TRIGGER,
        diskWrite: UNDEF_TRIGGER,
        networkIn: UNDEF_TRIGGER,
        networkOut: UNDEF_TRIGGER,
      },
      truenasDiskDefaults: { temperature: UNDEF_TRIGGER },
      vmwareDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
        usage: UNDEF_TRIGGER,
        diskRead: UNDEF_TRIGGER,
        diskWrite: UNDEF_TRIGGER,
        networkIn: UNDEF_TRIGGER,
        networkOut: UNDEF_TRIGGER,
      },
      agentDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
        diskTemperature: UNDEF_TRIGGER,
      },
      dockerDefaults: {
        cpu: UNDEF_TRIGGER,
        memory: UNDEF_TRIGGER,
        disk: UNDEF_TRIGGER,
      },
      storageDefault: UNDEF_TRIGGER,
    }),
  );

  it('falls back to factory defaults for guestDefaults fields', () => {
    expect(snapshot.guestDefaults.cpu).toBe(80);
    expect(snapshot.guestDefaults.memory).toBe(85);
    expect(snapshot.guestDefaults.disk).toBe(90);
    expect(snapshot.guestDefaults.diskRead).toBe(-1);
    expect(snapshot.guestDefaults.diskWrite).toBe(-1);
    expect(snapshot.guestDefaults.networkIn).toBe(-1);
    expect(snapshot.guestDefaults.networkOut).toBe(-1);
  });

  it('falls back to factory defaults for nodeDefaults fields', () => {
    expect(snapshot.nodeDefaults.cpu).toBe(80);
    expect(snapshot.nodeDefaults.memory).toBe(85);
    expect(snapshot.nodeDefaults.disk).toBe(90);
    expect(snapshot.nodeDefaults.temperature).toBe(80);
  });

  it('falls back to factory defaults for pbsDefaults and kubernetesDefaults', () => {
    expect(snapshot.pbsDefaults.cpu).toBe(80);
    expect(snapshot.pbsDefaults.memory).toBe(85);
    expect(snapshot.kubernetesDefaults.cpu).toBe(80);
    expect(snapshot.kubernetesDefaults.memory).toBe(85);
    expect(snapshot.kubernetesDefaults.disk).toBe(90);
    expect(snapshot.kubernetesDefaults.diskRead).toBe(-1);
    expect(snapshot.kubernetesDefaults.diskWrite).toBe(-1);
    expect(snapshot.kubernetesDefaults.networkIn).toBe(-1);
    expect(snapshot.kubernetesDefaults.networkOut).toBe(-1);
  });

  it('falls back to factory defaults for truenasDefaults and truenasDiskDefaults', () => {
    expect(snapshot.trueNASDefaults.cpu).toBe(80);
    expect(snapshot.trueNASDefaults.memory).toBe(85);
    expect(snapshot.trueNASDefaults.disk).toBe(85);
    expect(snapshot.trueNASDefaults.usage).toBe(85);
    expect(snapshot.trueNASDefaults.temperature).toBe(80);
    expect(snapshot.trueNASDefaults.diskRead).toBe(-1);
    expect(snapshot.trueNASDefaults.diskWrite).toBe(-1);
    expect(snapshot.trueNASDefaults.networkIn).toBe(-1);
    expect(snapshot.trueNASDefaults.networkOut).toBe(-1);
    expect(snapshot.trueNASDiskDefaults.temperature).toBe(55);
  });

  it('falls back to factory defaults for vmwareDefaults and agentDefaults', () => {
    expect(snapshot.vmwareDefaults.cpu).toBe(80);
    expect(snapshot.vmwareDefaults.memory).toBe(85);
    expect(snapshot.vmwareDefaults.disk).toBe(90);
    expect(snapshot.vmwareDefaults.usage).toBe(85);
    expect(snapshot.vmwareDefaults.diskRead).toBe(-1);
    expect(snapshot.vmwareDefaults.diskWrite).toBe(-1);
    expect(snapshot.vmwareDefaults.networkIn).toBe(-1);
    expect(snapshot.vmwareDefaults.networkOut).toBe(-1);
    expect(snapshot.agentDefaults.cpu).toBe(80);
    expect(snapshot.agentDefaults.memory).toBe(85);
    expect(snapshot.agentDefaults.disk).toBe(90);
    expect(snapshot.agentDefaults.diskTemperature).toBe(55);
  });

  it('falls back to factory defaults for dockerDefaults thresholds and storageDefault', () => {
    expect(snapshot.dockerDefaults.cpu).toBe(80);
    expect(snapshot.dockerDefaults.memory).toBe(85);
    expect(snapshot.dockerDefaults.disk).toBe(85);
    expect(snapshot.storageDefault).toBe(85);
  });
});

// ---------------------------------------------------------------------------
// Present-but-empty sections: getTriggerValue(undefined) returns 0, so
// the `?? FACTORY` chain does NOT fire — fields become 0.
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — present-but-empty section zeroes fields', () => {
  it('zeroes guestDefaults when the section object is empty', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ guestDefaults: {} }),
    );
    // getTriggerValue(undefined) = 0; 0 ?? 80 = 0 (the ?? never fires)
    expect(snapshot.guestDefaults.cpu).toBe(0);
    expect(snapshot.guestDefaults.memory).toBe(0);
    expect(snapshot.guestDefaults.disk).toBe(0);
    expect(snapshot.guestDefaults.diskRead).toBe(0);
    expect(snapshot.guestDefaults.diskWrite).toBe(0);
    expect(snapshot.guestDefaults.networkIn).toBe(0);
    expect(snapshot.guestDefaults.networkOut).toBe(0);
    expect(snapshot.guestDisableConnectivity).toBe(false);
    expect(snapshot.guestPoweredOffSeverity).toBe('warning');
  });

  it('zeroes nodeDefaults when the section object is empty', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ nodeDefaults: {} }),
    );
    expect(snapshot.nodeDefaults.cpu).toBe(0);
    expect(snapshot.nodeDefaults.memory).toBe(0);
    expect(snapshot.nodeDefaults.disk).toBe(0);
    expect(snapshot.nodeDefaults.temperature).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Present sections with real trigger values
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — guestDefaults specifics', () => {
  it('extracts poweredOffSeverity critical and disableConnectivity true', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        guestDefaults: {
          cpu: { trigger: 95, clear: 90 },
          memory: 88,
          disableConnectivity: true,
          poweredOffSeverity: 'critical',
        },
      }),
    );
    expect(snapshot.guestDefaults.cpu).toBe(95);
    expect(snapshot.guestDefaults.memory).toBe(88);
    expect(snapshot.guestDisableConnectivity).toBe(true);
    expect(snapshot.guestPoweredOffSeverity).toBe('critical');
  });

  it('defaults poweredOffSeverity to warning for non-critical values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        guestDefaults: { poweredOffSeverity: 'warning' },
      }),
    );
    expect(snapshot.guestPoweredOffSeverity).toBe('warning');
  });

  it('coerces falsy disableConnectivity values to false', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ guestDefaults: { disableConnectivity: 0 } }),
    );
    expect(snapshot.guestDisableConnectivity).toBe(false);
  });
});

describe('readAlertsConfigurationSnapshot — pbsDefaults', () => {
  it('extracts cpu and memory trigger values from hysteresis objects', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        pbsDefaults: {
          cpu: { trigger: 77, clear: 72 },
          memory: { trigger: 82, clear: 77 },
        },
      }),
    );
    expect(snapshot.pbsDefaults.cpu).toBe(77);
    expect(snapshot.pbsDefaults.memory).toBe(82);
  });
});

describe('readAlertsConfigurationSnapshot — kubernetesDefaults', () => {
  it('extracts all seven metric trigger values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        kubernetesDefaults: {
          cpu: { trigger: 75, clear: 70 },
          memory: { trigger: 78, clear: 73 },
          disk: { trigger: 82, clear: 77 },
          diskRead: { trigger: 100, clear: 95 },
          diskWrite: { trigger: 120, clear: 115 },
          networkIn: { trigger: 200, clear: 195 },
          networkOut: { trigger: 210, clear: 205 },
        },
      }),
    );
    expect(snapshot.kubernetesDefaults.cpu).toBe(75);
    expect(snapshot.kubernetesDefaults.memory).toBe(78);
    expect(snapshot.kubernetesDefaults.disk).toBe(82);
    expect(snapshot.kubernetesDefaults.diskRead).toBe(100);
    expect(snapshot.kubernetesDefaults.diskWrite).toBe(120);
    expect(snapshot.kubernetesDefaults.networkIn).toBe(200);
    expect(snapshot.kubernetesDefaults.networkOut).toBe(210);
  });
});

describe('readAlertsConfigurationSnapshot — truenasDefaults', () => {
  it('extracts all nine metric trigger values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        truenasDefaults: {
          cpu: { trigger: 71, clear: 66 },
          memory: { trigger: 76, clear: 71 },
          disk: { trigger: 81, clear: 76 },
          usage: { trigger: 86, clear: 81 },
          temperature: { trigger: 65, clear: 60 },
          diskRead: { trigger: 90, clear: 85 },
          diskWrite: { trigger: 95, clear: 90 },
          networkIn: { trigger: 150, clear: 145 },
          networkOut: { trigger: 160, clear: 155 },
        },
      }),
    );
    expect(snapshot.trueNASDefaults.cpu).toBe(71);
    expect(snapshot.trueNASDefaults.memory).toBe(76);
    expect(snapshot.trueNASDefaults.disk).toBe(81);
    expect(snapshot.trueNASDefaults.usage).toBe(86);
    expect(snapshot.trueNASDefaults.temperature).toBe(65);
    expect(snapshot.trueNASDefaults.diskRead).toBe(90);
    expect(snapshot.trueNASDefaults.diskWrite).toBe(95);
    expect(snapshot.trueNASDefaults.networkIn).toBe(150);
    expect(snapshot.trueNASDefaults.networkOut).toBe(160);
  });
});

describe('readAlertsConfigurationSnapshot — truenasDiskDefaults', () => {
  it('extracts temperature trigger value', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        truenasDiskDefaults: { temperature: { trigger: 50, clear: 45 } },
      }),
    );
    expect(snapshot.trueNASDiskDefaults.temperature).toBe(50);
  });
});

describe('readAlertsConfigurationSnapshot — vmwareDefaults full extraction', () => {
  it('extracts all eight metric trigger values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        vmwareDefaults: {
          cpu: { trigger: 73, clear: 68 },
          memory: { trigger: 79, clear: 74 },
          disk: { trigger: 83, clear: 78 },
          usage: { trigger: 87, clear: 82 },
          diskRead: { trigger: 110, clear: 105 },
          diskWrite: { trigger: 130, clear: 125 },
          networkIn: { trigger: 220, clear: 215 },
          networkOut: { trigger: 230, clear: 225 },
        },
      }),
    );
    expect(snapshot.vmwareDefaults.cpu).toBe(73);
    expect(snapshot.vmwareDefaults.memory).toBe(79);
    expect(snapshot.vmwareDefaults.disk).toBe(83);
    expect(snapshot.vmwareDefaults.usage).toBe(87);
    expect(snapshot.vmwareDefaults.diskRead).toBe(110);
    expect(snapshot.vmwareDefaults.diskWrite).toBe(130);
    expect(snapshot.vmwareDefaults.networkIn).toBe(220);
    expect(snapshot.vmwareDefaults.networkOut).toBe(230);
  });
});

describe('readAlertsConfigurationSnapshot — agentDefaults full extraction', () => {
  it('extracts all four metric trigger values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        agentDefaults: {
          cpu: { trigger: 72, clear: 67 },
          memory: { trigger: 77, clear: 72 },
          disk: { trigger: 82, clear: 77 },
          diskTemperature: { trigger: 60, clear: 55 },
        },
      }),
    );
    expect(snapshot.agentDefaults.cpu).toBe(72);
    expect(snapshot.agentDefaults.memory).toBe(77);
    expect(snapshot.agentDefaults.disk).toBe(82);
    expect(snapshot.agentDefaults.diskTemperature).toBe(60);
  });
});

// ---------------------------------------------------------------------------
// diskTempByType
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — diskTempByType', () => {
  it('normalizes keys to trimmed lowercase and keeps valid triggers', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        diskTempByType: {
          '  NVMe ': { trigger: 75, clear: 70 },
          SAS: { trigger: 68, clear: 63 },
          sata: { trigger: 58, clear: 53 },
        },
      }),
    );
    expect(snapshot.diskTempByType.nvme).toBe(75);
    expect(snapshot.diskTempByType.sas).toBe(68);
    expect(snapshot.diskTempByType.sata).toBe(58);
  });

  it('skips entries whose trigger is zero or negative', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        diskTempByType: {
          zero: 0,
          neg: { trigger: -5, clear: -10 },
          valid: { trigger: 60, clear: 55 },
        },
      }),
    );
    // 'zero' and 'neg' are dropped; factory defaults for nvme/sas/sata remain
    expect(snapshot.diskTempByType).not.toHaveProperty('zero');
    expect(snapshot.diskTempByType).not.toHaveProperty('neg');
    expect(snapshot.diskTempByType.valid).toBe(60);
    expect(snapshot.diskTempByType.nvme).toBe(70);
  });

  it('skips entries whose key is empty after trim', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        diskTempByType: {
          '   ': { trigger: 80, clear: 75 },
          real: { trigger: 65, clear: 60 },
        },
      }),
    );
    // The whitespace-only key normalises to '' which fails the
    // `normalizedKey && trigger > 0` guard, so only 'real' and the
    // factory defaults (nvme/sas/sata) survive.
    expect(Object.keys(snapshot.diskTempByType).sort()).toEqual(
      ['nvme', 'real', 'sas', 'sata'].sort(),
    );
    expect(snapshot.diskTempByType.real).toBe(65);
  });
});

// ---------------------------------------------------------------------------
// diskFillByType
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — diskFillByType', () => {
  it('copies the map as-is when present', () => {
    const fillMap = { ssd: { trigger: 90, clear: 85 }, hdd: { trigger: 95, clear: 90 } };
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ diskFillByType: fillMap }),
    );
    expect(snapshot.diskFillByType).toEqual(fillMap);
  });
});

// ---------------------------------------------------------------------------
// dockerDefaults
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — dockerDefaults field fallbacks', () => {
  it('falls back to factory for missing restartCount / restartWindow / memory pcts', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerDefaults: {
          cpu: { trigger: 85, clear: 80 },
          serviceWarnGapPercent: 20,
          serviceCriticalGapPercent: 30,
        },
      }),
    );
    expect(snapshot.dockerDefaults.restartCount).toBe(3);
    expect(snapshot.dockerDefaults.restartWindow).toBe(300);
    expect(snapshot.dockerDefaults.memoryWarnPct).toBe(90);
    expect(snapshot.dockerDefaults.memoryCriticalPct).toBe(95);
    expect(snapshot.dockerDefaults.cpu).toBe(85);
  });

  it('does not clamp critical gap when it is zero', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerDefaults: { serviceCriticalGapPercent: 0 },
      }),
    );
    expect(snapshot.dockerDefaults.serviceCriticalGapPercent).toBe(0);
  });

  it('does not clamp when warn <= critical', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerDefaults: { serviceWarnGapPercent: 20, serviceCriticalGapPercent: 40 },
      }),
    );
    expect(snapshot.dockerDefaults.serviceWarnGapPercent).toBe(20);
    expect(snapshot.dockerDefaults.serviceCriticalGapPercent).toBe(40);
  });

  it('coerces stateDisableConnectivity truthy and defaults poweredOffSeverity to warning', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerDefaults: {
          stateDisableConnectivity: 1,
          statePoweredOffSeverity: 'warning',
        },
      }),
    );
    expect(snapshot.dockerDisableConnectivity).toBe(true);
    expect(snapshot.dockerPoweredOffSeverity).toBe('warning');
  });

  it('reads statePoweredOffSeverity critical', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerDefaults: { statePoweredOffSeverity: 'critical' },
      }),
    );
    expect(snapshot.dockerPoweredOffSeverity).toBe('critical');
  });
});

// ---------------------------------------------------------------------------
// String-list fields
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — string-list null fallbacks', () => {
  it('defaults all four lists to empty arrays when null/undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerIgnoredContainerPrefixes: null,
        ignoredGuestPrefixes: undefined,
        guestTagWhitelist: null,
        guestTagBlacklist: undefined,
      }),
    );
    expect(snapshot.dockerIgnoredPrefixes).toEqual([]);
    expect(snapshot.ignoredGuestPrefixes).toEqual([]);
    expect(snapshot.guestTagWhitelist).toEqual([]);
    expect(snapshot.guestTagBlacklist).toEqual([]);
  });

  it('copies provided lists into snapshot fields verbatim', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        dockerIgnoredContainerPrefixes: ['redis-', 'nginx-'],
        ignoredGuestPrefixes: ['tmpl-'],
        guestTagWhitelist: ['prod'],
        guestTagBlacklist: ['deprecated'],
      }),
    );
    expect(snapshot.dockerIgnoredPrefixes).toEqual(['redis-', 'nginx-']);
    expect(snapshot.ignoredGuestPrefixes).toEqual(['tmpl-']);
    expect(snapshot.guestTagWhitelist).toEqual(['prod']);
    expect(snapshot.guestTagBlacklist).toEqual(['deprecated']);
  });
});

// ---------------------------------------------------------------------------
// storageDefault
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — storageDefault', () => {
  it('extracts trigger when storageDefault is a hysteresis object', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ storageDefault: { trigger: 92, clear: 87 } }),
    );
    expect(snapshot.storageDefault).toBe(92);
  });

  it('extracts trigger when storageDefault is a number (legacy)', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ storageDefault: 78 }),
    );
    expect(snapshot.storageDefault).toBe(78);
  });
});

// ---------------------------------------------------------------------------
// timeThresholds
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — timeThresholds', () => {
  it('extracts all eighteen time-threshold fields', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        timeThresholds: {
          guest: 10,
          node: 11,
          storage: 12,
          pbs: 13,
          agent: 14,
          'k8s-cluster': 15,
          'k8s-node': 16,
          'k8s-deployment': 17,
          'k8s-namespace': 18,
          pod: 19,
          'truenas-system': 20,
          'truenas-pool': 21,
          'truenas-dataset': 22,
          'truenas-disk': 23,
          'vmware-host': 24,
          'vmware-vm': 25,
          'vmware-datastore': 26,
          'vmware-network': 27,
        },
      }),
    );
    expect(snapshot.timeThresholds.guest).toBe(10);
    expect(snapshot.timeThresholds.node).toBe(11);
    expect(snapshot.timeThresholds.storage).toBe(12);
    expect(snapshot.timeThresholds.pbs).toBe(13);
    expect(snapshot.timeThresholds.agent).toBe(14);
    expect(snapshot.timeThresholds['k8s-cluster']).toBe(15);
    expect(snapshot.timeThresholds['k8s-node']).toBe(16);
    expect(snapshot.timeThresholds['k8s-deployment']).toBe(17);
    expect(snapshot.timeThresholds['k8s-namespace']).toBe(18);
    expect(snapshot.timeThresholds.pod).toBe(19);
    expect(snapshot.timeThresholds['truenas-system']).toBe(20);
    expect(snapshot.timeThresholds['truenas-pool']).toBe(21);
    expect(snapshot.timeThresholds['truenas-dataset']).toBe(22);
    expect(snapshot.timeThresholds['truenas-disk']).toBe(23);
    expect(snapshot.timeThresholds['vmware-host']).toBe(24);
    expect(snapshot.timeThresholds['vmware-vm']).toBe(25);
    expect(snapshot.timeThresholds['vmware-datastore']).toBe(26);
    expect(snapshot.timeThresholds['vmware-network']).toBe(27);
  });

  it('falls back to DEFAULT_DELAY_SECONDS for missing fields', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ timeThresholds: { guest: 30 } }),
    );
    expect(snapshot.timeThresholds.guest).toBe(30);
    expect(snapshot.timeThresholds.node).toBe(5);
    expect(snapshot.timeThresholds['vmware-network']).toBe(5);
  });
});

// ---------------------------------------------------------------------------
// metricTimeThresholds
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — metricTimeThresholds', () => {
  it('normalizes type and metric keys on the read path', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        metricTimeThresholds: {
          Guest: { CPU: 17, Memory: 22 },
        },
      }),
    );
    expect(snapshot.metricTimeThresholds).toEqual({ guest: { cpu: 17, memory: 22 } });
  });
});

// ---------------------------------------------------------------------------
// backupDefaults fallbacks
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — backupDefaults fallbacks', () => {
  it('falls back to factory freshHours / staleHours / alertOrphaned / ignoreVMIDs', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        backupDefaults: {
          enabled: true,
          warningDays: 5,
          criticalDays: 10,
        },
      }),
    );
    expect(snapshot.backupDefaults.freshHours).toBe(24);
    expect(snapshot.backupDefaults.staleHours).toBe(72);
    expect(snapshot.backupDefaults.alertOrphaned).toBe(true);
    expect(snapshot.backupDefaults.ignoreVMIDs).toEqual([]);
  });

  it('uses provided freshHours / staleHours / alertOrphaned values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        backupDefaults: {
          enabled: true,
          warningDays: 5,
          criticalDays: 10,
          freshHours: 12,
          staleHours: 48,
          alertOrphaned: false,
          ignoreVMIDs: ['100', '200'],
        },
      }),
    );
    expect(snapshot.backupDefaults.freshHours).toBe(12);
    expect(snapshot.backupDefaults.staleHours).toBe(48);
    expect(snapshot.backupDefaults.alertOrphaned).toBe(false);
    expect(snapshot.backupDefaults.ignoreVMIDs).toEqual(['100', '200']);
  });
});

// ---------------------------------------------------------------------------
// snapshotDefaults
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — snapshotDefaults', () => {
  it('extracts enabled / warningDays / criticalDays', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        snapshotDefaults: { enabled: true, warningDays: 20, criticalDays: 35 },
      }),
    );
    expect(snapshot.snapshotDefaults.enabled).toBe(true);
    expect(snapshot.snapshotDefaults.warningDays).toBe(20);
    expect(snapshot.snapshotDefaults.criticalDays).toBe(35);
  });

  it('coerces enabled to false when falsy', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ snapshotDefaults: { enabled: 0, warningDays: 0, criticalDays: 0 } }),
    );
    expect(snapshot.snapshotDefaults.enabled).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// pmgDefaults
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — pmgDefaults', () => {
  it('extracts all sixteen pmg fields', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        pmgDefaults: {
          queueTotalWarning: 600,
          queueTotalCritical: 1200,
          oldestMessageWarnMins: 45,
          oldestMessageCritMins: 90,
          deferredQueueWarn: 300,
          deferredQueueCritical: 700,
          holdQueueWarn: 150,
          holdQueueCritical: 400,
          quarantineSpamWarn: 3000,
          quarantineSpamCritical: 6000,
          quarantineVirusWarn: 3000,
          quarantineVirusCritical: 6000,
          quarantineGrowthWarnPct: 35,
          quarantineGrowthWarnMin: 350,
          quarantineGrowthCritPct: 70,
          quarantineGrowthCritMin: 700,
        },
      }),
    );
    expect(snapshot.pmgThresholds.queueTotalWarning).toBe(600);
    expect(snapshot.pmgThresholds.queueTotalCritical).toBe(1200);
    expect(snapshot.pmgThresholds.oldestMessageWarnMins).toBe(45);
    expect(snapshot.pmgThresholds.oldestMessageCritMins).toBe(90);
    expect(snapshot.pmgThresholds.deferredQueueWarn).toBe(300);
    expect(snapshot.pmgThresholds.deferredQueueCritical).toBe(700);
    expect(snapshot.pmgThresholds.holdQueueWarn).toBe(150);
    expect(snapshot.pmgThresholds.holdQueueCritical).toBe(400);
    expect(snapshot.pmgThresholds.quarantineSpamWarn).toBe(3000);
    expect(snapshot.pmgThresholds.quarantineSpamCritical).toBe(6000);
    expect(snapshot.pmgThresholds.quarantineVirusWarn).toBe(3000);
    expect(snapshot.pmgThresholds.quarantineVirusCritical).toBe(6000);
    expect(snapshot.pmgThresholds.quarantineGrowthWarnPct).toBe(35);
    expect(snapshot.pmgThresholds.quarantineGrowthWarnMin).toBe(350);
    expect(snapshot.pmgThresholds.quarantineGrowthCritPct).toBe(70);
    expect(snapshot.pmgThresholds.quarantineGrowthCritMin).toBe(700);
  });

  it('applies per-field hardcoded fallbacks when pmgDefaults is empty', () => {
    const snapshot = readAlertsConfigurationSnapshot(cfg({ pmgDefaults: {} }));
    expect(snapshot.pmgThresholds.queueTotalWarning).toBe(500);
    expect(snapshot.pmgThresholds.queueTotalCritical).toBe(1000);
    expect(snapshot.pmgThresholds.oldestMessageWarnMins).toBe(30);
    expect(snapshot.pmgThresholds.oldestMessageCritMins).toBe(60);
    expect(snapshot.pmgThresholds.deferredQueueWarn).toBe(200);
    expect(snapshot.pmgThresholds.deferredQueueCritical).toBe(500);
    expect(snapshot.pmgThresholds.holdQueueWarn).toBe(100);
    expect(snapshot.pmgThresholds.holdQueueCritical).toBe(300);
    expect(snapshot.pmgThresholds.quarantineSpamWarn).toBe(2000);
    expect(snapshot.pmgThresholds.quarantineSpamCritical).toBe(5000);
    expect(snapshot.pmgThresholds.quarantineVirusWarn).toBe(2000);
    expect(snapshot.pmgThresholds.quarantineVirusCritical).toBe(5000);
    expect(snapshot.pmgThresholds.quarantineGrowthWarnPct).toBe(25);
    expect(snapshot.pmgThresholds.quarantineGrowthWarnMin).toBe(250);
    expect(snapshot.pmgThresholds.quarantineGrowthCritPct).toBe(50);
    expect(snapshot.pmgThresholds.quarantineGrowthCritMin).toBe(500);
  });
});

// ---------------------------------------------------------------------------
// disableAll flags
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — disableAll flags', () => {
  it('reads true for every disableAll flag', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        disableAllNodes: true,
        disableAllGuests: true,
        disableAllAgents: true,
        disableAllStorage: true,
        disableAllPBS: true,
        disableAllPMG: true,
        disableAllDockerHosts: true,
        disableAllDockerServices: true,
        disableAllDockerContainers: true,
        disableAllKubernetes: true,
        disableAllTrueNAS: true,
        disableAllVMware: true,
        disableAllNodesOffline: true,
        disableAllGuestsOffline: true,
        disableAllAgentsOffline: true,
        disableAllPBSOffline: true,
        disableAllPMGOffline: true,
        disableAllDockerHostsOffline: true,
      }),
    );
    expect(snapshot.disableAllNodes).toBe(true);
    expect(snapshot.disableAllGuests).toBe(true);
    expect(snapshot.disableAllAgents).toBe(true);
    expect(snapshot.disableAllStorage).toBe(true);
    expect(snapshot.disableAllPBS).toBe(true);
    expect(snapshot.disableAllPMG).toBe(true);
    expect(snapshot.disableAllDockerHosts).toBe(true);
    expect(snapshot.disableAllDockerServices).toBe(true);
    expect(snapshot.disableAllDockerContainers).toBe(true);
    expect(snapshot.disableAllKubernetes).toBe(true);
    expect(snapshot.disableAllTrueNAS).toBe(true);
    expect(snapshot.disableAllVMware).toBe(true);
    expect(snapshot.disableAllNodesOffline).toBe(true);
    expect(snapshot.disableAllGuestsOffline).toBe(true);
    expect(snapshot.disableAllAgentsOffline).toBe(true);
    expect(snapshot.disableAllPBSOffline).toBe(true);
    expect(snapshot.disableAllPMGOffline).toBe(true);
    expect(snapshot.disableAllDockerHostsOffline).toBe(true);
  });

  it('defaults to false when explicitly set to false', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ disableAllNodes: false, disableAllPMG: false }),
    );
    expect(snapshot.disableAllNodes).toBe(false);
    expect(snapshot.disableAllPMG).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// schedule.quietHours
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — schedule.quietHours', () => {
  it('passes through object-style days directly', () => {
    const days = { monday: true, wednesday: false, custom: true };
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          quietHours: {
            enabled: true,
            start: '23:00',
            end: '07:00',
            timezone: 'America/New_York',
            days,
          },
        },
      }),
    );
    expect(snapshot.scheduleQuietHours.days).toEqual(days);
    expect(snapshot.scheduleQuietHours.enabled).toBe(true);
    expect(snapshot.scheduleQuietHours.start).toBe('23:00');
    expect(snapshot.scheduleQuietHours.end).toBe('07:00');
    expect(snapshot.scheduleQuietHours.timezone).toBe('America/New_York');
  });

  it('falls back to default days map when days is null', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          quietHours: {
            enabled: true,
            start: '23:00',
            end: '07:00',
            timezone: 'UTC',
            days: null,
          },
        },
      }),
    );
    expect(snapshot.scheduleQuietHours.days).toEqual({
      monday: true,
      tuesday: true,
      wednesday: true,
      thursday: true,
      friday: true,
      saturday: false,
      sunday: false,
    });
  });

  it('applies field fallbacks for enabled / start / end / timezone / suppress', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          quietHours: {
            enabled: undefined,
            start: '',
            end: '',
            timezone: '',
            days: [0],
          },
        },
      }),
    );
    expect(snapshot.scheduleQuietHours.enabled).toBe(false);
    expect(snapshot.scheduleQuietHours.start).toBe('22:00');
    expect(snapshot.scheduleQuietHours.end).toBe('08:00');
    expect(snapshot.scheduleQuietHours.timezone).toBe(localTimezone);
  });

  it('applies suppress sub-field fallbacks when suppress is absent', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          quietHours: {
            enabled: true,
            start: '01:00',
            end: '02:00',
            timezone: 'UTC',
            days: [1],
          },
        },
      }),
    );
    expect(snapshot.scheduleQuietHours.suppress).toEqual({
      performance: false,
      storage: false,
      offline: false,
    });
  });

  it('reads provided suppress sub-fields', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          quietHours: {
            enabled: true,
            start: '01:00',
            end: '02:00',
            timezone: 'UTC',
            days: [1],
            suppress: { performance: true, storage: true, offline: true },
          },
        },
      }),
    );
    expect(snapshot.scheduleQuietHours.suppress).toEqual({
      performance: true,
      storage: true,
      offline: true,
    });
  });

  it('keeps default quietHours when schedule exists but quietHours is absent', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 10 } }),
    );
    expect(snapshot.scheduleQuietHours.enabled).toBe(false);
    expect(snapshot.scheduleQuietHours.start).toBe('22:00');
  });
});

// ---------------------------------------------------------------------------
// schedule.cooldown
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — schedule.cooldown', () => {
  it('disables cooldown and zeroes minutes when value is zero', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 0 } }),
    );
    expect(snapshot.scheduleCooldown.enabled).toBe(false);
    expect(snapshot.scheduleCooldown.minutes).toBe(0);
  });

  it('enables and clamps cooldown minutes when value is positive', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 200 } }),
    );
    expect(snapshot.scheduleCooldown.enabled).toBe(true);
    expect(snapshot.scheduleCooldown.minutes).toBe(120); // clamped to max
  });

  it('keeps default cooldown when cooldown is undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { quietHours: undefined } }),
    );
    expect(snapshot.scheduleCooldown.enabled).toBe(true);
    expect(snapshot.scheduleCooldown.minutes).toBe(30);
  });

  it('uses fallbackMaxAlertsPerHour default when maxAlertsHour is undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 15 } }),
    );
    expect(snapshot.scheduleCooldown.maxAlerts).toBe(3); // MAX_ALERTS_DEFAULT
  });
});

// ---------------------------------------------------------------------------
// schedule.grouping
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — schedule.grouping', () => {
  it('uses numeric window and respects explicit enabled / byNode / byGuest', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          grouping: { enabled: true, window: 120, byNode: false, byGuest: true },
        },
      }),
    );
    expect(snapshot.scheduleGrouping.enabled).toBe(true);
    expect(snapshot.scheduleGrouping.window).toBe(2); // Math.round(120/60)
    expect(snapshot.scheduleGrouping.byNode).toBe(false);
    expect(snapshot.scheduleGrouping.byGuest).toBe(true);
  });

  it('falls back to GROUPING_WINDOW_DEFAULT_SECONDS when window is not a number', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          grouping: { window: 'not-a-number' },
        },
      }),
    );
    // 30 seconds / 60 = 0.5 -> Math.round -> 1
    expect(snapshot.scheduleGrouping.window).toBe(1);
  });

  it('clamps negative window to zero', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: { grouping: { window: -60 } },
      }),
    );
    expect(snapshot.scheduleGrouping.window).toBe(0);
  });

  it('infers enabled from window > 0 when enabled is undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: { grouping: { window: 60 } },
      }),
    );
    expect(snapshot.scheduleGrouping.enabled).toBe(true);
  });

  it('infers enabled as false when window is zero and enabled is undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: { grouping: { window: 0 } },
      }),
    );
    expect(snapshot.scheduleGrouping.enabled).toBe(false);
  });

  it('defaults byNode to true and byGuest to false when undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: { grouping: { enabled: true, window: 60 } },
      }),
    );
    expect(snapshot.scheduleGrouping.byNode).toBe(true);
    expect(snapshot.scheduleGrouping.byGuest).toBe(false);
  });

  it('keeps default grouping when grouping is absent', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 10 } }),
    );
    expect(snapshot.scheduleGrouping.enabled).toBe(true);
    expect(snapshot.scheduleGrouping.window).toBe(1); // GROUPING_WINDOW_DEFAULT_MINUTES
  });
});

// ---------------------------------------------------------------------------
// schedule.notifyOnResolve
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — schedule.notifyOnResolve', () => {
  it('sets to false when defined as false', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { notifyOnResolve: false } }),
    );
    expect(snapshot.notifyOnResolve).toBe(false);
  });

  it('keeps default true when notifyOnResolve is undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 10 } }),
    );
    expect(snapshot.notifyOnResolve).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// schedule.escalation
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — schedule.escalation', () => {
  it('extracts enabled and levels with after and notify', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          escalation: {
            enabled: true,
            levels: [
              { after: 30, notify: 'email' },
              { after: 60, notify: 'webhook' },
            ],
          },
        },
      }),
    );
    expect(snapshot.scheduleEscalation.enabled).toBe(true);
    expect(snapshot.scheduleEscalation.levels).toEqual([
      { after: 30, notify: 'email' },
      { after: 60, notify: 'webhook' },
    ]);
  });

  it('defaults level.after to 15 for non-number values', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          escalation: {
            enabled: true,
            levels: [{ after: 'soon', notify: 'all' }],
          },
        },
      }),
    );
    expect(snapshot.scheduleEscalation.levels[0].after).toBe(15);
  });

  it('defaults level.notify to all when falsy', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          escalation: {
            enabled: true,
            levels: [{ after: 20, notify: '' }],
          },
        },
      }),
    );
    expect(snapshot.scheduleEscalation.levels[0].notify).toBe('all');
  });

  it('defaults levels to empty array when absent', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          escalation: { enabled: false },
        },
      }),
    );
    expect(snapshot.scheduleEscalation.enabled).toBe(false);
    expect(snapshot.scheduleEscalation.levels).toEqual([]);
  });

  it('defaults levels to empty array when explicitly undefined', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({
        schedule: {
          escalation: { enabled: true, levels: undefined },
        },
      }),
    );
    expect(snapshot.scheduleEscalation.levels).toEqual([]);
  });

  it('keeps default escalation when escalation is absent', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ schedule: { cooldown: 10 } }),
    );
    expect(snapshot.scheduleEscalation.enabled).toBe(false);
    expect(snapshot.scheduleEscalation.levels).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// schedule absent entirely
// ---------------------------------------------------------------------------

describe('readAlertsConfigurationSnapshot — schedule absent', () => {
  it('keeps all schedule defaults when schedule key is missing', () => {
    const snapshot = readAlertsConfigurationSnapshot(cfg({}));
    expect(snapshot.scheduleQuietHours.enabled).toBe(false);
    expect(snapshot.scheduleCooldown.enabled).toBe(true);
    expect(snapshot.scheduleGrouping.enabled).toBe(true);
    expect(snapshot.scheduleEscalation.enabled).toBe(false);
    expect(snapshot.notifyOnResolve).toBe(true);
  });
});

// ===========================================================================
// buildAlertsConfigurationPayload
// ===========================================================================

describe('buildAlertsConfigurationPayload — additional branches', () => {
  it('emits disabled grouping with zero window when snapshot grouping is disabled', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.scheduleGrouping = { enabled: false, window: 5, byNode: true, byGuest: false };

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(alertConfig?.schedule?.grouping).toEqual({
      enabled: false,
      window: 0,
      byNode: true,
      byGuest: false,
    });
  });

  it('emits disabled grouping when enabled but window is zero', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.scheduleGrouping = { enabled: true, window: 0, byNode: true, byGuest: false };

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    // window >= 0 is true so groupingWindowSeconds = 0*60 = 0,
    // but groupingEnabled = enabled && (0 > 0) = false
    expect(alertConfig?.schedule?.grouping).toEqual({
      enabled: false,
      window: 0,
      byNode: true,
      byGuest: false,
    });
  });

  it('emits zero cooldown when snapshot cooldown is disabled', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.scheduleCooldown = { enabled: false, minutes: 30, maxAlerts: 5 };

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(alertConfig?.schedule?.cooldown).toBe(0);
  });

  it('defaults enabled to true and activation fields to undefined when config is null', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(alertConfig?.enabled).toBe(true);
    expect(alertConfig?.activationState).toBeUndefined();
    expect(alertConfig?.activationTime).toBeUndefined();
    expect(alertConfig?.observationWindowHours).toBeUndefined();
  });

  it('passes through activation fields when config and state are provided', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: 'pending_review',
      alertsActivationConfig: {
        enabled: false,
        activationTime: '2026-01-15T08:00:00Z',
        observationWindowHours: 48,
      },
    });

    expect(alertConfig?.enabled).toBe(false);
    expect(alertConfig?.activationState).toBe('pending_review');
    expect(alertConfig?.activationTime).toBe('2026-01-15T08:00:00Z');
    expect(alertConfig?.observationWindowHours).toBe(48);
  });

  it('falls back to 24/72/true for null freshHours/staleHours/alertOrphaned', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.backupDefaults = {
      enabled: true,
      warningDays: 5,
      criticalDays: 10,
      freshHours: undefined,
      staleHours: undefined,
      alertOrphaned: undefined,
      ignoreVMIDs: [],
    };

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(alertConfig?.backupDefaults?.freshHours).toBe(24);
    expect(alertConfig?.backupDefaults?.staleHours).toBe(72);
    expect(alertConfig?.backupDefaults?.alertOrphaned).toBe(true);
  });
});

// ===========================================================================
// cloneBackupDefaults (module-private, exercised via createDefault)
// ===========================================================================

describe('cloneBackupDefaults (via createDefaultAlertsConfigurationSnapshot)', () => {
  it('preserves all scalar backup-default fields through the shallow spread', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();

    expect(snapshot.backupDefaults).toEqual({
      enabled: false,
      warningDays: 7,
      criticalDays: 14,
      freshHours: 24,
      staleHours: 72,
      alertOrphaned: true,
      ignoreVMIDs: [],
    });
  });
});

// ===========================================================================
// dockerDefaults.updateAlertDelayHours (read + save round-trip)
// ===========================================================================

describe('dockerDefaults.updateAlertDelayHours', () => {
  it('defaults to 24 when the config omits the field', () => {
    const snapshot = readAlertsConfigurationSnapshot(cfg({ dockerDefaults: {} }));
    expect(snapshot.dockerDefaults.updateAlertDelayHours).toBe(24);
  });

  it('preserves a custom positive delay and the -1 off state on read', () => {
    const custom = readAlertsConfigurationSnapshot(
      cfg({ dockerDefaults: { updateAlertDelayHours: 48 } }),
    );
    expect(custom.dockerDefaults.updateAlertDelayHours).toBe(48);

    const off = readAlertsConfigurationSnapshot(
      cfg({ dockerDefaults: { updateAlertDelayHours: -1 } }),
    );
    expect(off.dockerDefaults.updateAlertDelayHours).toBe(-1);
  });

  it('maps 0 (backend "unset") to the factory 24 on read', () => {
    const snapshot = readAlertsConfigurationSnapshot(
      cfg({ dockerDefaults: { updateAlertDelayHours: 0 } }),
    );
    expect(snapshot.dockerDefaults.updateAlertDelayHours).toBe(24);
  });

  it('round-trips -1 and custom delays through buildAlertsConfigurationPayload', () => {
    for (const value of [-1, 6, 24, 72]) {
      const snapshot = createDefaultAlertsConfigurationSnapshot();
      snapshot.dockerDefaults.updateAlertDelayHours = value;

      const { alertConfig } = buildAlertsConfigurationPayload({
        snapshot,
        rawOverridesConfig: {},
        alertsActivationState: null,
        alertsActivationConfig: null,
      });

      expect(alertConfig?.dockerDefaults?.updateAlertDelayHours).toBe(value);
    }
  });

  it('never emits 0 on save (the backend would reset it to 24)', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.dockerDefaults.updateAlertDelayHours = 0;

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(alertConfig?.dockerDefaults?.updateAlertDelayHours).toBe(24);
  });

  it('keeps the field when saving unrelated settings (default snapshot save)', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();

    const { alertConfig } = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: null,
      alertsActivationConfig: null,
    });

    expect(alertConfig?.dockerDefaults?.updateAlertDelayHours).toBe(24);
  });
});
