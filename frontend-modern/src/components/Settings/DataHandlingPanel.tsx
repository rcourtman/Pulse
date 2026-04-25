import { For, Show, createMemo, type Component } from 'solid-js';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import Cloud from 'lucide-solid/icons/cloud';
import EyeOff from 'lucide-solid/icons/eye-off';
import Lock from 'lucide-solid/icons/lock';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import Button from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  buildDataHandlingPanelModel,
  type DataHandlingPostureItem,
} from './dataHandlingPanelModel';

const meterClassByTone: Record<DataHandlingPostureItem['tone'], string> = {
  neutral: 'bg-base-content/40',
  info: 'bg-sky-500 dark:bg-sky-400',
  success: 'bg-emerald-500 dark:bg-emerald-400',
  warning: 'bg-amber-500 dark:bg-amber-400',
  danger: 'bg-rose-500 dark:bg-rose-400',
};

const badgeClassByTone: Record<DataHandlingPostureItem['tone'], string> = {
  neutral: 'border border-border bg-surface text-muted',
  info: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-200',
  success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-200',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
  danger: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-200',
};

const formatCount = (value: number): string => new Intl.NumberFormat().format(value);

const errorMessageFor = (error: unknown): string => {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return 'Unable to load resource policy posture.';
};

const PostureMeter: Component<{ item: DataHandlingPostureItem }> = (props) => (
  <div class="rounded-md border border-border bg-surface-alt px-4 py-3">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="text-sm font-semibold text-base-content">{props.item.label}</p>
        <p class="mt-1 text-xs text-muted">{props.item.description}</p>
      </div>
      <span
        class={`shrink-0 rounded px-2 py-0.5 text-xs font-semibold ${badgeClassByTone[props.item.tone]}`}
      >
        {formatCount(props.item.count)}
      </span>
    </div>
    <div
      class="mt-3 h-2 overflow-hidden rounded bg-surface-hover"
      aria-label={`${props.item.label} ${props.item.percentage}%`}
    >
      <div
        class={`h-full rounded ${meterClassByTone[props.item.tone]}`}
        style={{ width: `${props.item.percentage}%` }}
      />
    </div>
  </div>
);

export const DataHandlingPanel: Component = () => {
  const resources = useUnifiedResources({ query: '', cacheKey: 'all-resources' });
  const model = createMemo(() => buildDataHandlingPanelModel(resources.policyPosture()));
  const errorMessage = createMemo(() => {
    const error = resources.error();
    return error ? errorMessageFor(error) : '';
  });

  return (
    <SettingsPanel
      title="Data Handling"
      description="Review resource classifications, handling boundaries, and redaction coverage."
      bodyClass="space-y-5"
      action={
        <Button
          type="button"
          variant="outline"
          size="sm"
          isLoading={resources.loading()}
          onClick={() => {
            void resources.refetch();
          }}
        >
          <RefreshCw class="mr-2 h-4 w-4" aria-hidden="true" />
          Refresh
        </Button>
      }
    >
      <Show when={errorMessage()}>
        {(message) => (
          <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-100">
            <div class="flex items-start gap-3">
              <AlertTriangle class="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <p>{message()}</p>
            </div>
          </div>
        )}
      </Show>

      <Show
        when={!resources.loading() || resources.policyPosture()}
        fallback={
          <div class="grid gap-3 md:grid-cols-3">
            {[1, 2, 3].map(() => (
              <div class="min-h-28 rounded-md border border-border bg-surface-alt p-4 animate-pulse">
                <div class="h-4 w-24 rounded bg-surface-hover" />
                <div class="mt-4 h-8 w-16 rounded bg-surface-hover" />
                <div class="mt-3 h-3 w-32 rounded bg-surface-hover" />
              </div>
            ))}
          </div>
        }
      >
        <div class="grid gap-3 md:grid-cols-3">
          <div class="rounded-md border border-border bg-surface-alt p-4">
            <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
              <ShieldCheck class="h-4 w-4 text-emerald-600 dark:text-emerald-300" />
              Governed Resources
            </div>
            <p class="mt-3 text-3xl font-semibold text-base-content">
              {formatCount(model().totalResources)}
            </p>
            <p class="mt-1 text-xs text-muted">Resources carrying policy metadata.</p>
          </div>
          <div class="rounded-md border border-border bg-surface-alt p-4">
            <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
              <Lock class="h-4 w-4 text-teal-600 dark:text-teal-300" />
              Local-Only
            </div>
            <p class="mt-3 text-3xl font-semibold text-base-content">
              {formatCount(model().localOnlyResources)}
            </p>
            <p class="mt-1 text-xs text-muted">Resources kept inside this Pulse instance.</p>
          </div>
          <div class="rounded-md border border-border bg-surface-alt p-4">
            <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
              <EyeOff class="h-4 w-4 text-muted" />
              Redaction Hints
            </div>
            <p class="mt-3 text-3xl font-semibold text-base-content">
              {formatCount(model().redactionHintCount)}
            </p>
            <p class="mt-1 text-xs text-muted">Field-level protections applied by policy.</p>
          </div>
        </div>

        <Show
          when={model().hasResources}
          fallback={
            <div class="rounded-md border border-border bg-surface-alt px-4 py-5">
              <p class="text-sm font-semibold text-base-content">No governed resources yet</p>
              <p class="mt-1 text-sm text-muted">
                Resource policy posture will appear after Pulse has canonical resource data.
              </p>
            </div>
          }
        >
          <div class="grid gap-4 xl:grid-cols-2">
            <section class="space-y-3">
              <div>
                <h3 class="text-sm font-semibold text-base-content">Sensitivity</h3>
                <p class="mt-1 text-xs text-muted">Classification applied to monitored resources.</p>
              </div>
              <div class="space-y-2">
                <For each={model().sensitivityItems}>{(item) => <PostureMeter item={item} />}</For>
              </div>
            </section>

            <section class="space-y-3">
              <div class="flex items-start gap-2">
                <Cloud class="mt-0.5 h-4 w-4 text-sky-600 dark:text-sky-300" />
                <div>
                  <h3 class="text-sm font-semibold text-base-content">Handling Boundary</h3>
                  <p class="mt-1 text-xs text-muted">
                    Routing policy for summaries and local-only resource data.
                  </p>
                </div>
              </div>
              <div class="space-y-2">
                <For each={model().routingItems}>{(item) => <PostureMeter item={item} />}</For>
              </div>
            </section>
          </div>

          <section class="rounded-md border border-border bg-surface-alt px-4 py-4">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">Redaction Coverage</h3>
                <p class="mt-1 text-xs text-muted">
                  Fields removed when a resource requires a guarded display.
                </p>
              </div>
              <Show when={!model().hasRedactions}>
                <span class="w-fit rounded bg-surface px-2 py-1 text-xs font-medium text-muted">
                  No active redaction hints
                </span>
              </Show>
            </div>
            <Show when={model().hasRedactions}>
              <div class="mt-4 flex flex-wrap gap-2">
                <For each={model().redactionItems}>
                  {(item) => (
                    <span
                      class={`inline-flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium ${badgeClassByTone[item.tone]}`}
                      title={item.description}
                    >
                      {item.label}
                      <span class="font-semibold">{formatCount(item.count)}</span>
                    </span>
                  )}
                </For>
              </div>
            </Show>
          </section>
        </Show>
      </Show>
    </SettingsPanel>
  );
};

export default DataHandlingPanel;
