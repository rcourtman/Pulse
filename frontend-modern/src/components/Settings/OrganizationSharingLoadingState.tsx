import type { Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonBlock,
  SettingsSkeletonTable,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationSharingLoadingState: Component = () => (
  <SettingsLoadingSkeleton padding="panel" label="Loading organization sharing">
    <div class="rounded-md border border-border p-4 space-y-3">
      <SettingsSkeletonBlock class="h-4 w-28" />
      <div class="grid gap-3 lg:grid-cols-2">
        <SettingsSkeletonBlock class="h-10 w-full" />
        <SettingsSkeletonBlock class="h-10 w-full" />
      </div>
      <SettingsSkeletonBlock class="h-16 w-full" />
      <div class="flex justify-end">
        <SettingsSkeletonBlock class="h-9 w-28" />
      </div>
    </div>

    <SettingsSkeletonTable
      titleWidth="w-32"
      rows={3}
      cells={[
        { class: 'h-4 w-36' },
        { class: 'h-4 w-28' },
        { class: 'h-5 w-20', radius: 'full' },
        { class: 'ml-auto h-6 w-16' },
      ]}
    />

    <SettingsSkeletonTable
      titleWidth="w-32"
      rows={2}
      cells={[
        { class: 'h-4 w-36' },
        { class: 'h-4 w-28' },
        { class: 'h-5 w-20', radius: 'full' },
        { class: 'ml-auto h-6 w-16' },
      ]}
    />
  </SettingsLoadingSkeleton>
);
