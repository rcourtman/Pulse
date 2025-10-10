import type { Temperature } from '@/types/api';

// Node configuration types

export interface ClusterEndpoint {
  NodeID: string;
  NodeName: string;
  Host: string;
  IP: string;
  Online: boolean;
  LastSeen: string;
}

export interface PVENodeConfig {
  id: string;
  name: string;
  host: string;
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
  // Cluster information
  isCluster?: boolean;
  clusterName?: string;
  clusterEndpoints?: ClusterEndpoint[];
}

export interface PBSNodeConfig {
  id: string;
  name: string;
  host: string;
  user: string;
  hasPassword?: boolean;
  hasToken?: boolean;
  tokenName?: string;
  tokenValue?: string;
  password?: string;
  fingerprint?: string;
  verifySSL: boolean;
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
  user: string;
  hasPassword?: boolean;
  hasToken?: boolean;
  tokenName?: string;
  tokenValue?: string;
  password?: string;
  fingerprint?: string;
  verifySSL: boolean;
  monitorMailStats: boolean;
  monitorQueues: boolean;
  monitorQuarantine: boolean;
  monitorDomainStats: boolean;
}

export type NodeConfig = (PVENodeConfig | PBSNodeConfig | PMGNodeConfig) & {
  type: 'pve' | 'pbs' | 'pmg';
  status?: 'connected' | 'disconnected' | 'error' | 'pending';
  temperature?: Temperature;
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
