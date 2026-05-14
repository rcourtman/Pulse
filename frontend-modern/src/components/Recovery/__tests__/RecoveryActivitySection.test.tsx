import { fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
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
        chartRangeDays={() => 30}
        isMobile={false}
        loading={() => false}
        onRangeChange={() => undefined}
        overallRollupsSummary={() => ({ total: 2, stale: 0, neverSucceeded: 0 })}
        dayFilterKey={() => null}
        timeline={timeline}
        toggleDayFilter={() => undefined}
      />
    ));

    const bars = screen.getByTestId('recovery-activity-bars');
    expect(bars.className).toContain('items-stretch');
    expect(bars.style.gap).toBe('3px');
    expect(bars.parentElement?.className).toContain('h-[136px] sm:h-[150px]');
    expect(bars.closest('.grid')?.firstElementChild?.className).toContain('h-[120px] sm:h-[134px]');
    expect(screen.getByTestId('recovery-activity-chart-scroll').className).toContain(
      'overflow-x-auto',
    );
    expect(
      screen.queryByText('Daily recovery points across the selected history window.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/^Range$/)).not.toBeInTheDocument();
    expect(screen.queryByText(/1.5 \/ day/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Lowest active day')).not.toBeInTheDocument();
    expect(screen.queryByText('Below normal')).not.toBeInTheDocument();
    expect(within(bars).getAllByRole('button', { name: /recovery point/i })).toHaveLength(2);
    expect(within(bars).getAllByRole('button', { pressed: false })).toHaveLength(2);
  });

  it('uses day-filter styling and a structured activity tooltip', () => {
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
        chartRangeDays={() => 30}
        isMobile={false}
        loading={() => false}
        onRangeChange={() => undefined}
        overallRollupsSummary={() => ({ total: 2, stale: 0, neverSucceeded: 0 })}
        dayFilterKey={() => '2026-02-14'}
        timeline={timeline}
        toggleDayFilter={() => undefined}
      />
    ));

    expect(screen.queryByTestId('recovery-activity-selected-summary')).not.toBeInTheDocument();

    const bars = screen.getByTestId('recovery-activity-bars');
    const buttons = within(bars).getAllByRole('button', { name: /recovery point/i });
    expect(buttons[0].className).not.toContain('opacity-40');
    expect(buttons[1].className).not.toContain('ring-blue-500');
    expect(within(buttons[0]).getByTestId('recovery-activity-bar-stack').className).toContain(
      'opacity-40',
    );
    expect(within(buttons[1]).getByTestId('recovery-activity-bar-stack').className).toContain(
      'ring-blue-500',
    );
    expect(within(bars).getByRole('button', { pressed: true })).toBe(buttons[1]);

    fireEvent.mouseEnter(buttons[1]);

    const tooltip = screen.getByTestId('recovery-activity-tooltip');
    expect(within(tooltip).getByText('Day filter')).toBeInTheDocument();
    expect(within(tooltip).getByText('2 recovery points')).toBeInTheDocument();
    expect(within(tooltip).getByText('Snapshots')).toBeInTheDocument();
    expect(within(tooltip).getByText('Local Copies')).toBeInTheDocument();
    expect(within(tooltip).getByText('Remote Copies')).toBeInTheDocument();
    expect(within(tooltip).getByText('2 (100%)')).toBeInTheDocument();
    expect(tooltip.textContent).not.toContain(' • ');

    fireEvent.mouseLeave(buttons[1]);
    expect(screen.queryByTestId('recovery-activity-tooltip')).not.toBeInTheDocument();
  });

  it('updates an open tooltip when the hovered day becomes selected', () => {
    const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
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
        chartRangeDays={() => 30}
        isMobile={false}
        loading={() => false}
        onRangeChange={() => undefined}
        overallRollupsSummary={() => ({ total: 2, stale: 0, neverSucceeded: 0 })}
        dayFilterKey={selectedDateKey}
        timeline={timeline}
        toggleDayFilter={(key) => setSelectedDateKey((previous) => (previous === key ? null : key))}
      />
    ));

    const bars = screen.getByTestId('recovery-activity-bars');
    const buttons = within(bars).getAllByRole('button', { name: /recovery point/i });

    fireEvent.mouseEnter(buttons[1]);
    const tooltip = screen.getByTestId('recovery-activity-tooltip');
    expect(within(tooltip).getByText('Timeline day')).toBeInTheDocument();

    fireEvent.click(buttons[1]);

    expect(within(tooltip).getByText('Day filter')).toBeInTheDocument();
    expect(screen.queryByTestId('recovery-activity-selected-summary')).not.toBeInTheDocument();
    const selectedButton = within(bars).getByRole('button', { pressed: true });
    const dimmedButton = within(bars).getByRole('button', { pressed: false });
    expect(selectedButton.className).not.toContain('ring-blue-500');
    expect(dimmedButton.className).not.toContain('opacity-40');
    expect(within(selectedButton).getByTestId('recovery-activity-bar-stack').className).toContain(
      'ring-blue-500',
    );
    expect(within(dimmedButton).getByTestId('recovery-activity-bar-stack').className).toContain(
      'opacity-40',
    );
  });
});
