import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReplicationJob } from '@/types/api';
import {
  ProxmoxReplicationTable,
  REPLICATION_MOBILE_COLUMNS,
  REPLICATION_MOBILE_COLUMN_WIDTHS,
  compactReplicationNextSyncText,
  formatMobileReplicationGuestLabel,
} from '../ProxmoxReplicationTable';
import replicationTableSource from '../ProxmoxReplicationTable.tsx?raw';

const replicationJob = (overrides: Partial<ReplicationJob> = {}): ReplicationJob => ({
  id: 'replication-100',
  instance: 'pve',
  jobId: '100-0',
  guestId: 100,
  guestName: 'web',
  sourceNode: 'pve-a',
  targetNode: 'pve-b',
  schedule: '*/15',
  enabled: true,
  lastSyncStatus: 'ok',
  lastSyncUnix: 1_700_000_000,
  lastSyncDurationSeconds: 125,
  failCount: 0,
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('ProxmoxReplicationTable', () => {
  it('uses five readable columns and compact time direction across phone widths', () => {
    expect(REPLICATION_MOBILE_COLUMNS).toEqual([
      'guest',
      'status',
      'route',
      'lastSync',
      'nextSync',
    ]);
    expect(REPLICATION_MOBILE_COLUMN_WIDTHS).toEqual({
      guest: 40,
      status: 16,
      route: 21.5,
      lastSync: 9.5,
      nextSync: 13,
    });
    expect(compactReplicationNextSyncText('34m overdue', 'overdue')).toBe('-34m');
    expect(compactReplicationNextSyncText('in 8m', 'normal')).toBe('8m');
    expect(formatMobileReplicationGuestLabel(replicationJob())).toBe('100 web');
  });

  it('removes phone tracking and excess padding from the tight last-sync column', () => {
    expect(replicationTableSource).toContain('![padding-inline:2px] !tracking-normal');
  });

  it('renders replication duration cells through the shared duration format', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_700_000_300_000);

    render(() => (
      <ProxmoxReplicationTable
        jobs={[replicationJob()]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    expect(screen.getByText('100 (web)')).toBeInTheDocument();
    expect(screen.getByText('100 (web)').closest('tr')).toHaveClass('h-8');
    expect(screen.getByText('2m 5s')).toBeInTheDocument();
  });

  it('preserves explicit replication duration labels from the backend', () => {
    render(() => (
      <ProxmoxReplicationTable
        jobs={[
          replicationJob({
            lastSyncDurationSeconds: undefined,
            lastSyncDurationHuman: 'backend duration',
          }),
        ]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    expect(screen.getByText('backend duration')).toBeInTheDocument();
  });

  it('reveals the whole error from the row disclosure at desktop width', async () => {
    const error =
      "command 'zfs send -- rpool/data/vm-100-disk-0@__replicate_100-0__' failed: exit code 255";
    render(() => (
      <ProxmoxReplicationTable
        jobs={[replicationJob({ lastSyncStatus: 'error', failCount: 3, error })]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    expect(document.querySelector('[data-replication-job-error]')).toBeNull();
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );

    const detail = document.querySelector('[data-replication-job-error]');
    expect(detail).toHaveTextContent(error);
    // The expansion wraps; only the one-line row cell truncates.
    expect(detail?.className).toContain('break-words');
    expect(detail?.className).not.toContain('truncate');
  });

  it('gives the error column more room than the 8% that clipped it', () => {
    expect(replicationTableSource).toContain("error: 'w-[13%]'");
    expect(replicationTableSource).not.toContain("error: 'w-[8%]'");
    expect(replicationTableSource).not.toContain('max-w-[18rem] truncate');
  });

  it('matches jobs by their error text', () => {
    expect(replicationTableSource).toMatch(/job\.lastSyncStatus,\s*job\.error,\s*job\.comment,/);
  });
});
