import { createSignal } from 'solid-js';

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';

import type { BackupAlertConfig, PMGThresholdDefaults, SnapshotAlertConfig } from '@/types/alerts';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { Resource as TableResource } from '@/features/alerts/thresholds/tableTypes';
import { useThresholdsTableState } from '../useThresholdsTableState';

// --- module-level state read lazily inside the vi.mock factories -----------------

let mockPathname = '/alerts/thresholds/proxmox';
let mockHash = '';
let mockAlertsEnabled = true;
let mockNodes: TableResource[] = [];

// Modeled as a real Solid signal so the hook's dockerIgnored sync `createEffect`
// reconciles against it exactly as it would against a live store.
const [dockerIgnoredStore, setDockerIgnoredStore] = createSignal<string[]>([]);
let metricTimeState: Record<string, Record<string, number>> = {};
let pmgThresholdsState: PMGThresholdDefaults = {};
let snapshotPrev: SnapshotAlertConfig = { enabled: false, warningDays: 3, criticalDays: 7 };
let backupPrev: BackupAlertConfig = {
  enabled: false,
  warningDays: 7,
  criticalDays: 14,
  freshHours: 24,
};

// Spies are (re)created inside buildProps so each test starts with a fresh call history.
let setHasUnsavedChangesSpy: ReturnType<typeof vi.fn>;
let setDockerIgnoredPrefixesSpy: ReturnType<typeof vi.fn>;
let resetDockerIgnoredPrefixesSpy: ReturnType<typeof vi.fn>;
let setSnapshotDefaultsSpy: ReturnType<typeof vi.fn>;
let setBackupDefaultsSpy: ReturnType<typeof vi.fn>;
let setPMGThresholdsSpy: ReturnType<typeof vi.fn>;
let setMetricTimeThresholdsSpy: ReturnType<typeof vi.fn>;

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ pathname: mockPathname, hash: mockHash }),
  useNavigate: () => vi.fn(),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ detectionEnabled: () => mockAlertsEnabled }),
}));

vi.mock('@/components/Alerts/Thresholds/hooks/useCollapsedSections', () => ({
  useCollapsedSections: () => ({
    collapseAll: vi.fn(),
    expandAll: vi.fn(),
    isCollapsed: () => false,
    setCollapsed: vi.fn(),
    toggleSection: vi.fn(),
  }),
}));

vi.mock('../useThresholdsData', () => ({
  useThresholdsData: () => ({
    agentDisksGroupedByAgent: () => ({}),
    agentGroupHeaderMeta: () => ({}),
    agentDisksWithOverrides: () => [],
    agentsWithOverrides: () => [],
    dockerContainersFlat: () => [],
    dockerContainersGroupedByHost: () => ({}),
    dockerHostGroupMeta: () => ({}),
    dockerHostsWithOverrides: () => [],
    guestsFlat: () => [],
    guestsGroupedByNode: () => ({}),
    guestGroupHeaderMeta: () => ({}),
    nodesWithOverrides: () => mockNodes,
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
    sanitizeBackupConfig: <T,>(value: T): T => value,
    sanitizeSnapshotConfig: <T,>(value: T): T => value,
    snapshotDefaultsRecord: () => ({}),
    snapshotFactoryConfig: () => ({ enabled: false }),
    snapshotFactoryDefaultsRecord: () => ({}),
    snapshotOverridesCount: () => 0,
  }),
}));

const buildProps = (opts: { withDockerReset?: boolean } = {}): ThresholdsTableProps => {
  resetDockerIgnoredPrefixesSpy = vi.fn(() => {
    setDockerIgnoredStore([]);
  });

  return {
    agentDefaults: {},
    agents: [],
    allGuests: () => [],
    allResources: [],
    backupDefaults: () => backupPrev,
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
      updateAlertDelayHours: 24,
    },
    dockerDisableConnectivity: () => false,
    dockerHosts: [],
    dockerIgnoredPrefixes: () => dockerIgnoredStore(),
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
    metricTimeThresholds: () => metricTimeState,
    nodeDefaults: {},
    nodes: [],
    overrides: () => [],
    pmgInstances: [],
    pmgThresholds: () => pmgThresholdsState,
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
    setBackupDefaults: (setBackupDefaultsSpy = vi.fn(
      (updater: BackupAlertConfig | ((prev: BackupAlertConfig) => BackupAlertConfig)) =>
        typeof updater === 'function' ? updater(backupPrev) : updater,
    )),
    setDockerDefaults: vi.fn(),
    setDockerDisableConnectivity: vi.fn(),
    setDockerIgnoredPrefixes: (setDockerIgnoredPrefixesSpy = vi.fn(
      (value: string[] | ((prev: string[]) => string[])) => {
        setDockerIgnoredStore(typeof value === 'function' ? value(dockerIgnoredStore()) : value);
      },
    )),
    setDockerPoweredOffSeverity: vi.fn(),
    setGuestDefaults: vi.fn(),
    setGuestDisableConnectivity: vi.fn(),
    setGuestPoweredOffSeverity: vi.fn(),
    setGuestTagBlacklist: vi.fn(),
    setGuestTagWhitelist: vi.fn(),
    setHasUnsavedChanges: (setHasUnsavedChangesSpy = vi.fn()),
    setIgnoredGuestPrefixes: vi.fn(),
    setMetricTimeThresholds: (setMetricTimeThresholdsSpy = vi.fn(
      (
        updater:
          | Record<string, Record<string, number>>
          | ((
              prev: Record<string, Record<string, number>>,
            ) => Record<string, Record<string, number>>),
      ) => {
        metricTimeState = typeof updater === 'function' ? updater(metricTimeState) : updater;
      },
    )),
    setNodeDefaults: vi.fn(),
    setOverrides: vi.fn(),
    setPMGThresholds: (setPMGThresholdsSpy = vi.fn(
      (updater: PMGThresholdDefaults | ((prev: PMGThresholdDefaults) => PMGThresholdDefaults)) => {
        pmgThresholdsState =
          typeof updater === 'function'
            ? (updater as (prev: PMGThresholdDefaults) => PMGThresholdDefaults)(pmgThresholdsState)
            : updater;
      },
    )),
    setRawOverridesConfig: vi.fn(),
    setSnapshotDefaults: (setSnapshotDefaultsSpy = vi.fn(
      (updater: SnapshotAlertConfig | ((prev: SnapshotAlertConfig) => SnapshotAlertConfig)) =>
        typeof updater === 'function' ? updater(snapshotPrev) : updater,
    )),
    setStorageDefault: vi.fn(),
    snapshotDefaults: () => snapshotPrev,
    storage: [],
    storageDefault: () => 85,
    timeThresholds: () => ({ agent: 0, guest: 0, node: 0, pbs: 0, storage: 0 }),
    ...(opts.withDockerReset ? { resetDockerIgnoredPrefixes: resetDockerIgnoredPrefixesSpy } : {}),
  } as unknown as ThresholdsTableProps;
};

const mount = (props: ThresholdsTableProps) => {
  let captured: ReturnType<typeof useThresholdsTableState> | undefined;
  const Harness = () => {
    captured = useThresholdsTableState(props);
    return null;
  };
  render(() => <Harness />);
  if (!captured) throw new Error('state was not captured');
  return captured;
};

beforeEach(() => {
  mockPathname = '/alerts/thresholds/proxmox';
  mockHash = '';
  mockAlertsEnabled = true;
  mockNodes = [];
  setDockerIgnoredStore([]);
  metricTimeState = {};
  pmgThresholdsState = {};
  snapshotPrev = { enabled: false, warningDays: 3, criticalDays: 7 };
  backupPrev = { enabled: false, warningDays: 7, criticalDays: 14, freshHours: 24 };
  localStorage.clear();
});

afterEach(() => {
  cleanup();
});

describe('useThresholdsTableState — branch coverage', () => {
  describe('handleDockerIgnoredChange', () => {
    it('writes the raw input, pushes the normalized prefixes, and flags unsaved changes', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.handleDockerIgnoredChange('  alpha \n\n beta \n');

      // The input signal is reconciled by the sync effect against the store the
      // handler just wrote; both sides ran real normalization to ['alpha','beta'].
      expect(captured.dockerIgnoredInput()).toBe('alpha\nbeta');
      expect(setDockerIgnoredPrefixesSpy).toHaveBeenCalledWith(['alpha', 'beta']);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });
  });

  describe('handleResetDockerIgnored', () => {
    it('uses the dedicated reset callback when one is provided (truthy arm)', () => {
      setDockerIgnoredStore(['stale-a', 'stale-b']);
      const props = buildProps({ withDockerReset: true });
      const captured = mount(props);

      captured.handleResetDockerIgnored();

      expect(resetDockerIgnoredPrefixesSpy).toHaveBeenCalledTimes(1);
      // The dedicated reset owns the store; setDockerIgnoredPrefixes must NOT also run.
      expect(setDockerIgnoredPrefixesSpy).not.toHaveBeenCalled();
      expect(captured.dockerIgnoredInput()).toBe('');
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('falls back to clearing via setDockerIgnoredPrefixes when no reset callback exists (falsy arm)', () => {
      const props = buildProps({ withDockerReset: false });
      const captured = mount(props);

      captured.handleResetDockerIgnored();

      expect(setDockerIgnoredPrefixesSpy).toHaveBeenCalledWith([]);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
      expect(captured.dockerIgnoredInput()).toBe('');
    });
  });

  describe('hasActiveAlert', () => {
    it('returns false up-front when alert detection is disabled', () => {
      mockAlertsEnabled = false;
      const props = buildProps();
      props.activeAlerts = { 'r-cpu': {} } as unknown as ThresholdsTableProps['activeAlerts'];
      const captured = mount(props);

      // Detection gate short-circuits before the lookup even happens.
      expect(captured.hasActiveAlert('r', 'cpu')).toBe(false);
    });

    it('returns false when no activeAlerts map is supplied', () => {
      const props = buildProps();
      // activeAlerts intentionally left undefined.
      const captured = mount(props);

      expect(captured.hasActiveAlert('r', 'cpu')).toBe(false);
    });

    it('distinguishes present vs absent "<id>-<metric>" keys when enabled', () => {
      const props = buildProps();
      props.activeAlerts = {
        'node-1-cpu': { id: 'x' },
      } as unknown as ThresholdsTableProps['activeAlerts'];
      const captured = mount(props);

      expect(captured.hasActiveAlert('node-1', 'cpu')).toBe(true);
      expect(captured.hasActiveAlert('node-1', 'memory')).toBe(false);
      expect(captured.hasActiveAlert('node-9', 'cpu')).toBe(false);
    });
  });

  describe('resourceMatchesOverrideFilter (via nodesWithOverrides memo + setOverrideFilter)', () => {
    it("'custom' filter keeps only resources carrying an override, connectivity-off, or severity flag", () => {
      mockNodes = [
        { id: 'a', name: 'a', hasOverride: true },
        { id: 'b', name: 'b', disableConnectivity: true },
        { id: 'c', name: 'c', poweredOffSeverity: 'critical' },
        { id: 'd', name: 'd' },
      ] as unknown as TableResource[];
      const props = buildProps();
      const captured = mount(props);

      captured.setOverrideFilter('custom');
      const visible = captured.nodesWithOverrides().map((r) => r.id);

      expect(visible).toEqual(['a', 'b', 'c']);
    });

    it("'disabled' filter keeps only disabled or connectivity-off resources", () => {
      mockNodes = [
        { id: 'a', name: 'a', disabled: true },
        { id: 'b', name: 'b', disableConnectivity: true },
        { id: 'c', name: 'c', hasOverride: true },
        { id: 'd', name: 'd' },
      ] as unknown as TableResource[];
      const props = buildProps();
      const captured = mount(props);

      captured.setOverrideFilter('disabled');
      const visible = captured.nodesWithOverrides().map((r) => r.id);

      // hasOverride alone is NOT enough for the 'disabled' filter.
      expect(visible).toEqual(['a', 'b']);
    });

    it("'all' filter (the default arm) keeps every resource regardless of flags", () => {
      mockNodes = [
        { id: 'a', name: 'a' },
        { id: 'b', name: 'b', disabled: true },
      ] as unknown as TableResource[];
      const props = buildProps();
      const captured = mount(props);

      captured.setOverrideFilter('all');
      expect(captured.nodesWithOverrides()).toHaveLength(2);
    });
  });

  describe('registerSection', () => {
    it('wires a newly registered section element so the #hash scroll effect scrolls it', () => {
      mockHash = '#pbs';
      const props = buildProps();
      const captured = mount(props);

      const element = { scrollIntoView: vi.fn() } as unknown as HTMLDivElement;
      // First registration: prev['pbs'] is undefined !== element -> new-entry arm.
      captured.registerSection('pbs')(element);

      expect(element.scrollIntoView).toHaveBeenCalledWith({ block: 'start' });
    });

    it('re-registering the same element is a no-op (same-reference arm)', () => {
      mockHash = '#pbs';
      const props = buildProps();
      const captured = mount(props);

      const element = { scrollIntoView: vi.fn() } as unknown as HTMLDivElement;
      captured.registerSection('pbs')(element);
      expect(element.scrollIntoView).toHaveBeenCalledTimes(1);

      // Same element -> registerSection returns prev unchanged (no spurious reactive update).
      captured.registerSection('pbs')(element);
      expect(element.scrollIntoView).toHaveBeenCalledTimes(1);
    });
  });

  describe('updateSnapshotDefaults', () => {
    it('applies a function updater against the previous snapshot config (function arm)', () => {
      const props = buildProps();
      const captured = mount(props);

      // The updater reads prev.criticalDays, proving it actually receives prev.
      captured.updateSnapshotDefaults((prev) => ({
        ...prev,
        warningDays: prev.criticalDays + 10,
      }));

      const result = setSnapshotDefaultsSpy.mock.results.at(-1)?.value as SnapshotAlertConfig;
      expect(result.warningDays).toBe(17); // 7 + 10
      expect(result.criticalDays).toBe(7); // preserved from prev
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('merges a partial object onto the previous config (object arm)', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.updateSnapshotDefaults({
        enabled: true,
        warningDays: 5,
        criticalDays: 9,
      });

      const result = setSnapshotDefaultsSpy.mock.results.at(-1)?.value as SnapshotAlertConfig;
      expect(result.warningDays).toBe(5);
      expect(result.criticalDays).toBe(9);
      expect(result.enabled).toBe(true);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });
  });

  describe('updateBackupDefaults', () => {
    it('applies a function updater against the previous backup config (function arm)', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.updateBackupDefaults((prev) => ({ ...prev, warningDays: prev.criticalDays * 2 }));

      const result = setBackupDefaultsSpy.mock.results.at(-1)?.value as BackupAlertConfig;
      expect(result.warningDays).toBe(28); // 14 * 2
      expect(result.criticalDays).toBe(14);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('merges a partial object onto the previous config (object arm)', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.updateBackupDefaults({ enabled: true, warningDays: 2, criticalDays: 4 });

      const result = setBackupDefaultsSpy.mock.results.at(-1)?.value as BackupAlertConfig;
      expect(result.warningDays).toBe(2);
      expect(result.criticalDays).toBe(4);
      expect(result.enabled).toBe(true);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });
  });

  describe('setPMGGlobalDefaults', () => {
    it('sanitizes a normalized value into the matching PMG column and flags unsaved changes (object arm + change)', () => {
      const props = buildProps();
      const captured = mount(props);

      // 'queue warn' is the normalized label for the queueTotalWarning column.
      captured.setPMGGlobalDefaults({ 'queue warn': 50 });

      expect(pmgThresholdsState.queueTotalWarning).toBe(50);
      expect(setPMGThresholdsSpy).toHaveBeenCalledTimes(1);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('accepts a function updater over the current normalized record (function arm)', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.setPMGGlobalDefaults((current) => ({ ...current, 'queue crit': 77 }));

      // 'queue crit' -> queueTotalCritical column; value is rounded.
      expect(pmgThresholdsState.queueTotalCritical).toBe(77);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('skips non-numeric / NaN entries without touching the column or flagging changes', () => {
      const props = buildProps();
      const captured = mount(props);

      // A string fails `typeof raw === 'number'`.
      captured.setPMGGlobalDefaults({ 'queue warn': 'nope' as unknown as number });
      // NaN passes typeof but fails `!Number.isNaN(raw)`.
      captured.setPMGGlobalDefaults({ 'queue crit': Number.NaN });

      expect(pmgThresholdsState.queueTotalWarning).toBeUndefined();
      expect(pmgThresholdsState.queueTotalCritical).toBeUndefined();
      expect(setHasUnsavedChangesSpy).not.toHaveBeenCalled();
    });

    it('does not flag unsaved changes when the sanitized value already matches the stored column', () => {
      // Seed the store so the incoming value matches -> "no change" arm.
      pmgThresholdsState = { queueTotalWarning: 50 };
      const props = buildProps();
      const captured = mount(props);

      captured.setPMGGlobalDefaults({ 'queue warn': 50 });

      expect(setHasUnsavedChangesSpy).not.toHaveBeenCalled();
    });

    it('clamps negative values to 0 and rounds decimals', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.setPMGGlobalDefaults({ 'queue warn': -7.4 });

      expect(pmgThresholdsState.queueTotalWarning).toBe(0);
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });
  });

  describe('hasSection', () => {
    it('reports a section present when its summary item survives the totals/tab filters', () => {
      const props = buildProps();
      props.nodes = [{ id: 'node-1' }] as unknown as ThresholdsTableProps['nodes'];
      const captured = mount(props);

      // 'nodes' item has total = props.nodes.length (1) and tab 'proxmox' (the active tab).
      expect(captured.hasSection('nodes')).toBe(true);
    });

    it('reports a section absent when there is no data to populate it', () => {
      const props = buildProps();
      const captured = mount(props);

      // No pbsInstances / no overrides -> pbs item filtered out of summaryItems.
      expect(captured.hasSection('pbs')).toBe(false);
    });
  });

  describe('startEditing / cancelEdit', () => {
    it('startEditing merges defaults with current thresholds and records the note when given', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.startEditing('res-1', { cpu: 90 }, { cpu: 80, memory: 70 }, 'a note');

      expect(captured.editingId()).toBe('res-1');
      // defaults spread first, currentThresholds win for shared keys; memory is retained from defaults.
      expect(captured.editingThresholds()).toEqual({ cpu: 90, memory: 70 });
      expect(captured.editingNote()).toBe('a note');
    });

    it('startEditing defaults the note to empty string when none is provided', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.startEditing('res-2', { cpu: 10 }, { cpu: 5 });

      expect(captured.editingNote()).toBe('');
    });

    it('cancelEdit clears the editing id, thresholds, and note', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.startEditing('res-3', { cpu: 90 }, { cpu: 80 }, 'temp');
      captured.cancelEdit();

      expect(captured.editingId()).toBeNull();
      expect(captured.editingThresholds()).toEqual({});
      expect(captured.editingNote()).toBe('');
    });
  });

  describe('handleBulkEdit / handleSaveBulkEdit', () => {
    it('handleBulkEdit stages ids/columns and opens the dialog', () => {
      const props = buildProps();
      const captured = mount(props);

      expect(captured.isBulkEditDialogOpen()).toBe(false);
      captured.handleBulkEdit(['id-a', 'id-b'], ['cpu', 'memory']);

      expect(captured.bulkEditIds()).toEqual(['id-a', 'id-b']);
      expect(captured.bulkEditColumns()).toEqual(['cpu', 'memory']);
      expect(captured.isBulkEditDialogOpen()).toBe(true);
    });

    it('handleSaveBulkEdit closes the dialog, clears the staged ids/columns, and persists', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.handleBulkEdit(['id-a'], ['cpu']);
      captured.handleSaveBulkEdit({ cpu: 88 });

      expect(captured.isBulkEditDialogOpen()).toBe(false);
      expect(captured.bulkEditIds()).toEqual([]);
      expect(captured.bulkEditColumns()).toEqual([]);
      // persistBulkEdit (real) always flags unsaved changes at the end of its run.
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });
  });

  describe('updateMetricDelay', () => {
    it('ignores a blank metric key without touching the store (early-return arm)', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', '   ', 5);

      expect(setMetricTimeThresholdsSpy).not.toHaveBeenCalled();
      expect(setHasUnsavedChangesSpy).not.toHaveBeenCalled();
    });

    it('creates a fresh per-type override, rounding decimals (set + change + assign arms)', () => {
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', ' CPU ', 5.7);

      // metric is trimmed + lowercased; value is rounded (6) and stored under the type.
      expect(metricTimeState).toEqual({ node: { cpu: 6 } });
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('clamps negative values to 0 when they differ from the stored value', () => {
      metricTimeState = { node: { cpu: 5 } };
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', 'cpu', -3);

      expect(metricTimeState).toEqual({ node: { cpu: 0 } });
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('treats an identical value as a no-op (same-value return-prev arm)', () => {
      metricTimeState = { node: { cpu: 5 } };
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', 'cpu', 5);

      expect(metricTimeState).toEqual({ node: { cpu: 5 } });
      expect(setHasUnsavedChangesSpy).not.toHaveBeenCalled();
    });

    it('deletes the metric and drops the empty type key when the last override is removed', () => {
      metricTimeState = { node: { cpu: 5 } };
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', 'cpu', null);

      expect(metricTimeState).toEqual({});
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('deletes the metric but keeps the type key when sibling overrides remain', () => {
      metricTimeState = { node: { cpu: 5, memory: 8 } };
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', 'cpu', null);

      expect(metricTimeState).toEqual({ node: { memory: 8 } });
      expect(setHasUnsavedChangesSpy).toHaveBeenCalledWith(true);
    });

    it('treats deletion of a metric that was never set as a no-op', () => {
      metricTimeState = { node: { memory: 8 } };
      const props = buildProps();
      const captured = mount(props);

      captured.updateMetricDelay('node', 'cpu', null);

      expect(metricTimeState).toEqual({ node: { memory: 8 } });
      expect(setHasUnsavedChangesSpy).not.toHaveBeenCalled();
    });
  });
});
