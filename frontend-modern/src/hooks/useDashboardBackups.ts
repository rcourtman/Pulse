import { createMemo } from 'solid-js';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import type { ProtectionRollup } from '@/types/recovery';

export interface DashboardBackupSummary {
  totalBackups: number;
  byOutcome: Partial<Record<'success' | 'warning' | 'failed' | 'running' | 'unknown', number>>;
  latestBackupTimestamp: number | null;
  hasData: boolean;
}

const parseTimestamp = (value: string | null | undefined): number | null => {
  const parsed = Date.parse(String(value || ''));
  if (!Number.isFinite(parsed) || parsed <= 0) return null;
  return parsed;
};

const normalizeOutcome = (value: string | null | undefined): 'success' | 'warning' | 'failed' | 'running' | 'unknown' => {
  const v = String(value || '').trim().toLowerCase();
  if (v === 'success' || v === 'warning' || v === 'failed' || v === 'running' || v === 'unknown') return v;
  return 'unknown';
};

export function useDashboardBackups() {
  const rollups = useRecoveryRollups();

  return createMemo<DashboardBackupSummary>(() => {
    const data: ProtectionRollup[] = rollups.rollups() || [];

    if (data.length === 0) {
      return { totalBackups: 0, byOutcome: {}, latestBackupTimestamp: null, hasData: false };
    }

    const byOutcome: Partial<Record<'success' | 'warning' | 'failed' | 'running' | 'unknown', number>> = {};
    let latestTimestamp: number | null = null;

    for (const rollup of data) {
      const outcome = normalizeOutcome(rollup.lastOutcome);
      byOutcome[outcome] = (byOutcome[outcome] ?? 0) + 1;

      const ts = parseTimestamp(rollup.lastAttemptAt || null);
      if (ts !== null) {
        if (latestTimestamp === null || ts > latestTimestamp) {
          latestTimestamp = ts;
        }
      }
    }

    return {
      totalBackups: data.length,
      byOutcome,
      latestBackupTimestamp: latestTimestamp,
      hasData: true,
    };
  });
}
