import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReplicationJob } from '@/types/api';
import { ProxmoxReplicationTable } from '../ProxmoxReplicationTable';

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
});
