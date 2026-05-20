import type { BackupTask, GuestSnapshot, StorageBackup } from '@/types/api';

// Sysadmin-oriented age buckets. These are how the page colour-codes
// freshness in row encodings and how the coverage strip splits guests.
//
// `recent`     captured ≤ 7 days ago
// `normal`     captured 7–30 days ago
// `stale`      captured 30–90 days ago
// `ancient`    captured > 90 days ago, or no timestamp
export type BackupAgeBucket = 'recent' | 'normal' | 'stale' | 'ancient';

export interface BackupAgeBucketPresentation {
  label: string;
  rowAccentClass: string;
  swatchClass: string;
}

const AGE_BUCKET_PRESENTATION: Record<BackupAgeBucket, BackupAgeBucketPresentation> = {
  recent: {
    label: '≤ 7d',
    rowAccentClass: 'bg-emerald-500',
    swatchClass: 'bg-emerald-500',
  },
  normal: {
    label: '7–30d',
    rowAccentClass: 'bg-sky-500',
    swatchClass: 'bg-sky-500',
  },
  stale: {
    label: '30–90d',
    rowAccentClass: 'bg-amber-500',
    swatchClass: 'bg-amber-500',
  },
  ancient: {
    label: '> 90d',
    rowAccentClass: 'bg-red-500',
    swatchClass: 'bg-red-500',
  },
};

export function getBackupAgeBucketPresentation(
  bucket: BackupAgeBucket,
): BackupAgeBucketPresentation {
  return AGE_BUCKET_PRESENTATION[bucket];
}

const DAY_MS = 24 * 60 * 60 * 1000;

export function classifyBackupAge(
  timestamp: string | number | undefined,
  now: number = Date.now(),
): BackupAgeBucket {
  if (timestamp === undefined || timestamp === null) return 'ancient';
  const ms = typeof timestamp === 'number' ? timestamp : Date.parse(timestamp);
  if (!Number.isFinite(ms)) return 'ancient';
  const ageDays = (now - ms) / DAY_MS;
  if (ageDays <= 7) return 'recent';
  if (ageDays <= 30) return 'normal';
  if (ageDays <= 90) return 'stale';
  return 'ancient';
}

// Per-tab row dot semantics that match each coverage strip's bucketing.
// The shared 4-bucket classifyBackupAge is too granular: a 14-day archive
// would land in `normal` (sky) in the row but `stale` (amber) in the
// archive coverage strip, since archives have a 7-day SLA. These helpers
// keep the row dot, strip segment, and strip label all reading the same
// colour for the same data.

export interface AgeSwatch {
  label: string;
  swatchClass: string;
}

export function classifyArchiveRowAge(
  timestamp: string | number | undefined,
  now: number = Date.now(),
): AgeSwatch {
  if (timestamp === undefined || timestamp === null)
    return { label: 'uncovered (>30d)', swatchClass: 'bg-red-500' };
  const ms = typeof timestamp === 'number' ? timestamp : Date.parse(timestamp);
  if (!Number.isFinite(ms)) return { label: 'uncovered (>30d)', swatchClass: 'bg-red-500' };
  const ageDays = (now - ms) / DAY_MS;
  if (ageDays <= 7) return { label: 'current (≤7d)', swatchClass: 'bg-emerald-500' };
  if (ageDays <= 30) return { label: 'stale (7–30d)', swatchClass: 'bg-amber-500' };
  return { label: 'uncovered (>30d)', swatchClass: 'bg-red-500' };
}

export function classifySnapshotRowAge(
  timestamp: string | number | undefined,
  now: number = Date.now(),
): AgeSwatch {
  if (timestamp === undefined || timestamp === null)
    return { label: 'ancient (>90d)', swatchClass: 'bg-red-500' };
  const ms = typeof timestamp === 'number' ? timestamp : Date.parse(timestamp);
  if (!Number.isFinite(ms)) return { label: 'ancient (>90d)', swatchClass: 'bg-red-500' };
  const ageDays = (now - ms) / DAY_MS;
  if (ageDays <= 30) return { label: 'recent (≤30d)', swatchClass: 'bg-emerald-500' };
  if (ageDays <= 90) return { label: 'stale (30–90d)', swatchClass: 'bg-amber-500' };
  return { label: 'ancient (>90d)', swatchClass: 'bg-red-500' };
}

export function guestKey(guest: { type?: string; instance?: string; vmid: number }): string {
  // instance disambiguates 100@cluster-a vs 100@cluster-b. type narrows
  // VM-100 vs CT-100, which can collide in the same instance.
  const t = (guest.type ?? '').toLowerCase() || '?';
  return `${guest.instance ?? ''}:${t}:${guest.vmid}`;
}

interface GuestSnapshotStats {
  key: string;
  type: string;
  vmid: number;
  instance: string;
  node: string;
  count: number;
  withRamCount: number;
  newestMs: number | undefined;
  oldestMs: number | undefined;
  totalBytes: number;
  // Newest first.
  snapshots: GuestSnapshot[];
}

export interface SnapshotCoverageSummary {
  totalGuests: number;
  staleGuests: number; // newest > 30d
  ancientGuests: number; // newest > 90d
  withRamGuests: number;
  totalSnapshots: number;
  // Sorted by newest snapshot descending (stalest at the end).
  guests: GuestSnapshotStats[];
}

export function buildSnapshotCoverageSummary(
  snapshots: readonly GuestSnapshot[],
  now: number = Date.now(),
): SnapshotCoverageSummary {
  const byGuest = new Map<string, GuestSnapshotStats>();
  for (const snap of snapshots) {
    const key = guestKey(snap);
    let stats = byGuest.get(key);
    if (!stats) {
      stats = {
        key,
        type: snap.type,
        vmid: snap.vmid,
        instance: snap.instance,
        node: snap.node,
        count: 0,
        withRamCount: 0,
        newestMs: undefined,
        oldestMs: undefined,
        totalBytes: 0,
        snapshots: [],
      };
      byGuest.set(key, stats);
    }
    stats.snapshots.push(snap);
    stats.count += 1;
    if (snap.vmstate) stats.withRamCount += 1;
    if (typeof snap.sizeBytes === 'number' && snap.sizeBytes > 0) {
      stats.totalBytes += snap.sizeBytes;
    }
    const ms = Date.parse(snap.time);
    if (Number.isFinite(ms)) {
      if (stats.newestMs === undefined || ms > stats.newestMs) stats.newestMs = ms;
      if (stats.oldestMs === undefined || ms < stats.oldestMs) stats.oldestMs = ms;
    }
  }

  for (const stats of byGuest.values()) {
    stats.snapshots.sort((a, b) => {
      const av = Date.parse(a.time);
      const bv = Date.parse(b.time);
      return (Number.isFinite(bv) ? bv : 0) - (Number.isFinite(av) ? av : 0);
    });
  }

  const guests = Array.from(byGuest.values()).sort((a, b) => {
    const av = a.newestMs ?? 0;
    const bv = b.newestMs ?? 0;
    return bv - av;
  });

  let staleGuests = 0;
  let ancientGuests = 0;
  let withRamGuests = 0;
  for (const g of guests) {
    const bucket = classifyBackupAge(g.newestMs, now);
    if (bucket === 'stale') staleGuests += 1;
    if (bucket === 'ancient') ancientGuests += 1;
    if (g.withRamCount > 0) withRamGuests += 1;
  }

  return {
    totalGuests: guests.length,
    staleGuests,
    ancientGuests,
    withRamGuests,
    totalSnapshots: snapshots.length,
    guests,
  };
}

export interface ArchiveCoverageSummary {
  // Guests with at least one archive in the last 7d.
  currentGuests: number;
  // Last archive 7–30d (warning territory for daily-backup policies).
  staleGuests: number;
  // Last archive > 30d.
  uncoveredGuests: number;
  // Distinct guests appearing in the archive set.
  totalGuests: number;
  totalArchives: number;
  totalBytes: number;
}

export function buildArchiveCoverageSummary(
  archives: readonly StorageBackup[],
  now: number = Date.now(),
): ArchiveCoverageSummary {
  const newestByGuest = new Map<string, number>();
  let totalBytes = 0;
  for (const arc of archives) {
    const key = guestKey(arc);
    const ms = Date.parse(arc.time);
    if (Number.isFinite(ms)) {
      const existing = newestByGuest.get(key);
      if (existing === undefined || ms > existing) newestByGuest.set(key, ms);
    } else if (!newestByGuest.has(key)) {
      newestByGuest.set(key, 0);
    }
    if (typeof arc.size === 'number' && arc.size > 0) totalBytes += arc.size;
  }

  // Backup coverage is judged on a 7-day SLA — any guest whose latest
  // archive is older than a week is at least stale. The label semantics
  // on the page (current ≤7d, stale 7–30d, uncovered >30d) reflect that
  // tighter threshold; classifyBackupAge has a coarser bucketing used
  // for row-level age dots, so we split it explicitly here.
  let currentGuests = 0;
  let staleGuests = 0;
  let uncoveredGuests = 0;
  for (const ms of newestByGuest.values()) {
    const bucket = classifyBackupAge(ms, now);
    if (bucket === 'recent') currentGuests += 1;
    else if (bucket === 'normal') staleGuests += 1;
    else uncoveredGuests += 1;
  }

  return {
    currentGuests,
    staleGuests,
    uncoveredGuests,
    totalGuests: newestByGuest.size,
    totalArchives: archives.length,
    totalBytes,
  };
}

export interface TaskOutcomeSummary {
  total: number;
  ok: number;
  failed: number;
  running: number;
  hasErrors: boolean;
}

export function buildTaskOutcomeSummary(tasks: readonly BackupTask[]): TaskOutcomeSummary {
  let ok = 0;
  let failed = 0;
  let running = 0;
  let hasErrors = false;
  for (const task of tasks) {
    const status = (task.status ?? '').toLowerCase();
    if (status === 'ok' || status === 'success' || status === 'completed') ok += 1;
    else if (status === 'failed' || status === 'error') failed += 1;
    else if (status === 'running') running += 1;
    if (task.error && task.error.trim().length > 0) hasErrors = true;
  }
  return { total: tasks.length, ok, failed, running, hasErrors };
}

// Median duration in seconds across the supplied set, used as the
// reference for the "duration vs typical" inline bar on Recent tasks.
// Returns 0 if no finite durations are present.
export function computeMedianTaskDurationSeconds(tasks: readonly BackupTask[]): number {
  const durations: number[] = [];
  for (const task of tasks) {
    const start = Date.parse(task.startTime ?? '');
    const end = Date.parse(task.endTime ?? '');
    if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) continue;
    durations.push((end - start) / 1000);
  }
  if (durations.length === 0) return 0;
  durations.sort((a, b) => a - b);
  const mid = Math.floor(durations.length / 2);
  if (durations.length % 2 === 1) return durations[mid];
  return (durations[mid - 1] + durations[mid]) / 2;
}

export function taskDurationSeconds(task: BackupTask): number | undefined {
  const start = Date.parse(task.startTime ?? '');
  const end = Date.parse(task.endTime ?? '');
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return undefined;
  return (end - start) / 1000;
}
