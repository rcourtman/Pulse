import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { MaintenanceVerificationReport } from '@/api/maintenanceVerification';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

const apiMocks = vi.hoisted(() => ({
  listMaintenanceVerificationsForResource: vi.fn(),
}));

vi.mock('@/api/maintenanceVerification', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/maintenanceVerification')>();
  return {
    ...actual,
    listMaintenanceVerificationsForResource: apiMocks.listMaintenanceVerificationsForResource,
  };
});

import { MaintenanceVerificationSection } from '../MaintenanceVerificationSection';

const report: MaintenanceVerificationReport = {
  id: 'mv-1',
  resourceId: 'vm:100',
  trigger: 'maintenance_window_end',
  status: 'healthy',
  startedAt: '2026-08-30T11:00:00Z',
  completedAt: '2026-08-30T11:55:00Z',
  windowStartedAt: '2026-08-30T11:00:00Z',
  windowEndedAt: '2026-08-30T11:50:00Z',
  evidence: {
    activeCriticalAlerts: 0,
    activeWarningAlerts: 0,
    activeCriticalFindings: 0,
    activeWarningFindings: 0,
    failedActionsSinceWindowStart: 0,
  },
  linkedFindingIds: [],
  linkedAlertIds: [],
  linkedActionIds: [],
  userOutcome: 'reviewed',
  reviewedAt: '2026-08-30T11:58:00Z',
  reviewedBy: 'admin',
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('MaintenanceVerificationSection relative ages', () => {
  it('keeps report window and review ages moving while the drawer stays open', async () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-08-30T12:00:00Z'),
    });
    apiMocks.listMaintenanceVerificationsForResource.mockResolvedValue({
      data: [report],
      meta: { resourceId: 'vm:100', limit: 25, total: 1 },
    });

    render(() => <MaintenanceVerificationSection resourceId="vm:100" />);

    expect(await screen.findByText('Window ended 10 mins ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 2 mins ago by admin')).toBeInTheDocument();

    // No report changes: the list is not re-read and only the clock moves.
    vi.advanceTimersByTime(2 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(screen.getByText('Window ended 2 hours ago')).toBeInTheDocument();
    expect(screen.getByText('Reviewed 2 hours ago by admin')).toBeInTheDocument();
  });
});
