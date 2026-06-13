import type { Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonMetricGrid,
  SettingsSkeletonProgressCard,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationBillingLoadingState: Component = () => (
  <SettingsLoadingSkeleton label="Loading organization billing">
    <SettingsSkeletonMetricGrid valueWidth="w-24" />

    <SettingsSkeletonProgressCard rows={2} titleWidth="w-36" />

    <SettingsSkeletonMetricGrid columns="two" count={2} labelWidth="w-24" valueWidth="w-40" />
  </SettingsLoadingSkeleton>
);
