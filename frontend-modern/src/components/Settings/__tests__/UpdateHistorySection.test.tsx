import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UpdateHistoryEntry } from '@/api/updates';

const mockListUpdateHistory = vi.fn();
const mockStoreRollbackUpdate = vi.fn();

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    listUpdateHistory: (...args: unknown[]) => mockListUpdateHistory(...args),
  },
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: () => ({ version: '6.0.5' }),
    rollbackUpdate: (...args: unknown[]) => mockStoreRollbackUpdate(...args),
  },
}));

import { UpdateHistorySection } from '../UpdateHistorySection';

const baseEntry: UpdateHistoryEntry = {
  event_id: '01JZSUCCESS',
  timestamp: '2026-07-09T14:39:00Z',
  action: 'update',
  channel: 'stable',
  version_from: '6.0.4',
  version_to: '6.0.5',
  deployment_type: 'systemd',
  initiated_by: 'user',
  initiated_via: 'ui',
  status: 'success',
  duration_ms: 30000,
  backup_path: '/var/lib/pulse/backup-20260709-143900',
};

describe('UpdateHistorySection', () => {
  beforeEach(() => {
    mockListUpdateHistory.mockReset();
    mockStoreRollbackUpdate.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('shows the empty state when no updates were applied', async () => {
    mockListUpdateHistory.mockResolvedValue([]);

    render(() => <UpdateHistorySection />);

    expect(
      await screen.findByText('No updates have been applied through Pulse yet.'),
    ).toBeInTheDocument();
  });

  it('offers rollback only for successful updates with a retained backup', async () => {
    const entries: UpdateHistoryEntry[] = [
      baseEntry,
      // Backup pruned by retention: backend cleared backup_path.
      {
        ...baseEntry,
        event_id: '01JZPRUNED',
        version_from: '6.0.3',
        version_to: '6.0.4',
        backup_path: undefined,
      },
      // Failed update: nothing to return to.
      {
        ...baseEntry,
        event_id: '01JZFAILED',
        status: 'failed',
        error: { message: 'checksum verification failed' },
      },
      // A recorded rollback never offers another rollback.
      {
        ...baseEntry,
        event_id: '01JZROLLBACK',
        action: 'rollback',
        version_from: '6.0.5',
        version_to: '6.0.4',
      },
    ];
    mockListUpdateHistory.mockResolvedValue(entries);

    render(() => <UpdateHistorySection />);

    await screen.findByText('Failed');
    expect(screen.getAllByRole('button', { name: 'Roll back' })).toHaveLength(1);
    expect(screen.getByText('Rollback')).toBeInTheDocument();
  });

  it('confirms which version a rollback restores before starting it', async () => {
    mockListUpdateHistory.mockResolvedValue([baseEntry]);
    mockStoreRollbackUpdate.mockResolvedValue(true);

    render(() => <UpdateHistorySection />);

    fireEvent.click(await screen.findByRole('button', { name: 'Roll back' }));

    expect(await screen.findByText('Roll back to Pulse v6.0.4?')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Roll back to v6.0.4' }));

    await waitFor(() =>
      expect(mockStoreRollbackUpdate).toHaveBeenCalledWith({
        eventId: '01JZSUCCESS',
        fromVersion: '6.0.5',
        toVersion: '6.0.4',
      }),
    );
    await waitFor(() =>
      expect(screen.queryByText('Roll back to Pulse v6.0.4?')).not.toBeInTheDocument(),
    );
  });
});
