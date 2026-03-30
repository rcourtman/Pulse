import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';

import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import { useThresholdsTableState } from '../useThresholdsTableState';

let mockPathname = '/alerts/thresholds/systems';
const navigateSpy = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ pathname: mockPathname }),
  useNavigate: () => navigateSpy,
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ activationState: () => 'active' }),
}));

vi.mock('@/components/Alerts/Thresholds/hooks/useCollapsedSections', () => ({
  useCollapsedSections: () => ({
    collapseAll: vi.fn(),
    expandAll: vi.fn(),
    isCollapsed: () => false,
    toggleSection: vi.fn(),
  }),
}));

vi.mock('../useThresholdsData', () => ({
  useThresholdsData: () => ({
    agentDisksGroupedByAgent: () => ({}),
    agentDisksWithOverrides: () => [],
    agentsWithOverrides: () => [],
    dockerContainersFlat: () => [],
    dockerContainersGroupedByHost: () => ({}),
    dockerHostGroupMeta: () => ({}),
    dockerHostsWithOverrides: () => [],
    guestsFlat: () => [],
    guestsGroupedByNode: () => ({}),
    guestGroupHeaderMeta: () => ({}),
    nodesWithOverrides: () => [],
    pbsServersWithOverrides: () => [],
    pmgGlobalDefaults: () => ({}),
    pmgServersWithOverrides: () => [],
    storageGroupedByNode: () => ({}),
    storageWithOverrides: () => [],
    totalDockerContainers: () => 0,
  }),
}));

vi.mock('../useThresholdsRecoveryDefaultsState', () => ({
  useThresholdsRecoveryDefaultsState: () => ({
    backupDefaultsRecord: () => ({}),
    backupFactoryConfig: () => ({ enabled: false }),
    backupFactoryDefaultsRecord: () => ({}),
    backupOverridesCount: () => 0,
    sanitizeBackupConfig: <T,>(value: T) => value,
    sanitizeSnapshotConfig: <T,>(value: T) => value,
    snapshotDefaultsRecord: () => ({}),
    snapshotFactoryConfig: () => ({ enabled: false }),
    snapshotFactoryDefaultsRecord: () => ({}),
    snapshotOverridesCount: () => 0,
  }),
}));

const buildProps = (): ThresholdsTableProps =>
  ({
    agentDefaults: {},
    agents: [],
    allGuests: () => [],
    allResources: [],
    backupDefaults: () => ({ criticalDays: 7, enabled: false, warningDays: 3 }),
    containerRuntimes: [],
    disableAllAgents: () => false,
    disableAllAgentsOffline: () => false,
    disableAllDockerContainers: () => false,
    disableAllDockerHosts: () => false,
    disableAllDockerHostsOffline: () => false,
    disableAllDockerServices: () => false,
    disableAllGuests: () => false,
    disableAllNodes: () => false,
    disableAllPBS: () => false,
    disableAllPMG: () => false,
    disableAllPMGOffline: () => false,
    disableAllStorage: () => false,
    dockerDefaults: {
      cpu: 80,
      disk: 90,
      memory: 85,
      memoryCriticalPct: 95,
      memoryWarnPct: 80,
      restartCount: 3,
      restartWindow: 10,
      serviceCriticalGapPercent: 15,
      serviceWarnGapPercent: 5,
    },
    dockerDisableConnectivity: () => false,
    dockerHosts: [],
    dockerIgnoredPrefixes: () => [],
    dockerPoweredOffSeverity: () => 'warning',
    factoryAgentDefaults: {},
    factoryDockerDefaults: {},
    factoryGuestDefaults: {},
    factoryNodeDefaults: {},
    factoryPBSDefaults: {},
    factoryStorageDefault: 85,
    guestDefaults: {},
    guestDisableConnectivity: () => false,
    guestPoweredOffSeverity: () => 'warning',
    guestTagBlacklist: () => [],
    guestTagWhitelist: () => [],
    ignoredGuestPrefixes: () => [],
    metricTimeThresholds: () => ({}),
    nodeDefaults: {},
    nodes: [],
    overrides: () => [],
    pmgInstances: [],
    pmgThresholds: () => ({}) as any,
    pbsDefaults: {},
    pbsInstances: [],
    rawOverridesConfig: () => ({}),
    setAgentDefaults: vi.fn(),
    setDisableAllAgents: vi.fn(),
    setDisableAllAgentsOffline: vi.fn(),
    setDisableAllDockerContainers: vi.fn(),
    setDisableAllDockerHosts: vi.fn(),
    setDisableAllDockerHostsOffline: vi.fn(),
    setDisableAllDockerServices: vi.fn(),
    setDisableAllGuests: vi.fn(),
    setDisableAllNodes: vi.fn(),
    setDisableAllPBS: vi.fn(),
    setDisableAllPMG: vi.fn(),
    setDisableAllPMGOffline: vi.fn(),
    setDisableAllStorage: vi.fn(),
    setDockerDefaults: vi.fn(),
    setDockerDisableConnectivity: vi.fn(),
    setDockerIgnoredPrefixes: vi.fn(),
    setDockerPoweredOffSeverity: vi.fn(),
    setGuestDefaults: vi.fn(),
    setGuestDisableConnectivity: vi.fn(),
    setGuestPoweredOffSeverity: vi.fn(),
    setGuestTagBlacklist: vi.fn(),
    setGuestTagWhitelist: vi.fn(),
    setHasUnsavedChanges: vi.fn(),
    setIgnoredGuestPrefixes: vi.fn(),
    setMetricTimeThresholds: vi.fn(),
    setNodeDefaults: vi.fn(),
    setOverrides: vi.fn(),
    setPMGThresholds: vi.fn(),
    setRawOverridesConfig: vi.fn(),
    setSnapshotDefaults: vi.fn(),
    setStorageDefault: vi.fn(),
    snapshotDefaults: () => ({
      criticalDays: 7,
      criticalSizeGiB: 16,
      enabled: false,
      warningDays: 3,
      warningSizeGiB: 8,
    }),
    storage: [],
    storageDefault: () => 85,
    timeThresholds: () => ({ agent: 0, guest: 0, node: 0, pbs: 0, storage: 0 }),
  }) as unknown as ThresholdsTableProps;

beforeEach(() => {
  mockPathname = '/alerts/thresholds/systems';
  navigateSpy.mockReset();
  localStorage.clear();
});

afterEach(() => {
  cleanup();
});

describe('useThresholdsTableState', () => {
  it('owns thresholds route state, help-banner persistence, and tab navigation', () => {
    let captured: ReturnType<typeof useThresholdsTableState> | undefined;

    const Harness = () => {
      captured = useThresholdsTableState(buildProps());
      return null;
    };

    render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(captured!.activeTab()).toBe('systems');
    expect(captured!.hasDockerSpecificControls()).toBe(false);

    captured!.dismissHelpBanner();
    expect(captured!.helpBannerDismissed()).toBe(true);
    expect(localStorage.getItem('pulse-thresholds-help-dismissed')).toBe('true');

    captured!.handleTabClick('docker');
    expect(navigateSpy).toHaveBeenCalledWith('/alerts/thresholds/containers');
  });
});
