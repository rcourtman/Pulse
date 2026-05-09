import { Component, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { OrganizationAccessLoadingState } from './OrganizationAccessLoadingState';
import { OrganizationAccessInvitationsSection } from './OrganizationAccessInvitationsSection';
import { OrganizationAccessManagementSection } from './OrganizationAccessManagementSection';
import { OrganizationAccessMembersSection } from './OrganizationAccessMembersSection';
import { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

export interface OrganizationAccessPanelProps {
  currentUser?: string;
}

export const OrganizationAccessPanel: Component<OrganizationAccessPanelProps> = (props) => {
  const state = useOrganizationAccessPanelState(props);
  const showOrganizationSurface = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();

  return (
    <Show
      when={showOrganizationSurface()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <div class="space-y-6">
        <SettingsPanel title="Organization Access" bodyClass="space-y-5">
          <Show when={!state.loading()} fallback={<OrganizationAccessLoadingState />}>
            <Show when={state.org()}>
              <OrganizationAccessInvitationsSection state={state} />
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
