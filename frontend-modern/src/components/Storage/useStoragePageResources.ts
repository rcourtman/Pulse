import { createMemo, type Accessor } from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource, ResourceType } from '@/types/resource';
import { asStorageAlertRecord } from '@/features/storageBackups/storageAlertState';
import type { Alert } from '@/types/api';
import {
  getPhysicalDiskAlertResourceIds,
  getPhysicalDiskAlerts,
  indexPhysicalDiskOwners,
} from './physicalDiskAlertOwner';

const STORAGE_PAGE_RESOURCES_QUERY = 'type=agent,pbs,storage,physical_disk,ceph';

export type StoragePageResourceSource = {
  resources: Accessor<Resource[]>;
  loading: Accessor<boolean>;
  error: Accessor<unknown>;
  refetch: () => Promise<Resource[]>;
  mutate: (value: Resource[] | ((previous: Resource[]) => Resource[])) => Resource[];
};

type UseStoragePageResourcesOptions = {
  resourceSource?: StoragePageResourceSource;
};

export const useStoragePageResources = (options: UseStoragePageResourcesOptions = {}) => {
  const { state, activeAlerts, connected, initialDataReceived, reconnecting, reconnect } =
    useWebSocket();
  // Platform shells can inject their already-prefetched route source. The
  // standalone page owns the same bounded resource family itself. Keeping one
  // source avoids an all-estate subscription and a duplicate Storage query.
  const resourceSource =
    options.resourceSource ??
    useUnifiedResources({
      query: STORAGE_PAGE_RESOURCES_QUERY,
      cacheKey: 'storage-page',
    });
  const alertsActivation = useAlertsActivation();

  const byType = (type: ResourceType): Resource[] =>
    resourceSource.resources().filter((resource) => resource.type === type);
  const nodes = createMemo(() => byType('agent'));
  const physicalDisks = createMemo(() => byType('physical_disk'));
  const cephResources = createMemo(() => byType('ceph'));
  const storageResourceRows = createMemo(() => byType('storage'));
  const storageResources: StoragePageResourceSource = {
    ...resourceSource,
    resources: storageResourceRows,
  };
  // A disk's heat is judged under the alert overrides of the machine whose
  // agent reports it; this page loads those machines and pools with the disks.
  const physicalDiskOwners = createMemo(() => indexPhysicalDiskOwners(resourceSource.resources()));
  const getDiskAlertResourceIds = (disk: Resource): string[] =>
    getPhysicalDiskAlertResourceIds(disk, physicalDiskOwners());
  const alertsEnabled = alertsActivation.detectionEnabled;
  // A disk drawer lists the disk's own open alerts: those on its unified
  // identity, its Proxmox disk key and its agent's temperature key.
  const getDiskAlerts = (disk: Resource): Alert[] =>
    getPhysicalDiskAlerts(
      disk,
      physicalDiskOwners(),
      asStorageAlertRecord(activeAlerts),
      alertsEnabled(),
    );

  return {
    state,
    activeAlerts,
    connected,
    initialDataReceived,
    reconnecting,
    reconnect,
    nodes,
    physicalDisks,
    getDiskAlertResourceIds,
    getDiskAlerts,
    cephResources,
    storageResources,
    alertsEnabled,
  };
};
