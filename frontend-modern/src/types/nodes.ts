import type { Temperature } from '@/types/api';

// Node configuration types

export interface ClusterEndpoint {
  NodeID: string;
  NodeName: string;
  Host: string;
  GuestURL?: string;
  IP: string;
  Online: boolean; // Proxmox's view: is the node online in the cluster?
  LastSeen: string;
  PulseReachable?: boolean | null; // Pulse's view: can Pulse reach this endpoint? null/undefined = not yet checked
  LastPulseCheck?: string | null;
  PulseError?: string; // Last error Pulse encountered connecting to this endpoint
}

export interface PVENodeConfig {
  id: string;
  name: string;
  host: string;
  guestURL?: string;
  user: string;
  hasPassword?: boolean;
  hasToken?: boolean;
  tokenName?: string;
  tokenValue?: string;
  password?: string;
  verifySSL: boolean;
  monitorVMs: boolean;
  monitorContainers: boolean;
  monitorStorage: boolean;
  monitorBackups: boolean;
  monitorPhysicalDisks: boolean;
  physicalDiskPollingMinutes?: number;
  temperatureMonitoringEnabled?: boolean | null;
  // Cluster information
  isCluster?: boolean;
  clusterName?: string;
  clusterEndpoints?: ClusterEndpoint[];
}

export interface PBSNodeConfig {
  id: string;
  name: string;
  host: string;
  guestURL?: string;
  user: string;
  hasPassword?: boolean;
  hasToken?: boolean;
  tokenName?: string;
  tokenValue?: string;
  password?: string;
  fingerprint?: string;
  verifySSL: boolean;
  temperatureMonitoringEnabled?: boolean | null;
  monitorDatastores: boolean;
  monitorSyncJobs: boolean;
  monitorVerifyJobs: boolean;
  monitorPruneJobs: boolean;
  monitorGarbageJobs: boolean;
}

export interface PMGNodeConfig {
  id: string;
  name: string;
  host: string;
  guestURL?: string;
  user: string;
  hasPassword?: boolean;
  hasToken?: boolean;
  tokenName?: string;
  tokenValue?: string;
  password?: string;
  fingerprint?: string;
  verifySSL: boolean;
  temperatureMonitoringEnabled?: boolean | null;
  monitorMailStats: boolean;
  monitorQueues: boolean;
  monitorQuarantine: boolean;
  monitorDomainStats: boolean;
}

export type NodeConfig = (PVENodeConfig | PBSNodeConfig | PMGNodeConfig) & {
  type: 'pve' | 'pbs' | 'pmg';
  status?: 'connected' | 'disconnected' | 'offline' | 'error' | 'pending';
  temperature?: Temperature;
  displayName?: string;
  source?: 'agent' | 'script' | ''; // How this node was registered
};

export type NodeConfigWithStatus = NodeConfig & {
  hasPassword?: boolean;
  hasToken?: boolean;
  status: 'connected' | 'disconnected' | 'offline' | 'error' | 'pending';
};

export interface NodesResponse {
  pve_instances: PVENodeConfig[];
  pbs_instances: PBSNodeConfig[];
  pmg_instances?: PMGNodeConfig[];
}

export interface NodeUpdateRequest {
  node: NodeConfig;
}

export interface NodeDeleteResponse {
  success: boolean;
  message: string;
}
