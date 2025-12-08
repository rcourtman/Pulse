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
        return byType('vm').map(r => {
            const platformData = r.platformData as Record<string, unknown> | undefined;
            return {
                id: r.id,
                vmid: platformData?.vmid as number ?? parseInt(r.id.split('-').pop() ?? '0', 10),
                name: r.name,
                node: platformData?.node as string ?? '',
                instance: platformData?.instance as string ?? r.platformId,
                status: r.status === 'running' ? 'running' : 'stopped',
                type: 'qemu',
                cpu: (r.cpu?.current ?? 0) / 100, // Convert from percentage to ratio for Dashboard
                cpus: platformData?.cpus as number ?? 1,
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
                diskRead: platformData?.diskRead as number ?? 0,
                diskWrite: platformData?.diskWrite as number ?? 0,
                uptime: r.uptime ?? 0,
                template: platformData?.template as boolean ?? false,
                lastBackup: platformData?.lastBackup as number ?? 0,
                tags: r.tags ?? [],
                lock: platformData?.lock as string ?? '',
                lastSeen: new Date(r.lastSeen).toISOString(),
            };
        });
    });

    // Convert resources to legacy Container format
    const asContainers = createMemo(() => {
        return byType('container').map(r => {
            const platformData = r.platformData as Record<string, unknown> | undefined;
            return {
                id: r.id,
                vmid: platformData?.vmid as number ?? parseInt(r.id.split('-').pop() ?? '0', 10),
                name: r.name,
                node: platformData?.node as string ?? '',
                instance: platformData?.instance as string ?? r.platformId,
                status: r.status === 'running' ? 'running' : 'stopped',
                type: 'lxc',
                cpu: (r.cpu?.current ?? 0) / 100, // Convert from percentage to ratio for Dashboard
                cpus: platformData?.cpus as number ?? 1,
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
                diskRead: platformData?.diskRead as number ?? 0,
                diskWrite: platformData?.diskWrite as number ?? 0,
                uptime: r.uptime ?? 0,
                template: platformData?.template as boolean ?? false,
                lastBackup: platformData?.lastBackup as number ?? 0,
                tags: r.tags ?? [],
                lock: platformData?.lock as string ?? '',
                lastSeen: new Date(r.lastSeen).toISOString(),
            };
        });
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

    // Convert resources to legacy Node format
    const asNodes = createMemo(() => {
        return byType('node').map(r => {
            const platformData = r.platformData as Record<string, unknown> | undefined;
            return {
                id: r.id,
                name: r.name,
                displayName: r.displayName,
                instance: r.platformId,
                host: platformData?.host as string ?? '',
                status: r.status,
                type: 'node',
                cpu: r.cpu?.current ?? 0,
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
                uptime: r.uptime ?? 0,
                loadAverage: platformData?.loadAverage as number[] ?? [],
                kernelVersion: platformData?.kernelVersion as string ?? '',
                pveVersion: platformData?.pveVersion as string ?? '',
                cpuInfo: platformData?.cpuInfo ?? { model: '', cores: 0, sockets: 0, mhz: '' },
                temperature: platformData?.temperature,
                lastSeen: new Date(r.lastSeen).toISOString(),
                connectionHealth: platformData?.connectionHealth as string ?? 'unknown',
                isClusterMember: platformData?.isClusterMember as boolean | undefined,
                clusterName: platformData?.clusterName as string | undefined,
            };
        });
    });

    // Convert resources to legacy DockerHost format (including nested containers)
    const asDockerHosts = createMemo(() => {
        const dockerHostResources = byType('docker-host');
        const dockerContainerResources = byType('docker-container');

        return dockerHostResources.map(h => {
            const platformData = h.platformData as Record<string, unknown> | undefined;

            // Find containers belonging to this host
            const hostContainers = dockerContainerResources
                .filter(c => c.parentId === h.id)
                .map(c => {
                    const cPlatform = c.platformData as Record<string, unknown> | undefined;
                    return {
                        id: c.id,
                        name: c.name,
                        image: cPlatform?.image as string ?? '',
                        state: c.status === 'running' ? 'running' : 'exited',
                        status: c.status,
                        health: cPlatform?.health as string | undefined,
                        cpuPercent: c.cpu?.current ?? 0,
                        memoryUsageBytes: c.memory?.used ?? 0,
                        memoryLimitBytes: c.memory?.total ?? 0,
                        memoryPercent: c.memory?.current ?? 0,
                        uptimeSeconds: c.uptime ?? 0,
                        restartCount: cPlatform?.restartCount as number ?? 0,
                        exitCode: cPlatform?.exitCode as number ?? 0,
                        createdAt: cPlatform?.createdAt as number ?? 0,
                        startedAt: cPlatform?.startedAt as number | undefined,
                        finishedAt: cPlatform?.finishedAt as number | undefined,
                        ports: cPlatform?.ports,
                        labels: cPlatform?.labels as Record<string, string> | undefined,
                        networks: cPlatform?.networks,
                    };
                });

            return {
                id: h.id,
                agentId: platformData?.agentId as string ?? h.id,
                hostname: h.identity?.hostname ?? h.name,
                displayName: h.displayName || h.name,
                customDisplayName: platformData?.customDisplayName as string | undefined,
                machineId: h.identity?.machineId,
                os: platformData?.os as string | undefined,
                kernelVersion: platformData?.kernelVersion as string | undefined,
                architecture: platformData?.architecture as string | undefined,
                runtime: platformData?.runtime as string ?? 'docker',
                runtimeVersion: platformData?.runtimeVersion as string | undefined,
                dockerVersion: platformData?.dockerVersion as string | undefined,
                cpus: platformData?.cpus as number ?? 0,
                totalMemoryBytes: h.memory?.total ?? 0,
                uptimeSeconds: h.uptime ?? 0,
                cpuUsagePercent: h.cpu?.current,
                loadAverage: platformData?.loadAverage as number[] | undefined,
                memory: h.memory ? {
                    total: h.memory.total ?? 0,
                    used: h.memory.used ?? 0,
                    free: h.memory.free ?? 0,
                    usage: h.memory.current,
                } : undefined,
                disks: platformData?.disks,
                networkInterfaces: platformData?.networkInterfaces,
                status: h.status === 'online' || h.status === 'running' ? 'online' : h.status,
                lastSeen: h.lastSeen,
                intervalSeconds: platformData?.intervalSeconds as number ?? 30,
                agentVersion: platformData?.agentVersion as string | undefined,
                containers: hostContainers,
                services: platformData?.services,
                tasks: platformData?.tasks,
                swarm: platformData?.swarm,
                tokenId: platformData?.tokenId as string | undefined,
                tokenName: platformData?.tokenName as string | undefined,
                tokenHint: platformData?.tokenHint as string | undefined,
                tokenLastUsedAt: platformData?.tokenLastUsedAt as number | undefined,
                hidden: platformData?.hidden as boolean | undefined,
                pendingUninstall: platformData?.pendingUninstall as boolean | undefined,
                command: platformData?.command,
                isLegacy: platformData?.isLegacy as boolean | undefined,
            };
        });
    });

    return {
        resources,
        asVMs,
        asContainers,
        asHosts,
        asNodes,
        asDockerHosts,
    };
}

// Re-export types and helpers
export type { Resource, ResourceType, PlatformType, ResourceStatus, ResourceFilter };
export { isInfrastructure, isWorkload, getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent };
