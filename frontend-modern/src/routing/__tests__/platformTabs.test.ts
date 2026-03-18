import { describe, expect, it } from 'vitest';
import { buildRecoveryPath, buildStoragePath } from '@/routing/resourceLinks';
import { buildStorageRecoveryTabSpecs } from '@/routing/platformTabs';

describe('buildStorageRecoveryTabSpecs', () => {
  it('returns canonical storage and recovery tabs', () => {
    const specs = buildStorageRecoveryTabSpecs();
    expect(specs).toHaveLength(2);
    expect(specs.map((spec) => spec.id)).toEqual(['storage', 'recovery']);
    expect(specs.map((spec) => spec.label)).toEqual(['Storage', 'Recovery']);
    expect(specs.map((spec) => spec.route)).toEqual([buildStoragePath(), buildRecoveryPath()]);
    expect(specs.map((spec) => spec.settingsRoute)).toEqual([
      '/settings/infrastructure/api/pbs',
      '/settings/system-recovery',
    ]);
    expect(specs.every((spec) => spec.badge === undefined)).toBe(true);
  });
});
