import type { PBSJobHealthEvidence } from '@/types/api';

// "Is my PBS OK?" needs the datastore's housekeeping, not just its fill: a
// failed sync means the offsite copy is stale, a failed verify means restores
// are unproven, and failed GC or prune means the store fills faster than its
// usage suggests. The backend already judges every job (posture + reason in
// pbsJobHealthPosture); this only picks the worst judgment per datastore and
// says it in a few words. It never re-derives health from raw task status.

export type DatastoreJobState = 'none' | 'ok' | 'unknown' | 'warning' | 'critical';

export interface DatastoreJobEntry {
  id: string;
  family: string;
  familyLabel: string;
  state: Exclude<DatastoreJobState, 'none'>;
  outcomeLabel: string;
}

export interface DatastoreJobSummary {
  state: DatastoreJobState;
  // Single-line cell text: the worst job ("Sync failed") or "OK". Further
  // problems are listed in the drawer and hover title, not squeezed in here.
  label: string;
  // One entry per job id, worst first, for the drawer and the hover title.
  entries: DatastoreJobEntry[];
}

// Backup jobs are already answered per guest by Coverage; this cell is about
// the datastore's own maintenance and offsite copy.
const DATASTORE_JOB_FAMILIES = ['garbage', 'prune', 'verify', 'sync'] as const;

const FAMILY_LABEL: Record<(typeof DATASTORE_JOB_FAMILIES)[number], string> = {
  garbage: 'GC',
  prune: 'Prune',
  verify: 'Verify',
  sync: 'Sync',
};

const STATE_RANK: Record<Exclude<DatastoreJobState, 'none'>, number> = {
  ok: 0,
  unknown: 1,
  warning: 2,
  critical: 3,
};

const normalize = (value: string | undefined): string => (value ?? '').trim().toLowerCase();

const isDatastoreFamily = (family: string): family is (typeof DATASTORE_JOB_FAMILIES)[number] =>
  (DATASTORE_JOB_FAMILIES as readonly string[]).includes(family);

const stateForPosture = (posture: string | undefined): Exclude<DatastoreJobState, 'none'> => {
  switch (normalize(posture)) {
    case 'healthy':
      return 'ok';
    case 'warning':
      return 'warning';
    case 'critical':
      return 'critical';
    default:
      return 'unknown';
  }
};

const outcomeLabelFor = (state: DatastoreJobEntry['state'], reason: string): string => {
  if (state === 'ok') return reason === 'running' ? 'running' : 'OK';
  if (state === 'critical') return 'failed';
  // Short plain words: on phones the phrase shares the narrow State column.
  if (state === 'warning') return reason === 'job-overdue' ? 'late' : 'warned';
  return reason === 'no-run-observed' ? 'not run' : 'unknown';
};

const runTime = (entry: PBSJobHealthEvidence): number => {
  const endedAt = entry.freshness?.lastRunEndTime
    ? Date.parse(entry.freshness.lastRunEndTime)
    : NaN;
  if (Number.isFinite(endedAt)) return endedAt;
  const seconds = entry['last-run-endtime'] ?? entry['task-endtime'];
  return typeof seconds === 'number' && Number.isFinite(seconds) ? seconds * 1000 : 0;
};

export function summarizeDatastoreJobs(
  evidence: readonly PBSJobHealthEvidence[] | undefined,
  datastoreName: string | undefined,
): DatastoreJobSummary {
  const store = datastoreName?.trim();
  if (!store || !evidence?.length) return { state: 'none', label: '—', entries: [] };

  // A family whose read failed for the whole server (no store attached) leaves
  // every datastore's result for that family unknown, never silently OK.
  const unreadFamilies = new Set(
    evidence
      .filter((entry) => !entry.store?.trim() && stateForPosture(entry.posture) !== 'ok')
      .map((entry) => normalize(entry.family))
      .filter(isDatastoreFamily),
  );

  // Task history can repeat a job; the newest run of each job id speaks for it.
  const newestById = new Map<string, PBSJobHealthEvidence>();
  for (const entry of evidence) {
    const family = normalize(entry.family);
    if (!isDatastoreFamily(family) || entry.enabled === false) continue;
    if (entry.store?.trim() !== store) continue;
    const id = entry.id?.trim() || `${family}:${store}`;
    const previous = newestById.get(id);
    if (!previous || runTime(entry) >= runTime(previous)) newestById.set(id, entry);
  }

  const entries: DatastoreJobEntry[] = [...newestById.entries()].map(([id, entry]) => {
    const family = normalize(entry.family) as (typeof DATASTORE_JOB_FAMILIES)[number];
    const state = stateForPosture(entry.posture);
    return {
      id,
      family,
      familyLabel: FAMILY_LABEL[family],
      state,
      outcomeLabel: outcomeLabelFor(state, normalize(entry.postureReason)),
    };
  });
  for (const family of unreadFamilies) {
    if (entries.some((entry) => entry.family === family && entry.state !== 'ok')) continue;
    entries.push({
      id: `${family}:unread`,
      family,
      familyLabel: FAMILY_LABEL[family],
      state: 'unknown',
      outcomeLabel: 'unknown',
    });
  }
  if (entries.length === 0) return { state: 'none', label: '—', entries: [] };

  const familyOrder = (family: string) => DATASTORE_JOB_FAMILIES.indexOf(family as never);
  entries.sort(
    (left, right) =>
      STATE_RANK[right.state] - STATE_RANK[left.state] ||
      familyOrder(left.family) - familyOrder(right.family) ||
      left.id.localeCompare(right.id),
  );

  const worst = entries[0];
  if (worst.state === 'ok') return { state: 'ok', label: 'OK', entries };
  return { state: worst.state, label: `${worst.familyLabel} ${worst.outcomeLabel}`, entries };
}

export const describeDatastoreJobs = (summary: DatastoreJobSummary): string =>
  summary.entries.length === 0
    ? 'No PBS jobs reported for this datastore'
    : summary.entries.map((entry) => `${entry.familyLabel} ${entry.outcomeLabel}`).join(' · ');
