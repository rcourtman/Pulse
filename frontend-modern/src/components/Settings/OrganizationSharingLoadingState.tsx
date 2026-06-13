import { For, type Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonBlock,
  SettingsSkeletonTable,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationSharingLoadingState: Component = () => (
  <SettingsLoadingSkeleton padding="panel" label="Loading organization sharing">
    <div class="rounded-md border border-border p-4 space-y-3">
      <SettingsSkeletonBlock class="h-4 w-24" />

      <div class="grid gap-3 lg:grid-cols-2">
        <div class="space-y-2">
          <SettingsSkeletonBlock class="h-3 w-28" />
          <SettingsSkeletonBlock class="h-10 w-full" />
        </div>
        <div class="space-y-2">
          <SettingsSkeletonBlock class="h-3 w-20" />
          <SettingsSkeletonBlock class="h-10 w-full" />
        </div>
      </div>

      <div class="rounded-md border border-border p-3 space-y-2">
        <SettingsSkeletonBlock class="h-3 w-32" />
        <SettingsSkeletonBlock class="h-10 w-full" />
      </div>

      <div class="grid gap-3 lg:grid-cols-3">
        <For each={Array.from({ length: 3 })}>
          {() => (
            <div class="space-y-2">
              <SettingsSkeletonBlock class="h-3 w-24" />
              <SettingsSkeletonBlock class="h-10 w-full" />
            </div>
          )}
        </For>
      </div>

      <div class="flex justify-end">
        <SettingsSkeletonBlock class="h-10 w-28" />
      </div>
    </div>

    <SettingsSkeletonTable
      titleWidth="w-28"
      rows={3}
      cells={[
        { class: 'h-4 w-40' },
        { class: 'h-4 w-24' },
        { class: 'h-4 w-14', radius: 'full' },
        { class: 'h-4 w-24' },
      ]}
    />

    <SettingsSkeletonTable
      titleWidth="w-28"
      rows={3}
      cells={[
        { class: 'h-4 w-24' },
        { class: 'h-4 w-40' },
        { class: 'h-4 w-14', radius: 'full' },
        { class: 'h-4 w-24' },
      ]}
    />
  </SettingsLoadingSkeleton>
);
