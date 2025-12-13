import { Component, Show, For, Accessor, Setter } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import Network from 'lucide-solid/icons/network';

const COMMON_DISCOVERY_SUBNETS = [
  '192.168.1.0/24',
  '192.168.0.0/24',
  '10.0.0.0/24',
  '172.16.0.0/24',
  '192.168.10.0/24',
];

interface NetworkSettingsPanelProps {
  // Discovery settings
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<'auto' | 'custom'>;
  discoverySubnetDraft: Accessor<string>;
  discoverySubnetError: Accessor<string | undefined>;
  savingDiscoverySettings: Accessor<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;

  // Network settings
  allowedOrigins: Accessor<string>;
  setAllowedOrigins: Setter<string>;
  allowEmbedding: Accessor<boolean>;
  setAllowEmbedding: Setter<boolean>;
  allowedEmbedOrigins: Accessor<string>;
  setAllowedEmbedOrigins: Setter<string>;
  webhookAllowedPrivateCIDRs: Accessor<string>;
  setWebhookAllowedPrivateCIDRs: Setter<string>;

  // Handlers
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  handleDiscoveryModeChange: (mode: 'auto' | 'custom') => Promise<void>;
  setDiscoveryMode: Setter<'auto' | 'custom'>;
  setDiscoverySubnetDraft: Setter<string>;
  setDiscoverySubnetError: Setter<string | undefined>;
  setLastCustomSubnet: Setter<string>;
  commitDiscoverySubnet: (value: string) => Promise<boolean>;
  setHasUnsavedChanges: Setter<boolean>;

  // Utility functions
  parseSubnetList: (value: string) => string[];
  normalizeSubnetList: (value: string) => string;
  isValidCIDR: (value: string) => boolean;
  currentDraftSubnetValue: () => string;

  // Ref for input
  discoverySubnetInputRef?: (el: HTMLInputElement) => void;
}

export const NetworkSettingsPanel: Component<NetworkSettingsPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      {/* Info Card */}
      <Card
        tone="info"
        padding="md"
        border={false}
        class="border border-blue-200 dark:border-blue-800"
      >
        <div class="flex items-start gap-3">
          <svg
            class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <div class="text-sm text-blue-800 dark:text-blue-200">
            <p class="font-medium mb-1">Configuration Priority</p>
            <ul class="space-y-1">
              <li>
                • Some env vars override settings (API_TOKENS, legacy API_TOKEN, PORTS, AUTH)
              </li>
              <li>• Changes made here are saved to system.json immediately</li>
              <li>• Settings persist unless overridden by env vars</li>
            </ul>
          </div>
        </div>
      </Card>

      {/* Main Network Card */}
      <Card
        padding="none"
        class="overflow-hidden border border-gray-200 dark:border-gray-700"
        border={false}
      >
        {/* Header */}
        <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
              <Network class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
            </div>
            <SectionHeader
              title="Network"
              description="Discovery, CORS, and embedding settings"
              size="sm"
              class="flex-1"
            />
          </div>
        </div>

        <div class="p-6 space-y-8">
          {/* Network Discovery Section */}
          <section class="space-y-5">
            <SectionHeader
              title="Network discovery"
              description="Control how Pulse scans for Proxmox services on your network."
              size="sm"
              align="left"
            />

            {/* Discovery Toggle */}
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div class="text-sm text-gray-600 dark:text-gray-400">
                <p class="font-medium text-gray-900 dark:text-gray-100">Automatic scanning</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  Enable discovery to surface Proxmox VE, PBS, and PMG endpoints automatically.
                </p>
              </div>
              <Toggle
                checked={props.discoveryEnabled()}
                onChange={async (e: ToggleChangeEvent) => {
                  if (props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()) {
                    e.preventDefault();
                    return;
                  }
                  const success = await props.handleDiscoveryEnabledChange(e.currentTarget.checked);
                  if (!success) {
                    e.currentTarget.checked = props.discoveryEnabled();
                  }
                }}
                disabled={props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()}
                containerClass="gap-2"
                label={
                  <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                    {props.discoveryEnabled() ? 'On' : 'Off'}
                  </span>
                }
              />
            </div>

            {/* Discovery Options (shown when enabled) */}
            <Show when={props.discoveryEnabled()}>
              <div class="space-y-4 rounded-lg border border-gray-200 bg-white/40 p-3 dark:border-gray-600 dark:bg-gray-800/40">
                <fieldset class="space-y-2">
                  <legend class="text-xs font-medium text-gray-700 dark:text-gray-300">
                    Scan scope
                  </legend>
                  <div class="space-y-2">
                    {/* Auto mode */}
                    <label
                      class={`flex items-start gap-3 rounded-lg border p-2 transition-colors ${
                        props.discoveryMode() === 'auto'
                          ? 'border-blue-200 bg-blue-50/80 dark:border-blue-700 dark:bg-blue-900/20'
                          : 'border-transparent hover:border-gray-200 dark:hover:border-gray-600'
                      }`}
                    >
                      <input
                        type="radio"
                        name="discoveryMode"
                        value="auto"
                        checked={props.discoveryMode() === 'auto'}
                        onChange={async () => {
                          if (props.discoveryMode() !== 'auto') {
                            await props.handleDiscoveryModeChange('auto');
                          }
                        }}
                        disabled={
                          props.envOverrides().discoverySubnet || props.savingDiscoverySettings()
                        }
                        class="mt-1 h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="space-y-1">
                        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                          Auto (slower, full scan)
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          Scans container, local, and gateway networks. Large networks may time out
                          after two minutes.
                        </p>
                      </div>
                    </label>

                    {/* Custom mode */}
                    <label
                      class={`flex items-start gap-3 rounded-lg border p-2 transition-colors ${
                        props.discoveryMode() === 'custom'
                          ? 'border-blue-200 bg-blue-50/80 dark:border-blue-700 dark:bg-blue-900/20'
                          : 'border-transparent hover:border-gray-200 dark:hover:border-gray-600'
                      }`}
                    >
                      <input
                        type="radio"
                        name="discoveryMode"
                        value="custom"
                        checked={props.discoveryMode() === 'custom'}
                        onChange={() => {
                          if (props.discoveryMode() !== 'custom') {
                            props.handleDiscoveryModeChange('custom');
                          }
                        }}
                        disabled={
                          props.envOverrides().discoverySubnet || props.savingDiscoverySettings()
                        }
                        class="mt-1 h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="space-y-1">
                        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                          Custom subnet (faster)
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          Limit discovery to one or more CIDR ranges to finish faster on large
                          networks.
                        </p>
                      </div>
                    </label>

                    {/* Common subnet presets */}
                    <Show when={props.discoveryMode() === 'custom'}>
                      <div class="flex flex-wrap items-center gap-2 pl-9 pr-2">
                        <span class="text-[0.68rem] uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          Common networks:
                        </span>
                        <For each={COMMON_DISCOVERY_SUBNETS}>
                          {(preset) => {
                            const baseValue = props.currentDraftSubnetValue();
                            const currentSelections = props.parseSubnetList(baseValue);
                            const isActive = currentSelections.includes(preset);
                            return (
                              <button
                                type="button"
                                class={`rounded border px-2.5 py-1 text-[0.7rem] transition-colors ${
                                  isActive
                                    ? 'border-blue-500 bg-blue-600 text-white dark:border-blue-400 dark:bg-blue-500'
                                    : 'border-gray-300 text-gray-700 hover:border-blue-400 hover:bg-blue-50 dark:border-gray-600 dark:text-gray-300 dark:hover:border-blue-500 dark:hover:bg-blue-900/30'
                                }`}
                                onClick={async () => {
                                  if (props.envOverrides().discoverySubnet) {
                                    return;
                                  }
                                  let selections = [...currentSelections];
                                  if (isActive) {
                                    selections = selections.filter((item) => item !== preset);
                                  } else {
                                    selections.push(preset);
                                  }

                                  if (selections.length === 0) {
                                    props.setDiscoverySubnetDraft('');
                                    props.setLastCustomSubnet('');
                                    props.setDiscoverySubnetError(
                                      'Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)'
                                    );
                                    return;
                                  }

                                  const updatedValue = props.normalizeSubnetList(
                                    selections.join(', ')
                                  );
                                  props.setDiscoveryMode('custom');
                                  props.setDiscoverySubnetDraft(updatedValue);
                                  props.setLastCustomSubnet(updatedValue);
                                  props.setDiscoverySubnetError(undefined);
                                  await props.commitDiscoverySubnet(updatedValue);
                                }}
                                disabled={props.envOverrides().discoverySubnet}
                                classList={{
                                  'cursor-not-allowed opacity-60':
                                    props.envOverrides().discoverySubnet,
                                }}
                              >
                                {preset}
                              </button>
                            );
                          }}
                        </For>
                      </div>
                    </Show>
                  </div>
                </fieldset>

                {/* Subnet Input */}
                <div class="space-y-2">
                  <div class="flex items-center justify-between gap-2">
                    <label
                      for="discoverySubnetInput"
                      class="text-xs font-medium text-gray-700 dark:text-gray-300"
                    >
                      Discovery subnet
                    </label>
                    <span
                      class="text-gray-400 hover:text-gray-500 dark:text-gray-500 dark:hover:text-gray-300"
                      title="Use CIDR notation (comma-separated for multiple), e.g. 192.168.1.0/24, 10.0.0.0/24. Smaller ranges keep scans quick."
                    >
                      <svg
                        class="h-4 w-4"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <circle cx="12" cy="12" r="10"></circle>
                        <path d="M12 16v-4"></path>
                        <path d="M12 8h.01"></path>
                      </svg>
                    </span>
                  </div>
                  <input
                    id="discoverySubnetInput"
                    ref={props.discoverySubnetInputRef}
                    type="text"
                    value={props.discoverySubnetDraft()}
                    placeholder={
                      props.discoveryMode() === 'auto'
                        ? 'auto (scan every network phase)'
                        : '192.168.1.0/24, 10.0.0.0/24'
                    }
                    class={`w-full rounded-lg border px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                      props.envOverrides().discoverySubnet
                        ? 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-600 dark:bg-amber-900/20 dark:text-amber-200 cursor-not-allowed opacity-60'
                        : 'border-gray-300 bg-white dark:border-gray-600 dark:bg-gray-900/70'
                    }`}
                    disabled={props.envOverrides().discoverySubnet}
                    onInput={(e) => {
                      if (props.envOverrides().discoverySubnet) {
                        return;
                      }
                      const rawValue = e.currentTarget.value;
                      props.setDiscoverySubnetDraft(rawValue);
                      if (props.discoveryMode() !== 'custom') {
                        props.setDiscoveryMode('custom');
                      }
                      props.setLastCustomSubnet(rawValue);
                      const trimmed = rawValue.trim();
                      if (!trimmed) {
                        props.setDiscoverySubnetError(undefined);
                        return;
                      }
                      if (!props.isValidCIDR(trimmed)) {
                        props.setDiscoverySubnetError(
                          'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)'
                        );
                      } else {
                        props.setDiscoverySubnetError(undefined);
                      }
                    }}
                    onBlur={async (e) => {
                      if (
                        props.envOverrides().discoverySubnet ||
                        props.discoveryMode() !== 'custom'
                      ) {
                        return;
                      }
                      const rawValue = e.currentTarget.value;
                      props.setDiscoverySubnetDraft(rawValue);
                      const trimmed = rawValue.trim();
                      if (!trimmed) {
                        props.setDiscoverySubnetError(
                          'Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)'
                        );
                        return;
                      }
                      if (!props.isValidCIDR(trimmed)) {
                        props.setDiscoverySubnetError(
                          'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)'
                        );
                        return;
                      }
                      props.setDiscoverySubnetError(undefined);
                      await props.commitDiscoverySubnet(rawValue);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault();
                        (e.currentTarget as HTMLInputElement).blur();
                      }
                    }}
                  />
                  <Show when={props.discoverySubnetError()}>
                    <p class="text-xs text-red-600 dark:text-red-400">
                      {props.discoverySubnetError()}
                    </p>
                  </Show>
                  <Show when={!props.discoverySubnetError() && props.discoveryMode() === 'auto'}>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      Auto scans every reachable network phase. Large networks may time out — switch
                      to custom subnets to narrow the search.
                    </p>
                  </Show>
                  <Show when={!props.discoverySubnetError() && props.discoveryMode() === 'custom'}>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      Example: 192.168.1.0/24, 10.0.0.0/24 (comma-separated). Smaller ranges finish
                      faster and avoid timeouts.
                    </p>
                  </Show>
                </div>
              </div>
            </Show>

            {/* Env override warning */}
            <Show when={props.envOverrides().discoveryEnabled || props.envOverrides().discoverySubnet}>
              <div class="rounded-lg border border-amber-200 bg-amber-100/80 p-3 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-200">
                Discovery settings are locked by environment variables. Update the service
                configuration and restart Pulse to change them here.
              </div>
            </Show>
          </section>

          {/* CORS Settings Section */}
          <section class="space-y-3">
            <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
              </svg>
              Network Settings
            </h4>
            <div class="space-y-2">
              <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                CORS Allowed Origins
              </label>
              <p class="text-xs text-gray-600 dark:text-gray-400">
                For reverse proxy setups (* = allow all, empty = same-origin only)
              </p>
              <div class="relative">
                <input
                  type="text"
                  value={props.allowedOrigins()}
                  onChange={(e) => {
                    if (!props.envOverrides().allowedOrigins) {
                      props.setAllowedOrigins(e.currentTarget.value);
                      props.setHasUnsavedChanges(true);
                    }
                  }}
                  disabled={props.envOverrides().allowedOrigins}
                  placeholder="* or https://example.com"
                  class={`w-full px-3 py-1.5 text-sm border rounded-lg ${
                    props.envOverrides().allowedOrigins
                      ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/20 cursor-not-allowed opacity-75'
                      : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800'
                  }`}
                />
                {props.envOverrides().allowedOrigins && (
                  <div class="mt-2 p-2 bg-amber-100 dark:bg-amber-900/30 border border-amber-300 dark:border-amber-700 rounded text-xs text-amber-800 dark:text-amber-200">
                    <div class="flex items-center gap-1">
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                        />
                      </svg>
                      <span>Overridden by ALLOWED_ORIGINS environment variable</span>
                    </div>
                    <div class="mt-1 text-amber-700 dark:text-amber-300">
                      Remove the env var and restart to enable UI configuration
                    </div>
                  </div>
                )}
              </div>
            </div>
          </section>

          {/* Embedding Section */}
          <section class="space-y-3">
            <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <rect x="3" y="4" width="18" height="14" rx="2"></rect>
                <path d="M7 20h10"></path>
              </svg>
              Embedding
            </h4>
            <p class="text-xs text-gray-600 dark:text-gray-400">
              Allow Pulse to be embedded in iframes (e.g., Homepage dashboard)
            </p>
            <div class="space-y-3">
              <div class="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="allowEmbedding"
                  checked={props.allowEmbedding()}
                  onChange={(e) => {
                    props.setAllowEmbedding(e.currentTarget.checked);
                    props.setHasUnsavedChanges(true);
                  }}
                  class="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                />
                <label for="allowEmbedding" class="text-sm text-gray-700 dark:text-gray-300">
                  Allow iframe embedding
                </label>
              </div>

              <Show when={props.allowEmbedding()}>
                <div class="space-y-2">
                  <label class="text-xs font-medium text-gray-700 dark:text-gray-300">
                    Allowed Embed Origins (optional)
                  </label>
                  <p class="text-xs text-gray-600 dark:text-gray-400">
                    Comma-separated list of origins that can embed Pulse (leave empty for same-origin
                    only)
                  </p>
                  <input
                    type="text"
                    value={props.allowedEmbedOrigins()}
                    onChange={(e) => {
                      props.setAllowedEmbedOrigins(e.currentTarget.value);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder="https://my.domain, https://dashboard.example.com"
                    class="w-full px-3 py-1.5 text-sm border rounded-lg border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
                  />
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    Example: If Pulse is at <code>pulse.my.domain</code> and your dashboard is at{' '}
                    <code>my.domain</code>, add <code>https://my.domain</code> here.
                  </p>
                </div>
              </Show>
            </div>
          </section>

          {/* Webhook Security Section */}
          <section class="space-y-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width={2}
                  d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                />
              </svg>
              Webhook Security
            </h3>
            <div class="space-y-3">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Allowed Private IP Ranges for Webhooks
                </label>
                <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                  By default, webhooks to private IP addresses are blocked for security. Enter
                  trusted CIDR ranges to allow webhooks to internal services (leave empty to block
                  all private IPs).
                </p>
                <input
                  type="text"
                  value={props.webhookAllowedPrivateCIDRs()}
                  onChange={(e) => {
                    props.setWebhookAllowedPrivateCIDRs(e.currentTarget.value);
                    props.setHasUnsavedChanges(true);
                  }}
                  placeholder="192.168.1.0/24, 10.0.0.0/8"
                  class="w-full px-3 py-1.5 text-sm border rounded-lg border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
                />
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Example: <code>192.168.1.0/24,10.0.0.0/8</code> allows webhooks to these private
                  networks. Localhost and cloud metadata services remain blocked.
                </p>
              </div>
            </div>
          </section>

          {/* Port Configuration Notice */}
          <Card
            tone="warning"
            padding="sm"
            border={false}
            class="border border-amber-200 dark:border-amber-800"
          >
            <p class="text-xs text-amber-800 dark:text-amber-200 mb-2">
              <strong>Port Configuration:</strong> Use{' '}
              <code class="font-mono bg-amber-100 dark:bg-amber-800 px-1 rounded">
                systemctl edit pulse
              </code>
            </p>
            <p class="text-xs text-amber-700 dark:text-amber-300 font-mono">
              [Service]
              <br />
              Environment="FRONTEND_PORT=8080"
              <br />
              <span class="text-xs text-amber-600 dark:text-amber-400">
                Then restart: sudo systemctl restart pulse
              </span>
            </p>
          </Card>
        </div>
      </Card>
    </div>
  );
};
