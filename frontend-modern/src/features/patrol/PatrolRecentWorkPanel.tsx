import { For, Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import CheckCircleIcon from 'lucide-solid/icons/circle-check';
import RefreshIcon from 'lucide-solid/icons/refresh-cw';
import { getPatrolWorkReceipts, type PatrolWorkReceipt } from '@/api/patrolAttention';
import { Button } from '@/components/shared/Button';
import {
  formatActionName,
  getActionResourcePresentation,
  verificationTruthLabel,
} from '@/features/actions/actionPresentation';
import { formatRelativeTime } from '@/utils/format';

const RECEIPT_LIMIT = 6;

const formatRecentWorkError = (error: unknown): string =>
  error instanceof Error ? error.message : 'Verified work could not be loaded.';

export function PatrolRecentWorkPanel() {
  const [items, setItems] = createSignal<PatrolWorkReceipt[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal('');
  const receipts = createMemo(() => items().slice(0, RECEIPT_LIMIT));

  const load = async (quiet = false) => {
    if (!quiet) setLoading(true);
    try {
      const response = await getPatrolWorkReceipts(RECEIPT_LIMIT);
      setItems(response.data);
      setError('');
    } catch (cause) {
      setError(formatRecentWorkError(cause));
    } finally {
      if (!quiet) setLoading(false);
    }
  };

  onMount(() => {
    void load();
    const refresh = () => {
      if (document.visibilityState === 'visible') void load(true);
    };
    const timer = window.setInterval(refresh, 30_000);
    document.addEventListener('visibilitychange', refresh);
    onCleanup(() => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refresh);
    });
  });

  return (
    <section
      class="overflow-hidden rounded-lg border border-border bg-surface"
      aria-labelledby="patrol-recent-work-title"
    >
      <div class="flex items-start justify-between gap-3 border-b border-border px-4 py-4 sm:px-5">
        <div>
          <h2 id="patrol-recent-work-title" class="text-base font-semibold text-base-content">
            Recently handled
          </h2>
          <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
            Concise receipts appear only after Patrol has verified the outcome.
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          class="gap-1.5"
          onClick={() => void load()}
          disabled={loading()}
          aria-label="Refresh recently handled work"
        >
          <RefreshIcon
            class={`h-4 w-4 ${loading() ? 'motion-safe:animate-spin' : ''}`}
            aria-hidden="true"
          />
          <span class="hidden sm:inline">Refresh</span>
        </Button>
      </div>

      <div aria-live="polite">
        <Show when={error()}>
          {(message) => (
            <div class="m-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200">
              <p class="font-semibold">Verified work is unavailable</p>
              <p class="mt-1 text-xs leading-5">{message()}</p>
            </div>
          )}
        </Show>

        <Show
          when={!loading() || receipts().length > 0}
          fallback={<p class="px-4 py-8 text-center text-sm text-muted">Loading verified work…</p>}
        >
          <Show
            when={receipts().length > 0}
            fallback={
              <div class="px-6 py-8 text-center">
                <CheckCircleIcon class="mx-auto h-8 w-8 text-muted" aria-hidden="true" />
                <h3 class="mt-3 text-sm font-semibold text-base-content">No verified work yet</h3>
                <p class="mx-auto mt-1 max-w-lg text-xs leading-5 text-muted">
                  Patrol will record a receipt here after an issue is resolved and its expected
                  outcome is confirmed.
                </p>
              </div>
            }
          >
            <ol class="divide-y divide-border" aria-label="Verified Patrol receipts">
              <For each={receipts()}>
                {(item) => {
                  const resolvedName = item.resourceName.trim();
                  const resource = getActionResourcePresentation(
                    item.resourceId,
                    resolvedName && resolvedName !== item.resourceId.trim()
                      ? {
                          id: item.resourceId,
                          name: resolvedName,
                          type: item.resourceType ?? '',
                        }
                      : undefined,
                  );
                  return (
                    <li class="flex items-start gap-3 px-4 py-3 sm:px-5">
                      <CheckCircleIcon
                        class="mt-0.5 h-5 w-5 shrink-0 text-emerald-500"
                        aria-hidden="true"
                      />
                      <div class="min-w-0 flex-1">
                        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
                          <span class="text-xs font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                            Verified
                          </span>
                          <span class="text-xs text-muted">
                            {formatRelativeTime(item.verifiedAt, { compact: true })}
                          </span>
                        </div>
                        <p class="mt-1 text-sm font-semibold text-base-content">
                          {formatActionName(item.capabilityName)} completed
                        </p>
                        <p class="mt-1 text-xs leading-5 text-muted">
                          {verificationTruthLabel('confirmed', item.evidenceClass)}
                        </p>
                        <p class="mt-1 truncate text-xs font-medium text-base-content">
                          {resource.label}
                          <Show when={resource.detail}>
                            {(detail) => <span class="text-muted"> · {detail()}</span>}
                          </Show>
                        </p>
                      </div>
                    </li>
                  );
                }}
              </For>
            </ol>
          </Show>
        </Show>
      </div>
    </section>
  );
}

export default PatrolRecentWorkPanel;
