import { describe, expect, it } from 'vitest';
import { buildRecoveryPath, buildStoragePath } from '@/routing/resourceLinks';
import { buildStorageBackupsTabSpecs } from '@/routing/platformTabs';

describe('buildStorageBackupsTabSpecs', () => {
  it('returns canonical storage and backups tabs', () => {
    const specs = buildStorageBackupsTabSpecs();
    expect(specs).toHaveLength(2);
    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'recovery']);
    expect(specs.map((spec) => spec.label)).toEqual(['Storage', 'Recovery']);
    expect(specs.map((spec) => spec.route)).toEqual([buildStoragePath(), buildRecoveryPath()]);
    expect(specs.map((spec) => spec.settingsRoute)).toEqual([
      '/settings/infrastructure/pbs',
      '/settings/system-recovery',
    ]);
    expect(specs.every((spec) => spec.badge === undefined)).toBe(true);
  });

  it('ignores argument (backward compat)', () => {
    const withTrue = buildStorageBackupsTabSpecs(true);
    const withFalse = buildStorageBackupsTabSpecs(false);
    const withNothing = buildStorageBackupsTabSpecs();
    expect(withTrue.map((spec) => spec.id)).toEqual(['storage', 'recovery']);
    expect(withFalse.map((spec) => spec.id)).toEqual(['storage', 'recovery']);
    expect(withNothing.map((spec) => spec.id)).toEqual(['storage', 'recovery']);
  });
});
