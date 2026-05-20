import { describe, expect, it } from 'vitest';

import type { BackupTask, GuestSnapshot, StorageBackup } from '@/types/api';
import {
  buildArchiveCoverageSummary,
  buildSnapshotCoverageSummary,
  buildTaskOutcomeSummary,
  classifyArchiveRowAge,
  classifyBackupAge,
  classifySnapshotRowAge,
  computeMedianTaskDurationSeconds,
  guestKey,
} from '../proxmoxBackupSummaryPresentation';

const DAY_MS = 24 * 60 * 60 * 1000;

function snapshot(overrides: Partial<GuestSnapshot> & { time: string }): GuestSnapshot {
  return {
    id: overrides.id ?? `${overrides.time}-${overrides.vmid ?? 0}`,
    name: overrides.name ?? 'snap',
    node: overrides.node ?? 'pve1',
    instance: overrides.instance ?? 'cluster-a',
    type: overrides.type ?? 'qemu',
    vmid: overrides.vmid ?? 100,
    time: overrides.time,
    description: overrides.description,
    parent: overrides.parent,
    vmstate: overrides.vmstate ?? false,
    sizeBytes: overrides.sizeBytes,
  };
}

function archive(overrides: Partial<StorageBackup> & { time: string }): StorageBackup {
  return {
    id: overrides.id ?? `${overrides.time}-${overrides.vmid ?? 0}`,
    storage: overrides.storage ?? 'backup-vault',
    node: overrides.node ?? 'pve1',
    instance: overrides.instance ?? 'cluster-a',
    type: overrides.type ?? 'qemu',
    vmid: overrides.vmid ?? 100,
    time: overrides.time,
    ctime: overrides.ctime ?? 0,
    size: overrides.size ?? 0,
    format: overrides.format ?? 'tar.zst',
    notes: overrides.notes,
    protected: overrides.protected ?? false,
    volid: overrides.volid ?? 'vol',
    isPBS: overrides.isPBS ?? false,
    verified: overrides.verified ?? false,
    verification: overrides.verification,
  };
}

function task(overrides: Partial<BackupTask> & { startTime: string }): BackupTask {
  return {
    id: overrides.id ?? `${overrides.startTime}-${overrides.vmid ?? 0}`,
    node: overrides.node ?? 'pve1',
    instance: overrides.instance ?? 'cluster-a',
    type: overrides.type ?? 'qemu',
    vmid: overrides.vmid ?? 100,
    status: overrides.status ?? 'ok',
    startTime: overrides.startTime,
    endTime: overrides.endTime,
    size: overrides.size,
    error: overrides.error,
  };
}

describe('proxmoxBackupSummaryPresentation', () => {
  describe('classifyBackupAge', () => {
    const now = Date.parse('2026-05-20T12:00:00Z');

    it('buckets by sysadmin-relevant age thresholds', () => {
      expect(classifyBackupAge(now - 1 * DAY_MS, now)).toBe('recent');
      expect(classifyBackupAge(now - 7 * DAY_MS + 1000, now)).toBe('recent');
      expect(classifyBackupAge(now - 20 * DAY_MS, now)).toBe('normal');
      expect(classifyBackupAge(now - 60 * DAY_MS, now)).toBe('stale');
      expect(classifyBackupAge(now - 200 * DAY_MS, now)).toBe('ancient');
    });

    it('treats undefined and unparseable timestamps as ancient', () => {
      expect(classifyBackupAge(undefined, now)).toBe('ancient');
      expect(classifyBackupAge('not a date', now)).toBe('ancient');
    });
  });

  describe('buildSnapshotCoverageSummary', () => {
    const now = Date.parse('2026-05-20T12:00:00Z');

    it('groups snapshots by guest and tallies stale/ancient counts', () => {
      const summary = buildSnapshotCoverageSummary(
        [
          snapshot({ vmid: 100, time: new Date(now - 2 * DAY_MS).toISOString(), vmstate: true }),
          snapshot({ vmid: 100, time: new Date(now - 50 * DAY_MS).toISOString() }),
          snapshot({ vmid: 101, time: new Date(now - 100 * DAY_MS).toISOString() }),
          snapshot({ vmid: 102, time: new Date(now - 40 * DAY_MS).toISOString() }),
        ],
        now,
      );

      expect(summary.totalGuests).toBe(3);
      expect(summary.totalSnapshots).toBe(4);
      // vmid 100 newest is 2d (recent), vmid 102 newest is 40d (stale),
      // vmid 101 newest is 100d (ancient).
      expect(summary.staleGuests).toBe(1);
      expect(summary.ancientGuests).toBe(1);
      expect(summary.withRamGuests).toBe(1);
    });

    it('sorts guests by newest snapshot descending', () => {
      const summary = buildSnapshotCoverageSummary(
        [
          snapshot({ vmid: 1, time: new Date(now - 30 * DAY_MS).toISOString() }),
          snapshot({ vmid: 2, time: new Date(now - 5 * DAY_MS).toISOString() }),
          snapshot({ vmid: 3, time: new Date(now - 15 * DAY_MS).toISOString() }),
        ],
        now,
      );

      expect(summary.guests.map((g) => g.vmid)).toEqual([2, 3, 1]);
    });

    it('sorts each guest`s snapshots newest-first', () => {
      const summary = buildSnapshotCoverageSummary(
        [
          snapshot({ vmid: 7, time: new Date(now - 20 * DAY_MS).toISOString(), name: 'old' }),
          snapshot({ vmid: 7, time: new Date(now - 1 * DAY_MS).toISOString(), name: 'new' }),
          snapshot({ vmid: 7, time: new Date(now - 10 * DAY_MS).toISOString(), name: 'mid' }),
        ],
        now,
      );

      const guest = summary.guests[0];
      expect(guest.snapshots.map((s) => s.name)).toEqual(['new', 'mid', 'old']);
    });
  });

  describe('buildArchiveCoverageSummary', () => {
    const now = Date.parse('2026-05-20T12:00:00Z');

    it('splits guests into current/stale/uncovered by newest archive age', () => {
      const summary = buildArchiveCoverageSummary(
        [
          archive({ vmid: 100, time: new Date(now - 1 * DAY_MS).toISOString(), size: 1000 }),
          archive({ vmid: 100, time: new Date(now - 40 * DAY_MS).toISOString(), size: 2000 }),
          archive({ vmid: 200, time: new Date(now - 14 * DAY_MS).toISOString(), size: 3000 }),
          archive({ vmid: 300, time: new Date(now - 50 * DAY_MS).toISOString(), size: 4000 }),
          archive({ vmid: 400, time: new Date(now - 120 * DAY_MS).toISOString(), size: 5000 }),
        ],
        now,
      );

      expect(summary.totalGuests).toBe(4);
      expect(summary.totalArchives).toBe(5);
      expect(summary.totalBytes).toBe(15_000);
      // 100 (newest 1d) → current; 200 (14d) → stale-window; 300 (50d) +
      // 400 (120d) → uncovered. The summary collapses the 7–30d "normal"
      // bucket into staleGuests since either way they are not "current".
      expect(summary.currentGuests).toBe(1);
      expect(summary.staleGuests).toBe(1);
      expect(summary.uncoveredGuests).toBe(2);
    });
  });

  describe('buildTaskOutcomeSummary', () => {
    it('counts outcomes and flags whether any errors exist', () => {
      const outcome = buildTaskOutcomeSummary([
        task({ startTime: '2026-05-20T01:00:00Z', status: 'OK' }),
        task({ startTime: '2026-05-20T02:00:00Z', status: 'failed', error: 'disk full' }),
        task({ startTime: '2026-05-20T03:00:00Z', status: 'running' }),
        task({ startTime: '2026-05-20T04:00:00Z', status: 'completed' }),
      ]);

      expect(outcome.total).toBe(4);
      expect(outcome.ok).toBe(2);
      expect(outcome.failed).toBe(1);
      expect(outcome.running).toBe(1);
      expect(outcome.hasErrors).toBe(true);
    });

    it('reports hasErrors=false when no task carries an error message', () => {
      const outcome = buildTaskOutcomeSummary([
        task({ startTime: '2026-05-20T01:00:00Z', status: 'ok' }),
        task({ startTime: '2026-05-20T02:00:00Z', status: 'ok' }),
      ]);
      expect(outcome.hasErrors).toBe(false);
    });
  });

  describe('computeMedianTaskDurationSeconds', () => {
    it('returns 0 when no durations are present', () => {
      expect(computeMedianTaskDurationSeconds([])).toBe(0);
      expect(
        computeMedianTaskDurationSeconds([
          task({ startTime: '2026-05-20T01:00:00Z' }), // no endTime
        ]),
      ).toBe(0);
    });

    it('computes the median across finite durations', () => {
      const median = computeMedianTaskDurationSeconds([
        task({ startTime: '2026-05-20T01:00:00Z', endTime: '2026-05-20T01:00:10Z' }), // 10s
        task({ startTime: '2026-05-20T01:00:00Z', endTime: '2026-05-20T01:01:00Z' }), // 60s
        task({ startTime: '2026-05-20T01:00:00Z', endTime: '2026-05-20T01:02:00Z' }), // 120s
      ]);
      expect(median).toBe(60);
    });
  });

  describe('classifyArchiveRowAge', () => {
    const now = Date.parse('2026-05-20T12:00:00Z');

    it('aligns row swatch with the archive coverage strip thresholds (≤7d / 7–30d / >30d)', () => {
      expect(classifyArchiveRowAge(now - 1 * DAY_MS, now).swatchClass).toBe('bg-emerald-500');
      expect(classifyArchiveRowAge(now - 7 * DAY_MS + 1000, now).swatchClass).toBe('bg-emerald-500');
      expect(classifyArchiveRowAge(now - 14 * DAY_MS, now).swatchClass).toBe('bg-amber-500');
      expect(classifyArchiveRowAge(now - 30 * DAY_MS + 1000, now).swatchClass).toBe('bg-amber-500');
      expect(classifyArchiveRowAge(now - 40 * DAY_MS, now).swatchClass).toBe('bg-red-500');
      expect(classifyArchiveRowAge(now - 200 * DAY_MS, now).swatchClass).toBe('bg-red-500');
    });

    it('treats undefined as the worst bucket so missing timestamps surface', () => {
      expect(classifyArchiveRowAge(undefined, now).swatchClass).toBe('bg-red-500');
    });
  });

  describe('classifySnapshotRowAge', () => {
    const now = Date.parse('2026-05-20T12:00:00Z');

    it('aligns row swatch with the snapshot coverage strip thresholds (≤30d / 30–90d / >90d)', () => {
      expect(classifySnapshotRowAge(now - 1 * DAY_MS, now).swatchClass).toBe('bg-emerald-500');
      expect(classifySnapshotRowAge(now - 30 * DAY_MS + 1000, now).swatchClass).toBe(
        'bg-emerald-500',
      );
      expect(classifySnapshotRowAge(now - 60 * DAY_MS, now).swatchClass).toBe('bg-amber-500');
      expect(classifySnapshotRowAge(now - 90 * DAY_MS + 1000, now).swatchClass).toBe(
        'bg-amber-500',
      );
      expect(classifySnapshotRowAge(now - 100 * DAY_MS, now).swatchClass).toBe('bg-red-500');
    });
  });

  describe('guestKey', () => {
    it('namespaces vmid by instance and type to avoid VM/CT collisions', () => {
      expect(guestKey({ type: 'qemu', instance: 'a', vmid: 100 })).toBe('a:qemu:100');
      expect(guestKey({ type: 'lxc', instance: 'a', vmid: 100 })).toBe('a:lxc:100');
      expect(guestKey({ type: 'qemu', instance: 'a', vmid: 100 })).not.toBe(
        guestKey({ type: 'qemu', instance: 'b', vmid: 100 }),
      );
    });
  });
});
