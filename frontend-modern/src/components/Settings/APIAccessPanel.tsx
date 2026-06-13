import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { ButtonLink } from '@/components/shared/Button';
import { API_TOKEN_ACCESS_PANEL_DESCRIPTION } from '@/utils/apiTokenPresentation';
import { API_TOKEN_SCOPES_DOC_URL } from '@/utils/docsLinks';
import APITokenManager from './APITokenManager';
import AgentIntegrationsPanel from './AgentIntegrationsPanel';

interface APIAccessPanelProps {
  currentTokenHint?: string;
  onTokensChanged: () => void;
  refreshing: boolean;
  canManage: boolean;
}

export const APIAccessPanel: Component<APIAccessPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      <SettingsPanel title="API Access" noPadding>
        <div class="space-y-3 p-4 sm:p-6 pb-6">
          <p class="text-sm text-muted">{API_TOKEN_ACCESS_PANEL_DESCRIPTION}</p>
          <ButtonLink
            href={API_TOKEN_SCOPES_DOC_URL}
            target="_blank"
            variant="info"
            size="settingsAction"
            class="w-fit gap-2 font-semibold"
          >
            View scope reference
          </ButtonLink>
        </div>
      </SettingsPanel>

      <APITokenManager
        currentTokenHint={props.currentTokenHint}
        onTokensChanged={props.onTokensChanged}
        refreshing={props.refreshing}
        canManage={props.canManage}
      />

      <AgentIntegrationsPanel />
    </div>
  );
};
