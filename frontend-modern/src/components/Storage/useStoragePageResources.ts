import { createMemo, type Accessor } from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource, ResourceType } from '@/types/resource';

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
  const alertsEnabled = alertsActivation.detectionEnabled;

  return {
    state,
    activeAlerts,
    connected,
    initialDataReceived,
    reconnecting,
    reconnect,
    nodes,
    physicalDisks,
    cephResources,
    storageResources,
    alertsEnabled,
  };
};
