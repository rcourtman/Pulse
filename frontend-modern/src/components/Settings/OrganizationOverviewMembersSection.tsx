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
      class="mt-4"
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
            return <OrganizationRoleBadge role={role} />;
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
    />
  </div>
);
