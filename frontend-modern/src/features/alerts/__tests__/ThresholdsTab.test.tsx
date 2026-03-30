import { render, cleanup } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ThresholdsTab } from '../tabs/ThresholdsTab';
import type { ThresholdsTabProps } from '../thresholds/thresholdsTabModel';

const captureThresholdsTableProps = vi.fn();

vi.mock('@/components/Alerts/ThresholdsTable', () => ({
  ThresholdsTable: (props: unknown) => {
    captureThresholdsTableProps(props);
    return null;
  },
}));

const buildProps = (): ThresholdsTabProps =>
  ({
    overrides: () => [],
    setOverrides: vi.fn(),
    rawOverridesConfig: () => ({}),
    setRawOverridesConfig: vi.fn(),
    allGuests: () => [],
    nodes: [],
    agents: [],
    storage: [],
    containerRuntimes: [],
    dockerHosts: [],
    allResources: [],
    pbsInstances: [],
    pmgInstances: [],
    pmgThresholds: () => ({}) as any,
    setPMGThresholds: vi.fn(),
    guestDefaults: () => ({}),
    setGuestDefaults: vi.fn(),
    guestDisableConnectivity: () => false,
    setGuestDisableConnectivity: vi.fn(),
    guestPoweredOffSeverity: () => 'warning' as const,
    setGuestPoweredOffSeverity: vi.fn(),
    nodeDefaults: () => ({}),
    setNodeDefaults: vi.fn(),
    pbsDefaults: () => ({}),
    setPBSDefaults: vi.fn(),
    agentDefaults: () => ({ cpu: 80 }),
    setAgentDefaults: vi.fn(),
    dockerDefaults: () => ({
      cpu: 80,
      memory: 85,
      disk: 85,
      restartCount: 3,
      restartWindow: 300,
      memoryWarnPct: 90,
      memoryCriticalPct: 95,
      serviceWarnGapPercent: 10,
      serviceCriticalGapPercent: 50,
    }),
    dockerDisableConnectivity: () => false,
    setDockerDisableConnectivity: vi.fn(),
    dockerPoweredOffSeverity: () => 'warning' as const,
    setDockerPoweredOffSeverity: vi.fn(),
    setDockerDefaults: vi.fn(),
    dockerIgnoredPrefixes: () => [],
    setDockerIgnoredPrefixes: vi.fn(),
    ignoredGuestPrefixes: () => [],
    setIgnoredGuestPrefixes: vi.fn(),
    guestTagWhitelist: () => [],
    setGuestTagWhitelist: vi.fn(),
    guestTagBlacklist: () => [],
    setGuestTagBlacklist: vi.fn(),
    storageDefault: () => 85,
    setStorageDefault: vi.fn(),
    resetGuestDefaults: vi.fn(),
    resetNodeDefaults: vi.fn(),
    resetPBSDefaults: vi.fn(),
    resetAgentDefaults: vi.fn(),
    resetDockerDefaults: vi.fn(),
    resetDockerIgnoredPrefixes: vi.fn(),
    resetStorageDefault: vi.fn(),
    factoryGuestDefaults: {},
    factoryNodeDefaults: {},
    factoryPBSDefaults: {},
    factoryAgentDefaults: { cpu: 80 },
    factoryDockerDefaults: {
      cpu: 80,
      memory: 85,
      disk: 85,
      restartCount: 3,
      restartWindow: 300,
      memoryWarnPct: 90,
      memoryCriticalPct: 95,
      serviceWarnGapPercent: 10,
      serviceCriticalGapPercent: 50,
    },
    factoryStorageDefault: 85,
    timeThresholds: () => ({ guest: 5, node: 5, storage: 5, pbs: 5, agent: 5 }),
    metricTimeThresholds: () => ({}),
    setMetricTimeThresholds: vi.fn(),
    snapshotDefaults: () => ({
      enabled: false,
      warningDays: 30,
      criticalDays: 45,
      warningSizeGiB: 0,
      criticalSizeGiB: 0,
    }),
    setSnapshotDefaults: vi.fn(),
    snapshotFactoryDefaults: {
      enabled: false,
      warningDays: 30,
      criticalDays: 45,
      warningSizeGiB: 0,
      criticalSizeGiB: 0,
    },
    resetSnapshotDefaults: vi.fn(),
    backupDefaults: () => ({
      enabled: false,
      warningDays: 7,
      criticalDays: 14,
      freshHours: 24,
      staleHours: 72,
      alertOrphaned: true,
      ignoreVMIDs: [],
    }),
    setBackupDefaults: vi.fn(),
    backupFactoryDefaults: {
      enabled: false,
      warningDays: 7,
      criticalDays: 14,
      freshHours: 24,
      staleHours: 72,
      alertOrphaned: true,
      ignoreVMIDs: [],
    },
    resetBackupDefaults: vi.fn(),
    setHasUnsavedChanges: vi.fn(),
    activeAlerts: {},
    removeAlerts: vi.fn(),
    disableAllNodes: () => false,
    setDisableAllNodes: vi.fn(),
    disableAllGuests: () => false,
    setDisableAllGuests: vi.fn(),
    disableAllAgents: () => false,
    setDisableAllAgents: vi.fn(),
    disableAllStorage: () => false,
    setDisableAllStorage: vi.fn(),
    disableAllPBS: () => false,
    setDisableAllPBS: vi.fn(),
    disableAllPMG: () => false,
    setDisableAllPMG: vi.fn(),
    disableAllDockerHosts: () => false,
    setDisableAllDockerHosts: vi.fn(),
    disableAllDockerServices: () => false,
    setDisableAllDockerServices: vi.fn(),
    disableAllDockerContainers: () => false,
    setDisableAllDockerContainers: vi.fn(),
    disableAllNodesOffline: () => false,
    setDisableAllNodesOffline: vi.fn(),
    disableAllGuestsOffline: () => false,
    setDisableAllGuestsOffline: vi.fn(),
    disableAllAgentsOffline: () => false,
    setDisableAllAgentsOffline: vi.fn(),
    disableAllPBSOffline: () => false,
    setDisableAllPBSOffline: vi.fn(),
    disableAllPMGOffline: () => false,
    setDisableAllPMGOffline: vi.fn(),
    disableAllDockerHostsOffline: () => false,
    setDisableAllDockerHostsOffline: vi.fn(),
  }) as unknown as ThresholdsTabProps;

describe('ThresholdsTab', () => {
  beforeEach(() => {
    captureThresholdsTableProps.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('passes resolved threshold props through the tab adapter without dropping function fields', () => {
    render(() => <ThresholdsTab {...buildProps()} />);

    expect(captureThresholdsTableProps).toHaveBeenCalledTimes(1);
    const props = captureThresholdsTableProps.mock.calls[0][0] as Record<string, unknown>;

    expect(typeof props.dockerIgnoredPrefixes).toBe('function');
    expect((props.dockerIgnoredPrefixes as () => string[])()).toEqual([]);
    expect(props.dockerHosts).toEqual([]);
    expect(props.containerRuntimes).toEqual([]);
    expect(typeof props.guestDefaults).toBe('object');
    expect(typeof props.dockerDefaults).toBe('object');
  });
});
