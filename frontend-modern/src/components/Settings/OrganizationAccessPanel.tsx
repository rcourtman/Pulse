import { Component, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import Users from 'lucide-solid/icons/users';
import { OrganizationAccessLoadingState } from './OrganizationAccessLoadingState';
import { OrganizationAccessManagementSection } from './OrganizationAccessManagementSection';
import { OrganizationAccessMembersSection } from './OrganizationAccessMembersSection';
import { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

export interface OrganizationAccessPanelProps {
  currentUser?: string;
}

export const OrganizationAccessPanel: Component<OrganizationAccessPanelProps> = (props) => {
  const state = useOrganizationAccessPanelState(props);

  return (
    <Show
      when={isMultiTenantEnabled()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <div class="space-y-6">
        <SettingsPanel
          title="Organization Access"
          description="Manage organization member roles and ownership transfers."
          icon={<Users class="w-5 h-5" />}
          bodyClass="space-y-5"
        >
          <Show when={!state.loading()} fallback={<OrganizationAccessLoadingState />}>
            <Show when={state.org()}>
              <OrganizationAccessManagementSection state={state} currentUser={props.currentUser} />
              <OrganizationAccessMembersSection state={state} currentUser={props.currentUser} />
            </Show>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationAccessPanel;
