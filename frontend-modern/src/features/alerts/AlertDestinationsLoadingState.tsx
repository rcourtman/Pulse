export function AlertDestinationsLoadingState() {
  return (
    <div class="flex w-full flex-col gap-6 animate-pulse pointer-events-none select-none md:gap-8">
      <div class="rounded-lg border border-border bg-surface p-6 space-y-4">
        <div class="flex items-center justify-between">
          <div class="space-y-2">
            <div class="h-5 w-40 rounded bg-surface-hover" />
            <div class="h-3 w-64 rounded bg-surface-hover" />
          </div>
          <div class="h-6 w-12 rounded-full bg-surface-hover" />
        </div>
        <div class="space-y-3">
          <div class="h-4 w-24 rounded bg-surface-hover" />
          <div class="h-10 w-full rounded bg-surface-hover" />
          <div class="h-4 w-32 rounded bg-surface-hover" />
          <div class="h-10 w-full rounded bg-surface-hover" />
        </div>
      </div>
      <div class="rounded-lg border border-border bg-surface p-6 space-y-4">
        <div class="flex items-center justify-between">
          <div class="space-y-2">
            <div class="h-5 w-44 rounded bg-surface-hover" />
            <div class="h-3 w-72 rounded bg-surface-hover" />
          </div>
          <div class="h-6 w-12 rounded-full bg-surface-hover" />
        </div>
        <div class="space-y-3">
          <div class="h-4 w-28 rounded bg-surface-hover" />
          <div class="h-10 w-full rounded bg-surface-hover" />
        </div>
      </div>
      <div class="rounded-lg border border-border bg-surface p-6 space-y-4">
        <div class="flex items-center justify-between">
          <div class="space-y-2">
            <div class="h-5 w-28 rounded bg-surface-hover" />
            <div class="h-3 w-56 rounded bg-surface-hover" />
          </div>
          <div class="h-4 w-20 rounded bg-surface-hover" />
        </div>
        <div class="h-10 w-full rounded bg-surface-hover" />
      </div>
    </div>
  );
}
