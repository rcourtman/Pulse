import type { Component } from 'solid-js';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonMetricGrid,
  SettingsSkeletonProgressCard,
} from '@/components/shared/SettingsLoadingSkeleton';

export const OrganizationBillingLoadingState: Component<{
  orgID: string;
  descriptionID: string;
}> = (props) => (
  <SettingsLoadingSkeleton label="Loading organization billing">
    <p id={props.descriptionID} class="text-sm text-muted">
      Checking billing and plan details for{' '}
      <bdi class="font-medium break-words text-base-content">{`${props.orgID}.`}</bdi>
    </p>
    <SettingsSkeletonMetricGrid valueWidth="w-24" />

    <SettingsSkeletonProgressCard rows={2} titleWidth="w-36" />

    <SettingsSkeletonMetricGrid columns="two" count={2} labelWidth="w-24" valueWidth="w-40" />
  </SettingsLoadingSkeleton>
);
