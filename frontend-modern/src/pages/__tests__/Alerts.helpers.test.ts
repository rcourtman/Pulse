import { describe, expect, it } from 'vitest';

import {
  ALERT_TAB_SEGMENTS,
  clampCooldownMinutes,
  clampMaxAlertsPerHour,
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  fallbackCooldownMinutes,
  fallbackMaxAlertsPerHour,
  extractTriggerValues,
  getTriggerValue,
  normalizeMetricDelayMap,
  pathForTab,
  tabFromPath,
  unifiedTypeToAlertDisplayType,
} from '../Alerts';
import type { RawOverrideConfig } from '@/types/alerts';
import type { ResourceType } from '@/types/resource';

describe('normalizeMetricDelayMap', () => {
  it('returns empty object when input is nullish', () => {
    expect(normalizeMetricDelayMap(undefined)).toEqual({});
    expect(normalizeMetricDelayMap(null)).toEqual({});
  });

  it('normalizes resource and metric keys while discarding invalid values', () => {
    const input = {
      Guest: {
        CPU: 10,
        ' ': 5,
        memory: -1,
        disk: Number.NaN,
      },
      node: {
        Temperature: 30,
        disk: 15.6,
      },
      ' ': {
        metric: 5,
      },
    };

    const result = normalizeMetricDelayMap(input);

    expect(result).toEqual({
      guest: {
        cpu: 10,
      },
      node: {
        temperature: 30,
        disk: 16,
      },
    });
  });

  it('drops metric groups that normalize to empty', () => {
    const result = normalizeMetricDelayMap({
      guest: {
        cpu: -1,
        mem: Number.NaN,
      },
    });

    expect(result).toEqual({});
  });
});

describe('tab path helpers', () => {
  it('maps tab to path', () => {
    expect(pathForTab('overview')).toBe('/alerts/overview');
    expect(pathForTab('schedule')).toBe('/alerts/schedule');
  });

  it('resolves tab from path', () => {
    expect(tabFromPath('/alerts')).toBe('overview');
    expect(tabFromPath('/alerts/thresholds')).toBe('thresholds');
    expect(tabFromPath('/alerts/thresholds/proxmox')).toBe('thresholds');
    expect(tabFromPath('/alerts/custom-rules')).toBe('thresholds');
    expect(tabFromPath('/foo/bar')).toBe('overview');
  });

  it('allows custom segments map', () => {
    const custom = { ...ALERT_TAB_SEGMENTS, overview: 'summary' as const };
    expect(pathForTab('overview', custom)).toBe('/alerts/summary');
    expect(tabFromPath('/alerts/summary', custom)).toBe('overview');
  });
});

describe('default schedule helpers', () => {
  it('creates quiet hours defaults', () => {
    const quiet = createDefaultQuietHours();
    const expectedTz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

    expect(quiet).toMatchObject({
      enabled: false,
      start: '22:00',
      end: '08:00',
      suppress: {
        performance: false,
        storage: false,
        offline: false,
      },
    });
    expect(quiet.timezone).toBe(expectedTz);
    expect(quiet.days).toEqual({
      monday: true,
      tuesday: true,
      wednesday: true,
      thursday: true,
      friday: true,
      saturday: false,
      sunday: false,
    });
  });

  it('creates cooldown defaults', () => {
    expect(createDefaultCooldown()).toEqual({
      enabled: true,
      minutes: 30,
      maxAlerts: 3,
    });
  });

  it('creates grouping defaults', () => {
    expect(createDefaultGrouping()).toEqual({
      enabled: true,
      window: 1,
      byNode: true,
      byGuest: false,
    });
  });

  it('creates escalation defaults', () => {
    expect(createDefaultEscalation()).toEqual({
      enabled: false,
      levels: [],
    });
  });
});

describe('cooldown sanitizers', () => {
  it('clamps cooldown minutes into valid range', () => {
    expect(clampCooldownMinutes(2)).toBe(5);
    expect(clampCooldownMinutes(60)).toBe(60);
    expect(clampCooldownMinutes(999)).toBe(120);
    expect(clampCooldownMinutes(undefined)).toBe(5);
  });

  it('provides sensible fallback when enabling cooldown', () => {
    expect(fallbackCooldownMinutes(0)).toBe(30);
    expect(fallbackCooldownMinutes(undefined)).toBe(30);
    expect(fallbackCooldownMinutes(2)).toBe(5);
  });

  it('clamps max alerts per hour', () => {
    expect(clampMaxAlertsPerHour(0)).toBe(1);
    expect(clampMaxAlertsPerHour(7)).toBe(7);
    expect(clampMaxAlertsPerHour(40)).toBe(10);
    expect(clampMaxAlertsPerHour(undefined)).toBe(1);
  });

  it('falls back to defaults for invalid max alerts values', () => {
    expect(fallbackMaxAlertsPerHour(undefined)).toBe(3);
    expect(fallbackMaxAlertsPerHour(0)).toBe(3);
    expect(fallbackMaxAlertsPerHour(50)).toBe(10);
  });
});

describe('threshold helper utilities', () => {
  it('extracts trigger values and ignores non-threshold keys', () => {
    const result = extractTriggerValues({
      cpu: { trigger: 80, clear: 70 },
      memory: { trigger: 85, clear: 75 },
      disabled: true,
      poweredOffSeverity: 'warning',
      customFlag: true,
      customLegacy: 42,
      label: 'ignored',
    } as RawOverrideConfig);

    expect(result).toEqual({
      cpu: 80,
      memory: 85,
      customFlag: 0,
      customLegacy: 42,
    });
  });

  it('getTriggerValue handles multiple input shapes', () => {
    expect(getTriggerValue(75)).toBe(75);
    expect(getTriggerValue({ trigger: 90, clear: 80 })).toBe(90);
    expect(getTriggerValue(true)).toBe(0);
    expect(getTriggerValue(undefined)).toBe(0);
  });
});

describe('unifiedTypeToAlertDisplayType', () => {
  it('maps vm to VM', () => {
    expect(unifiedTypeToAlertDisplayType('vm')).toBe('VM');
  });

  it('maps container and oci-container to CT', () => {
    expect(unifiedTypeToAlertDisplayType('container')).toBe('CT');
    expect(unifiedTypeToAlertDisplayType('oci-container')).toBe('CT');
  });

  it('maps docker-container to Container', () => {
    expect(unifiedTypeToAlertDisplayType('docker-container')).toBe('Container');
  });

  it('maps node to Node', () => {
    expect(unifiedTypeToAlertDisplayType('node')).toBe('Node');
  });

  it('maps host to Host', () => {
    expect(unifiedTypeToAlertDisplayType('host')).toBe('Host');
  });

  it('maps docker-host to Container Host', () => {
    expect(unifiedTypeToAlertDisplayType('docker-host')).toBe('Container Host');
  });

  it('maps storage and datastore to Storage', () => {
    expect(unifiedTypeToAlertDisplayType('storage')).toBe('Storage');
    expect(unifiedTypeToAlertDisplayType('datastore')).toBe('Storage');
  });

  it('maps pbs to PBS', () => {
    expect(unifiedTypeToAlertDisplayType('pbs')).toBe('PBS');
  });

  it('maps pmg to PMG', () => {
    expect(unifiedTypeToAlertDisplayType('pmg')).toBe('PMG');
  });

  it('maps k8s-cluster to K8s', () => {
    expect(unifiedTypeToAlertDisplayType('k8s-cluster')).toBe('K8s');
  });

  it('passes through unknown types', () => {
    expect(unifiedTypeToAlertDisplayType('other-type' as any)).toBe('other-type');
  });
});

describe('Unified selector parity', () => {
  it('maps all unified resource types to display types', () => {
    const cases: Array<[ResourceType, string]> = [
      ['node', 'Node'],
      ['host', 'Host'],
      ['docker-host', 'Container Host'],
      ['k8s-cluster', 'K8s'],
      ['k8s-node', 'k8s-node'],
      ['truenas', 'truenas'],
      ['vm', 'VM'],
      ['container', 'CT'],
      ['oci-container', 'CT'],
      ['docker-container', 'Container'],
      ['pod', 'pod'],
      ['jail', 'jail'],
      ['docker-service', 'docker-service'],
      ['k8s-deployment', 'k8s-deployment'],
      ['k8s-service', 'k8s-service'],
      ['storage', 'Storage'],
      ['datastore', 'Storage'],
      ['pool', 'pool'],
      ['dataset', 'dataset'],
      ['pbs', 'PBS'],
      ['pmg', 'PMG'],
    ];

    for (const [input, expected] of cases) {
      expect(unifiedTypeToAlertDisplayType(input)).toBe(expected);
    }
  });

  it('keeps guest override extraction shape aligned with legacy mapping', () => {
    const thresholds: RawOverrideConfig = {
      cpu: { trigger: 88, clear: 78 },
      memory: { trigger: 82, clear: 72 },
      disabled: true,
      disableConnectivity: true,
      poweredOffSeverity: 'critical',
    };

    const buildLegacyGuestOverride = (
      guestType: 'qemu' | 'lxc',
      id: string,
      name: string,
      vmid: number,
      node: string,
      instance: string,
    ) => ({
      id,
      name,
      type: 'guest' as const,
      resourceType: guestType === 'qemu' ? 'VM' : 'CT',
      vmid,
      node,
      instance,
      disabled: thresholds.disabled || false,
      disableConnectivity: thresholds.disableConnectivity || false,
      poweredOffSeverity:
        thresholds.poweredOffSeverity === 'critical'
          ? 'critical'
          : thresholds.poweredOffSeverity === 'warning'
            ? 'warning'
            : undefined,
      thresholds: extractTriggerValues(thresholds),
      backup: thresholds.backup,
      snapshot: thresholds.snapshot,
    });

    const buildUnifiedGuestOverride = (
      resourceType: 'vm' | 'container' | 'oci-container',
      id: string,
      name: string,
      vmid: number,
      node: string,
      instance: string,
    ) => ({
      id,
      name,
      type: 'guest' as const,
      resourceType: unifiedTypeToAlertDisplayType(resourceType),
      vmid,
      node,
      instance,
      disabled: thresholds.disabled || false,
      disableConnectivity: thresholds.disableConnectivity || false,
      poweredOffSeverity:
        thresholds.poweredOffSeverity === 'critical'
          ? 'critical'
          : thresholds.poweredOffSeverity === 'warning'
            ? 'warning'
            : undefined,
      thresholds: extractTriggerValues(thresholds),
      backup: thresholds.backup,
      snapshot: thresholds.snapshot,
    });

    expect(
      buildUnifiedGuestOverride('vm', 'vm-pve1-100', 'app-100', 100, 'pve1', 'pve1/qemu/100'),
    ).toEqual(
      buildLegacyGuestOverride('qemu', 'vm-pve1-100', 'app-100', 100, 'pve1', 'pve1/qemu/100'),
    );

    expect(
      buildUnifiedGuestOverride('container', 'ct-pve1-200', 'ct-200', 200, 'pve1', 'pve1/lxc/200'),
    ).toEqual(
      buildLegacyGuestOverride('lxc', 'ct-pve1-200', 'ct-200', 200, 'pve1', 'pve1/lxc/200'),
    );
  });
});
