import { render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Alert, MetricAlertStatus } from '@/types/api';

import { AlertHistoryTableAlertRow } from '../AlertHistoryTableAlertRow';
import type { HistoryItem } from '../alertHistoryModel';
import type { AlertHistoryState } from '../useAlertHistoryState';

// Open rows offer the Assistant investigation, which needs the router.
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

const NOW = Date.parse('2026-10-06T19:00:00Z');

function createState() {
  const [expandedIncidents] = createSignal(new Set<string>());
  const [resourceIncidentPanel] = createSignal(null);
  return {
    getIncidentRowKey: (alert: HistoryItem) => `${alert.id}-row`,
    expandedIncidents,
    resourceIncidentPanel,
    formatAlertRowTime: () => '18:30',
    formatAlertRowTimestamp: () => 'Tuesday, 6 October 2026 at 18:30:00',
  } as unknown as AlertHistoryState;
}

const historyItem = (overrides: Partial<HistoryItem>): HistoryItem => ({
  id: 'minipc::metric-threshold:temperature',
  source: 'alert',
  status: 'active',
  startTime: '2026-10-06T18:30:00Z',
  duration: '30m',
  resourceName: 'minipc',
  resourceType: 'Node',
  resourceId: 'minipc',
  node: 'minipc',
  severity: 'warning',
  title: 'Temperature',
  rawAlertType: 'temperature',
  description: 'Node temperature at 95.0°C',
  ...overrides,
});

const renderRow = (alert: HistoryItem) =>
  render(() => (
    <table>
      <tbody>
        <AlertHistoryTableAlertRow alert={alert} state={createState()} />
      </tbody>
    </table>
  ));

describe('AlertHistoryTableAlertRow', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  // minipc on 2026-10-06: the node read 78°C under its 80°C trigger and the
  // open row still said "Node temperature at 95.0°C".
  it('leads an open threshold alert with the reading now and follows it live', () => {
    const [liveAlert, setLiveAlert] = createStore<
      Pick<Alert, 'type' | 'value' | 'lastSeen' | 'metricStatus'>
    >({
      type: 'temperature',
      value: 95,
      lastSeen: '2026-10-06T18:40:00Z',
      metricStatus: {
        phase: 'latched',
        value: 78,
        unit: '°C',
        observedAt: new Date(NOW - 10_000).toISOString(),
        trigger: 80,
        recovery: 75,
        recoveryDelaySeconds: 300,
      },
    });
    renderRow(historyItem({ liveAlert }));

    const cell = screen.getByText('Temperature 78°C now, back under the 80°C alert level');
    expect(cell.getAttribute('title')?.split('\n')).toEqual([
      'Temperature 78°C now, back under the 80°C alert level',
      'Stays open until it reaches 75°C or lower and stays there for 5 minutes.',
      'Last reading at or above 80°C: 95°C, 20 mins ago',
    ]);
    expect(screen.queryByText('Node temperature at 95.0°C')).toBeNull();

    // The next evaluation reaches the clear level; the row follows it.
    setLiveAlert('metricStatus', (status) => ({
      ...(status as MetricAlertStatus),
      phase: 'recovering',
      value: 72,
      observedAt: new Date(NOW).toISOString(),
    }));
    expect(cell.textContent).toBe('Temperature 72°C now, recovering');

    // Evaluations stop: on the shared clock the reading stops being "now".
    vi.advanceTimersByTime(15 * 60 * 1000);
    expect(cell.textContent).toMatch(/^Last reading: Temperature 72°C, /);
  });

  it('keeps the recorded message on a resolved row', () => {
    renderRow(
      historyItem({
        status: 'resolved',
        endTime: '2026-10-06T18:50:00Z',
        description: 'Resolved: Node temperature at 74.0°C',
      }),
    );
    expect(screen.getByText('Resolved: Node temperature at 74.0°C')).toHaveAttribute(
      'title',
      'Resolved: Node temperature at 74.0°C',
    );
  });
});
