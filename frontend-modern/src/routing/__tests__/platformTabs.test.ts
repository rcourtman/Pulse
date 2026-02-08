import { describe, expect, it } from 'vitest';
import { buildBackupsPath, buildStoragePath } from '@/routing/resourceLinks';
import { buildStorageBackupsTabSpecs } from '@/routing/platformTabs';

describe('buildStorageBackupsTabSpecs', () => {
  it('returns canonical storage and backups tabs', () => {
    const specs = buildStorageBackupsTabSpecs();
    expect(specs).toHaveLength(2);
    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'backups']);
    expect(specs.map((spec) => spec.label)).toEqual(['Storage', 'Backups']);
    expect(specs.map((spec) => spec.route)).toEqual([buildStoragePath(), buildBackupsPath()]);
    expect(specs.map((spec) => spec.settingsRoute)).toEqual([
      '/settings/infrastructure/pbs',
      '/settings/system-backups',
    ]);
    expect(specs.every((spec) => spec.badge === undefined)).toBe(true);
  });

  it('ignores argument (backward compat)', () => {
    const withTrue = buildStorageBackupsTabSpecs(true);
    const withFalse = buildStorageBackupsTabSpecs(false);
    const withNothing = buildStorageBackupsTabSpecs();
    expect(withTrue.map((spec) => spec.id)).toEqual(['storage', 'backups']);
    expect(withFalse.map((spec) => spec.id)).toEqual(['storage', 'backups']);
    expect(withNothing.map((spec) => spec.id)).toEqual(['storage', 'backups']);
  });
});
