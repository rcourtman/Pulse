import { Component, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { ProxmoxDirectWorkspace } from './ProxmoxDirectWorkspace';
import { SettingsSectionNav } from './SettingsSectionNav';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';

export const ProxmoxSettingsPanel: Component<InfrastructurePlatformSettingsProps> = (props) => {
  const navigate = useNavigate();

  return (
    <>
      <SettingsSectionNav
        current={props.selectedAgent()}
        onSelect={props.onSelectAgent}
        class="mb-6"
      />

      <Show when={!props.embedded}>
        <CalloutCard
          class="mb-6"
          icon={
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          }
          description={
            <>
              <p>
                <strong>Recommended:</strong> use the unified agent for Proxmox hosts. It
                auto-creates the API token, links the host, and unlocks temperature monitoring plus
                Pulse Patrol automation.
              </p>
              <p class="text-xs text-blue-700 dark:text-blue-300">
                Use this fallback path only when you cannot install the unified agent on the host.
              </p>
            </>
          }
        >
          <button
            type="button"
            onClick={() => navigate('/settings')}
            class="text-sm font-medium text-blue-700 underline hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
          >
            Open infrastructure setup →
          </button>
        </CalloutCard>
      </Show>

      <ProxmoxDirectWorkspace {...props} />
    </>
  );
};
