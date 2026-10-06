import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Alert, MetricAlertStatus } from '@/types/api';
import {
  clampMaxAlertsPerHour,
  fallbackMaxAlertsPerHour,
  getLocalTimezone,
  createDefaultQuietHours,
  createDefaultCooldown,
  createDefaultGrouping,
  createDefaultResolveNotifications,
  createDefaultAppriseConfig,
  createDefaultEmailConfig,
  alertTypeDisplayLabel,
  clampEscalationDelayMinutes,
} from '@/features/alerts/helpers';
import {
  METRIC_ALERT_STATUS_STALE_MS,
  getAlertAttentionCopy,
  getMetricAlertLastBreachMs,
  getMetricAlertPresentation,
} from '@/features/alerts/metricAlertPresentation';

describe('alerts helpers', () => {
  describe('clampEscalationDelayMinutes', () => {
    it('keeps escalation schedules inside the supported interval', () => {
      expect(clampEscalationDelayMinutes(-1, 30)).toBe(5);
      expect(clampEscalationDelayMinutes(15, 30)).toBe(15);
      expect(clampEscalationDelayMinutes(181, 30)).toBe(180);
      expect(clampEscalationDelayMinutes(Number.NaN, 30)).toBe(30);
      expect(clampEscalationDelayMinutes(Number.NaN, Number.NaN)).toBe(5);
    });
  });

  describe('clampMaxAlertsPerHour', () => {
    it('returns default min for NaN', () => {
      expect(clampMaxAlertsPerHour(NaN)).toBe(1);
    });

    it('returns default min for undefined', () => {
      expect(clampMaxAlertsPerHour(undefined)).toBe(1);
    });

    it('returns default min for non-numeric', () => {
      expect(clampMaxAlertsPerHour('abc' as unknown as number)).toBe(1);
    });

    it('returns min for value below min', () => {
      expect(clampMaxAlertsPerHour(0)).toBe(1);
    });

    it('returns max for value above max', () => {
      expect(clampMaxAlertsPerHour(100)).toBe(10);
    });

    it('returns value when within range', () => {
      expect(clampMaxAlertsPerHour(5)).toBe(5);
    });

    it('handles negative values', () => {
      expect(clampMaxAlertsPerHour(-5)).toBe(1);
    });
  });

  describe('fallbackMaxAlertsPerHour', () => {
    it('returns default for NaN', () => {
      expect(fallbackMaxAlertsPerHour(NaN)).toBe(3);
    });

    it('returns default for undefined', () => {
      expect(fallbackMaxAlertsPerHour(undefined)).toBe(3);
    });

    it('returns default for 0', () => {
      expect(fallbackMaxAlertsPerHour(0)).toBe(3);
    });

    it('returns default for negative', () => {
      expect(fallbackMaxAlertsPerHour(-5)).toBe(3);
    });

    it('returns clamped value for positive', () => {
      expect(fallbackMaxAlertsPerHour(5)).toBe(5);
    });
  });

  describe('getLocalTimezone', () => {
    it('returns a timezone string', () => {
      const tz = getLocalTimezone();
      expect(typeof tz).toBe('string');
      expect(tz.length).toBeGreaterThan(0);
    });

    it('returns UTC as fallback', () => {
      const tz = getLocalTimezone();
      expect(tz).toMatch(/^[A-Za-z]+\/[A-Za-z_]+|UTC$/);
    });
  });

  describe('createDefaultQuietHours', () => {
    it('creates default quiet hours config', () => {
      const result = createDefaultQuietHours();

      expect(result.enabled).toBe(false);
      expect(result.start).toBe('22:00');
      expect(result.end).toBe('08:00');
      expect(result.timezone).toBe(getLocalTimezone());
    });

    it('has correct weekday defaults', () => {
      const result = createDefaultQuietHours();

      expect(result.days.monday).toBe(true);
      expect(result.days.tuesday).toBe(true);
      expect(result.days.wednesday).toBe(true);
      expect(result.days.thursday).toBe(true);
      expect(result.days.friday).toBe(true);
    });

    it('has correct weekend defaults', () => {
      const result = createDefaultQuietHours();

      expect(result.days.saturday).toBe(false);
      expect(result.days.sunday).toBe(false);
    });

    it('has correct suppress defaults', () => {
      const result = createDefaultQuietHours();

      expect(result.suppress.performance).toBe(false);
      expect(result.suppress.storage).toBe(false);
      expect(result.suppress.offline).toBe(false);
    });
  });

  describe('createDefaultCooldown', () => {
    it('creates default cooldown config', () => {
      const result = createDefaultCooldown();

      expect(result.enabled).toBe(true);
      expect(result.minutes).toBe(30);
      expect(result.maxAlerts).toBe(3);
    });
  });

  describe('createDefaultGrouping', () => {
    it('creates default grouping config', () => {
      const result = createDefaultGrouping();

      expect(result.enabled).toBe(true);
      expect(result.window).toBe(1);
      expect(result.byNode).toBe(true);
      expect(result.byGuest).toBe(false);
    });
  });

  describe('createDefaultResolveNotifications', () => {
    it('returns true by default', () => {
      expect(createDefaultResolveNotifications()).toBe(true);
    });
  });

  describe('createDefaultAppriseConfig', () => {
    it('creates default apprise config', () => {
      const result = createDefaultAppriseConfig();

      expect(result.enabled).toBe(false);
      expect(result.mode).toBe('cli');
      expect(result.targetsText).toBe('');
      expect(result.cliPath).toBe('apprise');
      expect(result.timeoutSeconds).toBe(15);
    });
  });

  describe('createDefaultEmailConfig', () => {
    it('creates default email config', () => {
      const result = createDefaultEmailConfig();

      expect(result.enabled).toBe(false);
      expect(result.from).toBe('');
      expect(result.to).toEqual([]);
    });
  });

  describe('alertTypeDisplayLabel', () => {
    it('maps standard metric types to uppercase', () => {
      expect(alertTypeDisplayLabel('cpu')).toBe('CPU');
      expect(alertTypeDisplayLabel('memory')).toBe('Memory');
      expect(alertTypeDisplayLabel('disk')).toBe('Disk');
      expect(alertTypeDisplayLabel('io')).toBe('I/O');
      expect(alertTypeDisplayLabel('swap')).toBe('Swap');
    });

    it('maps camelCase metric types', () => {
      expect(alertTypeDisplayLabel('diskRead')).toBe('Disk Read');
      expect(alertTypeDisplayLabel('diskWrite')).toBe('Disk Write');
      expect(alertTypeDisplayLabel('networkIn')).toBe('Network In');
      expect(alertTypeDisplayLabel('networkOut')).toBe('Network Out');
    });

    it('maps compound docker alert types', () => {
      expect(alertTypeDisplayLabel('docker-container-oom-kill')).toBe('OOM Kill');
      expect(alertTypeDisplayLabel('docker-container-restart-loop')).toBe('Restart Loop');
      expect(alertTypeDisplayLabel('docker-container-health')).toBe('Container Health');
      expect(alertTypeDisplayLabel('docker-container-state')).toBe('Container State');
      expect(alertTypeDisplayLabel('docker-container-memory-limit')).toBe('Memory Limit');
      expect(alertTypeDisplayLabel('docker-service-health')).toBe('Service Health');
    });

    it('maps storage/backup alert types', () => {
      expect(alertTypeDisplayLabel('snapshot-age')).toBe('Snapshot Age');
      expect(alertTypeDisplayLabel('backup-age')).toBe('Backup Age');
      expect(alertTypeDisplayLabel('zfs-pool-state')).toBe('Pool State');
      expect(alertTypeDisplayLabel('zfs-pool-errors')).toBe('Pool Errors');
      expect(alertTypeDisplayLabel('disk-health')).toBe('Disk Health');
      expect(alertTypeDisplayLabel('disk-wearout')).toBe('Disk Wearout');
      expect(alertTypeDisplayLabel('resource-incident')).toBe('Resource Health');
    });

    it('maps infrastructure alert types', () => {
      expect(alertTypeDisplayLabel('host-offline')).toBe('Host Offline');
      expect(alertTypeDisplayLabel('offline')).toBe('Offline');
      expect(alertTypeDisplayLabel('powered-off')).toBe('Powered Off');
      expect(alertTypeDisplayLabel('connectivity')).toBe('Connectivity');
    });

    it('title-cases unknown kebab-case types', () => {
      expect(alertTypeDisplayLabel('some-new-type')).toBe('Some New Type');
    });

    it('title-cases unknown underscore types', () => {
      expect(alertTypeDisplayLabel('some_new_type')).toBe('Some New Type');
    });

    it('handles single-word unknown types', () => {
      expect(alertTypeDisplayLabel('unknown')).toBe('Unknown');
    });
  });
});

const NOW = Date.parse('2026-10-06T10:52:00Z');

// minipc on 2026-10-06: the alert opened at 80°C, the node then read 71-76°C,
// and the drawer kept saying "Node temperature at 80.0°C".
const minipcAlert = (status?: Partial<MetricAlertStatus>): Alert => ({
  id: 'homelab-minipc::metric-threshold:temperature',
  type: 'temperature',
  level: 'warning',
  resourceId: 'homelab-minipc',
  resourceName: 'minipc',
  node: 'minipc',
  instance: 'homelab',
  message: 'Node temperature at 80.0°C',
  value: 80,
  threshold: 80,
  startTime: '2026-10-06T07:57:44Z',
  lastSeen: '2026-10-06T10:48:52Z',
  acknowledged: false,
  metricStatus: status
    ? {
        phase: 'latched',
        value: 76,
        unit: '°C',
        observedAt: new Date(NOW - 10_000).toISOString(),
        trigger: 80,
        recovery: 75,
        recoveryDelaySeconds: 300,
        ...status,
      }
    : undefined,
});

describe('getMetricAlertPresentation', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('leads with the reading now and what clears an alert held under its trigger', () => {
    const presentation = getMetricAlertPresentation(
      minipcAlert({ phase: 'latched', value: 76 }),
      NOW,
    );
    expect(presentation).toMatchObject({
      phase: 'latched',
      stale: false,
      summary: 'Temperature 76°C now, back under the 80°C alert level',
      detail: 'Stays open until it reaches 75°C or lower and stays there for 5 minutes.',
      phaseLabel: 'Alert still open',
    });
    expect(presentation?.lastBreach).toMatch(/^Last reading at or above 80°C: 80°C, /);
  });

  it('says a reading at the clear level is recovering, with progress once it has some', () => {
    expect(
      getMetricAlertPresentation(
        minipcAlert({ phase: 'recovering', value: 72, recoveryElapsedSeconds: 0 }),
        NOW,
      ),
    ).toMatchObject({
      summary: 'Temperature 72°C now, recovering',
      detail: 'Clears after 5 minutes at 75°C or lower.',
      phaseLabel: 'Recovering',
    });
    expect(
      getMetricAlertPresentation(
        minipcAlert({ phase: 'recovering', value: 72, recoveryElapsedSeconds: 120 }),
        NOW,
      )?.detail,
    ).toBe('Clears after 5 minutes at 75°C or lower, 2 minutes so far.');
  });

  it('names the alert level while the reading is still over it', () => {
    const presentation = getMetricAlertPresentation(
      minipcAlert({ phase: 'breaching', value: 82 }),
      NOW,
    );
    expect(presentation).toMatchObject({
      summary: 'Temperature 82°C, above the 80°C alert level',
      detail: 'Clears once it drops to 75°C or lower and stays there for 5 minutes.',
      phaseLabel: 'Above 80°C',
    });
    expect(presentation?.lastBreach).toBeUndefined();
  });

  it('stops calling an old evaluation "now"', () => {
    const presentation = getMetricAlertPresentation(
      minipcAlert({
        phase: 'latched',
        value: 76,
        observedAt: new Date(NOW - METRIC_ALERT_STATUS_STALE_MS - 60_000).toISOString(),
      }),
      NOW,
    );
    expect(presentation?.stale).toBe(true);
    expect(presentation?.summary).toMatch(/^Last reading: Temperature 76°C, /);
    expect(presentation?.summary).not.toContain('now');
    expect(presentation?.phaseLabel).toBe('No recent reading');
  });

  it('dates the last breach from the live status when the alert has no lastSeen', () => {
    // Websocket active alerts omit lastSeen; the hover read "Last reading at
    // or above 80°C: 80°C" with no time.
    const websocketAlert: Alert = {
      ...minipcAlert({
        phase: 'latched',
        value: 76,
        lastBreachAt: new Date(NOW - 3 * 60_000).toISOString(),
      }),
      lastSeen: undefined,
    };
    expect(getMetricAlertPresentation(websocketAlert, NOW)?.lastBreach).toBe(
      'Last reading at or above 80°C: 80°C, 3 mins ago',
    );
  });

  it('checks each breach time before choosing it', () => {
    const lastSeen = Date.parse('2026-10-06T10:48:52Z');
    const breachAt = (lastBreachAt?: string) =>
      getMetricAlertLastBreachMs(minipcAlert({ phase: 'latched', lastBreachAt }));
    expect(breachAt('2026-10-06T10:40:00Z')).toBe(Date.parse('2026-10-06T10:40:00Z'));
    // A missing, unparseable or Go zero time falls back to lastSeen.
    expect(breachAt(undefined)).toBe(lastSeen);
    expect(breachAt('not a time')).toBe(lastSeen);
    expect(breachAt('0001-01-01T00:00:00Z')).toBe(lastSeen);
    expect(
      getMetricAlertLastBreachMs({ ...minipcAlert({ phase: 'latched' }), lastSeen: undefined }),
    ).toBeUndefined();
  });

  it('shows the averaged value the rule follows beside the latest sample', () => {
    const alert: Alert = {
      ...minipcAlert(),
      type: 'cpu',
      metricStatus: {
        phase: 'latched',
        value: 84.2,
        rawValue: 61,
        evaluationWindowSeconds: 600,
        unit: '%',
        observedAt: new Date(NOW).toISOString(),
        trigger: 90,
        recovery: 80,
      },
    };
    expect(getMetricAlertPresentation(alert, NOW)).toMatchObject({
      summary: 'CPU averaged 84% over 10 minutes, latest 61% now, back under the 90% alert level',
      detail: 'Stays open until it reaches 80% or lower.',
    });
  });

  it('returns null without a live status so callers keep the alert message', () => {
    expect(getMetricAlertPresentation(minipcAlert(), NOW)).toBeNull();
    expect(getAlertAttentionCopy(minipcAlert(), NOW)).toEqual({
      message: 'Node temperature at 80.0°C',
    });
  });
});

describe('getAlertAttentionCopy', () => {
  it('replaces the stale breach message and keeps the breach in the hover text', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    const copy = getAlertAttentionCopy(minipcAlert({ phase: 'recovering', value: 72 }), NOW);
    vi.useRealTimers();
    expect(copy.message).toBe('Temperature 72°C now, recovering');
    expect(copy.detail).toBe('Clears after 5 minutes at 75°C or lower.');
    expect(copy.title).toContain('Last reading at or above 80°C: 80°C');
    expect(copy.message).not.toContain('80');
  });
});
