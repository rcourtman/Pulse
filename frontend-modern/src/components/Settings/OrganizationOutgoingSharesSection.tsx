import { Component, Show } from 'solid-js';
import './organizationSharingRows.css';
import { Button } from '@/components/shared/Button';
import {
  OrganizationRoleBadge,
  OrganizationShareStatusBadge,
} from '@/components/shared/OrganizationBadges';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { normalizeOrganizationShareRole } from '@/utils/organizationRolePresentation';
import { formatOrgDate } from '@/utils/orgUtils';
import {
  getOrganizationOutgoingSharesEmptyState,
  getOrganizationShareStatusDescription,
  getShareOrganizationLabel,
} from '@/utils/organizationSettingsPresentation';
import Trash2 from 'lucide-solid/icons/trash-2';
import type { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

interface OrganizationOutgoingSharesSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationOutgoingSharesSection: Component<
  OrganizationOutgoingSharesSectionProps
> = (props) => {
  return (
    <div class="space-y-2 p-4 sm:p-6">
      <h4 class="text-sm font-semibold text-base-content">Outgoing Shares</h4>
      <PulseDataGrid
        class={`organization-sharing-rows mt-4 ${!props.state.canManageCurrentOrg() ? 'organization-sharing-rows-readonly' : ''}`}
        data={props.state.outgoingShares()}
        columns={[
          {
            key: 'resourceName',
            label: 'Resource',
            render: (share) => (
              <div class="flex min-w-0 flex-col whitespace-normal [overflow-wrap:anywhere]">
                <bdi class="text-base-content">{share.resourceName || share.resourceId}</bdi>
                <bdi class="text-xs text-muted whitespace-pre-wrap">
                  {`${share.resourceType}:${share.resourceId}`}
                </bdi>
              </div>
            ),
          },
          {
            key: 'targetOrgId',
            label: 'Target Org',
            render: (share) => (
              <div class="flex min-w-0 flex-col items-start whitespace-normal [overflow-wrap:anywhere]">
                <div class="flex flex-wrap items-center gap-1">
                  <Show
                    when={
                      props.state.orgNameById().get(share.targetOrgId) &&
                      props.state.orgNameById().get(share.targetOrgId) !== share.targetOrgId
                    }
                  >
                    <bdi class="text-base-content">
                      {props.state.orgNameById().get(share.targetOrgId)}
                    </bdi>
                  </Show>
                  <span data-share-access class="organization-share-mobile-access inline-flex">
                    <span class="sr-only">Access:</span>
                    <OrganizationRoleBadge
                      role={normalizeOrganizationShareRole(share.accessRole)}
                      size="xs"
                    />
                  </span>
                </div>
                <bdi class="text-xs text-muted whitespace-pre-wrap">{share.targetOrgId}</bdi>
              </div>
            ),
          },
          {
            key: 'accessRole',
            label: 'Access',
            hiddenOnMobile: true,
            render: (share) => {
              const role = normalizeOrganizationShareRole(share.accessRole);
              return <OrganizationRoleBadge role={role} />;
            },
          },
          {
            key: 'status',
            label: 'Status',
            render: (share) => (
              <div class="organization-share-status flex min-w-0 flex-col items-start gap-1 whitespace-normal [overflow-wrap:anywhere]">
                <span class="organization-share-status-badge inline-flex items-center">
                  <OrganizationShareStatusBadge status={share.status} wrap />
                </span>
                <span class="organization-share-status-description text-xs text-muted">
                  {getOrganizationShareStatusDescription(
                    share.status,
                    share.acceptedAt,
                    share.acceptedBy,
                  )}
                </span>
              </div>
            ),
          },
          {
            key: 'createdAt',
            label: 'Requested',
            hiddenOnMobile: true,
            render: (share) => <span class="text-muted">{formatOrgDate(share.createdAt)}</span>,
          },
          {
            key: 'actions',
            label: 'Actions',
            align: 'right',
            render: (share) => (
              <Show when={props.state.canManageCurrentOrg()}>
                <Button
                  variant="dangerGhost"
                  size="xs"
                  class="gap-1"
                  aria-label={`Remove share of ${share.resourceType}:${share.resourceId} to ${getShareOrganizationLabel(share.targetOrgId, props.state.orgNameById().get(share.targetOrgId))}`}
                  onClick={() => {
                    void props.state.deleteShare(share);
                  }}
                  disabled={props.state.saving()}
                >
                  <Trash2 class="w-3.5 h-3.5" />
                  Remove
                </Button>
              </Show>
            ),
          },
        ]}
        keyExtractor={(share) => share.id}
        emptyState={getOrganizationOutgoingSharesEmptyState()}
        desktopMinWidth="100%"
      />
    </div>
  );
};
