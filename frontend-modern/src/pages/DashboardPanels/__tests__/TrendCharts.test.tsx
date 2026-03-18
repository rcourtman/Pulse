import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { TrendCharts } from '../TrendCharts';
import type { DashboardTrends } from '@/hooks/useDashboardTrends';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { HistoryTimeRange } from '@/api/charts';

function makeOverview(overrides: Partial<DashboardOverview> = {}): DashboardOverview {
  return {
    health: { totalResources: 0, byStatus: {}, criticalAlerts: 0, warningAlerts: 0 },
    infrastructure: { total: 0, byStatus: {}, byType: {}, topCPU: [], topMemory: [] },
    workloads: { total: 0, running: 0, stopped: 0, byType: {} },
    storage: { total: 0, totalCapacity: 0, totalUsed: 0, warningCount: 0, criticalCount: 0 },
    alerts: { activeCritical: 0, activeWarning: 0, total: 0 },
    problemResources: [],
    ...overrides,
  };
}

function makeTrends(overrides: Partial<DashboardTrends> = {}): DashboardTrends {
  return {
    infrastructure: { cpu: new Map(), memory: new Map() },
    storage: { capacity: null },
    loading: false,
    error: null,
    ...overrides,
  };
}

describe('TrendCharts', () => {
  it('does not render an error banner when trends.error is null', () => {
    const [range, setRange] = createSignal<HistoryTimeRange>('1h');
    render(() => (
      <TrendCharts
        trends={makeTrends({ error: null })}
        overview={makeOverview()}
        trendRange={range}
        setTrendRange={setRange}
      />
    ));

    expect(screen.queryByText('Unable to load trends')).toBeNull();
  });

  it('renders an error banner when trends.error is set', () => {
    const [range, setRange] = createSignal<HistoryTimeRange>('1h');
    render(() => (
      <TrendCharts
        trends={makeTrends({ error: 'metrics store unreachable' })}
        overview={makeOverview()}
        trendRange={range}
        setTrendRange={setRange}
      />
    ));

    expect(screen.getByText('Unable to load trends')).toBeTruthy();
  });
});
