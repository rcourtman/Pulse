import { Show, type Component } from 'solid-js';
import type { OrganizationShare } from '@/api/orgs';
import {
  OrganizationRoleBadge,
  OrganizationShareStatusBadge,
} from '@/components/shared/OrganizationBadges';
import { normalizeOrganizationShareRole } from '@/utils/organizationRolePresentation';
import { getOrganizationShareStatusDescription } from '@/utils/organizationSettingsPresentation';
import { formatOrgDate } from '@/utils/orgUtils';

export const SharingPeerCell: Component<{
  id: string;
  name?: string;
  accessRole: OrganizationShare['accessRole'];
}> = (props) => (
  <div class="flex min-w-0 flex-col items-start whitespace-normal [overflow-wrap:anywhere]">
    <div class="flex flex-wrap items-center gap-1">
      <Show when={props.name && props.name !== props.id}>
        <bdi class="text-base-content">{props.name}</bdi>
      </Show>
      <span data-share-access class="organization-share-mobile-access inline-flex">
        <span class="sr-only">Access:</span>
        <OrganizationRoleBadge role={normalizeOrganizationShareRole(props.accessRole)} size="xs" />
      </span>
    </div>
    <bdi class="text-xs text-muted whitespace-pre-wrap">{props.id}</bdi>
  </div>
);

export const renderShareResource = (share: OrganizationShare) => (
  <div class="flex min-w-0 flex-col whitespace-normal [overflow-wrap:anywhere]">
    <bdi class="text-base-content">{share.resourceName || share.resourceId}</bdi>
    <bdi class="text-xs text-muted whitespace-pre-wrap">
      {`${share.resourceType}:${share.resourceId}`}
    </bdi>
  </div>
);

export const renderShareAccess = (share: OrganizationShare) => (
  <OrganizationRoleBadge role={normalizeOrganizationShareRole(share.accessRole)} />
);

export const renderShareStatus = (share: OrganizationShare) => (
  <div class="organization-share-status flex min-w-0 flex-col items-start gap-1 whitespace-normal [overflow-wrap:anywhere]">
    <span class="organization-share-status-badge inline-flex items-center">
      <OrganizationShareStatusBadge status={share.status} wrap />
    </span>
    <span class="organization-share-status-description text-xs text-muted">
      {getOrganizationShareStatusDescription(share.status, share.acceptedAt, share.acceptedBy)}
    </span>
  </div>
);

export const renderShareRequested = (share: OrganizationShare) => (
  <span class="text-muted">{formatOrgDate(share.createdAt)}</span>
);
