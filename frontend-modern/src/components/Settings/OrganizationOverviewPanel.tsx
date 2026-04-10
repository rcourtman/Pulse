import { Component, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import Building2 from 'lucide-solid/icons/building-2';
import { OrganizationOverviewDetailsSection } from './OrganizationOverviewDetailsSection';
import { OrganizationOverviewLoadingState } from './OrganizationOverviewLoadingState';
import { OrganizationOverviewMembersSection } from './OrganizationOverviewMembersSection';
import { useOrganizationOverviewPanelState } from './useOrganizationOverviewPanelState';

export interface OrganizationOverviewPanelProps {
  currentUser?: string;
}

export const OrganizationOverviewPanel: Component<OrganizationOverviewPanelProps> = (props) => {
  const state = useOrganizationOverviewPanelState(props);
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
        <SettingsPanel
          title="Organization Overview"
          description="Review organization metadata, membership footprint, and edit the display name."
          icon={<Building2 class="w-5 h-5" />}
          noPadding
          bodyClass="divide-y divide-border"
        >
          <Show when={!state.loading()} fallback={<OrganizationOverviewLoadingState />}>
            <Show when={state.org()}>
              <OrganizationOverviewDetailsSection state={state} />
              <OrganizationOverviewMembersSection state={state} />
            </Show>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationOverviewPanel;
