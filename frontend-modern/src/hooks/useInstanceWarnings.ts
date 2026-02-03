import { createSignal, createEffect, onCleanup } from 'solid-js';

interface InstanceHealth {
  key: string;
  type: string;
  displayName: string;
  instance: string;
  warnings?: string[];
}

interface SchedulerHealth {
  instances: InstanceHealth[];
}

/**
 * Hook to fetch instance warnings from the scheduler health endpoint.
 * Returns warnings for PVE instances (e.g., backup permission issues).
 */
export function useInstanceWarnings() {
  const [warnings, setWarnings] = createSignal<Map<string, string[]>>(new Map());
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);

  const fetchWarnings = async () => {
    try {
      const response = await fetch('/api/monitoring/scheduler/health');
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      const data: SchedulerHealth = await response.json();

      const warningMap = new Map<string, string[]>();
      for (const instance of data.instances || []) {
        if (instance.warnings && instance.warnings.length > 0) {
          warningMap.set(instance.instance, instance.warnings);
        }
      }
      setWarnings(warningMap);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch warnings');
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    fetchWarnings();
    // Refresh every 60 seconds
    const interval = setInterval(fetchWarnings, 60000);
    onCleanup(() => clearInterval(interval));
  });

  return {
    warnings,
    loading,
    error,
    refetch: fetchWarnings,
  };
}
