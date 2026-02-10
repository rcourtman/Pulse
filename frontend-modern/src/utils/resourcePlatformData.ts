import { unwrap } from 'solid-js/store';
import type { Resource } from '@/types/resource';

export interface ProxmoxPlatformData {
  nodeName?: string;
  clusterName?: string;
  instance?: string;
  uptime?: number;
  temperature?: number;
  pveVersion?: string;
  cpuInfo?: { cores?: number; model?: string; sockets?: number };
}

export interface AgentPlatformData {
  agentId?: string;
  hostname?: string;
  uptimeSeconds?: number;
  temperature?: number;
}

export function getProxmoxData(r: Resource): ProxmoxPlatformData | undefined {
  const pd = r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;
  return pd?.proxmox as ProxmoxPlatformData | undefined;
}

export function getAgentData(r: Resource): AgentPlatformData | undefined {
  const pd = r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;
  return pd?.agent as AgentPlatformData | undefined;
}

/** Get agentId for a node resource — works for merged (proxmox+agent) resources */
export function getLinkedAgentId(r: Resource): string | undefined {
  return getAgentData(r)?.agentId;
}
