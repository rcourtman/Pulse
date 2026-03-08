import { createMemo } from 'solid-js';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import type { ProtectionRollup, RecoveryOutcome } from '@/types/recovery';
import { normalizeRecoveryOutcome } from '@/utils/recoveryOutcomePresentation';

export interface DashboardRecoverySummary {
  totalProtected: number;
  byOutcome: Partial<Record<RecoveryOutcome, number>>;
  latestEventTimestamp: number | null;
  hasData: boolean;
}

const parseTimestamp = (value: string | null | undefined): number | null => {
  const parsed = Date.parse(String(value || ''));
  if (!Number.isFinite(parsed) || parsed <= 0) return null;
  return parsed;
};

export function useDashboardRecovery() {
  const rollups = useRecoveryRollups();

  return createMemo<DashboardRecoverySummary>(() => {
    const data: ProtectionRollup[] = rollups.rollups() || [];

    if (data.length === 0) {
      return { totalProtected: 0, byOutcome: {}, latestEventTimestamp: null, hasData: false };
    }

    const byOutcome: Partial<Record<RecoveryOutcome, number>> = {};
    let latestTimestamp: number | null = null;

    for (const rollup of data) {
      const outcome = normalizeRecoveryOutcome(rollup.lastOutcome);
      byOutcome[outcome] = (byOutcome[outcome] ?? 0) + 1;

      const ts = parseTimestamp(rollup.lastAttemptAt || null);
      if (ts !== null) {
        if (latestTimestamp === null || ts > latestTimestamp) {
          latestTimestamp = ts;
        }
      }
    }

    return {
      totalProtected: data.length,
      byOutcome,
      latestEventTimestamp: latestTimestamp,
      hasData: true,
    };
  });
}
