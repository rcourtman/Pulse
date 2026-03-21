import { Component } from 'solid-js';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { getOrganizationOverviewMembersEmptyState } from '@/utils/organizationSettingsPresentation';
import { formatOrgDate, normalizeRole, roleBadgeClass } from '@/utils/orgUtils';
import type { useOrganizationOverviewPanelState } from './useOrganizationOverviewPanelState';

interface OrganizationOverviewMembersSectionProps {
  state: ReturnType<typeof useOrganizationOverviewPanelState>;
}

export const OrganizationOverviewMembersSection: Component<
  OrganizationOverviewMembersSectionProps
> = (props) => (
  <div class="space-y-2 p-4 sm:p-6">
    <h4 class="text-sm font-semibold text-base-content">Membership</h4>
    <div class="mt-4 -mx-4 sm:mx-0 overflow-x-auto w-full">
      <PulseDataGrid
        data={props.state.members()}
        columns={[
          {
            key: 'userId',
            label: 'User',
            render: (member) => <span class="text-base-content">{member.userId}</span>,
          },
          {
            key: 'role',
            label: 'Role',
            render: (member) => {
              const role = normalizeRole(member.role);
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
            key: 'addedAt',
            label: 'Added',
            render: (member) => <span class="text-muted">{formatOrgDate(member.addedAt)}</span>,
          },
        ]}
        keyExtractor={(member) => member.userId}
        emptyState={getOrganizationOverviewMembersEmptyState()}
        desktopMinWidth="560px"
        class="border-x-0 sm:border-x sm:border-t sm:border-b sm:rounded-md border-y border-border"
      />
    </div>
  </div>
);
