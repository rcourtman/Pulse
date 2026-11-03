import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { HistoryChartOverlay } from '@/components/shared/HistoryChartOverlay';
import type { HistoryChartState } from '@/components/shared/useHistoryChartState';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { SecurityStatusPresentationPolicy } from '@/types/config';

const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

const lockedChart = {
  loading: () => false,
  data: () => [],
  error: () => null,
  isLocked: () => true,
  lockDays: () => 7,
} as unknown as HistoryChartState;

const setPresentationPolicy = (policy: Partial<SecurityStatusPresentationPolicy>) =>
  syncSessionPresentationPolicy({
    presentationPolicy: { demoMode: false, readOnly: false, ...policy },
  } as Parameters<typeof syncSessionPresentationPolicy>[0]);

describe('HistoryChartOverlay plan lock', () => {
  afterEach(() => {
    cleanup();
    syncSessionPresentationPolicy(null);
  });

  it('links a locked range to Plans & Billing on a free install', () => {
    // Free self-hosted installs hide proactive upgrade prompts, but a lock the
    // operator opened by choosing a range still says where the plan lives.
    setPresentationPolicy({ hideCommercial: false, hideUpgrade: true });
    renderInRouter(() => <HistoryChartOverlay chart={lockedChart} />);

    expect(
      screen.getByText('Historical data beyond 7 days requires a higher license plan.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Plans & Billing' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/billing/plan',
    );
  });

  it('keeps the lock text only when commercial surfaces are hidden', () => {
    setPresentationPolicy({ hideCommercial: true, hideUpgrade: true });
    renderInRouter(() => <HistoryChartOverlay chart={lockedChart} />);

    expect(
      screen.getByText('Historical data beyond 7 days requires a higher license plan.'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('link')).toBeNull();
  });

  it('shows no lock or link when the chart asks to hide its lock', () => {
    setPresentationPolicy({ hideCommercial: false, hideUpgrade: false });
    renderInRouter(() => <HistoryChartOverlay chart={lockedChart} hideLock />);

    expect(screen.queryByText(/requires a higher license plan/)).toBeNull();
    expect(screen.queryByRole('link')).toBeNull();
  });
});
