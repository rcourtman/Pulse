import type { NodeConfigWithStatus } from '@/types/nodes';

export interface ConfiguredNodeCapabilityBadge {
  label: string;
  className: string;
}

const DEFAULT_CAPABILITY_BADGE_CLASS =
  'text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded';
const TEMPERATURE_CAPABILITY_BADGE_CLASS =
  'text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded';

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

  if (node.type === 'pve') {
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

  if (node.type === 'pbs') {
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

  if (node.type === 'pmg') {
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
