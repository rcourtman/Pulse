import type { Component } from 'solid-js';

export const OrganizationBillingLoadingState: Component = () => (
  <div class="space-y-5">
    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      {Array.from({ length: 4 }).map(() => (
        <div class="rounded-md border border-border p-3 space-y-2">
          <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
          <div class="h-5 w-24 animate-pulse rounded bg-surface-hover" />
        </div>
      ))}
    </div>

    <div class="space-y-3 rounded-md border border-border p-4">
      <div class="h-4 w-36 animate-pulse rounded bg-surface-hover" />
      {Array.from({ length: 2 }).map(() => (
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <div class="h-3 w-14 animate-pulse rounded bg-surface-hover" />
            <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
          </div>
          <div class="h-2 w-full animate-pulse rounded bg-surface-hover" />
        </div>
      ))}
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      {Array.from({ length: 2 }).map(() => (
        <div class="rounded-md border border-border p-3 space-y-2">
          <div class="h-3 w-24 animate-pulse rounded bg-surface-hover" />
          <div class="h-5 w-40 animate-pulse rounded bg-surface-hover" />
        </div>
      ))}
    </div>
  </div>
);
