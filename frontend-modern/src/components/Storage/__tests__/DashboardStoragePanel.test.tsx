import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import { DashboardStoragePanel } from '@/components/Storage/DashboardStoragePanel';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { TrendData } from '@/hooks/useDashboardTrends';

describe('DashboardStoragePanel', () => {
  it('renders the empty state when no storage resources exist', () => {
    const storage: DashboardOverview['storage'] = {
      total: 0,
      totalCapacity: 0,
      totalUsed: 0,
      warningCount: 0,
      criticalCount: 0,
    };

    render(() => <DashboardStoragePanel storage={storage} storageTrend={null} loading={false} />);

    expect(screen.getByText('No storage resources')).toBeInTheDocument();
  });

  it('renders storage usage, issue badges, and the 24h delta', () => {
    const storage: DashboardOverview['storage'] = {
      total: 2,
      totalCapacity: 4000,
      totalUsed: 2000,
      warningCount: 1,
      criticalCount: 1,
    };
    const storageTrend: TrendData = {
      points: [
        { timestamp: 1, value: 40 },
        { timestamp: 2, value: 50 },
      ],
      delta: 10,
      currentValue: 50,
    };

    render(() => (
      <DashboardStoragePanel storage={storage} storageTrend={storageTrend} loading={false} />
    ));

    expect(screen.getByRole('heading', { name: /Storage/i })).toBeInTheDocument();
    expect(screen.getByText('50%')).toBeInTheDocument();
    expect(screen.getByText('1 warnings')).toBeInTheDocument();
    expect(screen.getByText('1 critical')).toBeInTheDocument();
    expect(screen.getByText('24h: +10.0%')).toBeInTheDocument();
  });
});
