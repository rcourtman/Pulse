import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { API_TOKEN_ACCESS_PANEL_DESCRIPTION } from '@/utils/apiTokenPresentation';
import { API_TOKEN_SCOPES_DOC_URL } from '@/utils/docsLinks';
import APITokenManager from './APITokenManager';

interface APIAccessPanelProps {
  currentTokenHint?: string;
  onTokensChanged: () => void;
  refreshing: boolean;
  canManage: boolean;
}

export const APIAccessPanel: Component<APIAccessPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      <SettingsPanel
        title="API Access"
        noPadding
      >
        <div class="space-y-3 p-4 sm:p-6 pb-6">
          <p class="text-sm text-muted">{API_TOKEN_ACCESS_PANEL_DESCRIPTION}</p>
          <a
            href={API_TOKEN_SCOPES_DOC_URL}
            target="_blank"
            rel="noreferrer"
            class="inline-flex min-h-10 sm:min-h-10 w-fit items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200"
          >
            View scope reference
          </a>
        </div>
      </SettingsPanel>

      <APITokenManager
        currentTokenHint={props.currentTokenHint}
        onTokensChanged={props.onTokensChanged}
        refreshing={props.refreshing}
        canManage={props.canManage}
      />
    </div>
  );
};
