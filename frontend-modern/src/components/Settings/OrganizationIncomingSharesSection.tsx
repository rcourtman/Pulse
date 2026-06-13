import { Component, Show } from 'solid-js';
import { Button } from '@/components/shared/Button';
import {
  OrganizationRoleBadge,
  OrganizationShareStatusBadge,
} from '@/components/shared/OrganizationBadges';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { normalizeOrganizationShareRole } from '@/utils/organizationRolePresentation';
import { formatOrgDate } from '@/utils/orgUtils';
import {
  getOrganizationIncomingSharesEmptyState,
  getOrganizationShareStatusDescription,
} from '@/utils/organizationSettingsPresentation';
import type { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

interface OrganizationIncomingSharesSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationIncomingSharesSection: Component<
  OrganizationIncomingSharesSectionProps
> = (props) => {
  return (
    <div class="space-y-2 p-4 sm:p-6">
      <h4 class="text-sm font-semibold text-base-content">Incoming Shares</h4>
      <PulseDataGrid
        class="mt-4"
        data={props.state.incomingShares()}
        columns={[
          {
            key: 'sourceOrg',
            label: 'Source Org',
            render: (share) => (
              <span class="text-base-content">{share.sourceOrgName || share.sourceOrgId}</span>
            ),
          },
          {
            key: 'resource',
            label: 'Resource',
            render: (share) => (
              <div class="flex flex-col">
                <span class="text-base-content">{share.resourceName || share.resourceId}</span>
                <span class="text-xs text-muted">
                  {share.resourceType}:{share.resourceId}
                </span>
              </div>
            ),
          },
          {
            key: 'accessRole',
            label: 'Access',
            render: (share) => {
              const role = normalizeOrganizationShareRole(share.accessRole);
              return <OrganizationRoleBadge role={role} />;
            },
          },
          {
            key: 'status',
            label: 'Status',
            render: (share) => (
              <div class="flex flex-col gap-1">
                <OrganizationShareStatusBadge status={share.status} />
                <span class="text-xs text-muted">
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
            render: (share) => <span class="text-muted">{formatOrgDate(share.createdAt)}</span>,
          },
          {
            key: 'actions',
            label: 'Actions',
            align: 'right',
            render: (share) => (
              <Show when={props.state.canManageCurrentOrg()}>
                <div class="flex items-center justify-end gap-2">
                  <Show when={share.status === 'pending'}>
                    <Button
                      variant="successGhost"
                      size="xs"
                      onClick={() => {
                        void props.state.acceptIncomingShare(share);
                      }}
                      disabled={props.state.saving()}
                    >
                      Accept
                    </Button>
                  </Show>
                  <Button
                    variant="dangerGhost"
                    size="xs"
                    onClick={() => {
                      void props.state.declineIncomingShare(share);
                    }}
                    disabled={props.state.saving()}
                  >
                    {share.status === 'pending' ? 'Decline' : 'Remove'}
                  </Button>
                </div>
              </Show>
            ),
          },
        ]}
        keyExtractor={(share) => share.id}
        emptyState={getOrganizationIncomingSharesEmptyState()}
        desktopMinWidth="980px"
      />
    </div>
  );
};
