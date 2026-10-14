import { describe, expect, it } from 'vitest';
import type { DetailSection } from '@/components/shared/detailSectionModel';
import type { Alert } from '@/types/api';
import {
  collectDrawerExceptionRows,
  getDrawerChartAlerts,
  getDrawerMetricAlert,
  isDrawerAvailabilityHealthy,
  withoutDrawerExceptionRows,
} from '../drawerOverviewModel';

const alert = (overrides: Partial<Alert>): Alert =>
  ({ id: 'a', type: 'memory', level: 'warning', threshold: 85, ...overrides }) as Alert;

describe('getDrawerMetricAlert', () => {
  it('matches the metric and prefers a critical alert over a warning', () => {
    expect(
      getDrawerMetricAlert(
        [
          alert({ id: 'w', level: 'warning', threshold: 85 }),
          alert({ id: 'c', level: 'critical', threshold: 95 }),
          alert({ id: 'w2', level: 'warning', threshold: 80 }),
        ],
        'memory',
      ),
    ).toEqual({ severity: 'critical', threshold: 95 });
  });

  it('ignores other metrics and info-level alerts', () => {
    expect(
      getDrawerMetricAlert([alert({ type: 'cpu' }), alert({ level: 'info' })], 'memory'),
    ).toBeUndefined();
  });

  it('drops a non-finite threshold rather than drawing a line at NaN', () => {
    expect(getDrawerMetricAlert([alert({ threshold: Number.NaN })], 'memory')).toEqual({
      severity: 'warning',
      threshold: undefined,
    });
  });
});

describe('getDrawerChartAlerts', () => {
  it('converts I/O triggers from the alert engine MiB/s to the charts B/s', () => {
    expect(
      getDrawerChartAlerts(
        [
          alert({ type: 'diskRead', threshold: 100 }),
          alert({ type: 'networkIn', level: 'critical', threshold: 2 }),
        ],
        ['diskread', 'netin'],
      ),
    ).toEqual({
      diskread: { severity: 'warning', value: 100 * 1024 * 1024 },
      netin: { severity: 'critical', value: 2 * 1024 * 1024 },
    });
  });

  it('keeps percentage and temperature triggers in their own unit', () => {
    expect(
      getDrawerChartAlerts(
        [alert({ type: 'cpu', threshold: 90 }), alert({ type: 'temperature', threshold: 80 })],
        ['cpu', 'temperature'],
      ),
    ).toEqual({
      cpu: { severity: 'warning', value: 90 },
      temperature: { severity: 'warning', value: 80 },
    });
  });

  it('keeps the severity of an alert with no numeric trigger, without a line value', () => {
    expect(getDrawerChartAlerts([alert({ threshold: Number.NaN })], ['memory'])).toEqual({
      memory: { severity: 'warning' },
    });
  });

  it('maps disk temperature and storage capacity alerts onto the charts that show them', () => {
    expect(
      getDrawerChartAlerts(
        [
          alert({ type: 'diskTemperature', threshold: 55 }),
          alert({ type: 'usage', threshold: 90 }),
        ],
        ['smart_temp', 'usage'],
      ),
    ).toEqual({
      smart_temp: { severity: 'warning', value: 55 },
      usage: { severity: 'warning', value: 90 },
    });
    // A Proxmox disk's temperature alert reaches the same disk chart.
    expect(
      getDrawerChartAlerts([alert({ type: 'temperature', threshold: 60 })], ['smart_temp']),
    ).toEqual({ smart_temp: { severity: 'warning', value: 60 } });
  });

  it('ignores alerts on metrics the drawer does not chart', () => {
    expect(getDrawerChartAlerts([alert({ type: 'diskWrite' })], ['cpu', 'memory'])).toEqual({});
  });
});

describe('drawer exception rows', () => {
  const sections: DetailSection[] = [
    {
      label: 'Summary',
      rows: [
        { label: 'System', value: 'Debian' },
        { label: 'Updates', value: 'Unavailable', tone: 'warning' },
      ],
    },
    {
      label: 'Thermals',
      rows: [{ label: 'CPU record', value: '91°C', tone: 'danger' }],
    },
    { label: 'Telemetry', rows: [{ label: 'Status', value: 'Stopped', tone: 'warning' }] },
  ];

  it('lifts warning and danger rows from every section, once each', () => {
    expect(collectDrawerExceptionRows([...sections, sections[0]]).map((row) => row.label)).toEqual([
      'Updates',
      'CPU record',
      'Status',
    ]);
  });

  it('moves lifted rows out of Details and drops sections they emptied', () => {
    const remaining = withoutDrawerExceptionRows(sections);
    expect(remaining.map((section) => section.label)).toEqual(['Summary']);
    expect(remaining[0].rows.map((row) => row.label)).toEqual(['System']);
  });
});

describe('isDrawerAvailabilityHealthy', () => {
  const now = Date.parse('2026-10-13T10:00:00Z');
  const fresh = { pollIntervalSeconds: 30, lastChecked: '2026-10-13T09:59:30Z' };

  it('judges a fresh check by the aggregate state when present, else by availability', () => {
    expect(
      isDrawerAvailabilityHealthy({ ...fresh, aggregateState: 'healthy', available: false }, now),
    ).toBe(true);
    expect(
      isDrawerAvailabilityHealthy({ ...fresh, aggregateState: 'degraded', available: true }, now),
    ).toBe(false);
    expect(isDrawerAvailabilityHealthy({ ...fresh, available: true }, now)).toBe(true);
    expect(isDrawerAvailabilityHealthy({ ...fresh }, now)).toBe(false);
  });

  it('does not call an expired or undated success passing', () => {
    // Two missed polls: the success is stale evidence.
    expect(
      isDrawerAvailabilityHealthy(
        { available: true, pollIntervalSeconds: 30, lastChecked: '2026-10-13T09:58:00Z' },
        now,
      ),
    ).toBe(false);
    expect(
      isDrawerAvailabilityHealthy(
        { available: true, evidence: { validUntil: '2026-10-13T09:59:00Z' } } as never,
        now,
      ),
    ).toBe(false);
    // No interval and no validity window: freshness is unknown, not passing.
    expect(isDrawerAvailabilityHealthy({ available: true }, now)).toBe(false);
  });
});
