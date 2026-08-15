import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PatrolWorkReceipt, PatrolWorkReceiptListResponse } from '@/api/patrolAttention';

const apiMocks = vi.hoisted(() => ({ getReceipts: vi.fn() }));

vi.mock('@/api/patrolAttention', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/patrolAttention')>();
  return {
    ...original,
    getPatrolWorkReceipts: (...args: unknown[]) => apiMocks.getReceipts(...args),
  };
});

import { PatrolRecentWorkPanel } from '../PatrolRecentWorkPanel';

const receipt = (overrides: Partial<PatrolWorkReceipt> = {}): PatrolWorkReceipt => ({
  actionId: 'action-1',
  resourceId: 'docker:host/service/jellyfin',
  resourceName: 'Jellyfin',
  capabilityName: 'restart',
  verifiedAt: '2026-08-14T07:05:00Z',
  evidenceClass: 'agent_attested',
  originSurface: 'patrol',
  findingId: 'finding-1',
  ...overrides,
});

const response = (items: PatrolWorkReceipt[]): PatrolWorkReceiptListResponse => ({
  data: items,
  count: items.length,
  limit: 6,
});

describe('PatrolRecentWorkPanel', () => {
  beforeEach(() => apiMocks.getReceipts.mockReset());
  afterEach(cleanup);

  it('shows server-authored verified Patrol work receipts', async () => {
    apiMocks.getReceipts.mockResolvedValue(response([receipt()]));

    render(() => <PatrolRecentWorkPanel />);

    expect(await screen.findByText('Confirmed by executing agent')).toBeInTheDocument();
    expect(screen.getAllByText('Verified')).toHaveLength(1);
    expect(screen.getByText('Restart completed')).toBeInTheDocument();
    expect(screen.getByText('Jellyfin')).toBeInTheDocument();
    expect(apiMocks.getReceipts).toHaveBeenCalledWith(6);
  });

  it('turns a retired opaque resource id into a calm resource label', async () => {
    apiMocks.getReceipts.mockResolvedValue(
      response([
        receipt({
          resourceId: 'app-container-4c19e723fbfae98b',
          resourceName: 'app-container-4c19e723fbfae98b',
          evidenceClass: 'independent',
        }),
      ]),
    );

    render(() => <PatrolRecentWorkPanel />);

    expect(await screen.findByText('App container')).toBeInTheDocument();
    expect(screen.getByText(/…fae98b/)).toBeInTheDocument();
    expect(screen.getByText('Confirmed by independent observer')).toBeInTheDocument();
    expect(screen.queryByText('app-container-4c19e723fbfae98b')).not.toBeInTheDocument();
  });

  it('does not imply successful work when there is no verified receipt', async () => {
    apiMocks.getReceipts.mockResolvedValue(response([]));

    render(() => <PatrolRecentWorkPanel />);

    expect(
      await screen.findByRole('heading', { name: 'No verified work yet' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Verified')).not.toBeInTheDocument();
  });

  it('keeps the last truthful receipts visible if a background refresh fails', async () => {
    apiMocks.getReceipts.mockResolvedValueOnce(response([receipt()]));
    apiMocks.getReceipts.mockRejectedValueOnce(new Error('relay unavailable'));

    render(() => <PatrolRecentWorkPanel />);
    await screen.findByText('Confirmed by executing agent');
    document.dispatchEvent(new Event('visibilitychange'));

    await waitFor(() =>
      expect(screen.getByText('Verified work is unavailable')).toBeInTheDocument(),
    );
    expect(screen.getByText('Confirmed by executing agent')).toBeInTheDocument();
  });
});
