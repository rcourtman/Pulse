import { createMemo } from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { useResources } from '@/hooks/useResources';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { useAlertsActivation } from '@/stores/alertsActivation';

const STORAGE_PAGE_RESOURCES_QUERY = 'type=storage';

export const useStoragePageResources = () => {
  const { state, activeAlerts, connected, initialDataReceived, reconnecting, reconnect } =
    useWebSocket();
  const { byType } = useResources();
  // The storage adapter consumes canonical storage resources only. Recovery
  // needs workload inventory too, but sharing its broad query here made this
  // tab hydrate every guest in a large estate before doing useful work.
  const storageResources = useUnifiedResources({
    query: STORAGE_PAGE_RESOURCES_QUERY,
    cacheKey: 'storage-page',
  });
  const alertsActivation = useAlertsActivation();

  const nodes = createMemo(() => byType('agent'));
  const physicalDisks = createMemo(() => byType('physical_disk'));
  const cephResources = createMemo(() => byType('ceph'));
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
