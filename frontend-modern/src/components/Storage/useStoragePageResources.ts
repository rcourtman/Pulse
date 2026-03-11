import { createMemo } from 'solid-js';
import { useWebSocket } from '@/App';
import { useResources } from '@/hooks/useResources';
import { useStorageRecoveryResources } from '@/hooks/useUnifiedResources';
import { useAlertsActivation } from '@/stores/alertsActivation';

export const useStoragePageResources = () => {
  const { state, activeAlerts, connected, initialDataReceived, reconnecting, reconnect } =
    useWebSocket();
  const { byType } = useResources();
  const storageRecoveryResources = useStorageRecoveryResources();
  const alertsActivation = useAlertsActivation();

  const nodes = createMemo(() => byType('agent'));
  const physicalDisks = createMemo(() => byType('physical_disk'));
  const cephResources = createMemo(() => byType('ceph'));
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

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
    storageRecoveryResources,
    alertsEnabled,
  };
};
