import { buildBackupsPath, buildStoragePath } from './resourceLinks';

export type StorageBackupsTabSpec = {
  id: 'storage' | 'backups';
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  badge?: 'preview';
};

/**
 * Returns the canonical Storage + Backups tab specs for the platform nav.
 * Legacy dual-tab modes were removed in SB5-05.
 */
export function buildStorageBackupsTabSpecs(_planOrShowV2?: unknown): StorageBackupsTabSpec[] {
  return [
    {
      id: 'storage',
      label: 'Storage',
      route: buildStoragePath(),
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Storage pools, disks, and datastores',
    },
    {
      id: 'backups',
      label: 'Backups',
      route: buildBackupsPath(),
      settingsRoute: '/settings/system-backups',
      tooltip: 'Backup jobs and schedules',
    },
  ];
}
