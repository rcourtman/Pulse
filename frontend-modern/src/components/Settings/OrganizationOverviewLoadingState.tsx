import { Component, For } from 'solid-js';

export const OrganizationOverviewLoadingState: Component = () => (
  <div class="space-y-5 p-4 sm:p-6">
    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <For each={Array.from({ length: 4 })}>
        {() => (
          <div class="rounded-md border border-border p-3 space-y-2">
            <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
            <div class="h-5 w-28 animate-pulse rounded bg-surface-hover" />
          </div>
        )}
      </For>
    </div>

    <div class="space-y-2">
      <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
        <div class="h-10 w-20 animate-pulse rounded bg-surface-hover" />
      </div>
    </div>

    <div class="space-y-2">
      <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
      <div class="overflow-hidden rounded-md border border-border">
        <div class="h-10 w-full animate-pulse bg-surface-alt" />
        <For each={Array.from({ length: 3 })}>
          {() => (
            <div class="border-t border-border-subtle px-3 py-3">
              <div class="flex items-center gap-3">
                <div class="h-4 w-40 animate-pulse rounded bg-surface-hover" />
                <div class="h-4 w-14 animate-pulse rounded-full bg-surface-hover" />
                <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
              </div>
            </div>
          )}
        </For>
      </div>
    </div>
  </div>
);
