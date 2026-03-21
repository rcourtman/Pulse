import { Component, Show } from 'solid-js';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { normalizeOrganizationShareRole } from '@/utils/organizationRolePresentation';
import { formatOrgDate, roleBadgeClass } from '@/utils/orgUtils';
import { getOrganizationOutgoingSharesEmptyState } from '@/utils/organizationSettingsPresentation';
import Trash2 from 'lucide-solid/icons/trash-2';
import type { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

interface OrganizationOutgoingSharesSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationOutgoingSharesSection: Component<
  OrganizationOutgoingSharesSectionProps
> = (props) => (
  <div class="space-y-2 p-4 sm:p-6">
    <h4 class="text-sm font-semibold text-base-content">Outgoing Shares</h4>
    <div class="mt-4 -mx-4 sm:mx-0 overflow-x-auto w-full">
      <PulseDataGrid
        data={props.state.outgoingShares()}
        columns={[
          {
            key: 'resourceName',
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
            key: 'targetOrgId',
            label: 'Target Org',
            render: (share) => (
              <span class="text-base-content">
                {props.state.orgNameById().get(share.targetOrgId) || share.targetOrgId}
              </span>
            ),
          },
          {
            key: 'accessRole',
            label: 'Access',
            render: (share) => {
              const role = normalizeOrganizationShareRole(share.accessRole);
              return (
                <span
                  class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(role)}`}
                >
                  {role}
                </span>
              );
            },
          },
          {
            key: 'createdAt',
            label: 'Created',
            render: (share) => <span class="text-muted">{formatOrgDate(share.createdAt)}</span>,
          },
          {
            key: 'actions',
            label: 'Actions',
            align: 'right',
            render: (share) => (
              <Show when={props.state.canManageCurrentOrg()}>
                <button
                  type="button"
                  onClick={() => {
                    void props.state.deleteShare(share);
                  }}
                  disabled={props.state.saving()}
                  class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                  Remove
                </button>
              </Show>
            ),
          },
        ]}
        keyExtractor={(share) => share.id}
        emptyState={getOrganizationOutgoingSharesEmptyState()}
        desktopMinWidth="760px"
        class="border-x-0 sm:border-x sm:border-t sm:border-b sm:rounded-md border-y border-border"
      />
    </div>
  </div>
);
