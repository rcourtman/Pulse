import { Component, Show } from 'solid-js';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { normalizeOrganizationShareRole } from '@/utils/organizationRolePresentation';
import { formatOrgDate, roleBadgeClass } from '@/utils/orgUtils';
import {
  getOrganizationIncomingSharesEmptyState,
  getOrganizationShareStatusDescription,
  getOrganizationShareStatusLabel,
} from '@/utils/organizationSettingsPresentation';
import type { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

interface OrganizationIncomingSharesSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationIncomingSharesSection: Component<
  OrganizationIncomingSharesSectionProps
> = (props) => {
  const statusBadgeClass = (status: 'pending' | 'accepted') =>
    status === 'pending'
      ? 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
      : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200';

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
            key: 'status',
            label: 'Status',
            render: (share) => (
              <div class="flex flex-col gap-1">
                <span
                  class={`inline-flex w-fit rounded-full px-2 py-0.5 text-xs font-medium ${statusBadgeClass(share.status)}`}
                >
                  {getOrganizationShareStatusLabel(share.status)}
                </span>
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
                    <button
                      type="button"
                      onClick={() => {
                        void props.state.acceptIncomingShare(share);
                      }}
                      disabled={props.state.saving()}
                      class="inline-flex items-center rounded-md px-2 py-1 text-xs font-medium text-emerald-700 hover:bg-emerald-50 dark:text-emerald-300 dark:hover:bg-emerald-900 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      Accept
                    </button>
                  </Show>
                  <button
                    type="button"
                    onClick={() => {
                      void props.state.declineIncomingShare(share);
                    }}
                    disabled={props.state.saving()}
                    class="inline-flex items-center rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {share.status === 'pending' ? 'Decline' : 'Remove'}
                  </button>
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
