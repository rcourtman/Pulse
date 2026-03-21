import { Component, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import Share2 from 'lucide-solid/icons/share-2';
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
          title="Organization Sharing"
          description="Share views and resources across organizations with explicit role-based access."
          icon={<Share2 class="w-5 h-5" />}
          noPadding
          bodyClass="divide-y divide-border"
        >
          <Show when={!state.loading()} fallback={<OrganizationSharingLoadingState />}>
            <OrganizationSharingCreateSection state={state} />
            <OrganizationOutgoingSharesSection state={state} />
            <OrganizationIncomingSharesSection state={state} />
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationSharingPanel;
