import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import { ResourceAPI } from '@/api/resources';
import type { Resource } from '@/types/resource';

/**
 * The registry resource a record names, for `buildResourcePagePath` and its
 * siblings. One abortable lookup runs per id. Undefined means the current
 * lookup is pending; null means there is no id or no answer. Callers that
 * need to avoid a provisional link can distinguish those states. An answer
 * for an earlier id is never returned for the current one.
 */
export function useResolvedResource(
  id: Accessor<string | null | undefined>,
): Accessor<Resource | null | undefined> {
  const [resolved, setResolved] = createSignal<{ id: string; resource: Resource | null } | null>(
    null,
  );
  createEffect(() => {
    const current = id()?.trim();
    if (!current) return;
    const controller = new AbortController();
    void ResourceAPI.getResource(current, { signal: controller.signal })
      .then((resource) => {
        if (!controller.signal.aborted) setResolved({ id: current, resource });
      })
      .catch(() => {
        if (!controller.signal.aborted) setResolved({ id: current, resource: null });
      });
    onCleanup(() => controller.abort());
  });
  return () => {
    const current = id()?.trim();
    if (!current) return null;
    const value = resolved();
    return value?.id === current ? value.resource : undefined;
  };
}
