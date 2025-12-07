/**
 * useResources Hook
 * 
 * Provides reactive access to unified resources from the WebSocket state.
 * This is the primary way for frontend components to access resource data
 * as we migrate away from legacy arrays (nodes, vms, containers, etc.).
 * 
 * Example usage:
 * ```tsx
 * const { resources, infra, workloads, filtered } = useResources();
 * 
 * // Get all resources
 * <For each={resources()}>{r => <div>{r.name}</div>}</For>
 * 
 * // Get only infrastructure (nodes, hosts)
 * <For each={infra()}>{r => <div>{r.name}</div>}</For>
 * 
 * // Get filtered workloads
 * const vms = filtered({ types: ['vm'], statuses: ['running'] });
 * <For each={vms}>{r => <div>{r.name}</div>}</For>
 * ```
 */

import { createMemo, type Accessor } from 'solid-js';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type {
    Resource,
    ResourceType,
    PlatformType,
    ResourceStatus,
    ResourceFilter,
} from '@/types/resource';
import {
    isInfrastructure,
    isWorkload,
    getDisplayName,
    getCpuPercent,
    getMemoryPercent,
    getDiskPercent,
} from '@/types/resource';

export interface UseResourcesReturn {
    /** All unified resources */
    resources: Accessor<Resource[]>;

    /** Infrastructure resources only (nodes, hosts, docker-hosts) */
    infra: Accessor<Resource[]>;

    /** Workload resources only (vms, containers, docker-containers) */
    workloads: Accessor<Resource[]>;

    /** Resources by type */
    byType: (type: ResourceType) => Resource[];

    /** Resources by platform */
    byPlatform: (platform: PlatformType) => Resource[];

    /** Filter resources with multiple criteria */
    filtered: (filter: ResourceFilter) => Resource[];

    /** Get a single resource by ID */
    get: (id: string) => Resource | undefined;

    /** Get children of a resource (e.g., VMs of a node) */
    children: (parentId: string) => Resource[];

    /** Count of resources by status */
    statusCounts: Accessor<Record<ResourceStatus, number>>;

    /** Top consumers by CPU */
    topByCpu: (limit?: number) => Resource[];

    /** Top consumers by memory */
    topByMemory: (limit?: number) => Resource[];

    /** Whether resources are available */
    hasResources: Accessor<boolean>;
}

/**
 * Hook for accessing unified resources with reactive filtering.
 */
export function useResources(): UseResourcesReturn {
    // Get the WebSocket store instance
    const wsStore = getGlobalWebSocketStore();

    // All resources from WebSocket state
    const resources = createMemo<Resource[]>(() => {
        return wsStore.state.resources ?? [];
    });

    // Pre-filtered memos for common use cases
    const infra = createMemo<Resource[]>(() => {
        return resources().filter(isInfrastructure);
    });

    const workloads = createMemo<Resource[]>(() => {
        return resources().filter(isWorkload);
    });

    const hasResources = createMemo<boolean>(() => {
        return resources().length > 0;
    });

    // Status counts
    const statusCounts = createMemo<Record<ResourceStatus, number>>(() => {
        const counts: Record<ResourceStatus, number> = {
            online: 0,
            offline: 0,
            running: 0,
            stopped: 0,
            degraded: 0,
            paused: 0,
            unknown: 0,
        };

        for (const r of resources()) {
            if (r.status in counts) {
                counts[r.status as ResourceStatus]++;
            }
        }

        return counts;
    });

    // Filter by type
    const byType = (type: ResourceType): Resource[] => {
        return resources().filter(r => r.type === type);
    };

    // Filter by platform
    const byPlatform = (platform: PlatformType): Resource[] => {
        return resources().filter(r => r.platformType === platform);
    };

    // Complex filtering
    const filtered = (filter: ResourceFilter): Resource[] => {
        let result = resources();

        // Filter by types
        if (filter.types && filter.types.length > 0) {
            result = result.filter(r => filter.types!.includes(r.type));
        }

        // Filter by platforms
        if (filter.platforms && filter.platforms.length > 0) {
            result = result.filter(r => filter.platforms!.includes(r.platformType));
        }

        // Filter by statuses
        if (filter.statuses && filter.statuses.length > 0) {
            result = result.filter(r => filter.statuses!.includes(r.status));
        }

        // Filter by parent
        if (filter.parentId) {
            result = result.filter(r => r.parentId === filter.parentId);
        }

        // Filter by cluster
        if (filter.clusterId) {
            result = result.filter(r => r.clusterId === filter.clusterId);
        }

        // Filter by alerts
        if (filter.hasAlerts) {
            result = result.filter(r => r.alerts && r.alerts.length > 0);
        }

        // Search filter (name, displayName)
        if (filter.search && filter.search.trim()) {
            const term = filter.search.toLowerCase().trim();
            result = result.filter(r => {
                const name = getDisplayName(r).toLowerCase();
                const id = r.id.toLowerCase();
                return name.includes(term) || id.includes(term);
            });
        }

        return result;
    };

    // Get single resource by ID
    const get = (id: string): Resource | undefined => {
        return resources().find(r => r.id === id);
    };

    // Get children of a parent
    const children = (parentId: string): Resource[] => {
        return resources().filter(r => r.parentId === parentId);
    };

    // Top by CPU
    const topByCpu = (limit: number = 10): Resource[] => {
        return [...resources()]
            .filter(r => r.cpu && r.cpu.current > 0)
            .sort((a, b) => getCpuPercent(b) - getCpuPercent(a))
            .slice(0, limit);
    };

    // Top by memory
    const topByMemory = (limit: number = 10): Resource[] => {
        return [...resources()]
            .filter(r => r.memory)
            .sort((a, b) => getMemoryPercent(b) - getMemoryPercent(a))
            .slice(0, limit);
    };

    return {
        resources,
        infra,
        workloads,
        byType,
        byPlatform,
        filtered,
        get,
        children,
        statusCounts,
        topByCpu,
        topByMemory,
        hasResources,
    };
}

/**
 * Helper hook for resource-to-legacy-type conversion.
 * This helps during migration when components still expect legacy types.
 */
export function useResourcesAsLegacy() {
    const { resources, byType } = useResources();

    // Convert resources to legacy VM format
    const asVMs = createMemo(() => {
        return byType('vm').map(r => ({
            id: r.id,
            vmid: parseInt(r.id.split('-').pop() ?? '0', 10),
            name: r.name,
            node: r.parentId ?? '',
            instance: r.platformId,
            status: r.status === 'running' ? 'running' : 'stopped',
            type: 'qemu',
            cpu: r.cpu?.current ?? 0,
            cpus: 1, // Not available in unified model
            memory: r.memory ? {
                total: r.memory.total ?? 0,
                used: r.memory.used ?? 0,
                free: r.memory.free ?? 0,
                usage: r.memory.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            disk: r.disk ? {
                total: r.disk.total ?? 0,
                used: r.disk.used ?? 0,
                free: r.disk.free ?? 0,
                usage: r.disk.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            networkIn: r.network?.rxBytes ?? 0,
            networkOut: r.network?.txBytes ?? 0,
            diskRead: 0,
            diskWrite: 0,
            uptime: r.uptime ?? 0,
            template: false,
            lastBackup: 0,
            tags: r.tags ?? [],
            lock: '',
            lastSeen: new Date(r.lastSeen).toISOString(),
        }));
    });

    // Convert resources to legacy Container format
    const asContainers = createMemo(() => {
        return byType('container').map(r => ({
            id: r.id,
            vmid: parseInt(r.id.split('-').pop() ?? '0', 10),
            name: r.name,
            node: r.parentId ?? '',
            instance: r.platformId,
            status: r.status === 'running' ? 'running' : 'stopped',
            type: 'lxc',
            cpu: r.cpu?.current ?? 0,
            cpus: 1,
            memory: r.memory ? {
                total: r.memory.total ?? 0,
                used: r.memory.used ?? 0,
                free: r.memory.free ?? 0,
                usage: r.memory.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            disk: r.disk ? {
                total: r.disk.total ?? 0,
                used: r.disk.used ?? 0,
                free: r.disk.free ?? 0,
                usage: r.disk.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            networkIn: r.network?.rxBytes ?? 0,
            networkOut: r.network?.txBytes ?? 0,
            diskRead: 0,
            diskWrite: 0,
            uptime: r.uptime ?? 0,
            template: false,
            lastBackup: 0,
            tags: r.tags ?? [],
            lock: '',
            lastSeen: new Date(r.lastSeen).toISOString(),
        }));
    });

    // Convert resources to legacy Host format
    const asHosts = createMemo(() => {
        return byType('host').map(r => {
            // Extract platform-specific data if available
            const platformData = r.platformData as Record<string, unknown> | undefined;

            return {
                id: r.id,
                hostname: r.identity?.hostname ?? r.name,
                displayName: r.displayName || r.name,
                platform: platformData?.platform as string | undefined,
                osName: platformData?.osName as string | undefined,
                osVersion: platformData?.osVersion as string | undefined,
                kernelVersion: platformData?.kernelVersion as string | undefined,
                architecture: platformData?.architecture as string | undefined,
                cpuCount: platformData?.cpuCount as number | undefined,
                cpuUsage: r.cpu?.current,
                loadAverage: platformData?.loadAverage as number[] | undefined,
                memory: r.memory ? {
                    total: r.memory.total ?? 0,
                    used: r.memory.used ?? 0,
                    free: r.memory.free ?? 0,
                    usage: r.memory.current,
                } : { total: 0, used: 0, free: 0, usage: 0 },
                disks: platformData?.disks as Array<{
                    total: number;
                    used: number;
                    free: number;
                    usage: number;
                    mountpoint?: string;
                    type?: string;
                    device?: string;
                }> | undefined,
                diskIO: platformData?.diskIO as Array<{
                    device: string;
                    readBytes?: number;
                    writeBytes?: number;
                }> | undefined,
                networkInterfaces: platformData?.networkInterfaces as Array<{
                    name: string;
                    mac?: string;
                    addresses?: string[];
                    rxBytes?: number;
                    txBytes?: number;
                }> | undefined,
                sensors: platformData?.sensors as {
                    temperatureCelsius?: Record<string, number>;
                    fanRpm?: Record<string, number>;
                } | undefined,
                raid: platformData?.raid as Array<{
                    device: string;
                    name?: string;
                    level: string;
                    state: string;
                    totalDevices: number;
                    activeDevices: number;
                    workingDevices: number;
                    failedDevices: number;
                    spareDevices: number;
                    devices: Array<{ device: string; state: string; slot: number }>;
                    rebuildPercent: number;
                }> | undefined,
                status: r.status === 'online' || r.status === 'running' ? 'online' : r.status,
                uptimeSeconds: r.uptime,
                lastSeen: r.lastSeen,
                intervalSeconds: platformData?.intervalSeconds as number | undefined,
                agentVersion: platformData?.agentVersion as string | undefined,
                tokenId: platformData?.tokenId as string | undefined,
                tokenName: platformData?.tokenName as string | undefined,
                tags: r.tags,
            };
        });
    });

    return {
        resources,
        asVMs,
        asContainers,
        asHosts,
    };
}

// Re-export types and helpers
export type { Resource, ResourceType, PlatformType, ResourceStatus, ResourceFilter };
export { isInfrastructure, isWorkload, getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent };
