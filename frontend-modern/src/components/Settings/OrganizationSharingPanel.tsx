import { Component, Show } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  getOrganizationSharingReloadMessage,
  ORGANIZATION_SHARING_RELOAD_LABEL,
  ORGANIZATION_SHARING_UNAVAILABLE_TITLE,
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { OrganizationIncomingSharesSection } from './OrganizationIncomingSharesSection';
import { OrganizationOutgoingSharesSection } from './OrganizationOutgoingSharesSection';
import { OrganizationSharingCreateSection } from './OrganizationSharingCreateSection';
import { OrganizationSharingLoadingState } from './OrganizationSharingLoadingState';
import { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

export interface OrganizationSharingPanelProps {
  currentUser?: string;
}

export const OrganizationSharingPanel: Component<OrganizationSharingPanelProps> = (props) => {
  const state = useOrganizationSharingPanelState(props);
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
        <SettingsPanel title="Organization Sharing" noPadding bodyClass="divide-y divide-border">
          <Show when={!state.loading()} fallback={<OrganizationSharingLoadingState />}>
            <Show when={state.loadError()}>
              <div class="p-4 sm:p-6">
                <Card tone="warning" padding="md">
                  <div role="alert" class="space-y-3 text-sm">
                    <p class="font-semibold">{ORGANIZATION_SHARING_UNAVAILABLE_TITLE}</p>
                    <p>{state.loadError()}</p>
                    <p>{getOrganizationSharingReloadMessage(state.selectedOrgID())}</p>
                    <Button
                      onClick={() => void state.loadSharingData()}
                      disabled={state.loading() || state.saving()}
                      class="w-full sm:w-auto"
                    >
                      {ORGANIZATION_SHARING_RELOAD_LABEL}
                    </Button>
                  </div>
                </Card>
              </div>
            </Show>
            <Show when={!state.loadError() && state.org()}>
              {(currentOrg) => (
                <>
                  <div class="p-4 sm:px-6 space-y-2 text-sm text-muted break-words">
                    <p>
                      Sharing for{' '}
                      <span class="font-medium text-base-content" data-sharing-organization>
                        {currentOrg().displayName || currentOrg().id}
                      </span>{' '}
                      ({currentOrg().id})
                    </p>
                    <Show when={state.readOnly()}>
                      <p>This session is read-only. Sharing changes are disabled.</p>
                    </Show>
                  </div>
                  <OrganizationSharingCreateSection state={state} />
                  <OrganizationOutgoingSharesSection state={state} />
                  <OrganizationIncomingSharesSection state={state} />
                </>
              )}
            </Show>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationSharingPanel;
