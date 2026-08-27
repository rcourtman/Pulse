import { render, cleanup, fireEvent, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ThresholdsTab } from '../tabs/ThresholdsTab';
import type { ThresholdsTabProps } from '../thresholds/thresholdsTabModel';

const captureThresholdsTableProps = vi.fn();
const captureIntentPolicyProps = vi.fn();

vi.mock('@/components/Alerts/ThresholdsTable', () => ({
  ThresholdsTable: (props: unknown) => {
    captureThresholdsTableProps(props);
    return null;
  },
}));

vi.mock('../AlertIntentPolicyPanel', () => ({
  AlertIntentPolicyPanel: (props: unknown) => {
    captureIntentPolicyProps(props);
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
    kubernetesDefaults: () => ({
      cpu: 80,
      memory: 85,
      disk: 90,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    }),
    setKubernetesDefaults: vi.fn(),
    trueNASDefaults: () => ({
      cpu: 80,
      memory: 85,
      disk: 85,
      usage: 85,
      temperature: 80,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    }),
    setTrueNASDefaults: vi.fn(),
    trueNASDiskDefaults: () => ({ temperature: 55 }),
    setTrueNASDiskDefaults: vi.fn(),
    vmwareDefaults: () => ({
      cpu: 80,
      memory: 85,
      disk: 90,
      usage: 85,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    }),
    setVMwareDefaults: vi.fn(),
    agentDefaults: () => ({
      cpu: 80,
      smartHealthFailure: 1,
      smartPending: 4,
      smartCrcErrorDelta: 2,
      smartLifeWarning: 15,
      smartSpareCritical: 0,
    }),
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
    resetKubernetesDefaults: vi.fn(),
    resetTrueNASDefaults: vi.fn(),
    resetTrueNASDiskDefaults: vi.fn(),
    resetVMwareDefaults: vi.fn(),
    resetAgentDefaults: vi.fn(),
    resetDockerDefaults: vi.fn(),
    resetDockerIgnoredPrefixes: vi.fn(),
    resetStorageDefault: vi.fn(),
    factoryGuestDefaults: {},
    factoryNodeDefaults: {},
    factoryPBSDefaults: {},
    factoryKubernetesDefaults: {
      cpu: 80,
      memory: 85,
      disk: 90,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    },
    factoryTrueNASDefaults: {
      cpu: 80,
      memory: 85,
      disk: 85,
      usage: 85,
      temperature: 80,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    },
    factoryTrueNASDiskDefaults: { temperature: 55 },
    factoryVMwareDefaults: {
      cpu: 80,
      memory: 85,
      disk: 90,
      usage: 85,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    },
    factoryAgentDefaults: {
      cpu: 80,
      smartHealthFailure: 1,
      smartPending: 1,
      smartCrcErrorDelta: 1,
      smartLifeWarning: 10,
      smartSpareCritical: 10,
    },
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
    timeThresholds: () => ({
      guest: 5,
      node: 5,
      storage: 5,
      pbs: 5,
      agent: 5,
      'k8s-cluster': 5,
      'k8s-node': 5,
      'k8s-deployment': 5,
      'k8s-namespace': 5,
      pod: 5,
      'truenas-system': 5,
      'truenas-pool': 5,
      'truenas-dataset': 5,
      'truenas-disk': 5,
      'vmware-host': 5,
      'vmware-vm': 5,
      'vmware-datastore': 5,
      'vmware-network': 5,
    }),
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
    disableAllKubernetes: () => false,
    setDisableAllKubernetes: vi.fn(),
    disableAllTrueNAS: () => false,
    setDisableAllTrueNAS: vi.fn(),
    disableAllVMware: () => false,
    setDisableAllVMware: vi.fn(),
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
    captureIntentPolicyProps.mockReset();
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
    expect(props.agentDefaults).toMatchObject({
      smartHealthFailure: 1,
      smartPending: 4,
      smartCrcErrorDelta: 2,
      smartLifeWarning: 15,
      smartSpareCritical: 0,
    });
    expect(props.factoryAgentDefaults).toMatchObject({
      smartPending: 1,
      smartLifeWarning: 10,
      smartSpareCritical: 10,
    });
  });

  it('routes a resource-row delay action into the canonical intent policy panel', async () => {
    render(() => <ThresholdsTab {...buildProps()} />);

    const tableProps = captureThresholdsTableProps.mock.calls[0][0] as {
      onConfigureResourceIntent: (resourceId: string, signal: string) => void;
    };
    tableProps.onConfigureResourceIntent('vm-canonical', 'metric.memory');

    await waitFor(() => {
      const panelProps = captureIntentPolicyProps.mock.calls.at(-1)?.[0] as {
        selectionTarget?: {
          resourceId: string;
          signal: string;
          requestId: number;
        };
      };
      expect(panelProps.selectionTarget).toEqual({
        resourceId: 'vm-canonical',
        signal: 'metric.memory',
        requestId: 1,
      });
    });
  });

  it('configures the canonical workload fallback and shows effective inheritance', () => {
    const props = buildProps();
    const setMetricEvaluationWindows = vi.fn();
    const setHasUnsavedChanges = vi.fn();
    props.metricEvaluationWindows = () => ({
      all: { cpu: 300 },
      guest: { cpu: 900 },
    });
    props.setMetricEvaluationWindows = setMetricEvaluationWindows;
    props.setHasUnsavedChanges = setHasUnsavedChanges;

    render(() => <ThresholdsTab {...props} />);

    const workloadWindow = screen.getAllByLabelText(
      'All workloads CPU evaluation window',
    )[0] as HTMLSelectElement;
    const virtualMachineWindow = screen.getAllByLabelText(
      'Virtual machines CPU evaluation window',
    )[0] as HTMLSelectElement;
    expect(workloadWindow.value).toBe('900');
    expect(virtualMachineWindow.options[0].textContent).toBe('Inherit (15 minutes)');

    fireEvent.input(workloadWindow, { target: { value: '60' } });

    const update = setMetricEvaluationWindows.mock.calls[0][0] as (
      previous: Record<string, Record<string, number>>,
    ) => Record<string, Record<string, number>>;
    expect(update({ all: { cpu: 300 }, guest: { cpu: 900 } })).toEqual({
      all: { cpu: 300 },
      guest: { cpu: 60 },
    });
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
  });
});
