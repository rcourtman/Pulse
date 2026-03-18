import { describe, expect, it } from 'vitest';

import {
  ALERT_TAB_SEGMENTS,
  pathForTab,
  tabFromPath,
  clampCooldownMinutes,
  fallbackCooldownMinutes,
} from '@/features/alerts/types';
import {
  clampMaxAlertsPerHour,
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  fallbackMaxAlertsPerHour,
  extractTriggerValues,
  getTriggerValue,
  normalizeEmailConfigFromAPI,
  normalizeMetricDelayMap,
  unifiedTypeToAlertDisplayType,
} from '@/features/alerts/helpers';
import {
  getAlertIncidentAcknowledgedBadgeClass,
  getAlertIncidentEventFilterActionButtonClass,
  getAlertIncidentEventFilterChipClass,
  getAlertIncidentEventFilterContainerClass,
  getAlertIncidentNoteSaveButtonClass,
  getAlertIncidentNoteTextareaClass,
  getAlertIncidentTimelineCommandClass,
  getAlertIncidentTimelineDetailClass,
  getAlertIncidentTimelineEventCardClass,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertIncidentTimelineOutputClass,
  getAlertResourceIncidentCardClass,
  getAlertResourceIncidentSummaryRowClass,
  getAlertResourceIncidentToggleButtonClass,
  getAlertResourceIncidentTruncatedEventsLabel,
} from '@/utils/alertIncidentPresentation';
import {
  getAlertQuietSuppressCardClass,
  getAlertQuietSuppressCheckboxClass,
} from '@/utils/alertSchedulePresentation';
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

describe('quiet suppress presentation helpers', () => {
  it('returns the selected quiet suppress card presentation', () => {
    expect(getAlertQuietSuppressCardClass(true)).toBe(
      'flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-500',
    );
  });

  it('returns the selected quiet suppress checkbox presentation', () => {
    expect(getAlertQuietSuppressCheckboxClass(true)).toBe(
      'mt-1 flex h-4 w-4 items-center justify-center rounded border-2 border-blue-500 bg-blue-500',
    );
  });
});

describe('incident event filter presentation helpers', () => {
  it('returns the compact filter container presentation', () => {
    expect(getAlertIncidentEventFilterContainerClass('compact')).toBe(
      'flex flex-wrap items-center gap-2 text-[10px] text-muted',
    );
  });

  it('returns the shared action button presentation', () => {
    expect(getAlertIncidentEventFilterActionButtonClass()).toBe(
      'px-2 py-0.5 rounded border border-border text-muted hover:bg-surface-hover',
    );
  });

  it('returns the selected compact chip presentation', () => {
    expect(getAlertIncidentEventFilterChipClass(true, 'compact')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors border-blue-300 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
    );
  });
});

describe('incident timeline presentation helpers', () => {
  it('returns the acknowledged badge presentation', () => {
    expect(getAlertIncidentAcknowledgedBadgeClass()).toBe(
      'px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
    );
  });

  it('returns the surface event-card presentation', () => {
    expect(getAlertIncidentTimelineEventCardClass('surface')).toBe(
      'rounded border border-border bg-surface p-2',
    );
  });

  it('returns the note editor presentation', () => {
    expect(getAlertIncidentNoteTextareaClass()).toBe(
      'w-full rounded border border-border bg-surface p-2 text-xs text-base-content',
    );
    expect(getAlertIncidentNoteSaveButtonClass()).toBe(
      'px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed',
    );
  });

  it('returns the shared event detail presentation', () => {
    expect(getAlertIncidentTimelineMetaRowClass()).toBe(
      'flex flex-wrap items-center gap-2 text-xs text-muted',
    );
    expect(getAlertIncidentTimelineHeadingClass()).toBe('font-medium text-base-content');
    expect(getAlertIncidentTimelineDetailClass()).toBe('mt-1 text-xs text-base-content');
    expect(getAlertIncidentTimelineCommandClass()).toBe(
      'mt-1 font-mono text-xs text-base-content',
    );
    expect(getAlertIncidentTimelineOutputClass()).toBe('mt-1 text-xs text-muted');
  });

  it('returns the resource incident panel presentation', () => {
    expect(getAlertResourceIncidentCardClass()).toBe(
      'rounded border border-border bg-surface p-3',
    );
    expect(getAlertResourceIncidentSummaryRowClass()).toBe(
      'mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted',
    );
    expect(getAlertResourceIncidentToggleButtonClass()).toBe(
      'px-2 py-1 text-[10px] border rounded-md border-border text-muted hover:bg-surface-hover',
    );
    expect(getAlertResourceIncidentTruncatedEventsLabel(6)).toBe('Showing last 6 events');
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

describe('normalizeEmailConfigFromAPI', () => {
  it('preserves explicit zero values and false booleans', () => {
    const result = normalizeEmailConfigFromAPI({
      enabled: true,
      provider: 'custom',
      server: 'smtp.example.com',
      port: 0,
      username: 'user',
      password: 'pass',
      from: 'alerts@example.com',
      to: ['ops@example.com'],
      tls: false,
      startTLS: false,
      rateLimit: 0,
    });

    expect(result).toEqual({
      enabled: true,
      provider: 'custom',
      server: 'smtp.example.com',
      port: 0,
      username: 'user',
      password: 'pass',
      from: 'alerts@example.com',
      to: ['ops@example.com'],
      tls: false,
      startTLS: false,
      replyTo: '',
      maxRetries: 3,
      retryDelay: 5,
      rateLimit: 0,
    });
  });

  it('falls back to defaults for malformed payload types', () => {
    const malformed = {
      enabled: 'yes',
      provider: 123,
      server: ['smtp'],
      port: '587',
      username: null,
      password: {},
      from: true,
      to: ['ops@example.com', 42, null],
      tls: 'true',
      startTLS: {},
      rateLimit: '60',
    } as unknown as Partial<import('@/api/notifications').EmailConfig>;

    const result = normalizeEmailConfigFromAPI(malformed);

    expect(result).toEqual({
      enabled: false,
      provider: '',
      server: '',
      port: 587,
      username: '',
      password: '',
      from: '',
      to: ['ops@example.com'],
      tls: true,
      startTLS: false,
      replyTo: '',
      maxRetries: 3,
      retryDelay: 5,
      rateLimit: 60,
    });
  });
});

describe('unifiedTypeToAlertDisplayType', () => {
  it('maps vm to VM', () => {
    expect(unifiedTypeToAlertDisplayType('vm')).toBe('VM');
  });

  it('maps system-container and oci-container to Container', () => {
    expect(unifiedTypeToAlertDisplayType('system-container')).toBe('Container');
    expect(unifiedTypeToAlertDisplayType('oci-container')).toBe('Container');
  });

  it('maps app-container to Container', () => {
    expect(unifiedTypeToAlertDisplayType('app-container')).toBe('App Container');
  });

  it('maps agent to Agent', () => {
    expect(unifiedTypeToAlertDisplayType('agent')).toBe('Agent');
  });

  it('maps docker-host to Container Runtime', () => {
    expect(unifiedTypeToAlertDisplayType('docker-host')).toBe('Container Runtime');
  });

  it('maps storage and datastore to canonical labels', () => {
    expect(unifiedTypeToAlertDisplayType('storage')).toBe('Storage');
    expect(unifiedTypeToAlertDisplayType('datastore')).toBe('Datastore');
  });

  it('maps pbs to PBS', () => {
    expect(unifiedTypeToAlertDisplayType('pbs')).toBe('PBS');
  });

  it('maps pmg to PMG', () => {
    expect(unifiedTypeToAlertDisplayType('pmg')).toBe('PMG');
  });

  it('maps k8s-cluster to K8s Cluster', () => {
    expect(unifiedTypeToAlertDisplayType('k8s-cluster')).toBe('K8s Cluster');
  });

  it('passes through unknown types', () => {
    expect(unifiedTypeToAlertDisplayType('other-type' as any)).toBe('other-type');
  });
});

describe('Unified selector parity', () => {
  it('maps all unified resource types to display types', () => {
    const cases: Array<[ResourceType, string]> = [
      ['agent', 'Agent'],
      ['docker-host', 'Container Runtime'],
      ['k8s-cluster', 'K8s Cluster'],
      ['k8s-node', 'K8s Node'],
      ['truenas', 'TrueNAS'],
      ['vm', 'VM'],
      ['system-container', 'Container'],
      ['oci-container', 'Container'],
      ['app-container', 'App Container'],
      ['pod', 'Pod'],
      ['jail', 'Jail'],
      ['docker-service', 'Docker Service'],
      ['k8s-deployment', 'K8s Deployment'],
      ['k8s-service', 'K8s Service'],
      ['storage', 'Storage'],
      ['datastore', 'Datastore'],
      ['pool', 'Pool'],
      ['dataset', 'Dataset'],
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
      resourceType: guestType === 'qemu' ? 'VM' : 'Container',
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
      resourceType: 'vm' | 'system-container' | 'oci-container',
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
      buildUnifiedGuestOverride(
        'system-container',
        'ct-pve1-200',
        'ct-200',
        200,
        'pve1',
        'pve1/lxc/200',
      ),
    ).toEqual(
      buildLegacyGuestOverride('lxc', 'ct-pve1-200', 'ct-200', 200, 'pve1', 'pve1/lxc/200'),
    );
  });
});
