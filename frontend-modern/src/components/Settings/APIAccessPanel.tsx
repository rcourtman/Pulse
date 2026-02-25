import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import APITokenManager from './APITokenManager';
import BadgeCheck from 'lucide-solid/icons/badge-check';

interface APIAccessPanelProps {
  currentTokenHint?: string;
  onTokensChanged: () => void;
  refreshing: boolean;
}

export const APIAccessPanel: Component<APIAccessPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      <SettingsPanel
        title="API Access"
        description="Generate and manage scoped tokens for agents and automation."
        icon={<BadgeCheck class="w-5 h-5" strokeWidth={2} />}
        noPadding
      >
        <div class="space-y-3 p-4 sm:p-6 pb-6">
          <p class="text-sm text-muted">
            Generate scoped tokens for Docker agents, host agents, and automation pipelines. Tokens
            are shown once—store them securely and rotate when infrastructure changes.
          </p>
          <a
            href="https://github.com/rcourtman/Pulse/blob/main/docs/CONFIGURATION.md#token-scopes"
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
      />
    </div>
  );
};
