import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UpdateHistoryEntry } from '@/api/updates';

const mockListUpdateHistory = vi.fn();
const mockStoreRollbackUpdate = vi.fn();
let runningVersion: string | undefined = '6.0.5';

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    listUpdateHistory: (...args: unknown[]) => mockListUpdateHistory(...args),
  },
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: () => (runningVersion ? { version: runningVersion } : null),
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
    runningVersion = '6.0.5';
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
    const rollbackButtons = screen.getAllByRole('button', { name: 'Roll back' });
    expect(rollbackButtons).toHaveLength(1);
    expect(rollbackButtons[0]).toHaveClass('min-h-11', 'sm:min-h-0');
    expect(screen.getByText('Rollback')).toBeInTheDocument();
  });

  it('confirms which version a rollback restores before starting it', async () => {
    mockListUpdateHistory.mockResolvedValue([baseEntry]);
    mockStoreRollbackUpdate.mockResolvedValue(true);

    render(() => <UpdateHistorySection />);

    fireEvent.click(await screen.findByRole('button', { name: 'Roll back' }));

    expect(await screen.findByText('Roll back to Pulse v6.0.4?')).toBeInTheDocument();
    expect(screen.getByText('Running now: Pulse v6.0.5.')).toBeInTheDocument();
    expect(screen.getByText(/Restore scope is not reported for this backup/)).toHaveTextContent(
      'A scope-aware server keeps active runtime data in place',
    );
    expect(screen.getByText(/Older recovery can also/)).toHaveTextContent(
      'reverting later changes',
    );
    expect(screen.getByText(/Do not assume this will undo settings or alerts/)).toHaveTextContent(
      'Full-state recovery requires Pulse to be stopped',
    );
    expect(screen.getByRole('link', { name: 'Read the recovery instructions' })).toHaveAttribute(
      'href',
      '/docs/AUTO_UPDATE#manual-rollback',
    );
    expect(
      screen.queryByText(/Settings and alert changes made since that backup will be reverted/),
    ).not.toBeInTheDocument();
    expect(mockStoreRollbackUpdate).not.toHaveBeenCalled();

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

  it('distinguishes the running version from an older history entry without guessing restore scope', async () => {
    runningVersion = 'v6.6.0-rc.1';
    mockListUpdateHistory.mockResolvedValue([{ ...baseEntry, notes: 'restore everything' }]);
    render(() => <UpdateHistorySection />);
    fireEvent.click(await screen.findByRole('button', { name: 'Roll back' }));
    expect(screen.getByText('Running now: Pulse v6.6.0-rc.1.')).toBeInTheDocument();
    expect(screen.getByText(/This requests an installation rollback/)).toHaveTextContent(
      'before the update to v6.0.5',
    );
    expect(screen.getByText(/Restore scope is not reported/)).toBeInTheDocument();
    expect(screen.queryByText('restore everything')).not.toBeInTheDocument();
    expect(screen.queryByText(baseEntry.backup_path!)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(mockStoreRollbackUpdate).not.toHaveBeenCalled();
  });

  it('does not infer the running version from the selected update when version information is missing', async () => {
    runningVersion = undefined;
    mockListUpdateHistory.mockResolvedValue([baseEntry]);
    render(() => <UpdateHistorySection />);
    fireEvent.click(await screen.findByRole('button', { name: 'Roll back' }));
    expect(screen.getByText('Running now: Pulse unknown.')).toBeInTheDocument();
    expect(screen.getByText(/Restore scope is not reported/)).toBeInTheDocument();
  });

  it('retains the consent and refreshes history after a rejected rollback', async () => {
    mockListUpdateHistory.mockResolvedValue([baseEntry]);
    mockStoreRollbackUpdate.mockResolvedValue(false);
    render(() => <UpdateHistorySection />);
    fireEvent.click(await screen.findByRole('button', { name: 'Roll back' }));
    fireEvent.click(screen.getByRole('button', { name: 'Roll back to v6.0.4' }));
    await waitFor(() => expect(mockListUpdateHistory).toHaveBeenCalledTimes(2));
    expect(screen.getByRole('dialog')).toHaveTextContent('Restore scope is not reported');
    expect(screen.getByRole('button', { name: 'Roll back to v6.0.4' })).not.toBeDisabled();
  });

  it('prevents duplicate requests and cancellation while the rollback request is pending', async () => {
    mockListUpdateHistory.mockResolvedValue([baseEntry]);
    let finish!: (accepted: boolean) => void;
    mockStoreRollbackUpdate.mockReturnValue(
      new Promise<boolean>((resolve) => {
        finish = resolve;
      }),
    );
    render(() => <UpdateHistorySection />);
    fireEvent.click(await screen.findByRole('button', { name: 'Roll back' }));
    fireEvent.click(screen.getByRole('button', { name: 'Roll back to v6.0.4' }));
    expect(screen.getByRole('button', { name: 'Starting rollback...' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Starting rollback...' }));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(mockStoreRollbackUpdate).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    finish(true);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
