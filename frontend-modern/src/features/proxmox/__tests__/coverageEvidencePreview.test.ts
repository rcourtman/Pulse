import { describe, expect, it } from 'vitest';
import { selectCoverageRestoreEvidence } from '../proxmoxBackupsTablePresentation';

interface Entry {
  id: string;
  sourceKind: string;
  createdMs?: number;
  running?: boolean;
  failed?: boolean;
}
const kinds = ['pbs', 'archive', 'snapshot'] as const;
const dated = (sourceKind: string, length = 12): Entry[] =>
  Array.from({ length }, (_, index) => ({
    id: `${sourceKind}-dated-${index}`,
    sourceKind,
    createdMs: 1_000 + index,
  }));
const ids = (entries: readonly Entry[]) => entries.map((entry) => entry.id);

describe('Coverage uncertain evidence preview', () => {
  it.each(kinds)(
    'retains an undated completed %s point beside newer dated observations',
    (kind) => {
      const unknown: Entry = { id: `${kind}-undated`, sourceKind: kind };
      const input = [
        ...dated(kind),
        ...dated(kind === 'snapshot' ? 'archive' : 'snapshot', 15),
        unknown,
      ];
      const result = selectCoverageRestoreEvidence(input);
      expect(ids(result)).toContain(unknown.id);
      expect(result).toHaveLength(8);
      expect(ids(result)).toContain(`${kind}-dated-11`);
    },
  );

  it.each(['running', 'failed'] as const)(
    'retains an undated %s point without hiding completion',
    (state) => {
      const unknown: Entry = { id: `pbs-${state}`, sourceKind: 'pbs', [state]: true };
      const input = [...dated('pbs'), ...dated('snapshot', 15), unknown];
      const result = selectCoverageRestoreEvidence(input);
      expect(ids(result)).toContain(unknown.id);
      expect(ids(result)).toContain('pbs-dated-11');
      expect(result.find((entry) => entry.id === unknown.id)?.[state]).toBe(true);
      expect(result).toHaveLength(8);
    },
  );

  it('keeps a witness for every uncertain source and state even beyond the normal preview cap', () => {
    const input: Entry[] = kinds.flatMap((kind) => [
      ...dated(kind),
      { id: `${kind}-completed`, sourceKind: kind },
      { id: `${kind}-running`, sourceKind: kind, running: true },
      { id: `${kind}-failed`, sourceKind: kind, failed: true },
    ]);
    const result = selectCoverageRestoreEvidence(input);
    expect(result).toHaveLength(12);
    for (const kind of kinds) {
      for (const suffix of ['completed', 'running', 'failed', 'dated-11'])
        expect(ids(result)).toContain(`${kind}-${suffix}`);
    }
  });

  it('chooses the same bounded witnesses when an unordered inventory is reversed', () => {
    const input: Entry[] = [
      ...dated('pbs'),
      ...Array.from({ length: 10_000 }, (_, index) => ({
        id: `unknown-${String(index).padStart(5, '0')}`,
        sourceKind: 'pbs',
      })),
    ];
    const result = selectCoverageRestoreEvidence(input);
    expect(result).toHaveLength(8);
    expect(ids(result)).toContain('unknown-00000');
    expect(ids(selectCoverageRestoreEvidence([...input].reverse()))).toEqual(ids(result));
    expect(input[0].id).toBe('pbs-dated-0');
  });

  it('preserves the newest dated and completed controls and only expands at the caller limit', () => {
    const input: Entry[] = [
      ...dated('pbs', 20),
      { id: 'newest-running', sourceKind: 'pbs', createdMs: 2_000, running: true },
    ];
    const preview = selectCoverageRestoreEvidence(input);
    expect(preview).toHaveLength(8);
    expect(ids(preview).slice(0, 2)).toEqual(['newest-running', 'pbs-dated-19']);
    expect(selectCoverageRestoreEvidence(input, 16)).toHaveLength(16);
    expect(selectCoverageRestoreEvidence(input, 24)).toHaveLength(21);
    expect(selectCoverageRestoreEvidence([])).toEqual([]);
  });
});
