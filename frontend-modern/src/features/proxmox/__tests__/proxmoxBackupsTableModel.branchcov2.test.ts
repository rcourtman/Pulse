import { describe, expect, it } from 'vitest';

import type { PBSBackup } from '@/types/api';

import {
  classifyTaskStatus,
  cmpBool,
  cmpNumber,
  cmpString,
  pbsRepositoryLabel,
  pbsWorkloadLabel,
} from '../proxmoxBackupsTableModel';

// Same loose fixture builder the sibling test file uses: Partial overrides
// cast whole as PBSBackup so omitted required fields are undefined at runtime,
// which is exactly what lets us drive the defensive `??` / `?.` / `||` arms.
const pbs = (overrides: Partial<PBSBackup>): PBSBackup => overrides as PBSBackup;

// ---------------------------------------------------------------------------
// pbsWorkloadLabel — sibling tests cover ct/vm and the host arms. These cover
// the fallback branch (unknown/absent backupType) and the toLowerCase path.
// ---------------------------------------------------------------------------

describe('pbsWorkloadLabel — uncovered branches', () => {
  it('falls back to "Backup <vmid>" when backupType is absent and vmid is set', () => {
    // backupType omitted -> runtime undefined -> `backup.backupType ?? ''`
    // right operand fires; `backup.backupType?.trim()` short-circuits to
    // undefined -> `|| 'Backup'` defaults the kind; vmid truthy -> `${kind} ${vmid}`.
    expect(pbsWorkloadLabel(pbs({ vmid: '500' }))).toBe('Backup 500');
  });

  it('uppercases an unknown backupType and appends vmid', () => {
    // Fallback: trim().toUpperCase() yields a non-empty kind; vmid truthy arm.
    expect(pbsWorkloadLabel(pbs({ backupType: 'qemu', vmid: '555' }))).toBe('QEMU 555');
  });

  it('returns just the uppercased kind when vmid is empty for an unknown type', () => {
    // Fallback return `: kind` arm (vmid falsy).
    expect(pbsWorkloadLabel(pbs({ backupType: 'qemu', vmid: '' }))).toBe('QEMU');
  });

  it('defaults a whitespace-only backupType to "Backup" with no vmid', () => {
    // backupType present but trim() -> '' -> `|| 'Backup'`; empty vmid -> `: kind`.
    expect(pbsWorkloadLabel(pbs({ backupType: '   ', vmid: '' }))).toBe('Backup');
  });

  it('matches backupType case-insensitively on the ct arm', () => {
    // Exercises `(backup.backupType ?? '').toLowerCase()` normalizing 'CT' -> 'ct'.
    expect(pbsWorkloadLabel(pbs({ backupType: 'CT', vmid: '707' }))).toBe('LXC 707');
  });
});

// ---------------------------------------------------------------------------
// cmpString — sibling tests cover the av<bv / av>bv and single-blank asc arms.
// These cover both-blank, both-undefined, single-undefined, equal, and desc.
// ---------------------------------------------------------------------------

describe('cmpString — uncovered branches', () => {
  it('returns 0 when both values are empty strings', () => {
    // `!av && !bv` arm.
    expect(cmpString('', '', 'asc')).toBe(0);
    expect(cmpString('', '', 'desc')).toBe(0);
  });

  it('returns 0 when both values are undefined', () => {
    // Both `(a ?? '')` and `(b ?? '')` right operands fire, then `!av && !bv`.
    expect(cmpString(undefined, undefined, 'asc')).toBe(0);
    expect(cmpString(undefined, undefined, 'desc')).toBe(0);
  });

  it('pushes an undefined first value to the end regardless of direction', () => {
    // `if (!av) return 1` arm (a undefined -> av '').
    expect(cmpString(undefined, 'beta', 'asc')).toBe(1);
    expect(cmpString(undefined, 'beta', 'desc')).toBe(1);
  });

  it('pushes an undefined second value to the end regardless of direction', () => {
    // `if (!bv) return -1` arm (b undefined -> bv '').
    expect(cmpString('alpha', undefined, 'asc')).toBe(-1);
    expect(cmpString('alpha', undefined, 'desc')).toBe(-1);
  });

  it('returns 0 for equal non-empty values (desc negation yields -0)', () => {
    // `av === bv ? 0` arm. asc returns +0; desc does `-cmp` = `-0`. The two are
    // equal under `===` (sort-safe) but distinct under Object.is — asserted
    // explicitly below to pin the real desc-arm output.
    const asc = cmpString('alpha', 'alpha', 'asc');
    const desc = cmpString('alpha', 'alpha', 'desc');
    expect(asc).toBe(0);
    expect(desc === 0).toBe(true);
    expect(Object.is(desc, -0)).toBe(true);
  });

  it('keeps blank values last even under desc', () => {
    // Blank guards intentionally ignore direction; sibling only asserted asc.
    expect(cmpString('', 'beta', 'desc')).toBe(1);
    expect(cmpString('beta', '', 'desc')).toBe(-1);
  });
});

// ---------------------------------------------------------------------------
// cmpNumber — sibling tests cover a<b asc/desc, a=undefined, and a=NaN. These
// cover both-missing, b-missing, equal, non-finite coercion, and a>b desc.
// ---------------------------------------------------------------------------

describe('cmpNumber — uncovered branches', () => {
  it('returns 0 when both values are undefined', () => {
    // `av === undefined && bv === undefined` arm.
    expect(cmpNumber(undefined, undefined, 'asc')).toBe(0);
    expect(cmpNumber(undefined, undefined, 'desc')).toBe(0);
  });

  it('pushes an undefined second value to the end regardless of direction', () => {
    // `if (bv === undefined) return -1` arm (sibling only covers a undefined).
    expect(cmpNumber(5, undefined, 'asc')).toBe(-1);
    expect(cmpNumber(5, undefined, 'desc')).toBe(-1);
  });

  it('returns 0 for equal finite numbers (desc negation yields -0)', () => {
    // asc returns +0; desc does `-cmp` = `-0` for an equal pair.
    const asc = cmpNumber(5, 5, 'asc');
    const desc = cmpNumber(5, 5, 'desc');
    expect(asc).toBe(0);
    expect(desc === 0).toBe(true);
    expect(Object.is(desc, -0)).toBe(true);
  });

  it('treats +Infinity, -Infinity, and NaN as missing', () => {
    // `typeof === "number" && Number.isFinite(...)` rejects all three.
    expect(cmpNumber(Number.POSITIVE_INFINITY, 5, 'asc')).toBe(1);
    expect(cmpNumber(Number.NEGATIVE_INFINITY, 5, 'asc')).toBe(1);
    expect(cmpNumber(5, Number.NaN, 'asc')).toBe(-1);
  });

  it('negates the comparison under desc when the first value is larger', () => {
    // a > b -> cmp = +3; desc -> -3. Sibling desc case only had a < b (>0).
    expect(cmpNumber(5, 2, 'asc')).toBe(3);
    expect(cmpNumber(5, 2, 'desc')).toBe(-3);
  });
});

// ---------------------------------------------------------------------------
// classifyTaskStatus — sibling tests cover ok/SUCCESS, running, error, empty,
// and a lowercase unknown. These cover 'completed', 'failed', case-insensitive
// warning, null status, and original-case preservation in the fallback label.
// ---------------------------------------------------------------------------

describe('classifyTaskStatus — uncovered branches', () => {
  it('maps "completed" to the success variant with the exact full shape', () => {
    // Third OR operand of the success condition (sibling covers ok/SUCCESS).
    expect(classifyTaskStatus('completed')).toStrictEqual({
      variant: 'success',
      label: 'OK',
      toneClass: 'text-emerald-600 dark:text-emerald-300',
    });
  });

  it('maps "failed" to the danger variant with the exact full shape', () => {
    // Other OR operand of the danger condition (sibling only covers 'error').
    expect(classifyTaskStatus('failed')).toStrictEqual({
      variant: 'danger',
      label: 'Failed',
      toneClass: 'text-red-600 dark:text-red-300',
    });
  });

  it('maps "RUNNING" (upper) to warning via toLowerCase normalization', () => {
    // `normalized === 'running'` arm reached only because of `.toLowerCase()`.
    expect(classifyTaskStatus('RUNNING')).toStrictEqual({
      variant: 'warning',
      label: 'Running',
      toneClass: 'text-amber-600 dark:text-amber-300',
    });
  });

  it('treats a null status as empty and returns the muted em-dash', () => {
    // `(status ?? '')` right operand -> '' -> `!normalized` early-return arm.
    expect(
      classifyTaskStatus(null as unknown as Parameters<typeof classifyTaskStatus>[0]),
    ).toStrictEqual({
      variant: 'muted',
      label: '—',
      toneClass: 'text-muted',
    });
  });

  it('preserves the original case in the fallback label for unknown statuses', () => {
    // Terminal fallback returns `label: status` (the original, not normalized).
    expect(classifyTaskStatus('Paused')).toStrictEqual({
      variant: 'muted',
      label: 'Paused',
      toneClass: 'text-muted',
    });
  });
});

// ---------------------------------------------------------------------------
// pbsRepositoryLabel — sibling tests cover datastore+namespace and datastore
// with an absent namespace. These cover the em-dash datastore fallback, the
// whitespace/trim namespace paths, and the both-absent combination.
// ---------------------------------------------------------------------------

describe('pbsRepositoryLabel — uncovered branches', () => {
  it('falls back to an em-dash when datastore is absent', () => {
    // `backup.datastore || '—'` right operand.
    expect(pbsRepositoryLabel(pbs({ namespace: 'team' }))).toBe('— / team');
  });

  it('falls back to an em-dash when datastore is an empty string', () => {
    expect(pbsRepositoryLabel(pbs({ datastore: '', namespace: 'team' }))).toBe('— / team');
  });

  it('treats a whitespace-only namespace as root', () => {
    // `namespace?.trim()` -> '' (falsy) -> '(root)'.
    expect(pbsRepositoryLabel(pbs({ datastore: 'main', namespace: '   ' }))).toBe(
      'main / (root)',
    );
  });

  it('trims surrounding whitespace from a named namespace', () => {
    // `namespace?.trim()` -> 'team' (truthy, trimmed).
    expect(pbsRepositoryLabel(pbs({ datastore: 'main', namespace: '  team  ' }))).toBe(
      'main / team',
    );
  });

  it('falls back fully when both datastore and namespace are absent', () => {
    // namespace undefined -> `?.` short-circuits -> '(root)'; datastore -> '—'.
    expect(pbsRepositoryLabel(pbs({}))).toBe('— / (root)');
  });
});

// ---------------------------------------------------------------------------
// cmpBool — sibling tests cover true/false desc, true/false asc, and true/true
// asc. These cover false/false, false/true (both directions), and true/true desc.
// ---------------------------------------------------------------------------

describe('cmpBool — uncovered branches', () => {
  it('returns 0 for false vs false (desc negation yields -0)', () => {
    // Both `(a ? 1 : 0)` and `(b ? 1 : 0)` take their falsy arms -> cmp 0.
    const asc = cmpBool(false, false, 'asc');
    const desc = cmpBool(false, false, 'desc');
    expect(asc).toBe(0);
    expect(desc === 0).toBe(true);
    expect(Object.is(desc, -0)).toBe(true);
  });

  it('sorts false ahead of true under asc and behind under desc', () => {
    // a falsy / b truthy: cmp = 0 - 1 = -1; asc keeps -1, desc negates to +1.
    expect(cmpBool(false, true, 'asc')).toBe(-1);
    expect(cmpBool(false, true, 'desc')).toBe(1);
  });

  it('returns 0 for true vs true under desc (desc negation yields -0)', () => {
    // Sibling covers true/true asc; this nails the desc arm, whose `-cmp` of a
    // zero produces -0.
    const desc = cmpBool(true, true, 'desc');
    expect(desc === 0).toBe(true);
    expect(Object.is(desc, -0)).toBe(true);
  });
});
