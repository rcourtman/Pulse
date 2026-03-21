import { Component, For } from 'solid-js';

export const OrganizationSharingLoadingState: Component = () => (
  <div class="space-y-5 p-4 sm:p-6">
    <div class="rounded-md border border-border p-4 space-y-3">
      <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />

      <div class="grid gap-3 lg:grid-cols-2">
        <div class="space-y-2">
          <div class="h-3 w-28 animate-pulse rounded bg-surface-hover" />
          <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
        </div>
        <div class="space-y-2">
          <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
          <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
        </div>
      </div>

      <div class="rounded-md border border-border p-3 space-y-2">
        <div class="h-3 w-32 animate-pulse rounded bg-surface-hover" />
        <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
      </div>

      <div class="grid gap-3 lg:grid-cols-3">
        <For each={Array.from({ length: 3 })}>
          {() => (
            <div class="space-y-2">
              <div class="h-3 w-24 animate-pulse rounded bg-surface-hover" />
              <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
            </div>
          )}
        </For>
      </div>

      <div class="flex justify-end">
        <div class="h-10 w-28 animate-pulse rounded bg-surface-hover" />
      </div>
    </div>

    <div class="space-y-2">
      <div class="h-4 w-28 animate-pulse rounded bg-surface-hover" />
      <div class="overflow-hidden rounded-md border border-border">
        <div class="h-10 w-full animate-pulse bg-surface-alt" />
        <For each={Array.from({ length: 3 })}>
          {() => (
            <div class="border-t border-border-subtle px-3 py-3">
              <div class="flex items-center gap-3">
                <div class="h-4 w-40 animate-pulse rounded bg-surface-hover" />
                <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                <div class="h-4 w-14 animate-pulse rounded-full bg-surface-hover" />
                <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
              </div>
            </div>
          )}
        </For>
      </div>
    </div>

    <div class="space-y-2">
      <div class="h-4 w-28 animate-pulse rounded bg-surface-hover" />
      <div class="overflow-hidden rounded-md border border-border">
        <div class="h-10 w-full animate-pulse bg-surface-alt" />
        <For each={Array.from({ length: 3 })}>
          {() => (
            <div class="border-t border-border-subtle px-3 py-3">
              <div class="flex items-center gap-3">
                <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
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
