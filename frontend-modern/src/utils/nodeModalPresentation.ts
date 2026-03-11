import type { NodeConfig } from '@/types/nodes';

export type NodeModalNodeType = 'pve' | 'pbs' | 'pmg';

export type NodeModalAuthType = 'password' | 'token';
export type NodeModalSetupMode = 'agent' | 'auto' | 'manual';
export type NodeModalTestStatus = 'success' | 'warning' | 'error';

export interface NodeModalFormData {
  name: string;
  host: string;
  guestURL: string;
  authType: NodeModalAuthType;
  setupMode: NodeModalSetupMode;
  user: string;
  password: string;
  tokenName: string;
  tokenValue: string;
  fingerprint: string;
  verifySSL: boolean;
  monitorPhysicalDisks: boolean;
  physicalDiskPollingMinutes: number;
  monitorMailStats: boolean;
  monitorQueues: boolean;
  monitorQuarantine: boolean;
  monitorDomainStats: boolean;
}

export interface NodeModalTestResultPresentation {
  panelClass: string;
  textClass: string;
  icon: NodeModalTestStatus;
}

export function getNodeModalDefaultFormData(
  nodeType: NodeModalNodeType,
): NodeModalFormData {
  return {
    name: '',
    host: '',
    guestURL: '',
    authType: nodeType === 'pmg' ? 'password' : 'token',
    setupMode: 'agent',
    user: '',
    password: '',
    tokenName: '',
    tokenValue: '',
    fingerprint: '',
    verifySSL: true,
    monitorPhysicalDisks: false,
    physicalDiskPollingMinutes: 5,
    monitorMailStats: true,
    monitorQueues: true,
    monitorQuarantine: true,
    monitorDomainStats: false,
  };
}

export function getNodeProductName(nodeType: NodeModalNodeType): string {
  switch (nodeType) {
    case 'pve':
      return 'Proxmox VE';
    case 'pbs':
      return 'Proxmox Backup Server';
    case 'pmg':
      return 'Proxmox Mail Gateway';
  }
}

export function getNodeEndpointPlaceholder(nodeType: NodeModalNodeType): string {
  switch (nodeType) {
    case 'pve':
      return 'https://proxmox.example.com:8006';
    case 'pbs':
      return 'https://backup.example.com:8007';
    case 'pmg':
      return 'https://mail-gateway.example.com:8006';
  }
}

export function getNodeEndpointHelp(nodeType: NodeModalNodeType): string | null {
  switch (nodeType) {
    case 'pbs':
      return 'PBS requires HTTPS (not HTTP). Default port is 8007.';
    case 'pmg':
      return 'PMG API listens on HTTPS. Default port is 8006.';
    case 'pve':
      return null;
  }
}

export function getNodeGuestUrlPlaceholder(nodeType: NodeModalNodeType): string {
  switch (nodeType) {
    case 'pve':
      return 'https://pve.yourdomain.com';
    case 'pbs':
      return 'https://pbs.yourdomain.com';
    case 'pmg':
      return 'https://pmg.yourdomain.com';
  }
}

export function getNodeUsernamePlaceholder(nodeType: NodeModalNodeType): string {
  switch (nodeType) {
    case 'pbs':
      return 'admin@pbs';
    case 'pve':
    case 'pmg':
      return 'root@pam';
  }
}

export function getNodeUsernameHelp(nodeType: NodeModalNodeType): string | null {
  switch (nodeType) {
    case 'pbs':
      return 'Must include realm (e.g., admin@pbs).';
    case 'pmg':
      return 'Include realm (e.g., root@pam or api@pmg).';
    case 'pve':
      return null;
  }
}

export function getNodeTokenIdPlaceholder(nodeType: NodeModalNodeType): string {
  switch (nodeType) {
    case 'pve':
      return 'pulse-monitor@pve!pulse-token';
    case 'pbs':
      return 'pulse-monitor@pbs!pulse-token';
    case 'pmg':
      return 'pulse-monitor@pmg!pulse-token';
  }
}

export function getNodeMonitoringCoverageCopy(nodeType: NodeModalNodeType): string {
  if (nodeType === 'pmg') {
    return 'Pulse captures mail flow analytics, rejection causes, and quarantine visibility without additional scripts.';
  }
  return 'Pulse automatically tracks all supported resources for this node — virtual machines, containers, storage usage, backups, and PBS job activity — so you always get full visibility without extra configuration.';
}

export function getTemperatureMonitoringLockedCopy(): string {
  return 'Locked by environment variables. Remove the override (ENABLE_TEMPERATURE_MONITORING) and restart Pulse to manage it in the UI.';
}

export function buildNodeModalMonitoringPayload(
  nodeType: NodeModalNodeType,
  formData: NodeModalFormData,
): Partial<NodeConfig> {
  switch (nodeType) {
    case 'pve':
      return {
        monitorVMs: true,
        monitorContainers: true,
        monitorStorage: true,
        monitorBackups: true,
        monitorPhysicalDisks: formData.monitorPhysicalDisks,
        physicalDiskPollingMinutes: formData.physicalDiskPollingMinutes,
      };
    case 'pbs':
      return {
        monitorDatastores: true,
        monitorSyncJobs: true,
        monitorVerifyJobs: true,
        monitorPruneJobs: true,
        monitorGarbageJobs: true,
      };
    case 'pmg':
      return {
        monitorMailStats: formData.monitorMailStats,
        monitorQueues: formData.monitorQueues,
        monitorQuarantine: formData.monitorQuarantine,
        monitorDomainStats: formData.monitorDomainStats,
      };
  }
}

export function getNodeModalTestResultPresentation(
  status?: string | null,
): NodeModalTestResultPresentation {
  switch (status) {
    case 'success':
      return {
        panelClass:
          'mx-6 p-3 rounded-md text-sm bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 text-green-800 dark:text-green-200',
        textClass: 'text-green-800 dark:text-green-200',
        icon: 'success',
      };
    case 'warning':
      return {
        panelClass:
          'mx-6 p-3 rounded-md text-sm bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-200',
        textClass: 'text-amber-800 dark:text-amber-200',
        icon: 'warning',
      };
    default:
      return {
        panelClass:
          'mx-6 p-3 rounded-md text-sm bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200',
        textClass: 'text-red-800 dark:text-red-200',
        icon: 'error',
      };
  }
}
