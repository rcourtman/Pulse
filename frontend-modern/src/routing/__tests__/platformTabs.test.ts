import { describe, expect, it } from 'vitest';
import { BACKUPS_V2_PATH, STORAGE_V2_PATH, buildBackupsPath, buildStoragePath } from '@/routing/resourceLinks';
import { buildStorageBackupsTabSpecs } from '@/routing/platformTabs';
import { buildStorageBackupsRoutingPlan } from '@/routing/storageBackupsMode';

describe('buildStorageBackupsTabSpecs', () => {
  it('returns only canonical storage/backups tabs when v2 default is enabled', () => {
    const specs = buildStorageBackupsTabSpecs(buildStorageBackupsRoutingPlan('v2-default'));

    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'backups']);
    expect(specs.map((spec) => spec.label)).toEqual(['Storage', 'Backups']);
    expect(specs.map((spec) => spec.route)).toEqual([buildStoragePath(), buildBackupsPath()]);
    expect(specs.every((spec) => spec.badge === undefined)).toBe(true);
  });

  it('returns legacy + preview pairs when legacy default is active', () => {
    const specs = buildStorageBackupsTabSpecs(buildStorageBackupsRoutingPlan('legacy-default'));

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

  it('returns storage legacy + preview but backups as v2 default in backups-v2-default mode', () => {
    const specs = buildStorageBackupsTabSpecs(buildStorageBackupsRoutingPlan('backups-v2-default'));

    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'storage-v2', 'backups']);
    expect(specs.map((spec) => spec.label)).toEqual([
      'Storage (Legacy)',
      'Storage V2',
      'Backups',
    ]);
    expect(specs.map((spec) => spec.route)).toEqual([
      buildStoragePath(),
      STORAGE_V2_PATH,
      buildBackupsPath(),
    ]);
    // Only storage-v2 has preview badge; backups has no badge
    expect(specs.filter((spec) => spec.badge === 'preview').map((spec) => spec.id)).toEqual([
      'storage-v2',
    ]);
    expect(specs.find((spec) => spec.id === 'backups')?.badge).toBeUndefined();
  });

  it('returns storage as v2 default but backups legacy + preview in storage-v2-default mode', () => {
    const specs = buildStorageBackupsTabSpecs(buildStorageBackupsRoutingPlan('storage-v2-default'));

    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'backups', 'backups-v2']);
    expect(specs.map((spec) => spec.label)).toEqual([
      'Storage',
      'Backups (Legacy)',
      'Backups V2',
    ]);
    expect(specs.map((spec) => spec.route)).toEqual([
      buildStoragePath(),
      buildBackupsPath(),
      BACKUPS_V2_PATH,
    ]);
    expect(specs.filter((spec) => spec.badge === 'preview').map((spec) => spec.id)).toEqual([
      'backups-v2',
    ]);
    expect(specs.find((spec) => spec.id === 'storage')?.badge).toBeUndefined();
  });

  it('backward compat: boolean true produces v2-default tabs', () => {
    const specs = buildStorageBackupsTabSpecs(true);
    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'backups']);
  });

  it('backward compat: boolean false produces all-legacy tabs', () => {
    const specs = buildStorageBackupsTabSpecs(false);
    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'storage-v2', 'backups', 'backups-v2']);
  });
});
