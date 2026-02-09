/**
 * useResources Hook
 * 
 * Provides reactive access to unified resources via the REST-backed unified resources hook.
 * This is the primary way for frontend components to access resource data as we migrate away
 * from legacy arrays (nodes, vms, containers, etc.). Tests can still inject a store override.
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
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
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
type ResourceStoreLike = Pick<ReturnType<typeof getGlobalWebSocketStore>, 'state'>;

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
export function useResources(storeOverride?: ResourceStoreLike): UseResourcesReturn {
    // Get the WebSocket store instance
    const wsStore = storeOverride ?? getGlobalWebSocketStore();

    // Use REST-backed unified resources when no test override is provided.
    // The `query: ''` fetches ALL resource types (no type filter).
    const unifiedHook = storeOverride
        ? null
        : useUnifiedResources({ query: '', cacheKey: 'all-resources' });

    // All resources from the unified source.
    const resources = createMemo<Resource[]>(() => {
        if (unifiedHook) {
            return unifiedHook.resources() ?? [];
        }
        // Test override path: read directly from provided store
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

// Re-export types and helpers
export type { Resource, ResourceType, PlatformType, ResourceStatus, ResourceFilter };
export { isInfrastructure, isWorkload, getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent };
