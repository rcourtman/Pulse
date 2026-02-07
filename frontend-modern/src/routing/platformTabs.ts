import {
  BACKUPS_V2_PATH,
  STORAGE_V2_PATH,
  buildBackupsPath,
  buildStoragePath,
} from './resourceLinks';

const STORAGE_PATH = buildStoragePath();
const BACKUPS_PATH = buildBackupsPath();

export type StorageBackupsTabSpec = {
  id: 'storage' | 'storage-v2' | 'backups' | 'backups-v2';
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  badge?: string;
};

export const buildStorageBackupsTabSpecs = (
  showV2DefaultTabs: boolean,
): StorageBackupsTabSpec[] => {
  if (showV2DefaultTabs) {
    return [
      {
        id: 'storage',
        label: 'Storage',
        route: STORAGE_PATH,
        settingsRoute: '/settings/infrastructure/pbs',
        tooltip: 'Source-agnostic storage',
      },
      {
        id: 'backups',
        label: 'Backups',
        route: BACKUPS_PATH,
        settingsRoute: '/settings/backups',
        tooltip: 'Source-agnostic backups',
      },
    ];
  }

  return [
    {
      id: 'storage',
      label: 'Storage (Legacy)',
      route: STORAGE_PATH,
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Legacy storage page',
    },
    {
      id: 'storage-v2',
      label: 'Storage V2',
      route: STORAGE_V2_PATH,
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Source-agnostic storage preview',
      badge: 'preview',
    },
    {
      id: 'backups',
      label: 'Backups (Legacy)',
      route: BACKUPS_PATH,
      settingsRoute: '/settings/backups',
      tooltip: 'Legacy backups page',
    },
    {
      id: 'backups-v2',
      label: 'Backups V2',
      route: BACKUPS_V2_PATH,
      settingsRoute: '/settings/backups',
      tooltip: 'Source-agnostic backups preview',
      badge: 'preview',
    },
  ];
};
