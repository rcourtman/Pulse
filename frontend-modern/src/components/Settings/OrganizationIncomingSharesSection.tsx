import { Component, Show } from 'solid-js';
import './organizationSharingRows.css';
import { Button } from '@/components/shared/Button';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  getOrganizationIncomingSharesEmptyState,
  getOrganizationShareActionLabel,
} from '@/utils/organizationSettingsPresentation';
import {
  renderShareAccess,
  renderShareRequested,
  renderShareResource,
  renderShareStatus,
  SharingPeerCell,
} from './organizationSharingCells';
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
        class={`organization-sharing-rows mt-4 ${!props.state.canManageCurrentOrg() ? 'organization-sharing-rows-readonly' : ''}`}
        data={props.state.incomingShares()}
        columns={[
          {
            key: 'sourceOrg',
            label: 'Source Org',
            render: (share) => (
              <SharingPeerCell
                id={share.sourceOrgId}
                name={share.sourceOrgName}
                accessRole={share.accessRole}
              />
            ),
          },
          {
            key: 'resource',
            label: 'Resource',
            render: renderShareResource,
          },
          {
            key: 'accessRole',
            label: 'Access',
            hiddenOnMobile: true,
            render: renderShareAccess,
          },
          {
            key: 'status',
            label: 'Status',
            render: renderShareStatus,
          },
          {
            key: 'createdAt',
            label: 'Requested',
            hiddenOnMobile: true,
            render: renderShareRequested,
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
                      data-org-sharing-action="accept-incoming"
                      data-org-sharing-target={share.id}
                      aria-label={getOrganizationShareActionLabel(
                        'Accept',
                        share,
                        'from',
                        share.sourceOrgId,
                        share.sourceOrgName,
                      )}
                      onClick={() => {
                        void props.state.acceptIncomingShare(share);
                      }}
                      disabled={props.state.saving() || props.state.sharingChangesPaused()}
                    >
                      Accept
                    </Button>
                  </Show>
                  <Button
                    variant="dangerGhost"
                    size="xs"
                    data-org-sharing-action={
                      share.status === 'pending' ? 'decline-incoming' : 'remove-incoming'
                    }
                    data-org-sharing-target={share.id}
                    aria-label={getOrganizationShareActionLabel(
                      share.status === 'pending' ? 'Decline' : 'Remove',
                      share,
                      'from',
                      share.sourceOrgId,
                      share.sourceOrgName,
                    )}
                    onClick={() => {
                      void props.state.declineIncomingShare(share);
                    }}
                    disabled={props.state.saving() || props.state.sharingChangesPaused()}
                  >
                    {share.status === 'pending' ? 'Decline' : 'Remove'}
                  </Button>
                </div>
              </Show>
            ),
          },
        ]}
        keyExtractor={(share) => share.id}
        emptyState={getOrganizationIncomingSharesEmptyState(
          props.state.unavailableLists()?.incoming,
        )}
        desktopMinWidth="100%"
      />
    </div>
  );
};
