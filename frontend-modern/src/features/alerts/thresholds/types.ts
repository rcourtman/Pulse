import type {
    Alert,
    PBSInstance,
    PMGInstance,
} from '@/types/api';
import type {
    RawOverrideConfig,
    PMGThresholdDefaults,
    SnapshotAlertConfig,
    BackupAlertConfig,
} from '@/types/alerts';
import type { Resource } from '@/types/resource';

export type OverrideType =
    | 'guest'
    | 'node'
    | 'hostAgent'
    | 'hostDisk'
    | 'storage'
    | 'pbs'
    | 'pmg'
    | 'dockerHost'
    | 'dockerContainer';

export type OfflineState = 'off' | 'warning' | 'critical';

export interface Override {
    id: string;
    name: string;
    type: OverrideType;
    resourceType?: string;
    vmid?: number;
    node?: string;
    instance?: string;
    disabled?: boolean;
    disableConnectivity?: boolean; // For nodes only - disable offline alerts
    poweredOffSeverity?: 'warning' | 'critical';
    note?: string;
    backup?: BackupAlertConfig;
    snapshot?: SnapshotAlertConfig;
    thresholds: {
        cpu?: number;
        memory?: number;
        disk?: number;
        diskRead?: number;
        diskWrite?: number;
        networkIn?: number;
        networkOut?: number;
        usage?: number; // For storage devices
        temperature?: number; // For nodes only - CPU temperature in °C
    };
}

export interface SimpleThresholds {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    temperature?: number; // For nodes only
    diskTemperature?: number; // For host agents
    [key: string]: number | undefined; // Add index signature for compatibility
}

export interface ThresholdsTableProps {
    overrides: () => Override[];
    setOverrides: (overrides: Override[]) => void;
    rawOverridesConfig: () => Record<string, RawOverrideConfig>;
    setRawOverridesConfig: (config: Record<string, RawOverrideConfig>) => void;
    allGuests: () => Resource[];
    nodes: Resource[];
    hosts: Resource[];
    storage: Resource[];
    dockerHosts: Resource[];
    allResources: Resource[];
    pbsInstances?: PBSInstance[]; // PBS instances from state
    pmgInstances?: PMGInstance[]; // PMG instances from state
    pmgThresholds: () => PMGThresholdDefaults;
    setPMGThresholds: (
        value: PMGThresholdDefaults | ((prev: PMGThresholdDefaults) => PMGThresholdDefaults),
    ) => void;
    guestDefaults: SimpleThresholds;
    setGuestDefaults: (
        value:
            | Record<string, number | undefined>
            | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
    ) => void;
    guestDisableConnectivity: () => boolean;
    setGuestDisableConnectivity: (value: boolean) => void;
    guestPoweredOffSeverity: () => 'warning' | 'critical';
    setGuestPoweredOffSeverity: (value: 'warning' | 'critical') => void;
    nodeDefaults: SimpleThresholds;
    pbsDefaults?: SimpleThresholds;
    hostDefaults: SimpleThresholds;
    setNodeDefaults: (
        value:
            | Record<string, number | undefined>
            | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
    ) => void;
    setPBSDefaults?: (
        value:
            | Record<string, number | undefined>
            | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
    ) => void;
    setHostDefaults: (
        value:
            | Record<string, number | undefined>
            | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
    ) => void;
    dockerDefaults: {
        cpu: number;
        memory: number;
        disk: number;
        restartCount: number;
        restartWindow: number;
        memoryWarnPct: number;
        memoryCriticalPct: number;
        serviceWarnGapPercent: number;
        serviceCriticalGapPercent: number;
    };
    dockerDisableConnectivity: () => boolean;
    setDockerDisableConnectivity: (value: boolean) => void;
    dockerPoweredOffSeverity: () => 'warning' | 'critical';
    setDockerPoweredOffSeverity: (value: 'warning' | 'critical') => void;
    setDockerDefaults: (
        value:
            | {
                cpu: number;
                memory: number;
                disk: number;
                restartCount: number;
                restartWindow: number;
                memoryWarnPct: number;
                memoryCriticalPct: number;
                serviceWarnGapPercent: number;
                serviceCriticalGapPercent: number;
            }
            | ((prev: {
                cpu: number;
                memory: number;
                disk: number;
                restartCount: number;
                restartWindow: number;
                memoryWarnPct: number;
                memoryCriticalPct: number;
                serviceWarnGapPercent: number;
                serviceCriticalGapPercent: number;
            }) => {
                cpu: number;
                memory: number;
                disk: number;
                restartCount: number;
                restartWindow: number;
                memoryWarnPct: number;
                memoryCriticalPct: number;
                serviceWarnGapPercent: number;
                serviceCriticalGapPercent: number;
            }),
    ) => void;
    dockerIgnoredPrefixes: () => string[];
    setDockerIgnoredPrefixes: (value: string[] | ((prev: string[]) => string[])) => void;
    ignoredGuestPrefixes: () => string[];
    setIgnoredGuestPrefixes: (value: string[] | ((prev: string[]) => string[])) => void;
    guestTagWhitelist: () => string[];
    setGuestTagWhitelist: (value: string[] | ((prev: string[]) => string[])) => void;
    guestTagBlacklist: () => string[];
    setGuestTagBlacklist: (value: string[] | ((prev: string[]) => string[])) => void;
    storageDefault: () => number;
    setStorageDefault: (value: number) => void;
    resetGuestDefaults?: () => void;
    resetNodeDefaults?: () => void;
    resetPBSDefaults?: () => void;
    resetHostDefaults?: () => void;
    resetDockerDefaults?: () => void;
    resetDockerIgnoredPrefixes?: () => void;
    resetStorageDefault?: () => void;
    factoryGuestDefaults?: Record<string, number | undefined>;
    factoryNodeDefaults?: Record<string, number | undefined>;
    factoryPBSDefaults?: Record<string, number | undefined>;
    factoryHostDefaults?: Record<string, number | undefined>;
    factoryDockerDefaults?: Record<string, number | undefined>;
    factoryStorageDefault?: number;
    timeThresholds: () => { guest: number; node: number; storage: number; pbs: number; host: number };
    metricTimeThresholds: () => Record<string, Record<string, number>>;
    setMetricTimeThresholds: (
        value:
            | Record<string, Record<string, number>>
            | ((prev: Record<string, Record<string, number>>) => Record<string, Record<string, number>>),
    ) => void;
    snapshotDefaults: () => SnapshotAlertConfig;
    setSnapshotDefaults: (
        value: SnapshotAlertConfig | ((prev: SnapshotAlertConfig) => SnapshotAlertConfig),
    ) => void;
    snapshotFactoryDefaults?: SnapshotAlertConfig;
    resetSnapshotDefaults?: () => void;
    backupDefaults: () => BackupAlertConfig;
    setBackupDefaults: (
        value: BackupAlertConfig | ((prev: BackupAlertConfig) => BackupAlertConfig),
    ) => void;
    backupFactoryDefaults?: BackupAlertConfig;
    resetBackupDefaults?: () => void;
    setHasUnsavedChanges: (value: boolean) => void;
    activeAlerts?: Record<string, Alert>;
    removeAlerts?: (predicate: (alert: Alert) => boolean) => void;
    // Global disable flags
    disableAllNodes: () => boolean;
    setDisableAllNodes: (value: boolean) => void;
    disableAllGuests: () => boolean;
    setDisableAllGuests: (value: boolean) => void;
    disableAllHosts: () => boolean;
    setDisableAllHosts: (value: boolean) => void;
    disableAllStorage: () => boolean;
    setDisableAllStorage: (value: boolean) => void;
    disableAllPBS: () => boolean;
    setDisableAllPBS: (value: boolean) => void;
    disableAllPMG: () => boolean;
    setDisableAllPMG: (value: boolean) => void;
    disableAllDockerHosts: () => boolean;
    setDisableAllDockerHosts: (value: boolean) => void;
    disableAllDockerServices: () => boolean;
    setDisableAllDockerServices: (value: boolean) => void;
    disableAllDockerContainers: () => boolean;
    setDisableAllDockerContainers: (value: boolean) => void;
    // Global disable offline alerts flags
    disableAllNodesOffline: () => boolean;
    setDisableAllNodesOffline: (value: boolean) => void;
    disableAllGuestsOffline: () => boolean;
    setDisableAllGuestsOffline: (value: boolean) => void;
    disableAllHostsOffline: () => boolean;
    setDisableAllHostsOffline: (value: boolean) => void;
    disableAllPBSOffline: () => boolean;
    setDisableAllPBSOffline: (value: boolean) => void;
    disableAllPMGOffline: () => boolean;
    setDisableAllPMGOffline: (value: boolean) => void;
    disableAllDockerHostsOffline: () => boolean;
    setDisableAllDockerHostsOffline: (value: boolean) => void;
}
