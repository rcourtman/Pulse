import {
  BACKUPS_V2_PATH,
  STORAGE_V2_PATH,
  buildBackupsPath,
  buildStoragePath,
} from './resourceLinks';
import type { StorageBackupsRoutingPlan } from './storageBackupsMode';

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
  planOrShowV2: StorageBackupsRoutingPlan | boolean,
): StorageBackupsTabSpec[] => {
  // Normalize: accept boolean for backward compat (true → both v2, false → both legacy)
  const plan: StorageBackupsRoutingPlan =
    typeof planOrShowV2 === 'boolean'
      ? {
          mode: planOrShowV2 ? 'v2-default' : 'legacy-default',
          showV2DefaultTabs: planOrShowV2,
          primaryStorageView: planOrShowV2 ? 'v2' : 'legacy',
          primaryBackupsView: planOrShowV2 ? 'v2' : 'legacy',
        }
      : planOrShowV2;

  const tabs: StorageBackupsTabSpec[] = [];

  // Storage tabs
  if (plan.primaryStorageView === 'v2') {
    tabs.push({
      id: 'storage',
      label: 'Storage',
      route: STORAGE_PATH,
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Source-agnostic storage',
    });
  } else {
    tabs.push({
      id: 'storage',
      label: 'Storage (Legacy)',
      route: STORAGE_PATH,
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Legacy storage page',
    });
    tabs.push({
      id: 'storage-v2',
      label: 'Storage',
      route: STORAGE_V2_PATH,
      settingsRoute: '/settings/infrastructure/pbs',
      tooltip: 'Source-agnostic storage',
    });
  }

  // Backups tabs
  if (plan.primaryBackupsView === 'v2') {
    tabs.push({
      id: 'backups',
      label: 'Backups',
      route: BACKUPS_PATH,
      settingsRoute: '/settings/backups',
      tooltip: 'Source-agnostic backups',
    });
  } else {
    tabs.push({
      id: 'backups',
      label: 'Backups (Legacy)',
      route: BACKUPS_PATH,
      settingsRoute: '/settings/backups',
      tooltip: 'Legacy backups page',
    });
    tabs.push({
      id: 'backups-v2',
      label: 'Backups V2',
      route: BACKUPS_V2_PATH,
      settingsRoute: '/settings/backups',
      tooltip: 'Source-agnostic backups preview',
      badge: 'preview',
    });
  }

  return tabs;
};
