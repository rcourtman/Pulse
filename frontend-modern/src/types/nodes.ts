import type { Temperature } from '@/types/api';

export type TemperatureTransport =
  | 'disabled'
  | 'socket-proxy'
  | 'https-proxy'
  | 'ssh'
  | 'ssh-blocked';

// Node configuration types

export interface ClusterEndpoint {
  nodeId: string;
  nodeName: string;
  host: string;
  guestURL?: string;
  ip: string;
  ipOverride?: string;
  fingerprint?: string;
  online: boolean; // Proxmox's view: is the node online in the cluster?
  lastSeen: string;
  pulseReachable?: boolean | null; // Pulse's view: can Pulse reach this endpoint? null/undefined = not yet checked
  lastPulseCheck?: string | null;
  pulseError?: string; // Last error Pulse encountered connecting to this endpoint
}

// Write-only PUT payload entry: sets or clears the connection address Pulse
// uses for one cluster member (ClusterEndpoint.ipOverride). The discovered
// host and IP are rebuilt on every cluster re-discovery, so ipOverride is the
// only durable user-editable endpoint field. Empty string clears it.
export interface ClusterEndpointOverridePayload {
  nodeName: string;
  ipOverride: string;
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
  // Write-only: per-member connection address overrides included in PUT
  // payloads; never returned by the API.
  clusterEndpointOverrides?: ClusterEndpointOverridePayload[];
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
  temperatureTransport?: TemperatureTransport;
  source?: 'agent' | 'script' | ''; // How this node was registered
};

export type NodeConfigWithStatus = NodeConfig & {
  hasPassword?: boolean;
  hasToken?: boolean;
  status: 'connected' | 'disconnected' | 'offline' | 'error' | 'pending';
};
