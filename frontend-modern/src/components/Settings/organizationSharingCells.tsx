import { Show, createMemo, type Component } from 'solid-js';
import type { OrganizationShare } from '@/api/orgs';
import {
  getOrganizationRoleBadgeTone,
  OrganizationShareStatusBadge,
} from '@/components/shared/OrganizationBadges';
import { MetadataBadge, type MetadataBadgeSize } from '@/components/shared/MetadataBadge';
import { getOrganizationShareRoleOption } from '@/utils/organizationRolePresentation';
import { getOrganizationShareStatusDescription } from '@/utils/organizationSettingsPresentation';
import { formatOrgDate } from '@/utils/orgUtils';

const ShareAudienceBadge: Component<{
  role: OrganizationShare['accessRole'];
  size?: MetadataBadgeSize;
}> = (props) => {
  const option = createMemo(() => getOrganizationShareRoleOption(props.role));
  return (
    <MetadataBadge
      tone={getOrganizationRoleBadgeTone(option()?.value ?? '')}
      size={props.size}
      wrap
    >
      {option()?.label ?? 'Unknown'}
    </MetadataBadge>
  );
};

export const SharingPeerCell: Component<{
  id: string;
  name?: string;
  accessRole: OrganizationShare['accessRole'];
}> = (props) => (
  <div class="flex min-w-0 flex-col items-start whitespace-normal [overflow-wrap:anywhere]">
    <Show when={props.name && props.name !== props.id}>
      <bdi class="text-base-content">{props.name}</bdi>
    </Show>
    <bdi class="text-xs text-muted whitespace-pre-wrap">{props.id}</bdi>
    <span
      data-share-access
      class="organization-share-mobile-access inline-flex max-w-full flex-wrap items-center gap-x-1"
    >
      <span class="text-[10px] text-muted">Target audience:</span>
      <ShareAudienceBadge role={props.accessRole} size="xs" />
    </span>
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

export const renderShareAudience = (share: OrganizationShare) => (
  <ShareAudienceBadge role={share.accessRole} />
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
