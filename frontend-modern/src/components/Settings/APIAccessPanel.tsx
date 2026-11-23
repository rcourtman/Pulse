import { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ButtonLink } from '@/components/shared/Button';
import { API_TOKEN_ACCESS_PANEL_DESCRIPTION } from '@/utils/apiTokenPresentation';
import { API_TOKEN_SCOPES_DOC_URL } from '@/utils/docsLinks';
import APITokenManager from './APITokenManager';

interface APIAccessPanelProps {
  currentTokenHint?: string;
  currentUser?: string;
  onTokensChanged: () => void;
  refreshing: boolean;
  canManage: boolean;
}

export const APIAccessPanel: Component<APIAccessPanelProps> = (props) => {
  return (
    <div class="space-y-3 sm:space-y-6">
      <Card padding="none" class="border border-border">
        <div class="flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between sm:p-5">
          <p class="text-xs leading-relaxed text-muted sm:text-sm">
            {API_TOKEN_ACCESS_PANEL_DESCRIPTION}
          </p>
          <ButtonLink
            href={API_TOKEN_SCOPES_DOC_URL}
            target="_blank"
            variant="info"
            size="settingsAction"
            class="w-fit shrink-0 gap-2 font-semibold whitespace-nowrap"
          >
            View scope reference
          </ButtonLink>
        </div>
      </Card>

      <div data-testid="api-access-token-section">
        <APITokenManager
          currentTokenHint={props.currentTokenHint}
          currentUser={props.currentUser}
          onTokensChanged={props.onTokensChanged}
          refreshing={props.refreshing}
          canManage={props.canManage}
        />
      </div>
    </div>
  );
};
