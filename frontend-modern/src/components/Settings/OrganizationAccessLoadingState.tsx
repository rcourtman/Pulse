import type { Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonBlock,
  SettingsSkeletonTable,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationAccessLoadingState: Component = () => (
  <SettingsLoadingSkeleton label="Loading organization access">
    <div class="rounded-md border border-border p-4 space-y-3">
      <SettingsSkeletonBlock class="h-4 w-24" />
      <div class="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
        <SettingsSkeletonBlock class="h-10 w-full" />
        <SettingsSkeletonBlock class="h-10 w-28" />
        <SettingsSkeletonBlock class="h-10 w-16" />
      </div>
    </div>

    <SettingsSkeletonTable
      rows={4}
      rowLayoutClass="grid grid-cols-[1fr_auto_auto_auto] items-center gap-3"
      cells={[
        { class: 'h-4 w-40' },
        { class: 'h-7 w-24' },
        { class: 'h-4 w-20' },
        { class: 'ml-auto h-6 w-16' },
      ]}
    />
  </SettingsLoadingSkeleton>
);
