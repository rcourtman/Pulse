import { describe, expect, it } from 'vitest';
import {
  backupNodeMatches,
  backupNodeOptions,
  backupNodeScope,
  resolveBackupNodeSelection,
} from '../proxmoxBackupNodeScope';
import type { WorkloadReference } from '../proxmoxBackupRecoveryModel';
const node = (
  key: string,
  instance = 'east',
  nativeNode = 'pve-a',
  display = 'Shared',
): WorkloadReference => ({
  key,
  instance,
  nativeNode,
  node: display,
  type: 'vm',
  typeLabel: 'VM',
  vmid: '100',
  label: key,
});

describe('backup node scope identity', () => {
  it('keeps identical display/native names in different connections separate', () => {
    const a = node('a'),
      b = node('b', 'west');
    expect(backupNodeScope(a)?.key).not.toBe(backupNodeScope(b)?.key);
    expect(resolveBackupNodeSelection([a, b], 'Shared')).toEqual({ state: 'ambiguous' });
    const selected = resolveBackupNodeSelection([a, b], backupNodeScope(a)!.key);
    expect(backupNodeMatches(a, selected)).toBe(true);
    expect(backupNodeMatches(b, selected)).toBe(false);
  });
  it('does not merge renamed native nodes in one connection', () => {
    const a = node('a'),
      b = node('b', 'east', 'pve-b');
    expect(backupNodeOptions([a, b]).map((option) => option.label)).toEqual([
      'east / Shared (pve-a)',
      'east / Shared (pve-b)',
    ]);
    expect(resolveBackupNodeSelection([a, b], 'Shared').state).toBe('ambiguous');
  });
  it('preserves connection spelling, field boundaries and native node spelling', () => {
    const inputs = [
      node('a', 'East'),
      node('b', 'east'),
      node('c', 'east/pve-a'),
      node('d', 'east', 'pve-a/b'),
      node('e', 'east', 'PVE-a'),
    ];
    expect(new Set(inputs.map((value) => backupNodeScope(value)!.key)).size).toBe(inputs.length);
  });
  it('groups complete node identities across guests and picks a stable label independent of order', () => {
    const a = node('a'),
      b = node('b', 'east', 'pve-a', 'Other alias');
    expect(backupNodeOptions([a, b])).toEqual(backupNodeOptions([b, a]));
    expect(backupNodeOptions([a, b])).toHaveLength(1);
    expect(resolveBackupNodeSelection([a, b], 'Shared').state).toBe('matched');
  });
  it.each(['instance', 'nativeNode'] as const)(
    'keeps incomplete %s evidence local instead of pooling workloads',
    (field) => {
      const a = { ...node('a'), [field]: undefined },
        b = { ...node('b'), [field]: undefined };
      expect(backupNodeScope(a)?.key).not.toBe(backupNodeScope(b)?.key);
      expect(resolveBackupNodeSelection([a, b], 'Shared').state).toBe('ambiguous');
      expect(backupNodeOptions([a])[0].label).toContain('a');
    },
  );
  it('does not promote an absent saved selection to All or a different source', () => {
    const a = node('a'),
      b = node('b', 'west');
    const key = backupNodeScope(a)!.key;
    const missing = resolveBackupNodeSelection([b], key);
    expect(missing).toEqual({ state: 'missing' });
    expect(backupNodeMatches(b, missing)).toBe(false);
    expect(resolveBackupNodeSelection([a, b], '')).toEqual({ state: 'all' });
    expect(backupNodeMatches(a, { state: 'all' })).toBe(true);
  });
  it('keeps exact selection across friendly renames while retiring an obsolete legacy alias', () => {
    const before = node('a'),
      after = node('a', 'east', 'pve-a', 'New label');
    expect(resolveBackupNodeSelection([after], backupNodeScope(before)!.key)).toMatchObject({
      state: 'matched',
      label: 'east / New label (pve-a)',
    });
    expect(resolveBackupNodeSelection([after], 'Shared')).toEqual({ state: 'missing' });
  });
  it('does not offer a node for an unplaced workload', () => {
    expect(backupNodeOptions([{ ...node('a'), node: undefined, nativeNode: undefined }])).toEqual(
      [],
    );
  });
});
