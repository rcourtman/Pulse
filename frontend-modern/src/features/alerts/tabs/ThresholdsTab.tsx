import type { Alert, PBSInstance, PMGInstance } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { RawOverrideConfig, BackupAlertConfig, SnapshotAlertConfig, PMGThresholdDefaults } from '@/types/alerts';
import { ThresholdsTable } from '@/components/Alerts/ThresholdsTable';

import type { Override } from '../types';

export interface ThresholdsTabProps {
  allGuests: () => Resource[];
  pbsInstances: PBSInstance[];
  pmgInstances: PMGInstance[];
  nodes: Resource[];
  agents: Resource[];
  storage: Resource[];
  dockerHosts: Resource[];
  allResources: Resource[];
  guestDefaults: () => Record<string, number | undefined>;
  nodeDefaults: () => Record<string, number | undefined>;
  pbsDefaults: () => Record<string, number | undefined>;
  agentDefaults: () => Record<string, number | undefined>;
  dockerDefaults: () => {
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
  dockerPoweredOffSeverity: () => 'warning' | 'critical';
  dockerIgnoredPrefixes: () => string[];
  ignoredGuestPrefixes: () => string[];
  guestTagWhitelist: () => string[];
  guestTagBlacklist: () => string[];
  storageDefault: () => number;
  timeThresholds: () => {
    guest: number;
    node: number;
    storage: number;
    pbs: number;
    agent: number;
  };
  metricTimeThresholds: () => Record<string, Record<string, number>>;
  overrides: () => Override[];
  rawOverridesConfig: () => Record<string, RawOverrideConfig>;
  pmgThresholds: () => PMGThresholdDefaults;
  setPMGThresholds: (
    value: PMGThresholdDefaults | ((prev: PMGThresholdDefaults) => PMGThresholdDefaults),
  ) => void;
  setGuestDefaults: (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  guestDisableConnectivity: () => boolean;
  setGuestDisableConnectivity: (value: boolean) => void;
  guestPoweredOffSeverity: () => 'warning' | 'critical';
  setGuestPoweredOffSeverity: (value: 'warning' | 'critical') => void;
  setNodeDefaults: (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  setAgentDefaults: (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  setPBSDefaults: (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
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
  setDockerDisableConnectivity: (value: boolean) => void;
  setDockerPoweredOffSeverity: (value: 'warning' | 'critical') => void;
  setDockerIgnoredPrefixes: (value: string[] | ((prev: string[]) => string[])) => void;
  setIgnoredGuestPrefixes: (value: string[] | ((prev: string[]) => string[])) => void;
  setGuestTagWhitelist: (value: string[] | ((prev: string[]) => string[])) => void;
  setGuestTagBlacklist: (value: string[] | ((prev: string[]) => string[])) => void;
  setStorageDefault: (value: number) => void;
  setMetricTimeThresholds: (
    value:
      | Record<string, Record<string, number>>
      | ((prev: Record<string, Record<string, number>>) => Record<string, Record<string, number>>),
  ) => void;
  snapshotDefaults: () => SnapshotAlertConfig;
  setSnapshotDefaults: (
    value: SnapshotAlertConfig | ((prev: SnapshotAlertConfig) => SnapshotAlertConfig),
  ) => void;
  snapshotFactoryDefaults: SnapshotAlertConfig;
  resetSnapshotDefaults: () => void;
  backupDefaults: () => BackupAlertConfig;
  setBackupDefaults: (
    value: BackupAlertConfig | ((prev: BackupAlertConfig) => BackupAlertConfig),
  ) => void;
  backupFactoryDefaults: BackupAlertConfig;
  resetBackupDefaults: () => void;
  setOverrides: (value: Override[]) => void;
  setRawOverridesConfig: (value: Record<string, RawOverrideConfig>) => void;
  activeAlerts: Record<string, Alert>;
  setHasUnsavedChanges: (value: boolean) => void;
  hasUnsavedChanges: () => boolean;
  removeAlerts: (predicate: (alert: Alert) => boolean) => void;
  disableAllNodes: () => boolean;
  setDisableAllNodes: (value: boolean) => void;
  disableAllGuests: () => boolean;
  setDisableAllGuests: (value: boolean) => void;
  disableAllAgents: () => boolean;
  setDisableAllAgents: (value: boolean) => void;
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
  disableAllNodesOffline: () => boolean;
  setDisableAllNodesOffline: (value: boolean) => void;
  disableAllGuestsOffline: () => boolean;
  setDisableAllGuestsOffline: (value: boolean) => void;
  disableAllAgentsOffline: () => boolean;
  setDisableAllAgentsOffline: (value: boolean) => void;
  disableAllPBSOffline: () => boolean;
  setDisableAllPBSOffline: (value: boolean) => void;
  disableAllPMGOffline: () => boolean;
  setDisableAllPMGOffline: (value: boolean) => void;
  disableAllDockerHostsOffline: () => boolean;
  setDisableAllDockerHostsOffline: (value: boolean) => void;
  resetGuestDefaults?: () => void;
  resetNodeDefaults?: () => void;
  resetPBSDefaults?: () => void;
  resetAgentDefaults?: () => void;
  resetDockerDefaults?: () => void;
  resetDockerIgnoredPrefixes?: () => void;
  resetStorageDefault?: () => void;
  factoryGuestDefaults?: Record<string, number | undefined>;
  factoryNodeDefaults?: Record<string, number | undefined>;
  factoryPBSDefaults?: Record<string, number | undefined>;
  factoryAgentDefaults?: Record<string, number | undefined>;
  factoryDockerDefaults?: Record<string, number | undefined>;
  factoryStorageDefault?: number;
}

export function ThresholdsTab(props: ThresholdsTabProps) {
  return (
    <ThresholdsTable
      overrides={props.overrides}
      setOverrides={props.setOverrides}
      rawOverridesConfig={props.rawOverridesConfig}
      setRawOverridesConfig={props.setRawOverridesConfig}
      allGuests={props.allGuests}
      nodes={props.nodes}
      agents={props.agents}
      storage={props.storage}
      dockerHosts={props.dockerHosts}
      allResources={props.allResources}
      pbsInstances={props.pbsInstances}
      pmgInstances={props.pmgInstances}
      pmgThresholds={props.pmgThresholds}
      setPMGThresholds={props.setPMGThresholds}
      guestDefaults={props.guestDefaults()}
      guestDisableConnectivity={props.guestDisableConnectivity}
      setGuestDefaults={props.setGuestDefaults}
      setGuestDisableConnectivity={props.setGuestDisableConnectivity}
      guestPoweredOffSeverity={props.guestPoweredOffSeverity}
      setGuestPoweredOffSeverity={props.setGuestPoweredOffSeverity}
      nodeDefaults={props.nodeDefaults()}
      agentDefaults={props.agentDefaults()}
      pbsDefaults={props.pbsDefaults()}
      setNodeDefaults={props.setNodeDefaults}
      setAgentDefaults={props.setAgentDefaults}
      setPBSDefaults={props.setPBSDefaults}
      dockerDefaults={props.dockerDefaults()}
      dockerDisableConnectivity={props.dockerDisableConnectivity}
      dockerPoweredOffSeverity={props.dockerPoweredOffSeverity}
      setDockerDefaults={props.setDockerDefaults}
      setDockerDisableConnectivity={props.setDockerDisableConnectivity}
      setDockerPoweredOffSeverity={props.setDockerPoweredOffSeverity}
      dockerIgnoredPrefixes={props.dockerIgnoredPrefixes}
      setDockerIgnoredPrefixes={props.setDockerIgnoredPrefixes}
      ignoredGuestPrefixes={props.ignoredGuestPrefixes}
      setIgnoredGuestPrefixes={props.setIgnoredGuestPrefixes}
      guestTagWhitelist={props.guestTagWhitelist}
      setGuestTagWhitelist={props.setGuestTagWhitelist}
      guestTagBlacklist={props.guestTagBlacklist}
      setGuestTagBlacklist={props.setGuestTagBlacklist}
      storageDefault={props.storageDefault}
      setStorageDefault={props.setStorageDefault}
      timeThresholds={props.timeThresholds}
      metricTimeThresholds={props.metricTimeThresholds}
      setMetricTimeThresholds={props.setMetricTimeThresholds}
      backupDefaults={props.backupDefaults}
      setBackupDefaults={props.setBackupDefaults}
      backupFactoryDefaults={props.backupFactoryDefaults}
      resetBackupDefaults={props.resetBackupDefaults}
      snapshotDefaults={props.snapshotDefaults}
      setSnapshotDefaults={props.setSnapshotDefaults}
      snapshotFactoryDefaults={props.snapshotFactoryDefaults}
      resetSnapshotDefaults={props.resetSnapshotDefaults}
      setHasUnsavedChanges={props.setHasUnsavedChanges}
      activeAlerts={props.activeAlerts}
      removeAlerts={props.removeAlerts}
      disableAllNodes={props.disableAllNodes}
      setDisableAllNodes={props.setDisableAllNodes}
      disableAllGuests={props.disableAllGuests}
      setDisableAllGuests={props.setDisableAllGuests}
      disableAllAgents={props.disableAllAgents}
      setDisableAllAgents={props.setDisableAllAgents}
      disableAllStorage={props.disableAllStorage}
      setDisableAllStorage={props.setDisableAllStorage}
      disableAllPBS={props.disableAllPBS}
      setDisableAllPBS={props.setDisableAllPBS}
      disableAllPMG={props.disableAllPMG}
      setDisableAllPMG={props.setDisableAllPMG}
      disableAllDockerHosts={props.disableAllDockerHosts}
      setDisableAllDockerHosts={props.setDisableAllDockerHosts}
      disableAllDockerServices={props.disableAllDockerServices}
      setDisableAllDockerServices={props.setDisableAllDockerServices}
      disableAllDockerContainers={props.disableAllDockerContainers}
      setDisableAllDockerContainers={props.setDisableAllDockerContainers}
      disableAllNodesOffline={props.disableAllNodesOffline}
      setDisableAllNodesOffline={props.setDisableAllNodesOffline}
      disableAllGuestsOffline={props.disableAllGuestsOffline}
      setDisableAllGuestsOffline={props.setDisableAllGuestsOffline}
      disableAllAgentsOffline={props.disableAllAgentsOffline}
      setDisableAllAgentsOffline={props.setDisableAllAgentsOffline}
      disableAllPBSOffline={props.disableAllPBSOffline}
      setDisableAllPBSOffline={props.setDisableAllPBSOffline}
      disableAllPMGOffline={props.disableAllPMGOffline}
      setDisableAllPMGOffline={props.setDisableAllPMGOffline}
      disableAllDockerHostsOffline={props.disableAllDockerHostsOffline}
      setDisableAllDockerHostsOffline={props.setDisableAllDockerHostsOffline}
      resetGuestDefaults={props.resetGuestDefaults}
      resetNodeDefaults={props.resetNodeDefaults}
      resetPBSDefaults={props.resetPBSDefaults}
      resetAgentDefaults={props.resetAgentDefaults}
      resetDockerDefaults={props.resetDockerDefaults}
      resetDockerIgnoredPrefixes={props.resetDockerIgnoredPrefixes}
      resetStorageDefault={props.resetStorageDefault}
      factoryGuestDefaults={props.factoryGuestDefaults}
      factoryNodeDefaults={props.factoryNodeDefaults}
      factoryPBSDefaults={props.factoryPBSDefaults}
      factoryAgentDefaults={props.factoryAgentDefaults}
      factoryDockerDefaults={props.factoryDockerDefaults}
      factoryStorageDefault={props.factoryStorageDefault}
    />
  );
}
