export type StorageBackupsDefaultMode =
  | 'legacy-default'
  | 'backups-v2-default'
  | 'storage-v2-default'
  | 'v2-default';

export type StorageBackupsPrimaryView = 'legacy' | 'v2';

export type StorageBackupsRoutingPlan = {
  mode: StorageBackupsDefaultMode;
  showV2DefaultTabs: boolean;
  primaryStorageView: StorageBackupsPrimaryView;
  primaryBackupsView: StorageBackupsPrimaryView;
};

export const shouldRedirectStorageV2Route = (plan: StorageBackupsRoutingPlan): boolean =>
  plan.primaryStorageView === 'v2';

export const shouldRedirectBackupsV2Route = (plan: StorageBackupsRoutingPlan): boolean =>
  plan.primaryBackupsView === 'v2';

export const resolveStorageBackupsDefaultMode = (
  storageBackupsV2Enabled: boolean,
  storageV2RolledBack: boolean = false,
  backupsV2RolledBack: boolean = false,
): StorageBackupsDefaultMode => {
  if (storageBackupsV2Enabled) return 'v2-default';
  if (storageV2RolledBack && backupsV2RolledBack) return 'legacy-default';
  if (storageV2RolledBack) return 'backups-v2-default';
  if (backupsV2RolledBack) return 'storage-v2-default';
  return 'v2-default';
};

export const buildStorageBackupsRoutingPlan = (
  mode: StorageBackupsDefaultMode,
): StorageBackupsRoutingPlan => ({
  mode,
  showV2DefaultTabs: mode === 'v2-default',
  primaryStorageView: mode === 'legacy-default' || mode === 'backups-v2-default' ? 'legacy' : 'v2',
  primaryBackupsView: mode === 'legacy-default' || mode === 'storage-v2-default' ? 'legacy' : 'v2',
});

export const resolveStorageBackupsRoutingPlan = (
  storageBackupsV2Enabled: boolean,
  storageV2RolledBack: boolean = false,
  backupsV2RolledBack: boolean = false,
): StorageBackupsRoutingPlan =>
  buildStorageBackupsRoutingPlan(
    resolveStorageBackupsDefaultMode(
      storageBackupsV2Enabled,
      storageV2RolledBack,
      backupsV2RolledBack,
    ),
  );
