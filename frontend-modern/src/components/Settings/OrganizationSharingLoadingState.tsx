import type { Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonBlock,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationSharingLoadingState: Component = () => (
  <SettingsLoadingSkeleton padding="panel" label="Loading organization sharing">
    <SettingsSkeletonBlock class="h-2 w-24" />
  </SettingsLoadingSkeleton>
);
