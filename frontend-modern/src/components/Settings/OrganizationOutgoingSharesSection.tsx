import { Component, Show } from 'solid-js';
import './organizationSharingRows.css';
import { Button } from '@/components/shared/Button';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  getOrganizationOutgoingSharesEmptyState,
  getOrganizationShareActionLabel,
} from '@/utils/organizationSettingsPresentation';
import Trash2 from 'lucide-solid/icons/trash-2';
import {
  renderShareAudience,
  renderShareRequested,
  renderShareResource,
  renderShareStatus,
  SharingPeerCell,
} from './organizationSharingCells';
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
            render: renderShareResource,
          },
          {
            key: 'targetOrgId',
            label: 'Target Org',
            render: (share) => (
              <SharingPeerCell
                id={share.targetOrgId}
                name={props.state.orgNameById().get(share.targetOrgId)}
                accessRole={share.accessRole}
              />
            ),
          },
          {
            key: 'accessRole',
            label: 'Target audience',
            hiddenOnMobile: true,
            render: renderShareAudience,
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
                <Button
                  variant="dangerGhost"
                  size="xs"
                  class="gap-1"
                  data-org-sharing-action="remove-outgoing"
                  data-org-sharing-target={share.id}
                  aria-label={getOrganizationShareActionLabel(
                    'Remove',
                    share,
                    'to',
                    share.targetOrgId,
                    props.state.orgNameById().get(share.targetOrgId),
                  )}
                  onClick={() => {
                    void props.state.deleteShare(share);
                  }}
                  disabled={props.state.saving() || props.state.sharingChangesPaused()}
                >
                  <Trash2 class="w-3.5 h-3.5" />
                  Remove
                </Button>
              </Show>
            ),
          },
        ]}
        keyExtractor={(share) => share.id}
        emptyState={getOrganizationOutgoingSharesEmptyState(
          props.state.unavailableLists()?.outgoing,
        )}
        desktopMinWidth="100%"
      />
    </div>
  );
};
