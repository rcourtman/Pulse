import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';

interface ResourceDetailDrawerDebugTabProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
}

export const ResourceDetailDrawerDebugTab: Component<ResourceDetailDrawerDebugTabProps> = (
  props,
) => {
  const { drawer } = props;

  return (
    <div class="space-y-3">
      <div class="flex items-center justify-between gap-3">
        <div class="text-xs text-muted">
          Debug mode is enabled via localStorage (<code>pulse_debug_mode</code>).
        </div>
        <button
          type="button"
          onClick={drawer.handleCopyJson}
          class="rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
        >
          {drawer.copied() ? 'Copied' : 'Copy JSON'}
        </button>
      </div>

      <div class="mt-3 space-y-4">
        <div>
          <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
            Unified Resource
          </div>
          <pre class="max-h-[280px] overflow-auto rounded-md bg-base p-3 text-[11px] text-base-content">
            {JSON.stringify(props.resource, null, 2)}
          </pre>
        </div>

        <div>
          <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
            Identity Matching
          </div>
          <pre class="max-h-[220px] overflow-auto rounded-md bg-base p-3 text-[11px] text-base-content">
            {JSON.stringify(
              {
                identity: props.resource.identity,
                matchInfo: drawer.identityMatchInfo(),
              },
              null,
              2,
            )}
          </pre>
        </div>

        <div>
          <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
            Sources
          </div>
          <div class="space-y-2">
            <For each={drawer.sourceSections()}>
              {(section) => {
                const status = drawer.sourceStatus()[section.id];
                const lastSeenText = formatRelativeTime(status?.lastSeen);
                return (
                  <details class="rounded-md border border-border bg-surface p-3">
                    <summary class="flex cursor-pointer list-none items-center justify-between text-sm font-medium text-base-content">
                      <span>{section.label}</span>
                      <span class="text-[11px] text-muted">
                        {status?.status ?? 'unknown'}
                        {lastSeenText ? ` • ${lastSeenText}` : ''}
                      </span>
                    </summary>
                    <Show when={status?.error}>
                      <div class="mt-2 text-[11px] text-amber-600 dark:text-amber-300">
                        {status?.error}
                      </div>
                    </Show>
                    <pre class="mt-3 max-h-[220px] overflow-auto rounded-md bg-base p-3 text-[11px] text-base-content">
                      {JSON.stringify(section.payload ?? {}, null, 2)}
                    </pre>
                  </details>
                );
              }}
            </For>
          </div>
        </div>
      </div>
    </div>
  );
};
