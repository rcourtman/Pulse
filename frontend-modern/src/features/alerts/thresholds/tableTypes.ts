import type { ResourcePolicy } from '@/types/resource';
import type { BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
import type { AlertResourceThresholdMap } from '@/components/Alerts/alertResourceTableModel';

export type ThresholdsActiveTab = 'proxmox' | 'pmg' | 'agents' | 'docker';

export interface Resource {
  id: string;
  name: string;
  displayName?: string;
  policy?: ResourcePolicy;
  aiSafeSummary?: string;
  rawName?: string;
  node?: string;
  instance?: string;
  host?: string;
  type?: string;
  resourceType?: string;
  subtitle?: string;
  thresholds?: AlertResourceThresholdMap;
  defaults?: AlertResourceThresholdMap;
  disabled?: boolean;
  disableConnectivity?: boolean;
  poweredOffSeverity?: 'warning' | 'critical';
  hasOverride?: boolean;
  status?: string;
  vmid?: number;
  cpu?: number;
  memory?: number;
  uptime?: number;
  clusterName?: string;
  isClusterMember?: boolean;
  delaySeconds?: number;
  editScope?: 'snapshot' | 'backup';
  isEnabled?: boolean;
  toggleEnabled?: () => void;
  toggleTitleEnabled?: string;
  toggleTitleDisabled?: string;
  editable?: boolean;
  note?: string;
  backup?: ({ enabled?: boolean } & Partial<BackupAlertConfig>) | undefined;
  snapshot?: ({ enabled?: boolean } & Partial<SnapshotAlertConfig>) | undefined;
  [key: string]: unknown;
}

export interface GroupHeaderMeta {
  type?: 'agent' | 'node' | 'default';
  displayName?: string;
  rawName?: string;
  host?: string;
  status?: string;
  clusterName?: string;
  isClusterMember?: boolean;
}
