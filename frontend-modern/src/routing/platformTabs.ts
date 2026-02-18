import { buildRecoveryPath, buildStoragePath } from './resourceLinks';

export type StorageRecoveryTabSpec = {
  id: 'storage' | 'recovery';
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  badge?: 'preview';
};

// Backwards-compat for older call sites.
export type StorageBackupsTabSpec = StorageRecoveryTabSpec;

/**
 * Returns the canonical Storage + Recovery tab specs for the platform nav.
 * Legacy dual-tab modes were removed in SB5-05.
 */
export function buildStorageRecoveryTabSpecs(_planOrShowV2?: unknown): StorageRecoveryTabSpec[] {
  return [
    {
      id: 'storage',
      label: 'Storage',
      route: buildStoragePath(),
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Storage pools, disks, and datastores',
    },
    {
      id: 'recovery',
      label: 'Recovery',
      route: buildRecoveryPath(),
      settingsRoute: '/settings/system-recovery',
      tooltip: 'Backup, snapshot, and replication activity',
    },
  ];
}

// Backwards-compat for older call sites.
export function buildStorageBackupsTabSpecs(_planOrShowV2?: unknown): StorageBackupsTabSpec[] {
  return buildStorageRecoveryTabSpecs(_planOrShowV2);
}
