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
import { unwrap } from 'solid-js/store';
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
import type {
    Container,
    DockerHost,
    Host,
    Node,
    PMGInstance,
    PBSInstance,
    PBSDatastore,
    Storage,
    VM,
} from '@/types/api';

type ResourceStoreLike = Pick<ReturnType<typeof getGlobalWebSocketStore>, 'state'>;
type StorageMetaBridge = {
    type?: string;
    content?: string;
    contentTypes?: string[];
    shared?: boolean;
    isCeph?: boolean;
    isZfs?: boolean;
};

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

export interface UseAlertsResourcesReturn {
    nodes: Accessor<Node[]>;
    vms: Accessor<VM[]>;
    containers: Accessor<Container[]>;
    storage: Accessor<Storage[]>;
    hosts: Accessor<Host[]>;
    dockerHosts: Accessor<DockerHost[]>;
    ready: Accessor<boolean>;
}

export interface UseAIChatResourcesReturn {
    nodes: Accessor<Node[]>;
    vms: Accessor<VM[]>;
    containers: Accessor<Container[]>;
    dockerHosts: Accessor<DockerHost[]>;
    hosts: Accessor<Host[]>;
    isCluster: Accessor<boolean>;
}

/**
 * Hook for accessing unified resources with reactive filtering.
 */
export function useResources(storeOverride?: ResourceStoreLike): UseResourcesReturn {
    // Get the WebSocket store instance
    const wsStore = storeOverride ?? getGlobalWebSocketStore();

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
 * 
 * IMPORTANT: This hook includes fallback logic for initial page loads.
 * The unified resources array is populated by monitor broadcasts, but the
 * initial WebSocket state may not include it. In that case, we fall back
 * to the legacy arrays (nodes, vms, containers, etc.) from the WebSocket state.
 */
export function useResourcesAsLegacy(storeOverride?: ResourceStoreLike) {
    const wsStore = storeOverride ?? getGlobalWebSocketStore();
    const { resources, byType } = useResources(wsStore);

    // Check if we have unified resources (populated after first broadcast)
    const hasUnifiedResources = createMemo(() => resources().length > 0);

    // Prefer legacy arrays when they exist. The backend currently broadcasts both the
    // legacy arrays and unified resources, and the legacy arrays are already reconciled
    // by id in the WebSocket store (stable identities, fine-grained updates).
    // Only synthesize legacy types from unified resources when it looks like the backend
    // isn't providing that legacy field (e.g., resources include the type but legacy array is empty).
    const hasVmResources = createMemo(() => (resources() || []).some((r) => r.type === 'vm'));
    const hasNodeResources = createMemo(() => (resources() || []).some((r) => r.type === 'node'));
    const hasContainerResources = createMemo(() =>
        (resources() || []).some((r) => r.type === 'container' || r.type === 'oci-container'),
    );
    const hasHostResources = createMemo(() => (resources() || []).some((r) => r.type === 'host'));
    const hasDockerHostResources = createMemo(() => (resources() || []).some((r) => r.type === 'docker-host'));
    const hasStorageResources = createMemo(() =>
        (resources() || []).some((r) => r.type === 'storage' || r.type === 'datastore'),
    );
    const hasPBSResources = createMemo(() => (resources() || []).some((r) => r.type === 'pbs'));
    const hasPMGResources = createMemo(() => (resources() || []).some((r) => r.type === 'pmg'));

    // Convert resources to legacy VM format
    // Falls back to legacy state.vms array when unified resources aren't yet populated
    const asVMs = createMemo(() => {
        const legacy = wsStore.state.vms ?? [];
        // If we don't have unified resources yet, use legacy arrays directly
        if (!hasUnifiedResources()) {
            // Spread to create new array reference for reactivity (see asHosts for details)
            return [...legacy];
        }
        // If legacy data exists (or there are no VM resources), keep using it.
        if (legacy.length > 0 || !hasVmResources()) {
            return [...legacy];
        }

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
                // IP and OS fields - crucial for the Dashboard columns
                ipAddresses: (platformData?.ipAddresses as string[] | undefined) ?? (r.identity?.ips as string[] | undefined),
                osName: platformData?.osName as string | undefined,
                osVersion: platformData?.osVersion as string | undefined,
                agentVersion: platformData?.agentVersion as string | undefined,
                networkInterfaces: platformData?.networkInterfaces as Array<{
                    name: string;
                    mac?: string;
                    addresses?: string[];
                }> | undefined,
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
    // Falls back to legacy state.containers array when unified resources aren't yet populated
    const asContainers = createMemo(() => {
        const legacy = wsStore.state.containers ?? [];
        // If we don't have unified resources yet, use legacy arrays directly
        if (!hasUnifiedResources()) {
            // Spread to create new array reference for reactivity (see asHosts for details)
            return [...legacy];
        }
        // If legacy data exists (or there are no container resources), keep using it.
        if (legacy.length > 0 || !hasContainerResources()) {
            return [...legacy];
        }

        // Include both traditional LXC containers and OCI containers (Proxmox VE 9.1+).
        const containerResources = [...byType('container'), ...byType('oci-container')];

        return containerResources.map(r => {
            const platformData = r.platformData as Record<string, unknown> | undefined;
            const isOCI =
                r.type === 'oci-container' ||
                platformData?.isOci === true ||
                platformData?.type === 'oci';
            const legacyType = isOCI ? 'oci' : ((platformData?.type as string) ?? 'lxc');
            return {
                id: r.id,
                vmid: platformData?.vmid as number ?? parseInt(r.id.split('-').pop() ?? '0', 10),
                name: r.name,
                node: platformData?.node as string ?? '',
                instance: platformData?.instance as string ?? r.platformId,
                status: r.status === 'running' ? 'running' : 'stopped',
                type: legacyType,
                isOci: isOCI,
                osTemplate: platformData?.osTemplate as string | undefined,
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
                // IP and OS fields - crucial for the Dashboard columns
                ipAddresses: (platformData?.ipAddresses as string[] | undefined) ?? (r.identity?.ips as string[] | undefined),
                osName: platformData?.osName as string | undefined,
                osVersion: platformData?.osVersion as string | undefined,
                networkInterfaces: platformData?.networkInterfaces as Array<{
                    name: string;
                    mac?: string;
                    addresses?: string[];
                }> | undefined,
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
    // Falls back to legacy state.hosts array when unified resources aren't yet populated
    const asHosts = createMemo(() => {
        const legacy = wsStore.state.hosts ?? [];
        // If we don't have unified resources yet, use legacy arrays directly
        if (!hasUnifiedResources()) {
            // IMPORTANT: Spread to create a new array reference. The store's array is
            // updated in-place by reconcile(), but createMemo only notifies dependents
            // when the returned value changes by reference. Without spreading, downstream
            // memos like sortedHosts() wouldn't re-run when host properties change.
            return [...legacy];
        }
        // If legacy data exists (or there are no host resources), keep using it.
        if (legacy.length > 0 || !hasHostResources()) {
            // Same as above - spread to create new reference for reactivity
            return [...legacy];
        }

        return byType('host').map((r) => {
            // Extract platform-specific data - unwrap SolidJS Proxy objects into plain JS objects
            const platformData = r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;

            // Interfaces from platformData
            const interfaces = platformData?.interfaces as Array<{
                name: string;
                mac?: string;
                addresses?: string[];
                rxBytes?: number;
                txBytes?: number;
            }> | undefined;

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
                // Map backend 'interfaces' to frontend 'networkInterfaces'
                networkInterfaces: interfaces,
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
                tags: r.tags ? [...r.tags] : undefined,
            };
        });
    });

    // Convert resources to legacy Node format
    // Falls back to legacy state.nodes array when unified resources aren't yet populated
    const asNodes = createMemo(() => {
        const legacy = wsStore.state.nodes ?? [];
        // If we don't have unified resources yet, use legacy arrays directly
        if (!hasUnifiedResources()) {
            // Spread to create new array reference for reactivity (see asHosts for details)
            return [...legacy];
        }
        // If legacy data exists (or there are no node resources), keep using it.
        if (legacy.length > 0 || !hasNodeResources()) {
            return [...legacy];
        }

        return byType('node').map(r => {
            // Unwrap SolidJS Proxy objects into plain JS objects
            const platformData = r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;

            // Build temperature object from unified resource
            // The unified resource has temperature as a simple number,
            // but legacy components expect the full Temperature struct
            let temperature = undefined;
            if (r.temperature !== undefined && r.temperature !== null && r.temperature > 0) {
                temperature = {
                    cpuPackage: r.temperature,
                    cpuMax: r.temperature,
                    available: true,
                    hasCPU: true,
                    hasGPU: false,
                    hasNVMe: false,
                    lastUpdate: new Date(r.lastSeen).toISOString(),
                };
            }

            return {
                id: r.id,
                name: r.name,
                displayName: r.displayName,
                instance: r.platformId,
                host: platformData?.host as string ?? '',
                status: r.status,
                type: 'node',
                cpu: (r.cpu?.current ?? 0) / 100, // Convert from percentage to ratio for legacy components
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
                temperature,
                lastSeen: new Date(r.lastSeen).toISOString(),
                connectionHealth: platformData?.connectionHealth as string ?? 'unknown',
                isClusterMember: platformData?.isClusterMember as boolean | undefined,
                clusterName: platformData?.clusterName as string | undefined,
            };
        });
    });

    // Convert resources to legacy DockerHost format (including nested containers)
    // Falls back to legacy state.dockerHosts array when unified resources aren't yet populated
    const asDockerHosts = createMemo(() => {
        const legacy = wsStore.state.dockerHosts ?? [];
        // If we don't have unified resources yet, use legacy arrays directly
        if (!hasUnifiedResources()) {
            // Spread to create new array reference for reactivity (see asHosts for details)
            return [...legacy];
        }
        // If legacy data exists (or there are no docker-host resources), keep using it.
        if (legacy.length > 0 || !hasDockerHostResources()) {
            return [...legacy];
        }

        const dockerHostResources = byType('docker-host');
        const dockerContainerResources = byType('docker-container');

        return dockerHostResources.map(h => {
            // Unwrap SolidJS Proxy objects into plain JS objects
            const platformData = h.platformData ? (unwrap(h.platformData) as Record<string, unknown>) : undefined;

            // Find containers belonging to this host
            const hostContainers = dockerContainerResources
                .filter(c => c.parentId === h.id)
                .map(c => {
                    const cPlatform = c.platformData ? (unwrap(c.platformData) as Record<string, unknown>) : undefined;
                    // Extract original container ID from compound resource ID (hostID/containerID)
                    // This is needed for sparkline metrics to match the sampler which uses original IDs
                    const originalContainerId = c.id.includes('/') ? c.id.split('/').pop()! : c.id;
                    return {
                        id: originalContainerId,
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
                        mounts: cPlatform?.mounts,
                        blockIo: cPlatform?.blockIo,
                        writableLayerBytes: cPlatform?.writableLayerBytes as number | undefined,
                        rootFilesystemBytes: cPlatform?.rootFilesystemBytes as number | undefined,
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
                // Map backend 'interfaces' to frontend 'networkInterfaces'
                networkInterfaces: platformData?.interfaces,
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

    const toLegacyStorageStatus = (
        status: string | undefined,
        active: boolean | undefined,
        enabled: boolean | undefined,
    ): string => {
        if (active === false || enabled === false) return 'offline';
        switch ((status ?? '').toLowerCase()) {
            case 'online':
            case 'running':
            case 'available':
                return 'available';
            case 'degraded':
            case 'offline':
            case 'stopped':
            case 'unknown':
                return 'offline';
            default:
                return 'offline';
        }
    };

    const normalizeStorageMeta = (value: unknown): StorageMetaBridge | undefined => {
        if (!value || typeof value !== 'object') return undefined;
        const candidate = value as Record<string, unknown>;
        const contentTypes = Array.isArray(candidate.contentTypes)
            ? candidate.contentTypes.filter(
                (item): item is string => typeof item === 'string' && item.trim().length > 0,
            )
            : undefined;

        return {
            type: typeof candidate.type === 'string' ? candidate.type : undefined,
            content: typeof candidate.content === 'string' ? candidate.content : undefined,
            contentTypes,
            shared: typeof candidate.shared === 'boolean' ? candidate.shared : undefined,
            isCeph: typeof candidate.isCeph === 'boolean' ? candidate.isCeph : undefined,
            isZfs: typeof candidate.isZfs === 'boolean' ? candidate.isZfs : undefined,
        };
    };

    const readStorageMeta = (
        resource: Resource,
        platformData: Record<string, unknown> | undefined,
    ): StorageMetaBridge | undefined => {
        const directMeta = normalizeStorageMeta((resource as Resource & { storage?: unknown }).storage);
        if (directMeta) return directMeta;
        return normalizeStorageMeta(platformData?.storage);
    };

    const resolveStorageContent = (
        storageMeta: StorageMetaBridge | undefined,
        platformData: Record<string, unknown> | undefined,
        fallback: string,
    ): string => {
        const directContent = (storageMeta?.content || '').trim();
        if (directContent) return directContent;
        if ((storageMeta?.contentTypes || []).length > 0) return (storageMeta?.contentTypes || []).join(',');
        const legacyContent = (platformData?.content as string | undefined)?.trim();
        return legacyContent || fallback;
    };

    const isStorageEnabledStatus = (status: string | undefined): boolean => {
        const normalized = (status || '').toLowerCase();
        return normalized !== 'offline' && normalized !== 'stopped';
    };

    const isStorageActiveStatus = (status: string | undefined): boolean => {
        const normalized = (status || '').toLowerCase();
        return normalized === 'online' || normalized === 'running' || normalized === 'available';
    };

    const buildStorageBridgeKey = (storage: Pick<Storage, 'id' | 'instance' | 'node' | 'name' | 'type'>): string => {
        const instance = (storage.instance || '').trim().toLowerCase();
        const node = (storage.node || '').trim().toLowerCase();
        const name = (storage.name || '').trim().toLowerCase();
        const type = (storage.type || '').trim().toLowerCase();
        if (instance || node || name || type) {
            return `identity:${instance}|${node}|${name}|${type}`;
        }
        return `id:${(storage.id || '').trim().toLowerCase()}`;
    };

    const mergeStorageBridgeRecord = (
        current: Storage,
        incoming: Storage,
        preferIncoming: boolean,
    ): Storage => {
        const preferred = preferIncoming ? incoming : current;
        const secondary = preferred === current ? incoming : current;
        return {
            ...secondary,
            ...preferred,
            nodes: preferred.nodes ?? secondary.nodes,
            nodeIds: preferred.nodeIds ?? secondary.nodeIds,
            nodeCount: preferred.nodeCount ?? secondary.nodeCount,
            pbsNames: preferred.pbsNames ?? secondary.pbsNames,
            zfsPool: preferred.zfsPool ?? secondary.zfsPool,
        };
    };

    const toLegacyPbsDatastoreStatus = (status: string | undefined): string => {
        switch ((status ?? '').toLowerCase()) {
            case 'online':
            case 'running':
            case 'available':
                return 'available';
            case 'degraded':
            case 'offline':
            case 'stopped':
            case 'unknown':
                return 'offline';
            default:
                return status || 'unknown';
        }
    };

    const toLegacyPbsInstanceStatus = (status: string | undefined): string => {
        switch ((status ?? '').toLowerCase()) {
            case 'online':
            case 'running':
                return 'online';
            case 'offline':
            case 'stopped':
                return 'offline';
            case 'degraded':
                return 'degraded';
            default:
                return status || 'unknown';
        }
    };

    const toLegacyPmgStatus = (status: string | undefined): string => {
        switch ((status ?? '').toLowerCase()) {
            case 'online':
            case 'running':
                return 'online';
            case 'offline':
            case 'stopped':
                return 'offline';
            case 'degraded':
                return 'degraded';
            default:
                return status || 'unknown';
        }
    };

    // Convert resources to legacy PBS format.
    const asPBS = createMemo<PBSInstance[]>(() => {
        const legacy = wsStore.state.pbs ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        if (!hasPBSResources() && !hasStorageResources()) {
            return [...legacy];
        }

        const datastoreResources = byType('datastore');
        const datastoresByInstance = new Map<string, PBSDatastore[]>();
        const instanceNameByID = new Map<string, string>();

        datastoreResources.forEach((resource) => {
            const platformData = resource.platformData
                ? (unwrap(resource.platformData) as Record<string, unknown>)
                : undefined;
            const instanceID =
                (platformData?.pbsInstanceId as string | undefined) ||
                resource.parentId ||
                resource.platformId ||
                'pbs';
            const instanceName =
                (platformData?.pbsInstanceName as string | undefined) ||
                instanceNameByID.get(instanceID) ||
                instanceID;
            instanceNameByID.set(instanceID, instanceName);

            const total = resource.disk?.total ?? 0;
            const used = resource.disk?.used ?? 0;
            const free = resource.disk?.free ?? Math.max(total - used, 0);
            const usage = resource.disk?.current ?? (total > 0 ? (used / total) * 100 : 0);

            const datastore: PBSDatastore = {
                name: resource.name,
                total,
                used,
                free,
                usage,
                status: toLegacyPbsDatastoreStatus(resource.status),
                error: (platformData?.error as string | undefined) || '',
                namespaces: [],
                deduplicationFactor:
                    typeof platformData?.deduplicationFactor === 'number'
                        ? (platformData.deduplicationFactor as number)
                        : undefined,
            };

            const existing = datastoresByInstance.get(instanceID) || [];
            existing.push(datastore);
            datastoresByInstance.set(instanceID, existing);
        });

        const fromResources: PBSInstance[] = byType('pbs').map((resource) => {
            const platformData = resource.platformData
                ? (unwrap(resource.platformData) as Record<string, unknown>)
                : undefined;
            const instanceID = resource.id;
            const resourceDatastores = datastoresByInstance.get(instanceID) || [];
            const memoryUsed = resource.memory?.used ?? (platformData?.memoryUsed as number | undefined) ?? 0;
            const memoryTotal =
                resource.memory?.total ?? (platformData?.memoryTotal as number | undefined) ?? 0;

            return {
                id: instanceID,
                name: resource.name,
                host: (platformData?.host as string | undefined) || resource.platformId || '',
                status: toLegacyPbsInstanceStatus(resource.status),
                version: (platformData?.version as string | undefined) || '',
                cpu: resource.cpu?.current ?? 0,
                memory: resource.memory?.current ?? 0,
                memoryUsed,
                memoryTotal,
                uptime: resource.uptime ?? 0,
                datastores: resourceDatastores,
                backupJobs: [],
                syncJobs: [],
                verifyJobs: [],
                pruneJobs: [],
                garbageJobs: [],
                connectionHealth:
                    (platformData?.connectionHealth as string | undefined) ||
                    (resource.status === 'degraded' ? 'unhealthy' : 'healthy'),
                lastSeen: new Date(resource.lastSeen).toISOString(),
            };
        });

        datastoresByInstance.forEach((datastores, instanceID) => {
            if (fromResources.some((pbs) => pbs.id === instanceID)) return;
            const name = instanceNameByID.get(instanceID) || instanceID;
            fromResources.push({
                id: instanceID,
                name,
                host: instanceID,
                status: 'unknown',
                version: '',
                cpu: 0,
                memory: 0,
                memoryUsed: 0,
                memoryTotal: 0,
                uptime: 0,
                datastores,
                backupJobs: [],
                syncJobs: [],
                verifyJobs: [],
                pruneJobs: [],
                garbageJobs: [],
                connectionHealth: 'unknown',
                lastSeen: new Date().toISOString(),
            });
        });

        if (legacy.length === 0) {
            return fromResources;
        }

        const merged = legacy.map((pbs) => {
            const next = fromResources.find((item) => item.id === pbs.id);
            if (!next) return pbs;
            const datastoreByName = new Map<string, PBSDatastore>();
            (pbs.datastores || []).forEach((datastore) => datastoreByName.set(datastore.name, datastore));
            (next.datastores || []).forEach((datastore) => datastoreByName.set(datastore.name, datastore));

            return {
                ...pbs,
                status: next.status || pbs.status,
                version: next.version || pbs.version,
                cpu: Number.isFinite(next.cpu) ? next.cpu : pbs.cpu,
                memory: Number.isFinite(next.memory) ? next.memory : pbs.memory,
                memoryUsed: Number.isFinite(next.memoryUsed) ? next.memoryUsed : pbs.memoryUsed,
                memoryTotal: Number.isFinite(next.memoryTotal) ? next.memoryTotal : pbs.memoryTotal,
                uptime: Number.isFinite(next.uptime) ? next.uptime : pbs.uptime,
                connectionHealth: next.connectionHealth || pbs.connectionHealth,
                datastores: Array.from(datastoreByName.values()),
            };
        });

        fromResources.forEach((pbs) => {
            if (!merged.some((existing) => existing.id === pbs.id)) {
                merged.push(pbs);
            }
        });

        return merged;
    });

    // Convert resources to legacy storage format.
    const asStorage = createMemo<Storage[]>(() => {
        const legacy = wsStore.state.storage ?? [];
        const pbsDatastoresFromPBS = asPBS().flatMap((instance) =>
            (instance.datastores || []).map((datastore) => {
                const total = Number.isFinite(datastore.total) ? datastore.total : 0;
                const used = Number.isFinite(datastore.used) ? datastore.used : 0;
                const free = Number.isFinite(datastore.free) ? datastore.free : Math.max(total - used, 0);
                const usage = total > 0 ? (used / total) * 100 : 0;
                const instanceLabel = instance.name || instance.host || instance.id || 'PBS';
                const status = datastore.status || instance.status;
                const enabled = isStorageEnabledStatus(status);
                const active = isStorageActiveStatus(status);
                return {
                    id: `pbs-${instance.id || instanceLabel}-${datastore.name}`,
                    name: datastore.name || 'PBS Datastore',
                    node: instanceLabel,
                    instance: instance.id || instanceLabel,
                    type: 'pbs',
                    status: toLegacyPbsDatastoreStatus(status),
                    total,
                    used,
                    free,
                    usage,
                    content: 'backup',
                    shared: false,
                    enabled,
                    active,
                    nodes: [instanceLabel],
                    pbsNames: [datastore.name],
                } as Storage;
            }),
        );

        const storageResources = resources().filter((resource) => resource.type === 'storage' || resource.type === 'datastore');
        const synthesized: Storage[] = storageResources.map((resource) => {
            const platformData = resource.platformData
                ? (unwrap(resource.platformData) as Record<string, unknown>)
                : undefined;
            const storageMeta = readStorageMeta(resource, platformData);
            const hasStorageMeta = Boolean(storageMeta);
            const total = resource.disk?.total ?? 0;
            const used = resource.disk?.used ?? 0;
            const free = resource.disk?.free ?? Math.max(total - used, 0);
            const usage = resource.disk?.current ?? (total > 0 ? (used / total) * 100 : 0);
            const statusEnabled = isStorageEnabledStatus(resource.status);
            const statusActive = isStorageActiveStatus(resource.status);
            const legacyEnabledHint = platformData?.enabled as boolean | undefined;
            const legacyActiveHint = platformData?.active as boolean | undefined;
            const enabled = hasStorageMeta ? statusEnabled : (legacyEnabledHint ?? statusEnabled);
            const active = hasStorageMeta ? statusActive : (legacyActiveHint ?? statusActive);

            if (resource.type === 'datastore') {
                const instanceID =
                    (platformData?.pbsInstanceId as string | undefined) ||
                    resource.parentId ||
                    resource.platformId ||
                    'pbs';
                const instanceName =
                    (platformData?.pbsInstanceName as string | undefined) || instanceID;
                return {
                    id: resource.id,
                    name: resource.name,
                    node: instanceName,
                    instance: instanceID,
                    type: storageMeta?.type || 'pbs',
                    status: toLegacyStorageStatus(resource.status, active, enabled),
                    total,
                    used,
                    free,
                    usage,
                    content: resolveStorageContent(storageMeta, platformData, 'backup'),
                    shared:
                        hasStorageMeta
                            ? storageMeta?.shared === true
                            : Boolean(platformData?.shared),
                    enabled,
                    active,
                    nodes: [instanceName],
                    nodeIds: [`${instanceID}-${instanceName}`],
                    nodeCount: 1,
                    pbsNames: [resource.name],
                };
            }

            const node = (platformData?.node as string | undefined) || '';
            const instance = (platformData?.instance as string | undefined) || resource.platformId || '';
            const nodes = platformData?.nodes as string[] | undefined;

            return {
                id: resource.id,
                name: resource.name,
                node,
                instance,
                type: storageMeta?.type || (platformData?.type as string | undefined) || resource.type,
                status: toLegacyStorageStatus(resource.status, active, enabled),
                total,
                used,
                free,
                usage,
                content: resolveStorageContent(storageMeta, platformData, ''),
                shared:
                    hasStorageMeta
                        ? storageMeta?.shared === true
                        : Boolean(platformData?.shared),
                enabled,
                active,
                nodes,
                zfsPool: platformData?.zfsPool as Storage['zfsPool'],
            };
        });

        const recordsByKey = new Map<string, Storage>();
        const appendRecords = (items: Storage[], preferIncoming: boolean) => {
            items.forEach((item) => {
                const key = buildStorageBridgeKey(item);
                const existing = recordsByKey.get(key);
                if (!existing) {
                    recordsByKey.set(key, item);
                    return;
                }
                recordsByKey.set(key, mergeStorageBridgeRecord(existing, item, preferIncoming));
            });
        };

        appendRecords(legacy, false);
        appendRecords(pbsDatastoresFromPBS, false);

        if (!hasUnifiedResources()) {
            return Array.from(recordsByKey.values());
        }

        appendRecords(synthesized, true);
        return Array.from(recordsByKey.values());
    });

    // Convert resources to legacy PMG format.
    const asPMG = createMemo<PMGInstance[]>(() => {
        const legacy = wsStore.state.pmg ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        if (!hasPMGResources()) {
            return [...legacy];
        }

        const fromResources: PMGInstance[] = byType('pmg').map((resource) => {
            const platformData = resource.platformData
                ? (unwrap(resource.platformData) as Record<string, unknown>)
                : undefined;
            const queueTotal = Number(platformData?.queueTotal || 0);
            const queueStatus =
                queueTotal > 0
                    ? {
                        active: Number(platformData?.queueActive || 0),
                        deferred: Number(platformData?.queueDeferred || 0),
                        hold: Number(platformData?.queueHold || 0),
                        incoming: Number(platformData?.queueIncoming || 0),
                        total: queueTotal,
                        oldestAge: 0,
                        updatedAt:
                            (platformData?.lastUpdated as string | undefined) ||
                            new Date(resource.lastSeen).toISOString(),
                    }
                    : undefined;

            return {
                id: resource.id,
                name: resource.name,
                host: (platformData?.host as string | undefined) || resource.platformId || '',
                status: toLegacyPmgStatus(resource.status),
                version: (platformData?.version as string | undefined) || '',
                nodes: queueStatus
                    ? [
                        {
                            name: resource.name,
                            status: toLegacyPmgStatus(resource.status),
                            queueStatus,
                        },
                    ]
                    : [],
                mailStats:
                    Number(platformData?.mailCountTotal || 0) > 0
                        ? {
                            timeframe: '24h',
                            countTotal: Number(platformData?.mailCountTotal || 0),
                            countIn: 0,
                            countOut: 0,
                            spamIn: Number(platformData?.spamIn || 0),
                            spamOut: 0,
                            virusIn: Number(platformData?.virusIn || 0),
                            virusOut: 0,
                            bouncesIn: 0,
                            bouncesOut: 0,
                            bytesIn: 0,
                            bytesOut: 0,
                            greylistCount: 0,
                            junkIn: 0,
                            averageProcessTimeMs: 0,
                            rblRejects: 0,
                            pregreetRejects: 0,
                            updatedAt:
                                (platformData?.lastUpdated as string | undefined) ||
                                new Date(resource.lastSeen).toISOString(),
                        }
                        : undefined,
                mailCount: [],
                spamDistribution: [],
                quarantine: undefined,
                connectionHealth:
                    (platformData?.connectionHealth as string | undefined) ||
                    (resource.status === 'degraded' ? 'unhealthy' : 'healthy'),
                lastSeen: new Date(resource.lastSeen).toISOString(),
                lastUpdated:
                    (platformData?.lastUpdated as string | undefined) ||
                    new Date(resource.lastSeen).toISOString(),
            };
        });

        if (legacy.length === 0) {
            return fromResources;
        }

        const merged = legacy.map((pmg) => {
            const next = fromResources.find((item) => item.id === pmg.id);
            if (!next) return pmg;
            return {
                ...pmg,
                status: next.status || pmg.status,
                version: next.version || pmg.version,
                connectionHealth: next.connectionHealth || pmg.connectionHealth,
                lastSeen: next.lastSeen || pmg.lastSeen,
                lastUpdated: next.lastUpdated || pmg.lastUpdated,
            };
        });

        fromResources.forEach((pmg) => {
            if (!merged.some((existing) => existing.id === pmg.id)) {
                merged.push(pmg);
            }
        });

        return merged;
    });

    return {
        resources,
        asVMs,
        asContainers,
        asHosts,
        asNodes,
        asDockerHosts,
        asStorage,
        asPBS,
        asPMG,
    };
}

/**
 * Alerts-focused legacy resource selectors used during convergence.
 * Thin wrapper over useResourcesAsLegacy with no conversion logic.
 */
export function useAlertsResources(storeOverride?: ResourceStoreLike): UseAlertsResourcesReturn {
    const {
        asNodes,
        asVMs,
        asContainers,
        asStorage,
        asHosts,
        asDockerHosts,
    } = useResourcesAsLegacy(storeOverride);

    const ready = createMemo(() => asNodes().length > 0);

    return {
        nodes: asNodes as Accessor<Node[]>,
        vms: asVMs as Accessor<VM[]>,
        containers: asContainers as Accessor<Container[]>,
        storage: asStorage as Accessor<Storage[]>,
        hosts: asHosts as Accessor<Host[]>,
        dockerHosts: asDockerHosts as Accessor<DockerHost[]>,
        ready,
    };
}

/**
 * AI-chat-focused legacy resource selectors used during convergence.
 * Thin wrapper over useResourcesAsLegacy with no conversion logic.
 */
export function useAIChatResources(storeOverride?: ResourceStoreLike): UseAIChatResourcesReturn {
    const {
        asNodes,
        asVMs,
        asContainers,
        asDockerHosts,
        asHosts,
    } = useResourcesAsLegacy(storeOverride);

    const isCluster = createMemo(() => asNodes().length > 1);

    return {
        nodes: asNodes as Accessor<Node[]>,
        vms: asVMs as Accessor<VM[]>,
        containers: asContainers as Accessor<Container[]>,
        dockerHosts: asDockerHosts as Accessor<DockerHost[]>,
        hosts: asHosts as Accessor<Host[]>,
        isCluster,
    };
}

// Re-export types and helpers
export type { Resource, ResourceType, PlatformType, ResourceStatus, ResourceFilter };
export { isInfrastructure, isWorkload, getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent };
