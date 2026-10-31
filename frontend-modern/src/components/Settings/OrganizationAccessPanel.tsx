import { Component, Show } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  getOrganizationAccessReloadMessage,
  ORGANIZATION_ACCESS_RELOAD_LABEL,
  ORGANIZATION_ACCESS_UNAVAILABLE_TITLE,
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
            <Show when={state.loadError()}>
              <Card tone="warning" padding="md">
                <div role="alert" class="space-y-3 text-sm">
                  <p class="font-semibold">{ORGANIZATION_ACCESS_UNAVAILABLE_TITLE}</p>
                  <p>{state.loadError()}</p>
                  <p>{getOrganizationAccessReloadMessage(state.selectedOrgID())}</p>
                  <Button
                    onClick={() => void state.loadOrganizationAccess()}
                    disabled={state.loading() || state.saving()}
                    class="w-full sm:w-auto"
                  >
                    {ORGANIZATION_ACCESS_RELOAD_LABEL}
                  </Button>
                </div>
              </Card>
            </Show>
            <Show when={!state.loadError() && state.org()}>
              {(currentOrg) => (
                <>
                  <p class="text-sm text-muted break-words">
                    Access for{' '}
                    <span class="font-medium text-base-content">
                      {currentOrg().displayName || currentOrg().id}
                    </span>{' '}
                    ({currentOrg().id})
                  </p>
                  <OrganizationAccessInvitationsSection state={state} />
                  <OrganizationAccessManagementSection
                    state={state}
                    currentUser={props.currentUser}
                  />
                  <OrganizationAccessMembersSection state={state} currentUser={props.currentUser} />
                </>
              )}
            </Show>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationAccessPanel;
