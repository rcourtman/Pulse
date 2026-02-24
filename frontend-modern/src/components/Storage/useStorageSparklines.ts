import { createSignal, createEffect, onCleanup } from 'solid-js';
import { ChartsAPI, type MetricPoint } from '@/api/charts';

/** Module-level cache so remounted components immediately show previous data. */
const sparklineCache = new Map<string, MetricPoint[]>();

/**
 * Lazily fetch storage usage sparkline data for a pool.
 * Only fetches when `enabled` is true (i.e., the pool's group is expanded and visible).
 * Re-fetches every 60 seconds while enabled.
 */
export function useStorageSparkline(resourceId: () => string, enabled: () => boolean) {
  const [data, setData] = createSignal<MetricPoint[]>(sparklineCache.get(resourceId()) ?? []);
  const [loading, setLoading] = createSignal(false);

  let timer: ReturnType<typeof setInterval> | undefined;

  const fetchData = async () => {
    const id = resourceId();
    if (!id || !enabled()) return;

    setLoading(true);
    try {
      const response = await ChartsAPI.getMetricsHistory({
        resourceType: 'storage',
        resourceId: id,
        metric: 'usage',
        range: '7d',
        maxPoints: 60,
      });

      if ('points' in response && response.points) {
        const points = response.points.map((p) => ({ timestamp: p.timestamp, value: p.value }));
        sparklineCache.set(id, points);
        setData(points);
      }
    } catch {
      // Silently fail — sparkline just won't show data
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    if (enabled()) {
      fetchData();
      timer = setInterval(fetchData, 60_000);
    } else {
      if (timer) clearInterval(timer);
      timer = undefined;
    }
  });

  onCleanup(() => {
    if (timer) clearInterval(timer);
  });

  return { data, loading };
}
