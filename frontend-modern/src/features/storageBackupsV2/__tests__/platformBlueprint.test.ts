import { describe, expect, it } from 'vitest';
import { KNOWN_STORAGE_BACKUP_PLATFORMS } from '@/features/storageBackupsV2/models';
import {
  PLATFORM_BLUEPRINTS,
  PLATFORM_BLUEPRINT_BY_ID,
} from '@/features/storageBackupsV2/platformBlueprint';

describe('platformBlueprint', () => {
  it('defines one blueprint per known platform', () => {
    expect(PLATFORM_BLUEPRINTS).toHaveLength(KNOWN_STORAGE_BACKUP_PLATFORMS.length);

    const ids = PLATFORM_BLUEPRINTS.map((blueprint) => blueprint.id).sort();
    const known = [...KNOWN_STORAGE_BACKUP_PLATFORMS].sort();
    expect(ids).toEqual(known);
  });

  it('keeps capability lists deduplicated per platform', () => {
    for (const blueprint of PLATFORM_BLUEPRINTS) {
      expect(new Set(blueprint.storageCapabilities).size).toBe(blueprint.storageCapabilities.length);
      expect(new Set(blueprint.backupCapabilities).size).toBe(blueprint.backupCapabilities.length);
      expect(PLATFORM_BLUEPRINT_BY_ID.get(blueprint.id)).toEqual(blueprint);
    }
  });

  it('marks Kubernetes and NAS platforms as next-stage targets', () => {
    const nextIds = PLATFORM_BLUEPRINTS.filter((blueprint) => blueprint.stage === 'next').map(
      (blueprint) => blueprint.id,
    );
    expect(nextIds).toEqual(expect.arrayContaining(['kubernetes', 'truenas', 'unraid']));
  });
});
