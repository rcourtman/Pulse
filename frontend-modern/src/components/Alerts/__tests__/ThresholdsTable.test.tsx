import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, fireEvent, screen, cleanup } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { ThresholdsTable, normalizeDockerIgnoredInput } from '../ThresholdsTable';
import type { PMGThresholdDefaults, SnapshotAlertConfig, BackupAlertConfig } from '@/types/alerts';
import type { Host } from '@/types/api';

let mockPathname = '/alerts/thresholds/containers';

vi.mock('@solidjs/router', () => ({
  useNavigate: () => vi.fn(),
  useLocation: () => ({ pathname: mockPathname }),
}));

vi.mock('../ResourceTable', () => ({
  ResourceTable: (props: { title?: string }) => (
    <div data-testid={`resource-table-${props.title ?? 'unnamed'}`} />
  ),
  Resource: () => null,
  GroupHeaderMeta: () => null,
}));

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  mockPathname = '/alerts/thresholds/containers';
});

const DEFAULT_PMG_THRESHOLDS: PMGThresholdDefaults = {
  queueTotalWarning: 100,
  queueTotalCritical: 200,
  oldestMessageWarnMins: 30,
  oldestMessageCritMins: 60,
  deferredQueueWarn: 50,
  deferredQueueCritical: 75,
  holdQueueWarn: 25,
  holdQueueCritical: 50,
  quarantineSpamWarn: 10,
  quarantineSpamCritical: 20,
  quarantineVirusWarn: 5,
  quarantineVirusCritical: 10,
  quarantineGrowthWarnPct: 25,
  quarantineGrowthWarnMin: 10,
  quarantineGrowthCritPct: 50,
  quarantineGrowthCritMin: 20,
};

const DEFAULT_DOCKER_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 85,
  restartCount: 3,
  restartWindow: 300,
  memoryWarnPct: 90,
  memoryCriticalPct: 95,
  serviceWarnGapPercent: 10,
  serviceCriticalGapPercent: 50,
};

const baseProps = () => ({
  overrides: () => [],
  setOverrides: vi.fn(),
  rawOverridesConfig: () => ({}),
  setRawOverridesConfig: vi.fn(),
  allGuests: () => [],
  nodes: [],
  hosts: [],
  storage: [],
  dockerHosts: [],
  pbsInstances: [],
  pmgInstances: [],
  pmgThresholds: () => DEFAULT_PMG_THRESHOLDS,
  setPMGThresholds: vi.fn(),
  guestDefaults: {},
  setGuestDefaults: vi.fn(),
  guestDisableConnectivity: () => false,
  setGuestDisableConnectivity: vi.fn(),
  guestPoweredOffSeverity: () => 'warning' as const,
  setGuestPoweredOffSeverity: vi.fn(),
  nodeDefaults: {},
  setNodeDefaults: vi.fn(),
  hostDefaults: { cpu: 80, memory: 85, disk: 90 },
  setHostDefaults: vi.fn(),
  dockerDefaults: DEFAULT_DOCKER_DEFAULTS,
  dockerDisableConnectivity: () => false,
  setDockerDisableConnectivity: vi.fn(),
  dockerPoweredOffSeverity: () => 'warning' as const,
  setDockerPoweredOffSeverity: vi.fn(),
  setDockerDefaults: vi.fn(),
  storageDefault: () => 85,
  setStorageDefault: vi.fn(),
  resetGuestDefaults: vi.fn(),
  resetNodeDefaults: vi.fn(),
  resetHostDefaults: vi.fn(),
  resetDockerDefaults: vi.fn(),
  resetDockerIgnoredPrefixes: undefined as (() => void) | undefined,
  resetStorageDefault: vi.fn(),
  factoryGuestDefaults: {},
  factoryNodeDefaults: {},
  factoryHostDefaults: { cpu: 80, memory: 85, disk: 90 },
  factoryDockerDefaults: DEFAULT_DOCKER_DEFAULTS,
  factoryStorageDefault: 85,
  backupDefaults: () => ({ enabled: false, warningDays: 7, criticalDays: 14 }),
  setBackupDefaults: vi.fn(),
  backupFactoryDefaults: { enabled: false, warningDays: 7, criticalDays: 14 } as BackupAlertConfig,
  resetBackupDefaults: vi.fn(),
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
  } as SnapshotAlertConfig,
  resetSnapshotDefaults: vi.fn(),
  timeThresholds: () => ({ guest: 5, node: 5, storage: 5, pbs: 5 }),
  metricTimeThresholds: () => ({}),
  setMetricTimeThresholds: vi.fn(),
  activeAlerts: {},
  removeAlerts: vi.fn(),
  disableAllNodes: () => false,
  setDisableAllNodes: vi.fn(),
  disableAllGuests: () => false,
  setDisableAllGuests: vi.fn(),
  disableAllHosts: () => false,
  setDisableAllHosts: vi.fn(),
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
  disableAllHostsOffline: () => false,
  setDisableAllHostsOffline: vi.fn(),
  disableAllPBSOffline: () => false,
  setDisableAllPBSOffline: vi.fn(),
  disableAllPMGOffline: () => false,
  setDisableAllPMGOffline: vi.fn(),
  disableAllDockerHostsOffline: () => false,
  setDisableAllDockerHostsOffline: vi.fn(),
  ignoredGuestPrefixes: () => [] as string[],
  setIgnoredGuestPrefixes: vi.fn(),
  guestTagWhitelist: () => [] as string[],
  setGuestTagWhitelist: vi.fn(),
  guestTagBlacklist: () => [] as string[],
  setGuestTagBlacklist: vi.fn(),
});

const renderThresholdsTable = (options?: {
  initialPrefixes?: string[];
  includeReset?: boolean;
  hosts?: Host[];
}) => {
  let setDockerIgnoredPrefixesMock!: ReturnType<typeof vi.fn>;
  let resetDockerIgnoredPrefixesMock: ReturnType<typeof vi.fn> | undefined;
  let setHasUnsavedChangesMock!: ReturnType<typeof vi.fn>;
  let getPrefixes!: () => string[];

  const result = render(() => {
    const [prefixes, setPrefixes] = createSignal(options?.initialPrefixes ?? []);
    getPrefixes = prefixes;
    const [dockerDefaults, setDockerDefaultsState] = createSignal({ ...DEFAULT_DOCKER_DEFAULTS });

    setHasUnsavedChangesMock = vi.fn();

    setDockerIgnoredPrefixesMock = vi.fn((next: string[]) => {
      setPrefixes(next);
    });

    resetDockerIgnoredPrefixesMock =
      options?.includeReset === false
        ? undefined
        : vi.fn(() => {
          setPrefixes([]);
        });

    const base = baseProps();

    const props = {
      ...base,
      hosts: options?.hosts ?? base.hosts,
      dockerIgnoredPrefixes: () => prefixes(),
      setDockerIgnoredPrefixes: (value: string[] | ((prev: string[]) => string[])) => {
        const next = typeof value === 'function' ? value(prefixes()) : value;
        setDockerIgnoredPrefixesMock(next);
        setPrefixes(next);
      },
      get dockerDefaults() {
        return dockerDefaults();
      },
      setDockerDefaults: (
        value:
          | typeof DEFAULT_DOCKER_DEFAULTS
          | ((
            prev: typeof DEFAULT_DOCKER_DEFAULTS,
          ) => typeof DEFAULT_DOCKER_DEFAULTS),
      ) => {
        const next =
          typeof value === 'function'
            ? value(dockerDefaults())
            : { ...value };
        setDockerDefaultsState(next);
      },
      setHasUnsavedChanges: (value: boolean) => {
        setHasUnsavedChangesMock(value);
      },
      resetDockerIgnoredPrefixes: resetDockerIgnoredPrefixesMock,
    };

    return <ThresholdsTable {...props} />;
  });

  return {
    ...result,
    setDockerIgnoredPrefixesMock,
    resetDockerIgnoredPrefixesMock,
    setHasUnsavedChangesMock,
    getPrefixes,
  };
};

describe('normalizeDockerIgnoredInput', () => {
  it('trims whitespace and removes empty lines', () => {
    expect(normalizeDockerIgnoredInput(' runner-  \n\n #system \n\t \njob-')).toEqual([
      'runner-',
      '#system',
      'job-',
    ]);
  });

  it('returns empty array for blank input', () => {
    expect(normalizeDockerIgnoredInput('   \n ')).toEqual([]);
  });
});

describe('ThresholdsTable hosts tab', () => {
  it('renders host agents table when hosts tab is active', () => {
    mockPathname = '/alerts/thresholds/hosts';
    const host: Host = {
      id: 'host-1',
      hostname: 'host-1.local',
      displayName: 'Host 1',
      memory: {
        total: 1024,
        used: 512,
        free: 512,
        usage: 50,
      },
      status: 'online',
      lastSeen: 1,
    };

    renderThresholdsTable({ includeReset: false, hosts: [host] });

    expect(screen.getByTestId('resource-table-Host Agents')).toBeInTheDocument();
  });
});

describe('ThresholdsTable docker ignored prefixes', () => {
  it('updates prefixes when textarea is edited', () => {
    const { setDockerIgnoredPrefixesMock, setHasUnsavedChangesMock, getPrefixes } =
      renderThresholdsTable({ includeReset: false });

    const textarea = screen.getByPlaceholderText('runner-') as HTMLTextAreaElement;
    fireEvent.input(textarea, { target: { value: '  runner- \n #system \n' } });

    expect(setDockerIgnoredPrefixesMock).toHaveBeenCalledWith(['runner-', '#system']);
    expect(getPrefixes()).toEqual(['runner-', '#system']);
    expect(textarea).toHaveValue('runner-\n#system');
    expect(setHasUnsavedChangesMock).toHaveBeenCalledWith(true);
  });

  it('invokes reset handler and clears prefixes', () => {
    const {
      resetDockerIgnoredPrefixesMock,
      setDockerIgnoredPrefixesMock,
      setHasUnsavedChangesMock,
      getPrefixes,
    } = renderThresholdsTable({ initialPrefixes: ['runner-'], includeReset: true });

    const textarea = screen.getByPlaceholderText('runner-') as HTMLTextAreaElement;
    expect(textarea).toHaveValue('runner-');

    const resetButton = screen.getByRole('button', { name: /reset/i });
    fireEvent.click(resetButton);

    expect(resetDockerIgnoredPrefixesMock).toHaveBeenCalledTimes(1);
    expect(setDockerIgnoredPrefixesMock).not.toHaveBeenCalled();
    expect(getPrefixes()).toEqual([]);
    expect(textarea).toHaveValue('');
    expect(setHasUnsavedChangesMock).toHaveBeenCalledWith(true);
  });

  it('preserves trailing newlines when typing', () => {
    const { getPrefixes } = renderThresholdsTable({ includeReset: false });
    const textarea = screen.getByPlaceholderText('runner-') as HTMLTextAreaElement;

    // Simulate typing "abc"
    fireEvent.input(textarea, { target: { value: 'abc' } });
    expect(getPrefixes()).toEqual(['abc']);
    expect(textarea).toHaveValue('abc');

    // Simulate hitting Enter ("abc\n")
    fireEvent.input(textarea, { target: { value: 'abc\n' } });
    // Prop should still be ['abc'] as "abc" trimmed is "abc", "\n" trimmed is empty
    expect(getPrefixes()).toEqual(['abc']);
    // Value should NOT be reset to "abc" by the effect, it should remain "abc\n"
    expect(textarea).toHaveValue('abc\n');

    // Simulate typing "d" ("abc\nd")
    fireEvent.input(textarea, { target: { value: 'abc\nd' } });
    expect(getPrefixes()).toEqual(['abc', 'd']);
    expect(textarea).toHaveValue('abc\nd');
  });
});

describe('ThresholdsTable service gap validation', () => {
  it('shows a validation message when the critical gap falls below the warning gap', () => {
    renderThresholdsTable({ includeReset: false });

    const warnInput = screen.getByLabelText('Warning gap %') as HTMLInputElement;
    const critInput = screen.getByLabelText('Critical gap %') as HTMLInputElement;

    fireEvent.input(warnInput, { target: { value: '40' } });
    fireEvent.input(critInput, { target: { value: '20' } });

    expect(
      screen.getByText(/critical gap must be greater than or equal to the warning gap/i),
    ).toBeInTheDocument();
  });
});
