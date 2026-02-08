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
type DockerHostContainer = NonNullable<DockerHost['containers']>[number];

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

function resourceToLegacyVM(r: Resource): VM {
    const platformData = r.platformData as Record<string, unknown> | undefined;
    return {
        id: r.id,
        vmid: platformData?.vmid as number ?? parseInt(r.id.split('-').pop() ?? '0', 10),
        name: r.name,
        node: platformData?.node as string ?? '',
        instance: platformData?.instance as string ?? r.platformId,
        status: r.status === 'running' ? 'running' : 'stopped',
        type: 'qemu',
        cpu: (r.cpu?.current ?? 0) / 100,
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
}

function resourceToLegacyContainer(r: Resource): Container {
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
        cpu: (r.cpu?.current ?? 0) / 100,
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
}

function resourceToLegacyHost(r: Resource): Host {
    const platformData = r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;
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
}

function resourceToLegacyNode(r: Resource): Node {
    const platformData = r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;

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
        cpu: (r.cpu?.current ?? 0) / 100,
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
        cpuInfo: (platformData?.cpuInfo as Node['cpuInfo'] | undefined) ?? { model: '', cores: 0, sockets: 0, mhz: '' },
        temperature,
        lastSeen: new Date(r.lastSeen).toISOString(),
        connectionHealth: platformData?.connectionHealth as string ?? 'unknown',
        isClusterMember: platformData?.isClusterMember as boolean | undefined,
        clusterName: platformData?.clusterName as string | undefined,
    };
}

function dockerHostsFromResources(
    dockerHostResources: Resource[],
    dockerContainerResources: Resource[],
): DockerHost[] {
    return dockerHostResources.map((hostResource) => {
        const platformData = hostResource.platformData
            ? (unwrap(hostResource.platformData) as Record<string, unknown>)
            : undefined;

        const hostContainers = dockerContainerResources
            .filter(containerResource => containerResource.parentId === hostResource.id)
            .map((containerResource) => {
                const containerPlatformData = containerResource.platformData
                    ? (unwrap(containerResource.platformData) as Record<string, unknown>)
                    : undefined;
                const originalContainerId = containerResource.id.includes('/')
                    ? containerResource.id.split('/').pop()!
                    : containerResource.id;
                return {
                    id: originalContainerId,
                    name: containerResource.name,
                    image: containerPlatformData?.image as string ?? '',
                    state: containerResource.status === 'running' ? 'running' : 'exited',
                    status: containerResource.status,
                    health: containerPlatformData?.health as string | undefined,
                    cpuPercent: containerResource.cpu?.current ?? 0,
                    memoryUsageBytes: containerResource.memory?.used ?? 0,
                    memoryLimitBytes: containerResource.memory?.total ?? 0,
                    memoryPercent: containerResource.memory?.current ?? 0,
                    uptimeSeconds: containerResource.uptime ?? 0,
                    restartCount: containerPlatformData?.restartCount as number ?? 0,
                    exitCode: containerPlatformData?.exitCode as number ?? 0,
                    createdAt: containerPlatformData?.createdAt as number ?? 0,
                    startedAt: containerPlatformData?.startedAt as number | undefined,
                    finishedAt: containerPlatformData?.finishedAt as number | undefined,
                    ports: containerPlatformData?.ports as DockerHostContainer['ports'],
                    labels: containerPlatformData?.labels as Record<string, string> | undefined,
                    networks: containerPlatformData?.networks as DockerHostContainer['networks'],
                    mounts: containerPlatformData?.mounts as DockerHostContainer['mounts'],
                    blockIo: containerPlatformData?.blockIo as DockerHostContainer['blockIo'],
                    writableLayerBytes: containerPlatformData?.writableLayerBytes as number | undefined,
                    rootFilesystemBytes: containerPlatformData?.rootFilesystemBytes as number | undefined,
                };
            });

        return {
            id: hostResource.id,
            agentId: platformData?.agentId as string ?? hostResource.id,
            hostname: hostResource.identity?.hostname ?? hostResource.name,
            displayName: hostResource.displayName || hostResource.name,
            customDisplayName: platformData?.customDisplayName as string | undefined,
            machineId: hostResource.identity?.machineId,
            os: platformData?.os as string | undefined,
            kernelVersion: platformData?.kernelVersion as string | undefined,
            architecture: platformData?.architecture as string | undefined,
            runtime: platformData?.runtime as string ?? 'docker',
            runtimeVersion: platformData?.runtimeVersion as string | undefined,
            dockerVersion: platformData?.dockerVersion as string | undefined,
            cpus: platformData?.cpus as number ?? 0,
            totalMemoryBytes: hostResource.memory?.total ?? 0,
            uptimeSeconds: hostResource.uptime ?? 0,
            cpuUsagePercent: hostResource.cpu?.current,
            loadAverage: platformData?.loadAverage as number[] | undefined,
            memory: hostResource.memory ? {
                total: hostResource.memory.total ?? 0,
                used: hostResource.memory.used ?? 0,
                free: hostResource.memory.free ?? 0,
                usage: hostResource.memory.current,
            } : undefined,
            disks: platformData?.disks as DockerHost['disks'],
            networkInterfaces: platformData?.interfaces as DockerHost['networkInterfaces'],
            status: hostResource.status === 'online' || hostResource.status === 'running'
                ? 'online'
                : hostResource.status,
            lastSeen: hostResource.lastSeen,
            intervalSeconds: platformData?.intervalSeconds as number ?? 30,
            agentVersion: platformData?.agentVersion as string | undefined,
            containers: hostContainers,
            services: platformData?.services as DockerHost['services'],
            tasks: platformData?.tasks as DockerHost['tasks'],
            swarm: platformData?.swarm as DockerHost['swarm'],
            tokenId: platformData?.tokenId as string | undefined,
            tokenName: platformData?.tokenName as string | undefined,
            tokenHint: platformData?.tokenHint as string | undefined,
            tokenLastUsedAt: platformData?.tokenLastUsedAt as number | undefined,
            hidden: platformData?.hidden as boolean | undefined,
            pendingUninstall: platformData?.pendingUninstall as boolean | undefined,
            command: platformData?.command as DockerHost['command'],
            isLegacy: platformData?.isLegacy as boolean | undefined,
        };
    });
}

function toLegacyStorageStatus(
    status: string | undefined,
    active: boolean | undefined,
    enabled: boolean | undefined,
): string {
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
}

function normalizeStorageMeta(value: unknown): StorageMetaBridge | undefined {
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
}

function readStorageMeta(
    resource: Resource,
    platformData: Record<string, unknown> | undefined,
): StorageMetaBridge | undefined {
    const directMeta = normalizeStorageMeta((resource as Resource & { storage?: unknown }).storage);
    if (directMeta) return directMeta;
    return normalizeStorageMeta(platformData?.storage);
}

function resolveStorageContent(
    storageMeta: StorageMetaBridge | undefined,
    platformData: Record<string, unknown> | undefined,
    fallback: string,
): string {
    const directContent = (storageMeta?.content || '').trim();
    if (directContent) return directContent;
    if ((storageMeta?.contentTypes || []).length > 0) return (storageMeta?.contentTypes || []).join(',');
    const legacyContent = (platformData?.content as string | undefined)?.trim();
    return legacyContent || fallback;
}

function isStorageEnabledStatus(status: string | undefined): boolean {
    const normalized = (status || '').toLowerCase();
    return normalized !== 'offline' && normalized !== 'stopped';
}

function isStorageActiveStatus(status: string | undefined): boolean {
    const normalized = (status || '').toLowerCase();
    return normalized === 'online' || normalized === 'running' || normalized === 'available';
}

function buildStorageBridgeKey(storage: Pick<Storage, 'id' | 'instance' | 'node' | 'name' | 'type'>): string {
    const instance = (storage.instance || '').trim().toLowerCase();
    const node = (storage.node || '').trim().toLowerCase();
    const name = (storage.name || '').trim().toLowerCase();
    const type = (storage.type || '').trim().toLowerCase();
    if (instance || node || name || type) {
        return `identity:${instance}|${node}|${name}|${type}`;
    }
    return `id:${(storage.id || '').trim().toLowerCase()}`;
}

function mergeStorageBridgeRecord(
    current: Storage,
    incoming: Storage,
    preferIncoming: boolean,
): Storage {
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
}

function resourceToLegacyStorage(resource: Resource): Storage {
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
        const instanceName = (platformData?.pbsInstanceName as string | undefined) || instanceID;
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
 * Alerts-focused legacy resource selectors used during convergence.
 * Uses unified resources directly with local legacy conversion and fallback.
 */
export function useAlertsResources(storeOverride?: ResourceStoreLike): UseAlertsResourcesReturn {
    const wsStore = storeOverride ?? getGlobalWebSocketStore();
    const { resources, byType } = useResources(wsStore);
    const hasUnifiedResources = createMemo(() => resources().length > 0);

    const nodes = createMemo<Node[]>(() => {
        const legacy = wsStore.state.nodes ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return byType('node').map(resourceToLegacyNode);
    });

    const vms = createMemo<VM[]>(() => {
        const legacy = wsStore.state.vms ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return byType('vm').map(resourceToLegacyVM);
    });

    const containers = createMemo<Container[]>(() => {
        const legacy = wsStore.state.containers ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return [...byType('container'), ...byType('oci-container')].map(resourceToLegacyContainer);
    });

    const hosts = createMemo<Host[]>(() => {
        const legacy = wsStore.state.hosts ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return byType('host').map(resourceToLegacyHost);
    });

    const dockerHosts = createMemo<DockerHost[]>(() => {
        const legacy = wsStore.state.dockerHosts ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return dockerHostsFromResources(byType('docker-host'), byType('docker-container'));
    });

    const storage = createMemo<Storage[]>(() => {
        const legacy = wsStore.state.storage ?? [];
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

        if (!hasUnifiedResources()) {
            return Array.from(recordsByKey.values());
        }

        const unifiedStorage = resources()
            .filter((resource) => resource.type === 'storage' || resource.type === 'datastore')
            .map(resourceToLegacyStorage);

        appendRecords(unifiedStorage, true);
        return Array.from(recordsByKey.values());
    });

    const ready = createMemo(() => nodes().length > 0);

    return {
        nodes,
        vms,
        containers,
        storage,
        hosts,
        dockerHosts,
        ready,
    };
}

/**
 * AI-chat-focused legacy resource selectors used during convergence.
 * Uses unified resources directly with local legacy conversion and fallback.
 */
export function useAIChatResources(storeOverride?: ResourceStoreLike): UseAIChatResourcesReturn {
    const wsStore = storeOverride ?? getGlobalWebSocketStore();
    const { resources, byType } = useResources(wsStore);
    const hasUnifiedResources = createMemo(() => resources().length > 0);

    const nodes = createMemo<Node[]>(() => {
        const legacy = wsStore.state.nodes ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return byType('node').map(resourceToLegacyNode);
    });

    const vms = createMemo<VM[]>(() => {
        const legacy = wsStore.state.vms ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return byType('vm').map(resourceToLegacyVM);
    });

    const containers = createMemo<Container[]>(() => {
        const legacy = wsStore.state.containers ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return [...byType('container'), ...byType('oci-container')].map(resourceToLegacyContainer);
    });

    const dockerHosts = createMemo<DockerHost[]>(() => {
        const legacy = wsStore.state.dockerHosts ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return dockerHostsFromResources(byType('docker-host'), byType('docker-container'));
    });

    const hosts = createMemo<Host[]>(() => {
        const legacy = wsStore.state.hosts ?? [];
        if (!hasUnifiedResources()) {
            return [...legacy];
        }
        return byType('host').map(resourceToLegacyHost);
    });

    const isCluster = createMemo(() => nodes().length > 1);

    return {
        nodes,
        vms,
        containers,
        dockerHosts,
        hosts,
        isCluster,
    };
}

// Re-export types and helpers
export type { Resource, ResourceType, PlatformType, ResourceStatus, ResourceFilter };
export { isInfrastructure, isWorkload, getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent };
