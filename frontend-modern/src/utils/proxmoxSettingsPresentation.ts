import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Mail from 'lucide-solid/icons/mail';
import type { Component } from 'solid-js';
import type { NodeConfig } from '@/types/nodes';

export type ProxmoxNodeType = 'pve' | 'pbs' | 'pmg';

export interface DiscoveredProxmoxServer {
  type: ProxmoxNodeType;
  ip: string;
  port: number;
  hostname?: string;
}

export interface ProxmoxVariantPresentation {
  title: string;
  addLabel: string;
  emptyTitle: string;
  emptyDescription: string;
  scanningLabel: string;
  emptyIcon: Component<{ class?: string }>;
  nameFromServer: (server: DiscoveredProxmoxServer) => string;
  titleFromServer: (server: DiscoveredProxmoxServer) => string;
}

export const PROXMOX_VARIANT_PRESENTATION: Record<ProxmoxNodeType, ProxmoxVariantPresentation> = {
  pve: {
    title: 'Proxmox VE nodes',
    addLabel: 'Add Proxmox VE Connection',
    emptyTitle: 'No Proxmox VE connections configured',
    emptyDescription:
      'Add a Proxmox VE connection when the unified agent is not available on the host',
    scanningLabel: 'Scanning your network for Proxmox VE servers…',
    emptyIcon: Server,
    nameFromServer: (server) => server.hostname || `pve-${server.ip}`,
    titleFromServer: (server) => server.hostname || `Proxmox VE at ${server.ip}`,
  },
  pbs: {
    title: 'Proxmox Backup Server nodes',
    addLabel: 'Add Backup Server Connection',
    emptyTitle: 'No Proxmox Backup Server connections configured',
    emptyDescription:
      'Add a Proxmox Backup Server connection when the unified agent is not available on the host',
    scanningLabel: 'Scanning your network for Proxmox Backup Servers…',
    emptyIcon: HardDrive,
    nameFromServer: (server) => server.hostname || `pbs-${server.ip}`,
    titleFromServer: (server) => server.hostname || `Backup Server at ${server.ip}`,
  },
  pmg: {
    title: 'Proxmox Mail Gateway nodes',
    addLabel: 'Add Mail Gateway Connection',
    emptyTitle: 'No Proxmox Mail Gateway connections configured',
    emptyDescription:
      'Add a Proxmox Mail Gateway connection when the unified agent is not available on the host',
    scanningLabel: 'Scanning your network for Proxmox Mail Gateway servers…',
    emptyIcon: Mail,
    nameFromServer: (server) => server.hostname || `pmg-${server.ip}`,
    titleFromServer: (server) => server.hostname || `Mail Gateway at ${server.ip}`,
  },
};

export function getProxmoxVariantPresentation(type: ProxmoxNodeType): ProxmoxVariantPresentation {
  return PROXMOX_VARIANT_PRESENTATION[type];
}

export function buildProxmoxDiscoveryPrefillNode(
  server: DiscoveredProxmoxServer,
): Partial<NodeConfig> {
  const variant = getProxmoxVariantPresentation(server.type);
  const baseNode = {
    type: server.type,
    name: variant.nameFromServer(server),
    host: `https://${server.ip}:${server.port}`,
    verifySSL: false,
  } as const;

  switch (server.type) {
    case 'pve':
      return {
        ...baseNode,
        user: '',
        tokenName: '',
        tokenValue: '',
        monitorVMs: true,
        monitorContainers: true,
        monitorStorage: true,
        monitorBackups: true,
        monitorPhysicalDisks: false,
      };
    case 'pbs':
      return {
        ...baseNode,
        user: '',
        tokenName: '',
        tokenValue: '',
        monitorDatastores: true,
        monitorSyncJobs: true,
        monitorVerifyJobs: true,
        monitorPruneJobs: true,
        monitorGarbageJobs: true,
      };
    case 'pmg':
      return {
        ...baseNode,
        user: '',
        monitorMailStats: true,
        monitorQueues: true,
        monitorQuarantine: true,
        monitorDomainStats: false,
      };
  }
}
