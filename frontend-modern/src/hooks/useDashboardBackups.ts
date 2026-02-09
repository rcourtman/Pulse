import { createMemo, type Accessor } from 'solid-js';
import { useWebSocket } from '@/App';
import { buildBackupRecords } from '@/features/storageBackups/backupAdapters';
import type { BackupOutcome } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';

export interface DashboardBackupSummary {
  totalBackups: number;
  byOutcome: Partial<Record<BackupOutcome, number>>;
  latestBackupTimestamp: number | null;
  hasData: boolean;
}

export function useDashboardBackups(resources: Accessor<Resource[]>) {
  const { state } = useWebSocket();

  return createMemo<DashboardBackupSummary>(() => {
    const currentResources = resources();
    const records = buildBackupRecords({ state: state as any, resources: currentResources });

    if (records.length === 0) {
      return { totalBackups: 0, byOutcome: {}, latestBackupTimestamp: null, hasData: false };
    }

    const byOutcome: Partial<Record<BackupOutcome, number>> = {};
    let latestTimestamp: number | null = null;

    for (const record of records) {
      byOutcome[record.outcome] = (byOutcome[record.outcome] ?? 0) + 1;
      if (record.completedAt !== null) {
        if (latestTimestamp === null || record.completedAt > latestTimestamp) {
          latestTimestamp = record.completedAt;
        }
      }
    }

    return {
      totalBackups: records.length,
      byOutcome,
      latestBackupTimestamp: latestTimestamp,
      hasData: true,
    };
  });
}

