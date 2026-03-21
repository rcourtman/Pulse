import { Component, For } from 'solid-js';

export const OrganizationAccessLoadingState: Component = () => (
  <div class="space-y-5">
    <div class="rounded-md border border-border p-4 space-y-3">
      <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
      <div class="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
        <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
        <div class="h-10 w-28 animate-pulse rounded bg-surface-hover" />
        <div class="h-10 w-16 animate-pulse rounded bg-surface-hover" />
      </div>
    </div>

    <div class="overflow-hidden rounded-md border border-border">
      <div class="h-10 w-full animate-pulse bg-surface-alt" />
      <For each={Array.from({ length: 4 })}>
        {() => (
          <div class="border-t border-border-subtle px-3 py-3">
            <div class="grid grid-cols-[1fr_auto_auto_auto] items-center gap-3">
              <div class="h-4 w-40 animate-pulse rounded bg-surface-hover" />
              <div class="h-7 w-24 animate-pulse rounded bg-surface-hover" />
              <div class="h-4 w-20 animate-pulse rounded bg-surface-hover" />
              <div class="ml-auto h-6 w-16 animate-pulse rounded bg-surface-hover" />
            </div>
          </div>
        )}
      </For>
    </div>
  </div>
);
