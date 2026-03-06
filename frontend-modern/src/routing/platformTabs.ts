import { buildRecoveryPath, buildStoragePath } from './resourceLinks';

export type StorageRecoveryTabSpec = {
  id: 'storage' | 'recovery';
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  badge?: 'preview';
};

/**
 * Returns the canonical Storage + Recovery tab specs for the platform nav.
 * Legacy dual-tab modes were removed in SB5-05.
 */
export function buildStorageRecoveryTabSpecs(): StorageRecoveryTabSpec[] {
  return [
    {
      id: 'storage',
      label: 'Storage',
      route: buildStoragePath(),
      settingsRoute: '/settings/infrastructure/api/pbs',
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
