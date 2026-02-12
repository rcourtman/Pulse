import { createSignal, onCleanup } from 'solid-js';
import { AIAPI } from '@/api/ai';
import type { AnomalyReport, AnomaliesResponse } from '@/types/aiIntelligence';

// Store anomalies with their resource IDs as keys
type AnomalyStore = Map<string, Map<string, AnomalyReport>>;

// Global store for anomaly data
const [anomalyStore, setAnomalyStore] = createSignal<AnomalyStore>(new Map());
const [isLoading, setIsLoading] = createSignal(false);
const [error, setError] = createSignal<string | null>(null);
const [lastUpdate, setLastUpdate] = createSignal<Date | null>(null);

// Refresh interval (30 seconds)
const REFRESH_INTERVAL = 30000;

let refreshTimer: ReturnType<typeof setInterval> | null = null;
let activeSubscribers = 0;

// Fetch anomalies from the API
async function fetchAnomalies(): Promise<void> {
    if (isLoading()) return;

    setIsLoading(true);
    setError(null);

    try {
        const response: AnomaliesResponse = await AIAPI.getAnomalies();

        // Build a map of resource_id -> metric -> anomaly
        const newStore: AnomalyStore = new Map();

        for (const anomaly of response.anomalies) {
            if (!newStore.has(anomaly.resource_id)) {
                newStore.set(anomaly.resource_id, new Map());
            }
            newStore.get(anomaly.resource_id)!.set(anomaly.metric, anomaly);
        }

        setAnomalyStore(newStore);
        setLastUpdate(new Date());
    } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch anomalies');
    } finally {
        setIsLoading(false);
    }
}

// Start the refresh timer
function startRefreshTimer(): void {
    if (refreshTimer) return;

    // Initial fetch
    void fetchAnomalies();

    // Set up interval for periodic refresh
    refreshTimer = setInterval(fetchAnomalies, REFRESH_INTERVAL);
}

// Stop the refresh timer
function stopRefreshTimer(): void {
    if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = null;
    }
}

function useAnomalySubscription(): void {
    activeSubscribers += 1;
    if (activeSubscribers === 1) {
        startRefreshTimer();
    }

    onCleanup(() => {
        activeSubscribers = Math.max(0, activeSubscribers - 1);
        if (activeSubscribers === 0) {
            stopRefreshTimer();
        }
    });
}

/**
 * Hook to get anomaly data for a specific resource and metric.
 * Returns the anomaly if present, or null if the metric is within baseline.
 */
export function useAnomalyForMetric(
    resourceId: () => string | undefined,
    metric: () => 'cpu' | 'memory' | 'disk'
): () => AnomalyReport | null {
    useAnomalySubscription();

    return () => {
        const rid = resourceId();
        if (!rid) return null;

        const store = anomalyStore();
        const resourceAnomalies = store.get(rid);
        if (!resourceAnomalies) return null;

        return resourceAnomalies.get(metric()) || null;
    };
}

/**
 * Hook to get all anomalies for a specific resource.
 */
export function useAnomaliesForResource(
    resourceId: () => string | undefined
): () => AnomalyReport[] {
    useAnomalySubscription();

    return () => {
        const rid = resourceId();
        if (!rid) return [];

        const store = anomalyStore();
        const resourceAnomalies = store.get(rid);
        if (!resourceAnomalies) return [];

        return Array.from(resourceAnomalies.values());
    };
}

/**
 * Hook to get all anomalies across all resources.
 */
export function useAllAnomalies(): {
    anomalies: () => AnomalyReport[];
    count: () => number;
    isLoading: () => boolean;
    error: () => string | null;
    lastUpdate: () => Date | null;
    refresh: () => void;
} {
    useAnomalySubscription();

    return {
        anomalies: () => {
            const store = anomalyStore();
            const all: AnomalyReport[] = [];
            for (const resourceAnomalies of store.values()) {
                for (const anomaly of resourceAnomalies.values()) {
                    all.push(anomaly);
                }
            }
            return all;
        },
        count: () => {
            const store = anomalyStore();
            let count = 0;
            for (const resourceAnomalies of store.values()) {
                count += resourceAnomalies.size;
            }
            return count;
        },
        isLoading,
        error,
        lastUpdate,
        refresh: fetchAnomalies,
    };
}

/**
 * Hook to check if a resource has any anomalies.
 */
export function useHasAnomalies(resourceId: () => string | undefined): () => boolean {
    useAnomalySubscription();

    return () => {
        const rid = resourceId();
        if (!rid) return false;

        const store = anomalyStore();
        const resourceAnomalies = store.get(rid);
        return resourceAnomalies ? resourceAnomalies.size > 0 : false;
    };
}

// Cleanup when the module is unloaded (for HMR)
if (import.meta.hot) {
    import.meta.hot.dispose(() => {
        activeSubscribers = 0;
        stopRefreshTimer();
    });
}
