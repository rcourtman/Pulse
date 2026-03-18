import type {
  NodeConfigWithStatus,
  PBSNodeConfig,
  PMGNodeConfig,
  PVENodeConfig,
} from '@/types/nodes';

export interface ConfiguredNodeCapabilityBadge {
  label: string;
  className: string;
}

const DEFAULT_CAPABILITY_BADGE_CLASS =
  'text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded';
const TEMPERATURE_CAPABILITY_BADGE_CLASS =
  'text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded';

type PVENodeWithStatus = NodeConfigWithStatus & PVENodeConfig & { type: 'pve' };
type PBSNodeWithStatus = NodeConfigWithStatus & PBSNodeConfig & { type: 'pbs' };
type PMGNodeWithStatus = NodeConfigWithStatus & PMGNodeConfig & { type: 'pmg' };

function isPVENode(node: NodeConfigWithStatus): node is PVENodeWithStatus {
  return node.type === 'pve' && 'monitorVMs' in node;
}

function isPBSNode(node: NodeConfigWithStatus): node is PBSNodeWithStatus {
  return node.type === 'pbs' && 'monitorDatastores' in node;
}

function isPMGNode(node: NodeConfigWithStatus): node is PMGNodeWithStatus {
  return node.type === 'pmg' && 'monitorMailStats' in node;
}

export function isConfiguredNodeTemperatureMonitoringEnabled(
  node: NodeConfigWithStatus,
  globalEnabled: boolean,
): boolean {
  if (
    node.temperatureMonitoringEnabled !== undefined &&
    node.temperatureMonitoringEnabled !== null
  ) {
    return node.temperatureMonitoringEnabled;
  }
  return globalEnabled;
}

export function getConfiguredNodeCapabilityBadges(
  node: NodeConfigWithStatus,
  globalTemperatureMonitoringEnabled: boolean,
): ConfiguredNodeCapabilityBadge[] {
  const badges: ConfiguredNodeCapabilityBadge[] = [];

  if (isPVENode(node)) {
    if (node.monitorVMs) badges.push({ label: 'VMs', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    if (node.monitorContainers) {
      badges.push({ label: 'Containers', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorStorage) {
      badges.push({ label: 'Storage', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorBackups) {
      badges.push({ label: 'Recovery', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorPhysicalDisks) {
      badges.push({ label: 'Physical Disks', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (isConfiguredNodeTemperatureMonitoringEnabled(node, globalTemperatureMonitoringEnabled)) {
      badges.push({ label: 'Temperature', className: TEMPERATURE_CAPABILITY_BADGE_CLASS });
    }
    return badges;
  }

  if (isPBSNode(node)) {
    if (node.monitorDatastores) {
      badges.push({ label: 'Datastores', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorSyncJobs) {
      badges.push({ label: 'Sync Jobs', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorVerifyJobs) {
      badges.push({ label: 'Verify Jobs', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorPruneJobs) {
      badges.push({ label: 'Prune Jobs', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorGarbageJobs) {
      badges.push({ label: 'Garbage Collection', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (isConfiguredNodeTemperatureMonitoringEnabled(node, globalTemperatureMonitoringEnabled)) {
      badges.push({ label: 'Temperature', className: TEMPERATURE_CAPABILITY_BADGE_CLASS });
    }
    return badges;
  }

  if (isPMGNode(node)) {
    if (node.monitorMailStats) {
      badges.push({ label: 'Mail stats', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorQueues) {
      badges.push({ label: 'Queues', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorQuarantine) {
      badges.push({ label: 'Quarantine', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
    if (node.monitorDomainStats) {
      badges.push({ label: 'Domain stats', className: DEFAULT_CAPABILITY_BADGE_CLASS });
    }
  }

  return badges;
}
