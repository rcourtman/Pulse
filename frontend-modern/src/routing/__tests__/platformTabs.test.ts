import { describe, expect, it } from 'vitest';
import { BACKUPS_V2_PATH, STORAGE_V2_PATH, buildBackupsPath, buildStoragePath } from '@/routing/resourceLinks';
import { buildStorageBackupsTabSpecs } from '@/routing/platformTabs';

describe('buildStorageBackupsTabSpecs', () => {
  it('returns only canonical storage/backups tabs when v2 default is enabled', () => {
    const specs = buildStorageBackupsTabSpecs(true);

    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'backups']);
    expect(specs.map((spec) => spec.label)).toEqual(['Storage', 'Backups']);
    expect(specs.map((spec) => spec.route)).toEqual([buildStoragePath(), buildBackupsPath()]);
    expect(specs.every((spec) => spec.badge === undefined)).toBe(true);
  });

  it('returns legacy + preview pairs when v2 default is disabled', () => {
    const specs = buildStorageBackupsTabSpecs(false);

    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'storage-v2', 'backups', 'backups-v2']);
    expect(specs.map((spec) => spec.label)).toEqual([
      'Storage (Legacy)',
      'Storage V2',
      'Backups (Legacy)',
      'Backups V2',
    ]);
    expect(specs.map((spec) => spec.route)).toEqual([
      buildStoragePath(),
      STORAGE_V2_PATH,
      buildBackupsPath(),
      BACKUPS_V2_PATH,
    ]);
    expect(specs.filter((spec) => spec.badge === 'preview').map((spec) => spec.id)).toEqual([
      'storage-v2',
      'backups-v2',
    ]);
  });
});
