import type { Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonBlock,
  SettingsSkeletonMetricGrid,
  SettingsSkeletonTable,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationOverviewLoadingState: Component = () => (
  <SettingsLoadingSkeleton padding="panel" label="Loading organization overview">
    <SettingsSkeletonMetricGrid valueWidth="w-28" />

    <div class="space-y-2">
      <SettingsSkeletonBlock class="h-4 w-24" />
      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <SettingsSkeletonBlock class="h-10 w-full" />
        <SettingsSkeletonBlock class="h-10 w-20" />
      </div>
    </div>

    <SettingsSkeletonTable
      titleWidth="w-24"
      rows={3}
      cells={[{ class: 'h-4 w-40' }, { class: 'h-4 w-14', radius: 'full' }, { class: 'h-4 w-24' }]}
    />
  </SettingsLoadingSkeleton>
);
