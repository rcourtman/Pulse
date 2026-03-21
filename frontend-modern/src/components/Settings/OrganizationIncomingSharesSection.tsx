import { Component } from 'solid-js';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { normalizeOrganizationShareRole } from '@/utils/organizationRolePresentation';
import { formatOrgDate, roleBadgeClass } from '@/utils/orgUtils';
import { getOrganizationIncomingSharesEmptyState } from '@/utils/organizationSettingsPresentation';
import type { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

interface OrganizationIncomingSharesSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationIncomingSharesSection: Component<
  OrganizationIncomingSharesSectionProps
> = (props) => (
  <div class="space-y-2 p-4 sm:p-6">
    <h4 class="text-sm font-semibold text-base-content">Incoming Shares</h4>
    <div class="mt-4 -mx-4 sm:mx-0 overflow-x-auto w-full">
      <PulseDataGrid
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
            key: 'createdAt',
            label: 'Shared',
            render: (share) => <span class="text-muted">{formatOrgDate(share.createdAt)}</span>,
          },
        ]}
        keyExtractor={(share) => share.id}
        emptyState={getOrganizationIncomingSharesEmptyState()}
        desktopMinWidth="620px"
        class="border-x-0 sm:border-x sm:border-t sm:border-b sm:rounded-md border-y border-border"
      />
    </div>
  </div>
);
