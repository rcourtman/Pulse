import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { RecoveryActivitySection } from '@/components/Recovery/RecoveryActivitySection';

describe('RecoveryActivitySection', () => {
  it('renders the bar row with stretch sizing so bars can take visible height', () => {
    const timeline = () => ({
      points: [
        { key: '2026-02-13', label: 'Feb 13', total: 1, snapshot: 1, local: 0, remote: 0 },
        { key: '2026-02-14', label: 'Feb 14', total: 2, snapshot: 0, local: 2, remote: 0 },
      ],
      axisMax: 2,
      labelEvery: 1,
    });

    render(() => (
      <RecoveryActivitySection
        activitySummary={() => ({ totalPoints: 3, activeDays: 2, averagePerDay: 1.5 })}
        activeClusterLabel={() => ''}
        activeItemTypeLabel={() => ''}
        activeNamespaceLabel={() => ''}
        activeNodeLabel={() => ''}
        chartRangeDays={() => 30}
        clearClusterFilter={() => undefined}
        clearItemTypeFilter={() => undefined}
        clearNamespaceFilter={() => undefined}
        clearNodeFilter={() => undefined}
        clearSelectedDate={() => undefined}
        isMobile={false}
        loading={() => false}
        overallRollupsSummary={() => ({ total: 2, stale: 0, neverSucceeded: 0 })}
        selectedDateKey={() => null}
        selectedDateLabel={() => ''}
        timeline={timeline}
        toggleSelectedDate={() => undefined}
      />
    ));

    const bars = screen.getByTestId('recovery-activity-bars');
    expect(bars.className).toContain('items-stretch');
    expect(bars.parentElement?.className).toContain('h-20');
    expect(
      screen.queryByText('Daily recovery points across the selected history window.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/^Range$/)).not.toBeInTheDocument();
    expect(screen.queryByText(/1.5 \/ day/i)).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /recovery points/i })).toHaveLength(2);
  });
});
