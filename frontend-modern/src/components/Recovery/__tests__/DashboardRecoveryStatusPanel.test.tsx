import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import { DashboardRecoveryStatusPanel } from '@/components/Recovery/DashboardRecoveryStatusPanel';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';

describe('DashboardRecoveryStatusPanel', () => {
  it('renders the empty state when no recovery data exists', () => {
    const recovery: DashboardRecoverySummary = {
      totalProtected: 0,
      byOutcome: {},
      latestEventTimestamp: null,
      hasData: false,
    };

    render(() => <DashboardRecoveryStatusPanel recovery={recovery} />);

    expect(screen.getByText('No recovery data available')).toBeInTheDocument();
  });

  it('renders summary counts and stale messaging for old recovery activity', () => {
    const recovery: DashboardRecoverySummary = {
      totalProtected: 3,
      byOutcome: { success: 2, failed: 1 },
      latestEventTimestamp: Date.now() - 25 * 60 * 60_000,
      hasData: true,
    };

    render(() => <DashboardRecoveryStatusPanel recovery={recovery} />);

    expect(screen.getByRole('heading', { name: 'Recovery Status' })).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('success')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.getByText('Last recovery point over 24 hours ago')).toBeInTheDocument();
  });
});
