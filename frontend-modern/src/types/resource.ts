// Unified Resource type definitions - matches backend internal/resources/resource.go

import type { StatusIndicatorVariant } from '@/utils/status';

export type ResourceType =
    | 'node'
    | 'vm'
    | 'container'
    | 'host'
    | 'docker_host'
    | 'docker_container'
    | 'pbs_instance'
    | 'storage'
    | 'datastore';

export type PlatformType =
    | 'proxmox_pve'
    | 'proxmox_pbs'
    | 'proxmox_pmg'
    | 'docker'
    | 'host_agent'
    | 'multi'
    | 'unknown';

export type ResourceStatus =
    | 'online'
    | 'offline'
    | 'running'
    | 'stopped'
    | 'paused'
    | 'degraded'
    | 'unknown';

export type SourceType = 'api' | 'agent' | 'hybrid';

export interface MetricValue {
    current: number;   // Current value as percentage
    total?: number;    // Total capacity (bytes for memory/disk)
    used?: number;     // Used amount (bytes for memory/disk)
    free?: number;     // Free amount (bytes for memory/disk)
}

export interface NetworkMetric {
    inBytes: number;
    outBytes: number;
    inRate: number;
    outRate: number;
}

export interface ResourceAlert {
    id: string;
    severity: 'warning' | 'critical';
    message: string;
    timestamp: string;
}

export interface ResourceIdentity {
    hostname?: string;
    machineId?: string;
    primaryIp?: string;
}

export interface Resource {
    id: string;
    name: string;
    displayName: string;
    type: ResourceType;
    platform: PlatformType;
    sourceType: SourceType;
    sourceId: string;
    parentId?: string;
    status: ResourceStatus;
    cpu?: MetricValue;
    memory?: MetricValue;
    disk?: MetricValue;
    network?: NetworkMetric;
    uptime?: number;
    temperature?: number;
    tags?: string[];
    metadata?: Record<string, string>;
    alerts?: ResourceAlert[];
    identity?: ResourceIdentity;
    lastSeen?: string;
    createdAt?: string;
    updatedAt?: string;
    // Platform-specific data
    platformData?: Record<string, unknown>;
}

export interface StoreStats {
    totalResources: number;
    byType: Record<string, number>;
    byPlatform: Record<string, number>;
    byStatus: Record<string, number>;
    withAlerts: number;
    lastUpdated: string;
}

export interface ResourcesResponse {
    resources: Resource[];
    count: number;
    stats: StoreStats;
}

// Type guards
export function isInfrastructureType(type: ResourceType): boolean {
    return ['node', 'host', 'docker_host', 'pbs_instance'].includes(type);
}

export function isWorkloadType(type: ResourceType): boolean {
    return ['vm', 'container', 'docker_container'].includes(type);
}

// Display helpers
export const RESOURCE_TYPE_LABELS: Record<ResourceType, string> = {
    node: 'Proxmox Node',
    vm: 'Virtual Machine',
    container: 'LXC Container',
    host: 'Host Machine',
    docker_host: 'Docker Host',
    docker_container: 'Docker Container',
    pbs_instance: 'PBS Instance',
    storage: 'Storage',
    datastore: 'Datastore',
};

export const PLATFORM_LABELS: Record<PlatformType, string> = {
    proxmox_pve: 'Proxmox VE',
    proxmox_pbs: 'Proxmox PBS',
    proxmox_pmg: 'Proxmox PMG',
    docker: 'Docker',
    host_agent: 'Host Agent',
    multi: 'Multiple',
    unknown: 'Unknown',
};

export const STATUS_LABELS: Record<ResourceStatus, string> = {
    online: 'Online',
    offline: 'Offline',
    running: 'Running',
    stopped: 'Stopped',
    paused: 'Paused',
    degraded: 'Degraded',
    unknown: 'Unknown',
};

export function getStatusVariant(status: ResourceStatus): StatusIndicatorVariant {
    switch (status) {
        case 'online':
        case 'running':
            return 'success';
        case 'paused':
        case 'degraded':
            return 'warning';
        case 'offline':
        case 'stopped':
            return 'danger';
        default:
            return 'muted';
    }
}
