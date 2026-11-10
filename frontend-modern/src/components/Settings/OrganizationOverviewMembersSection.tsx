import { Component } from 'solid-js';
import { OrganizationRoleBadge } from '@/components/shared/OrganizationBadges';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { getOrganizationOverviewMembersEmptyState } from '@/utils/organizationSettingsPresentation';
import { formatOrgDate, normalizeRole } from '@/utils/orgUtils';
import type { useOrganizationOverviewPanelState } from './useOrganizationOverviewPanelState';

interface OrganizationOverviewMembersSectionProps {
  state: ReturnType<typeof useOrganizationOverviewPanelState>;
}

export const OrganizationOverviewMembersSection: Component<
  OrganizationOverviewMembersSectionProps
> = (props) => (
  <div class="space-y-2 p-4 sm:p-6">
    <h4 class="text-sm font-semibold text-base-content">Membership</h4>
    <PulseDataGrid
      class="mt-4 [&_table]:table-fixed sm:[&_table]:table-auto [&_td]:py-1 sm:[&_td]:py-3.5 [&_th:nth-child(2)]:w-20 sm:[&_th:nth-child(2)]:w-auto"
      data={props.state.members()}
      columns={[
        {
          key: 'userId',
          label: 'User',
          render: (member) => (
            <span class="block whitespace-normal text-base-content [overflow-wrap:anywhere] sm:whitespace-nowrap">
              {member.userId}
            </span>
          ),
        },
        {
          key: 'role',
          label: 'Role',
          render: (member) => {
            const role = normalizeRole(member.role);
            return <OrganizationRoleBadge role={role} />;
          },
        },
        {
          key: 'addedAt',
          label: 'Added',
          hiddenOnMobile: true,
          render: (member) => <span class="text-muted">{formatOrgDate(member.addedAt)}</span>,
        },
      ]}
      keyExtractor={(member) => member.userId}
      emptyState={getOrganizationOverviewMembersEmptyState()}
      desktopMinWidth="560px"
    />
  </div>
);
