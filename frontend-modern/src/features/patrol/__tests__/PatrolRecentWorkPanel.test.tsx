import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AttentionItem, AttentionListResponse } from '@/api/patrolAttention';

const apiMocks = vi.hoisted(() => ({ getList: vi.fn() }));

vi.mock('@/api/patrolAttention', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/patrolAttention')>();
  return {
    ...original,
    getPatrolAttention: (...args: unknown[]) => apiMocks.getList(...args),
  };
});

import { PatrolRecentWorkPanel } from '../PatrolRecentWorkPanel';

const receipt = (overrides: Partial<AttentionItem> = {}): AttentionItem => ({
  id: 'receipt-1',
  operationalRecordId: 'record-1',
  subjectResourceId: 'docker:host/service/jellyfin',
  subjectResourceName: 'Jellyfin',
  kind: 'availability',
  title: 'Jellyfin playback restored',
  plainLanguageSummary: 'Playback recovered.',
  severity: 'warning',
  state: 'resolved',
  firstObservedAt: '2026-08-14T07:00:00Z',
  lastObservedAt: '2026-08-14T07:05:00Z',
  evidenceFreshness: 'fresh',
  evidenceCompleteness: 'complete',
  relatedResources: [],
  availableActions: [
    {
      targetResourceId: 'docker:host/service/jellyfin',
      capability: 'restart_service',
      kind: 'restart',
      label: 'Restart Jellyfin',
      mode: 'execute',
      risk: 'low',
      approval: 'not-required',
      eligibility: 'eligible',
      reasons: [],
      evidenceIds: ['evidence-1'],
      expectedPostcondition: 'Playback health checks return to normal.',
      verificationPolicy: 'availability',
      requiresApproval: false,
    },
  ],
  verificationState: 'succeeded',
  ...overrides,
});

const response = (items: AttentionItem[]): AttentionListResponse => ({
  data: items,
  summary: {
    activeCount: 0,
    openCount: 0,
    acknowledgedCount: 0,
    suppressedCount: 0,
    uncertainCount: 0,
    resolvedCount: items.length,
    calm: true,
    coverageState: 'current',
    evaluatedAt: '2026-08-14T07:05:00Z',
  },
  meta: { page: 1, limit: 50, total: items.length, totalPages: items.length ? 1 : 0 },
});

describe('PatrolRecentWorkPanel', () => {
  beforeEach(() => apiMocks.getList.mockReset());
  afterEach(cleanup);

  it('shows only successfully verified resolved work as receipts', async () => {
    apiMocks.getList.mockResolvedValue(
      response([
        receipt(),
        receipt({ id: 'failed', verificationState: 'failed' }),
        receipt({ id: 'open', state: 'open' }),
      ]),
    );

    render(() => <PatrolRecentWorkPanel />);

    expect(await screen.findByText('Playback recovered.')).toBeInTheDocument();
    expect(screen.getAllByText('Verified')).toHaveLength(1);
    expect(apiMocks.getList).toHaveBeenCalledWith('resolved', 1, 50);
  });

  it('does not imply successful work when there is no verified receipt', async () => {
    apiMocks.getList.mockResolvedValue(response([receipt({ verificationState: 'unknown' })]));

    render(() => <PatrolRecentWorkPanel />);

    expect(
      await screen.findByRole('heading', { name: 'No verified work yet' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Verified')).not.toBeInTheDocument();
  });

  it('keeps the last truthful receipts visible if a background refresh fails', async () => {
    apiMocks.getList.mockResolvedValueOnce(response([receipt()]));
    apiMocks.getList.mockRejectedValueOnce(new Error('relay unavailable'));

    render(() => <PatrolRecentWorkPanel />);
    await screen.findByText('Playback recovered.');
    document.dispatchEvent(new Event('visibilitychange'));

    await waitFor(() =>
      expect(screen.getByText('Verified work is unavailable')).toBeInTheDocument(),
    );
    expect(screen.getByText('Playback recovered.')).toBeInTheDocument();
  });
});
