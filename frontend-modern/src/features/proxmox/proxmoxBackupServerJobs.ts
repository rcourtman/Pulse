import type { PBSJobHealthEvidence } from '@/types/api';
import type { Resource } from '@/types/resource';

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

  // A family whose read failed for the whole server (a partial read with no
  // store attached) leaves every datastore's result for that family unknown,
  // never silently OK. PBS task history arrives with no store too: a failed or
  // warned run there keeps that same server-wide signal, but a run whose
  // status is merely unclear is not a failed read, and one such run among
  // hundreds must not mark every datastore unknown while its configured jobs
  // report their results.
  const unreadFamilies = new Set(
    evidence
      .filter((entry) => {
        if (entry.store?.trim()) return false;
        const state = stateForPosture(entry.posture);
        if (state === 'ok') return false;
        const failedRead = entry.evidenceScope === 'partial-read' || Boolean(entry.error?.trim());
        return failedRead || state === 'critical' || state === 'warning';
      })
      .map((entry) => normalize(entry.family))
      .filter(isDatastoreFamily),
  );

  // Task history can repeat a job; the newest run of each job speaks for it.
  // Job ids are unique only within a family, so the family is part of the key.
  const newestById = new Map<string, PBSJobHealthEvidence>();
  for (const entry of evidence) {
    const family = normalize(entry.family);
    if (!isDatastoreFamily(family) || entry.enabled === false) continue;
    if (entry.store?.trim() !== store) continue;
    const id = `${family}:${entry.id?.trim() || store}`;
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

// Backup health lists guests, but a failed sync, verify, prune or GC run is a
// datastore problem: the guests' own backups can be fine while the offsite
// copy is stale (#1815). Each failed family becomes one row naming the
// datastores it hit, from the same judgment the Jobs column shows. Only a
// failure pinned to a datastore counts; storeless history stays "unknown" in
// that column and never reaches this list, matching the backend alert.
export interface DatastoreJobFailureGroup {
  family: string;
  label: string;
  datastores: string[];
}

const FAILURE_ROW_ORDER = ['sync', 'verify', 'prune', 'garbage'] as const;

export function collectDatastoreJobFailures(
  servers: readonly Resource[],
): DatastoreJobFailureGroup[] {
  const pbsServers = servers.filter((server) => server.type === 'pbs');
  const nameServer = pbsServers.length > 1;
  const byFamily = new Map<string, string[]>();
  for (const server of pbsServers) {
    for (const datastore of server.pbs?.datastores ?? []) {
      const summary = summarizeDatastoreJobs(server.pbs?.jobHealthEvidence, datastore.name);
      const failed = new Set(
        summary.entries.filter((entry) => entry.state === 'critical').map((entry) => entry.family),
      );
      const name = nameServer ? `${datastore.name} (${server.name})` : datastore.name;
      for (const family of failed) byFamily.set(family, [...(byFamily.get(family) ?? []), name]);
    }
  }
  return FAILURE_ROW_ORDER.filter((family) => byFamily.has(family)).map((family) => ({
    family,
    label: `${FAMILY_LABEL[family]} failed`,
    datastores: [...byFamily.get(family)!].sort((left, right) => left.localeCompare(right)),
  }));
}
