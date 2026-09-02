import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PatrolDigest } from '@/api/patrol';

const apiMocks = vi.hoisted(() => ({ getDigest: vi.fn() }));

vi.mock('@/api/patrol', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/patrol')>();
  return {
    ...original,
    getPatrolDigest: (...args: unknown[]) => apiMocks.getDigest(...args),
  };
});

vi.mock('@/components/shared/Button', () => ({
  Button: (props: { children?: unknown; onClick?: () => void; 'aria-label'?: string }) => (
    <button type="button" aria-label={props['aria-label']} onClick={props.onClick}>
      {props.children as never}
    </button>
  ),
  ButtonLink: (props: { href: string; children?: unknown }) => (
    <a href={props.href}>{props.children as never}</a>
  ),
}));

import {
  PatrolWeeklyDigestCard,
  buildDigestTiles,
  describeDigestInvestigationOutcomes,
  describeDigestOpenFindings,
} from '../PatrolWeeklyDigestCard';

const digest = (overrides: Partial<PatrolDigest> = {}): PatrolDigest => ({
  generated_at: '2026-09-01T12:00:00Z',
  window: {
    start: '2026-08-25T12:00:00Z',
    end: '2026-09-01T12:00:00Z',
    days: 7,
    history_complete: true,
  },
  mode: 'approval',
  runs: {
    total: 38,
    scheduled: 34,
    event_triggered: 3,
    manual: 1,
    failed: 0,
    checks: 1520,
    resources_covered: 40,
    last_run_at: '2026-09-01T11:00:00Z',
  },
  findings: {
    new: 12,
    open_by_severity: { critical: 1, warning: 3, watch: 0, info: 0 },
    resolved: 9,
    auto_resolved: 7,
    dismissed: 2,
    suppressed: 0,
  },
  investigations: { total: 4, by_outcome: { fix_verified: 2, needs_attention: 1, resolved: 1 } },
  actions: {
    proposed: 3,
    approved: 2,
    rejected: 0,
    executed: 2,
    verified: 1,
    failed: 0,
    pending: 1,
  },
  alerts: { reviewed: 5 },
  spend: {
    estimated_usd: 1.2345,
    pricing_known: true,
    input_tokens: 4_000_000,
    output_tokens: 200_000,
    calls: 40,
  },
  ...overrides,
});

describe('PatrolWeeklyDigestCard', () => {
  beforeEach(() => apiMocks.getDigest.mockReset());
  afterEach(cleanup);

  it('adds up the week in customer terms and links pending fixes to Actions', async () => {
    apiMocks.getDigest.mockResolvedValue(digest());

    render(() => <PatrolWeeklyDigestCard />);

    expect(await screen.findByText('Patrol runs')).toBeInTheDocument();
    expect(apiMocks.getDigest).toHaveBeenCalledWith(7);
    expect(screen.getByText('38')).toBeInTheDocument();
    expect(screen.getByText('1520 checks across 40 resources.')).toBeInTheDocument();
    expect(screen.getByText('5 alerts looked into.')).toBeInTheDocument();
    expect(screen.getByText('New issues')).toBeInTheDocument();
    expect(screen.getByText('4 still open (1 critical, 3 warning).')).toBeInTheDocument();
    expect(screen.getByText('7 cleared by Patrol on its own.')).toBeInTheDocument();
    expect(screen.getByText('2 dismissed by you.')).toBeInTheDocument();
    expect(screen.getByText('1 need you,')).toBeInTheDocument();
    expect(screen.getByText('2 fixed and verified.')).toBeInTheDocument();
    expect(screen.getByText('1 of 2 verified afterwards.')).toBeInTheDocument();
    const pendingLink = screen.getByText('1 fix waiting for your approval');
    expect(pendingLink.closest('a')).toHaveAttribute('href', '/actions');
    expect(screen.getByText('$1.23')).toBeInTheDocument();
    expect(screen.getByText('40 model calls.')).toBeInTheDocument();
    // The page header already states the mode sentence; the card must not repeat it.
    expect(screen.queryByText(/every change waits for your approval/)).not.toBeInTheDocument();
    expect(screen.getByText(/Last 7 days/)).toBeInTheDocument();
    // Forensic vocabulary stays out of the customer summary.
    expect(screen.queryByText(/evidence class/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/verdict/i)).not.toBeInTheDocument();
  });

  it('says plainly when Patrol has not run and when history is cut short', async () => {
    apiMocks.getDigest.mockResolvedValueOnce(
      digest({
        runs: {
          total: 0,
          scheduled: 0,
          event_triggered: 0,
          manual: 0,
          failed: 0,
          checks: 0,
          resources_covered: 0,
        },
      }),
    );
    render(() => <PatrolWeeklyDigestCard />);
    expect(await screen.findByText('Patrol has not run in the last 7 days')).toBeInTheDocument();
    expect(screen.queryByText('Patrol runs')).not.toBeInTheDocument();
    cleanup();

    apiMocks.getDigest.mockResolvedValueOnce(
      digest({
        window: {
          start: '2026-08-25T12:00:00Z',
          end: '2026-09-01T12:00:00Z',
          days: 7,
          history_complete: false,
          history_since: '2026-08-29T08:00:00Z',
        },
      }),
    );
    render(() => <PatrolWeeklyDigestCard />);
    expect(await screen.findByText(/older runs are no longer kept/)).toBeInTheDocument();
  });

  it('reports a failed load without pretending the week was empty', async () => {
    apiMocks.getDigest.mockRejectedValueOnce(new Error('digest offline'));
    render(() => <PatrolWeeklyDigestCard />);
    expect(await screen.findByText("This week's summary is unavailable")).toBeInTheDocument();
    expect(screen.getByText('digest offline')).toBeInTheDocument();
    expect(screen.queryByText(/Patrol has not run/)).not.toBeInTheDocument();
  });

  it('keeps tile copy honest for watch-only installs and unknown pricing', () => {
    const tiles = buildDigestTiles(
      digest({
        mode: 'monitor',
        investigations: { total: 0, by_outcome: {} },
        actions: {
          proposed: 0,
          approved: 0,
          rejected: 0,
          executed: 0,
          verified: 0,
          failed: 0,
          pending: 0,
        },
        spend: {
          estimated_usd: 0.5,
          pricing_known: false,
          input_tokens: 1,
          output_tokens: 1,
          calls: 3,
        },
      }),
    );
    const byId = Object.fromEntries(tiles.map((tile) => [tile.id, tile]));
    expect(byId.investigated.details).toEqual([
      'Patrol is watch only, so it reports issues without investigating them.',
    ]);
    expect(byId.actions.details).toEqual(['Patrol is watch only, so no fixes were proposed.']);
    expect(byId.spend.details).toEqual([
      '3 model calls.',
      'Some calls used a model with no known price.',
    ]);
    expect(describeDigestOpenFindings(digest({ findings: { ...digest().findings, new: 0 } }))).toBe(
      'Nothing new was raised.',
    );
    expect(
      describeDigestOpenFindings(
        digest({
          findings: {
            ...digest().findings,
            open_by_severity: { critical: 0, warning: 0, watch: 0, info: 0 },
          },
        }),
      ),
    ).toBe('All of them have since cleared.');
    expect(
      describeDigestInvestigationOutcomes({ resolved: 3, fix_failed: 1, cannot_fix: 2 }),
    ).toEqual(['1 fix failed', '2 could not fix']);
  });
});
