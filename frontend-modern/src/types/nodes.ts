// Node configuration types

export interface PVENodeConfig {
  id: string;
  name: string;
  host: string;
  user: string;
  tokenName?: string;
  tokenValue?: string;
  password?: string;
  verifySSL: boolean;
  monitorVMs: boolean;
  monitorContainers: boolean;
  monitorStorage: boolean;
  monitorBackups: boolean;
}

export interface PBSNodeConfig {
  id: string;
  name: string;
  host: string;
  user: string;
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

export type NodeConfig = (PVENodeConfig | PBSNodeConfig) & {
  type: 'pve' | 'pbs';
};

export interface NodesResponse {
  pve_instances: PVENodeConfig[];
  pbs_instances: PBSNodeConfig[];
}

export interface NodeUpdateRequest {
  node: NodeConfig;
}

export interface NodeDeleteResponse {
  success: boolean;
  message: string;
}