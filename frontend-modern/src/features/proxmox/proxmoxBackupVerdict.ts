import type { ProtectionPosturePolicy } from '@/types/recovery';
import type { WorkloadCoverageRow } from './proxmoxBackupRecoveryModel';
import { formatPostureWindow } from './proxmoxBackupsTablePresentation';

// The Backups page answers "is everything backed up?" with one sentence, then
// lists what is not, grouped by the server's posture reason so 700 guests with
// overdue verification read as one line instead of 700 amber rows.

export type BackupVerdictTone = 'protected' | 'attention' | 'unprotected' | 'unknown' | 'checking';
export type BackupExceptionTone = 'attention' | 'unprotected' | 'unknown';

export interface BackupExceptionGroup {
  /** The posture reason, or the bare state when the assessment named none. */
  key: string;
  label: string;
  tone: BackupExceptionTone;
  rows: WorkloadCoverageRow[];
}

export interface BackupVerdict {
  headline: string;
  tone: BackupVerdictTone;
  rated: number;
  protected: number;
  checking: number;
  notEvaluated: number;
  exceptions: BackupExceptionGroup[];
}

// Short, plain words: on a phone the label shares one line with its count.
const reasonLabel = (reason: string, policy: ProtectionPosturePolicy | null): string | null => {
  switch (reason) {
    case 'source-failed':
      return 'Backup source reported a failure';
    case 'backup-future-dated':
      return 'Backup dated in the future';
    case 'stale': {
      const seconds = policy?.freshnessWindowSeconds ?? 0;
      return Number.isFinite(seconds) && seconds > 0
        ? `No backup in ${formatPostureWindow(seconds)}`
        : 'Backup overdue';
    }
    case 'verification-future-dated':
      return 'Verification dated in the future';
    case 'verification-overdue':
      return 'Not verified recently';
    case 'history-partial':
      return 'Backup history partly unreadable';
    case 'history-unreadable':
      return 'Backup history unreadable';
    case 'no-history':
      return 'No backup history';
    case 'snapshots-only':
      return 'Snapshots only, no backup';
    case 'no-backup':
      return 'No successful backup';
    default:
      return null;
  }
};

const STATE_FALLBACK_LABEL: Record<BackupExceptionTone, string> = {
  attention: 'Needs attention',
  unprotected: 'Not protected',
  unknown: 'Status unknown',
};

// Each server reason belongs to exactly one state. A reason this build does
// not know, or an assessment stored before reasons existed, falls back to a
// group per state rather than being dropped or guessed.
export const backupExceptionGroupKey = (row: WorkloadCoverageRow): string | null => {
  if (row.posture !== 'attention' && row.posture !== 'unprotected' && row.posture !== 'unknown') {
    return null;
  }
  const reason = row.protectionPosture?.reason;
  return reason && reasonLabel(reason, null) ? reason : row.posture;
};

const TONE_ORDER: Record<BackupExceptionTone, number> = {
  unprotected: 0,
  attention: 1,
  unknown: 2,
};

export function buildBackupVerdict(
  rows: readonly WorkloadCoverageRow[],
  policy: ProtectionPosturePolicy | null,
): BackupVerdict {
  let protectedCount = 0;
  let checking = 0;
  let notEvaluated = 0;
  let hasUnprotected = false;
  let hasAttention = false;
  const groups = new Map<string, BackupExceptionGroup>();
  for (const row of rows) {
    if (row.isOrphaned) continue;
    if (row.posture === 'protected') protectedCount += 1;
    else if (row.posture === 'checking') checking += 1;
    else if (row.posture === 'not-evaluated') notEvaluated += 1;
    const key = backupExceptionGroupKey(row);
    if (!key) continue;
    const tone = row.posture as BackupExceptionTone;
    hasUnprotected ||= tone === 'unprotected';
    hasAttention ||= tone === 'attention';
    let group = groups.get(key);
    if (!group) {
      group = {
        key,
        label: reasonLabel(key, policy) ?? STATE_FALLBACK_LABEL[tone],
        tone,
        rows: [],
      };
      groups.set(key, group);
    }
    group.rows.push(row);
  }

  const exceptions = [...groups.values()].sort(
    (a, b) =>
      TONE_ORDER[a.tone] - TONE_ORDER[b.tone] ||
      b.rows.length - a.rows.length ||
      a.label.localeCompare(b.label),
  );
  for (const group of exceptions) {
    group.rows.sort((a, b) => a.workload.label.localeCompare(b.workload.label));
  }
  const exceptionCount = exceptions.reduce((sum, group) => sum + group.rows.length, 0);
  const rated = protectedCount + exceptionCount;

  let headline: string;
  let tone: BackupVerdictTone;
  if (rated === 0) {
    headline = checking > 0 ? 'Checking backups…' : 'Nothing to rate yet';
    tone = checking > 0 ? 'checking' : 'unknown';
  } else if (exceptionCount === 0) {
    headline = rated === 1 ? '1 of 1 protected' : `All ${rated} protected`;
    tone = 'protected';
  } else {
    headline = `${protectedCount} of ${rated} protected`;
    tone = hasUnprotected ? 'unprotected' : hasAttention ? 'attention' : 'unknown';
  }

  return {
    headline,
    tone,
    rated,
    protected: protectedCount,
    checking,
    notEvaluated,
    exceptions,
  };
}
