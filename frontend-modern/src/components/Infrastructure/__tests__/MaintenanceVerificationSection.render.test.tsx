import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { MaintenanceVerificationReport } from '@/api/maintenanceVerification';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

const api = vi.hoisted(() => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/utils/apiClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/utils/apiClient')>()),
  apiFetchJSON: api.apiFetchJSON,
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { error: vi.fn(), success: vi.fn() },
}));

import { MaintenanceVerificationSection } from '../MaintenanceVerificationSection';

const start = Date.parse('2026-10-11T04:00:00Z');
const iso = (offset: number) => new Date(start + offset).toISOString();
const resourceId = 'cluster:pve-a:101';
const report = (
  overrides: Partial<MaintenanceVerificationReport> = {},
): MaintenanceVerificationReport => ({
  id: 'report:window',
  resourceId,
  trigger: 'maintenance_window_ended',
  status: 'healthy',
  startedAt: iso(-30_000),
  completedAt: iso(-30_000),
  windowStartedAt: iso(-3_600_000),
  windowEndedAt: iso(-30_000),
  evidence: {
    activeCriticalAlerts: 0,
    activeWarningAlerts: 2,
    activeCriticalFindings: 0,
    activeWarningFindings: 1,
    failedActionsSinceWindowStart: 0,
    metricRecovery: { samplesAfterEnd: 3, trend: 'improving' },
  },
  linkedFindingIds: [],
  linkedAlertIds: [],
  linkedActionIds: [],
  recommendation: 'Recorded recovery evidence',
  ...overrides,
});
const response = (reports: MaintenanceVerificationReport[]) => ({ data: reports });
const mount = async (reports: MaintenanceVerificationReport[]) => {
  api.apiFetchJSON.mockResolvedValue(response(reports));
  const rendered = render(() => <MaintenanceVerificationSection resourceId={resourceId} />);
  await vi.advanceTimersByTimeAsync(0);
  return rendered;
};
const mutations = () =>
  api.apiFetchJSON.mock.calls.filter(([, options]) => options?.method === 'POST');

describe('durable maintenance verification evidence', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    resetCreateNonSuspendingQueryCacheForTest();
    vi.setSystemTime(start);
    api.apiFetchJSON.mockReset();
  });
  afterEach(() => {
    cleanup();
    resetCreateNonSuspendingQueryCacheForTest();
    vi.useRealTimers();
  });

  it('ages held window and review events without fetching, rewriting or rerunning the report', async () => {
    const saved = report({
      userOutcome: 'reviewed',
      reviewedAt: iso(-30_000),
      reviewedBy: 'fixture-admin',
    });
    const original = JSON.stringify(saved);
    await mount([saved]);
    expect(screen.getByText('Window ended 30s ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 30s ago by fixture-admin')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(screen.getByText('Window ended 1 min ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 1 min ago by fixture-admin')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(3_600_000);
    expect(screen.getByText('Window ended 1 hour ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 1 hour ago by fixture-admin')).toBeInTheDocument();
    expect(screen.getByTestId('maintenance-verification-status-healthy')).toHaveTextContent(
      'Healthy',
    );
    expect(screen.queryByTestId('maintenance-verification-review')).not.toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
    expect(mutations()).toEqual([]);
    expect(JSON.stringify(saved)).toBe(original);
  });

  it('shows when a check completed, separately from the earlier maintenance window and review', async () => {
    await mount([
      report({
        windowEndedAt: iso(-86_400_000),
        userOutcome: 'reviewed',
        reviewedAt: iso(-10_000),
      }),
    ]);
    expect(screen.getByText('Checked 30s ago')).toBeInTheDocument();
    expect(screen.getByText('Window ended 1 day ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 10s ago')).toBeInTheDocument();
    expect(
      screen.getByText(/Reports record past checks, not current health\./),
    ).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.getByText('Checked 1 min ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 1 min ago')).toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
  });

  it('keeps every recorded verdict and its counts unchanged as several reports age', async () => {
    const states = ['pending', 'healthy', 'needs_review', 'failed_verification'] as const;
    const records = states.map((status) => report({ id: `report:${status}`, status }));
    const original = JSON.stringify(records);
    await mount(records);
    await vi.advanceTimersByTimeAsync(120_000);
    for (const status of states) {
      const row = screen.getByTestId(`maintenance-verification-status-${status}`).closest('li')!;
      expect(within(row).getByText('Window ended 2 mins ago')).toBeInTheDocument();
      expect(within(row).getByText('Checked 2 mins ago')).toBeInTheDocument();
      expect(within(row).getByText('Warning alerts').nextElementSibling).toHaveTextContent('2');
      expect(within(row).getByText('Metric samples').nextElementSibling).toHaveTextContent('3');
      expect(within(row).getByTestId('maintenance-verification-review')).toBeEnabled();
    }
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
    expect(JSON.stringify(records)).toBe(original);
  });

  it.each(['', 'invalid', '0001-01-01T00:00:00Z'])(
    'keeps absent or malformed event times unknown (%j)',
    async (time) => {
      await mount([
        report({
          windowEndedAt: time,
          completedAt: time,
          userOutcome: 'reviewed',
          reviewedAt: time,
        }),
      ]);
      await vi.advanceTimersByTimeAsync(60_000);
      expect(screen.getByText('Window ended unknown')).toBeInTheDocument();
      expect(screen.getByText('Checked unknown')).toBeInTheDocument();
      expect(screen.getByText('Reviewed unknown')).toBeInTheDocument();
      expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps clock-skewed past events as ages rather than inventing a scheduled operation', async () => {
    await mount([report({ completedAt: iso(30_000), windowEndedAt: iso(30_000) })]);
    expect(screen.getByText('Checked 0s ago')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(90_000);
    expect(screen.getByText('Checked 1 min ago')).toBeInTheDocument();
    expect(screen.getByText('Window ended 1 min ago')).toBeInTheDocument();
    expect(mutations()).toEqual([]);
  });

  it('withdraws the old resource reports and ignores a late old-resource response across ticks', async () => {
    let finishOld!: (value: ReturnType<typeof response>) => void;
    api.apiFetchJSON.mockImplementation((path: string) =>
      path.includes('old-resource')
        ? new Promise((resolve) => {
            finishOld = resolve;
          })
        : Promise.resolve(response([])),
    );
    const [id, setId] = createSignal('old-resource');
    render(() => <MaintenanceVerificationSection resourceId={id()} />);
    await vi.advanceTimersByTimeAsync(0);
    setId('empty-resource');
    await vi.advanceTimersByTimeAsync(0);
    finishOld(response([report()]));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.queryByTestId('maintenance-verification-report')).not.toBeInTheDocument();
    expect(screen.getByText(/No verification reports yet/)).toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(2);
  });

  it('does not revive withdrawn reports after an access-denied resource read', async () => {
    api.apiFetchJSON
      .mockResolvedValueOnce(response([report()]))
      .mockRejectedValueOnce(new Error('Access denied'));
    const [id, setId] = createSignal(resourceId);
    render(() => <MaintenanceVerificationSection resourceId={id()} />);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByTestId('maintenance-verification-report')).toBeInTheDocument();
    setId('denied-resource');
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.queryByTestId('maintenance-verification-report')).not.toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(2);
  });

  it('requires an explicit review, keeps pending admission disabled and uses the returned review fields', async () => {
    const saved = report();
    await mount([saved]);
    let finishReview!: (value: MaintenanceVerificationReport) => void;
    api.apiFetchJSON.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishReview = resolve;
        }),
    );
    const button = screen.getByTestId('maintenance-verification-review');
    fireEvent.click(button);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(button).toBeDisabled();
    expect(button).toHaveTextContent('Marking…');
    fireEvent.click(button);
    expect(mutations()).toHaveLength(1);
    expect(mutations()[0][0]).toBe('/api/maintenance-verifications/report%3Awindow/review');
    expect(screen.getByTestId('maintenance-verification-status-healthy')).toHaveTextContent(
      'Healthy',
    );
    expect(screen.queryByText(/Reviewed/)).not.toBeInTheDocument();
    const reviewed = {
      ...saved,
      userOutcome: 'reviewed' as const,
      reviewedAt: iso(60_000),
      reviewedBy: 'fixture-admin',
    };
    api.apiFetchJSON.mockResolvedValue(response([reviewed]));
    finishReview(reviewed);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.queryByTestId('maintenance-verification-review')).not.toBeInTheDocument();
    expect(screen.getByText('Reviewed 0s ago by fixture-admin')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.getByText('Reviewed 1 min ago by fixture-admin')).toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(3);
  });

  it('reruns only after a click, keeps the pending action disabled and retains the old report beside the new check', async () => {
    const saved = report();
    await mount([saved]);
    let finishRerun!: (value: MaintenanceVerificationReport) => void;
    api.apiFetchJSON.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishRerun = resolve;
        }),
    );
    const button = screen.getByTestId('maintenance-verification-rerun');
    fireEvent.click(button);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(mutations()).toHaveLength(1);
    expect(mutations()[0][0]).toBe(
      '/api/resources/cluster%3Apve-a%3A101/maintenance-verifications/rerun',
    );
    const rerun = report({
      id: 'report:window-rerun-1',
      completedAt: iso(60_000),
      status: 'needs_review',
    });
    api.apiFetchJSON.mockResolvedValue(response([rerun, saved]));
    finishRerun(rerun);
    await vi.advanceTimersByTimeAsync(0);
    expect(button).toBeEnabled();
    expect(screen.getAllByTestId('maintenance-verification-report')).toHaveLength(2);
    expect(screen.getByText('Checked 0s ago')).toBeInTheDocument();
    expect(screen.getByTestId('maintenance-verification-status-healthy')).toHaveTextContent(
      'Healthy',
    );
    expect(screen.getByTestId('maintenance-verification-status-needs_review')).toHaveTextContent(
      'Needs review',
    );
    await vi.advanceTimersByTimeAsync(60_000);
    expect(mutations()).toHaveLength(1);
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(3);
  });

  it.each(['review', 'rerun'])(
    'does not retry a refused %s on a clock tick or alter the report',
    async (action) => {
      await mount([report()]);
      api.apiFetchJSON.mockRejectedValueOnce(new Error('Synthetic write refusal'));
      fireEvent.click(screen.getByTestId(`maintenance-verification-${action}`));
      await vi.advanceTimersByTimeAsync(60_000);
      expect(screen.getByTestId(`maintenance-verification-${action}`)).toBeEnabled();
      expect(screen.getByTestId('maintenance-verification-status-healthy')).toHaveTextContent(
        'Healthy',
      );
      expect(screen.getByText('Window ended 1 min ago')).toBeInTheDocument();
      expect(mutations()).toHaveLength(1);
      expect(api.apiFetchJSON).toHaveBeenCalledTimes(2);
    },
  );

  it('does not describe a pending first read as an empty report history', async () => {
    let finishRead!: (value: ReturnType<typeof response>) => void;
    api.apiFetchJSON.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishRead = resolve;
        }),
    );
    render(() => <MaintenanceVerificationSection resourceId={resourceId} />);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByText('Loading verification reports…')).toBeInTheDocument();
    expect(screen.queryByText(/No verification reports yet/)).not.toBeInTheDocument();
    finishRead(response([]));
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByText(/No verification reports yet/)).toBeInTheDocument();
  });

  it('withdraws loaded reports immediately while another resource read is pending', async () => {
    api.apiFetchJSON
      .mockResolvedValueOnce(response([report()]))
      .mockImplementationOnce(() => new Promise(() => {}));
    const [id, setId] = createSignal(resourceId);
    render(() => <MaintenanceVerificationSection resourceId={id()} />);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByTestId('maintenance-verification-report')).toBeInTheDocument();
    setId('new-resource-pending');
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.queryByTestId('maintenance-verification-report')).not.toBeInTheDocument();
    expect(screen.getByText('Loading verification reports…')).toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(2);
  });

  it('distinguishes a failed first read from empty history and retries only after an explicit read request', async () => {
    api.apiFetchJSON.mockRejectedValueOnce(
      Object.assign(new Error('Synthetic server internals must not render'), { status: 503 }),
    );
    render(() => <MaintenanceVerificationSection resourceId={resourceId} />);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByTestId('maintenance-verification-load-error')).toHaveTextContent(
      'Verification reports unavailable',
    );
    expect(screen.queryByText(/No verification reports yet/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Synthetic server internals/)).not.toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
    let finishRetry!: (value: ReturnType<typeof response>) => void;
    api.apiFetchJSON.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishRetry = resolve;
        }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Retry reports' }));
    expect(screen.getByRole('button', { name: 'Retry reports' })).toBeDisabled();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(2);
    finishRetry(response([report()]));
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.queryByTestId('maintenance-verification-load-error')).not.toBeInTheDocument();
    expect(screen.getByTestId('maintenance-verification-report')).toBeInTheDocument();
    expect(mutations()).toEqual([]);
  });

  it('retains qualified historical reports after a same-resource refresh fails, without inventing a successful review', async () => {
    const saved = report();
    await mount([saved]);
    api.apiFetchJSON
      .mockResolvedValueOnce(saved)
      .mockRejectedValueOnce(Object.assign(new Error('Synthetic read failure'), { status: 503 }));
    fireEvent.click(screen.getByTestId('maintenance-verification-review'));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.getByTestId('maintenance-verification-load-error')).toHaveTextContent(
      'Last loaded reports remain below.',
    );
    expect(screen.getByTestId('maintenance-verification-status-healthy')).toHaveTextContent(
      'Healthy',
    );
    expect(screen.getByText('Window ended 1 min ago')).toBeInTheDocument();
    expect(screen.queryByText(/Reviewed/)).not.toBeInTheDocument();
    expect(mutations()).toHaveLength(1);
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(3);
  });

  it.each([401, 403])(
    'withdraws reports on a denied same-resource refresh (%s), rather than offering them as authorised history',
    async (status) => {
      const saved = report();
      await mount([saved]);
      api.apiFetchJSON
        .mockResolvedValueOnce(saved)
        .mockRejectedValueOnce(
          Object.assign(new Error('Synthetic access internals must not render'), { status }),
        );
      fireEvent.click(screen.getByTestId('maintenance-verification-rerun'));
      await vi.advanceTimersByTimeAsync(60_000);
      expect(screen.queryByTestId('maintenance-verification-report')).not.toBeInTheDocument();
      expect(screen.queryByText(/No verification reports yet/)).not.toBeInTheDocument();
      expect(screen.getByTestId('maintenance-verification-load-error')).toHaveTextContent(
        status === 401
          ? 'Sign in again or check your API token.'
          : 'Access denied. Check your permissions and license plan.',
      );
      expect(screen.queryByText(/Synthetic access internals/)).not.toBeInTheDocument();
      expect(api.apiFetchJSON).toHaveBeenCalledTimes(3);
      const reviewed = report({ userOutcome: 'reviewed', reviewedAt: iso(60_000) });
      api.apiFetchJSON.mockResolvedValueOnce(response([reviewed]));
      fireEvent.click(screen.getByRole('button', { name: 'Retry reports' }));
      await vi.advanceTimersByTimeAsync(0);
      expect(screen.getByText('Reviewed 0s ago')).toBeInTheDocument();
      expect(screen.queryByTestId('maintenance-verification-load-error')).not.toBeInTheDocument();
      expect(api.apiFetchJSON).toHaveBeenCalledTimes(4);
      expect(mutations()).toHaveLength(1);
    },
  );
});
