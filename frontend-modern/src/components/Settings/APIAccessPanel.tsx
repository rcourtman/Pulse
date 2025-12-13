import { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
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
      <Card
        padding="none"
        class="overflow-hidden border border-gray-200 dark:border-gray-700"
        border={false}
      >
        <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
              <BadgeCheck class="w-5 h-5 text-blue-600 dark:text-blue-300" />
            </div>
            <SectionHeader
              title="API Access"
              description="Generate scoped tokens for agents and automation"
              size="sm"
              class="flex-1"
            />
          </div>
        </div>
        <div class="p-6 space-y-3">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Generate scoped tokens for Docker agents, host agents, and automation pipelines. Tokens
            are shown once—store them securely and rotate when infrastructure changes.
          </p>
          <a
            href="https://github.com/rcourtman/Pulse/blob/main/docs/CONFIGURATION.md#token-scopes"
            target="_blank"
            rel="noreferrer"
            class="inline-flex w-fit items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200"
          >
            View scope reference
          </a>
        </div>
      </Card>

      <APITokenManager
        currentTokenHint={props.currentTokenHint}
        onTokensChanged={props.onTokensChanged}
        refreshing={props.refreshing}
      />
    </div>
  );
};
