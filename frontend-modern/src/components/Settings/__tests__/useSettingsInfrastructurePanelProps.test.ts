import { createRoot, createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { useSettingsInfrastructurePanelProps } from '../useSettingsInfrastructurePanelProps';

const createServiceResource = (
  type: 'pbs' | 'pmg',
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: `${type}-1`,
    type,
    name: `${type}-resource`,
    displayName: `${type.toUpperCase()} Main`,
    platformId: '',
    platformType: type === 'pbs' ? 'proxmox-pbs' : 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    platformData:
      type === 'pbs'
        ? { pbs: { hostname: 'pbs.local', instanceId: 'pbs-main' } }
        : { pmg: { hostname: 'pmg.local', instanceId: 'pmg-main' } },
    ...overrides,
  }) as Resource;

const mountHook = (resources: Resource[]) => {
  let dispose = () => {};
  let hookState!: ReturnType<typeof useSettingsInfrastructurePanelProps>;

  createRoot((d) => {
    dispose = d;
    const [_showNodeModal, setShowNodeModal] = createSignal(false);
    const [_editingNode, setEditingNode] = createSignal(null);
    const [_currentNodeType, setCurrentNodeType] = createSignal<'pve' | 'pbs' | 'pmg'>('pve');
    const [_modalResetKey, setModalResetKey] = createSignal(0);

    hookState = useSettingsInfrastructurePanelProps({
      selectedAgent: () => 'pve',
      onSelectAgent: () => {},
      resources: () => resources,
      discoverySettings: {
        discoveryEnabled: () => false,
        discoveryMode: () => 'auto',
        savingDiscoverySettings: () => false,
      } as Parameters<typeof useSettingsInfrastructurePanelProps>[0]['discoverySettings'],
      systemSettings: {
        envOverrides: () => ({}),
        temperatureMonitoringEnabled: () => true,
        temperatureMonitoringLocked: () => false,
        savingTemperatureSetting: () => false,
        handleTemperatureMonitoringChange: async () => {},
        disableDockerUpdateActions: () => false,
        disableDockerUpdateActionsLocked: () => false,
        savingDockerUpdateActions: () => false,
        handleDisableDockerUpdateActionsChange: async () => {},
      } as Parameters<typeof useSettingsInfrastructurePanelProps>[0]['systemSettings'],
      infrastructureSettings: {
        initialLoadComplete: () => true,
        discoveryScanStatus: () => ({ scanning: false }),
        discoveredNodes: () => [],
        pveNodes: () => [],
        pbsNodes: () => [],
        pmgNodes: () => [],
        triggerDiscoveryScan: async () => {},
        loadDiscoveredNodes: async () => {},
        handleDiscoveryEnabledChange: async () => true,
        testNodeConnection: () => {},
        requestDeleteNode: () => {},
        refreshClusterNodes: async () => {},
        setShowNodeModal,
        editingNode: () => _editingNode(),
        setEditingNode,
        setCurrentNodeType,
        modalResetKey: () => _modalResetKey(),
        setModalResetKey,
        isNodeModalVisible: () => false,
        resolveTemperatureMonitoringEnabled: () => true,
        handleNodeTemperatureMonitoringChange: async () => {},
        saveNode: async () => {},
        showDeleteNodeModal: () => false,
        cancelDeleteNode: () => {},
        deleteNode: async () => {},
        deleteNodeLoading: () => false,
        nodePendingDeleteLabel: () => '',
        nodePendingDeleteHost: () => '',
        nodePendingDeleteType: () => '',
        nodePendingDeleteTypeLabel: () => '',
      } as Parameters<typeof useSettingsInfrastructurePanelProps>[0]['infrastructureSettings'],
      securityStatus: () => null,
    });
  });

  return { dispose, hookState };
};

describe('useSettingsInfrastructurePanelProps', () => {
  it('keeps governed PBS and PMG entries on local operator identity', () => {
    const { hookState, dispose } = mountHook([
      createServiceResource('pbs', {
        displayName: 'PBS Main',
        policy: {
          display: {
            mode: 'governed',
            summary: 'backup server resource; status online; sources pbs',
          },
        },
      }),
      createServiceResource('pmg', {
        displayName: 'PMG Main',
        policy: {
          display: {
            mode: 'governed',
            summary: 'mail gateway resource; status online; sources pmg',
          },
        },
      }),
    ]);

    const panelProps = hookState.getInfrastructurePanelProps();

    expect(panelProps.pbsInstances()).toEqual([
      expect.objectContaining({ name: 'PBS Main' }),
    ]);
    expect(panelProps.pmgInstances()).toEqual([
      expect.objectContaining({ name: 'PMG Main' }),
    ]);

    dispose();
  });
});
