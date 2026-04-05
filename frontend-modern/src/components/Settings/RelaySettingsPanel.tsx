import { Component, Show } from 'solid-js';
import RadioTower from 'lucide-solid/icons/radio-tower';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { StatusDot } from '@/components/shared/StatusDot';
import { Toggle } from '@/components/shared/Toggle';
import { Card } from '@/components/shared/Card';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { getSettingsConfigurationLoadingState } from '@/utils/settingsShellPresentation';
import {
  RELAY_ENABLE_HELP_TEXT,
  RELAY_INFO_MESSAGE_CLASS,
  RELAY_INFO_TITLE_CLASS,
  RELAY_INLINE_ACTION_CLASS,
  RELAY_LAST_ERROR_CLASS,
  RELAY_LICENSE_REQUIRED_MESSAGE,
  RELAY_PAIRING_AVAILABILITY_MESSAGE,
  RELAY_PAIRING_AVAILABILITY_TITLE,
  RELAY_PRIMARY_BUTTON_CLASS,
  RELAY_PRIMARY_LINK_CLASS,
  RELAY_READONLY_NOTICE_CLASS,
  RELAY_SETTINGS_DESCRIPTION,
} from '@/utils/relayPresentation';
import { UPGRADE_TRIAL_LABEL } from '@/utils/upgradePresentation';
import { RelayPairingSection } from './RelayPairingSection';
import {
  useRelaySettingsPanelState,
  type RelaySettingsPanelProps,
} from './useRelaySettingsPanelState';

export const RelaySettingsPanel: Component<RelaySettingsPanelProps> = (props) => {
  const state = useRelaySettingsPanelState(props);

  // Pro feature gate
  if (!state.relayEnabled()) {
    return (
      <SettingsPanel
        title="Remote Access"
        description={RELAY_SETTINGS_DESCRIPTION}
        icon={<RadioTower size={20} strokeWidth={2} />}
      >
        <Show when={!state.loading()} fallback={<div class="text-sm ">Loading...</div>}>
          <Card tone="info" padding="md">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex items-start gap-3 flex-1">
                <RadioTower size={20} class="text-blue-500 mt-0.5 flex-shrink-0" strokeWidth={2} />
                <div>
                  <p class="text-sm font-medium text-base-content">Remote Access (Relay)</p>
                  <p class="text-sm text-muted mt-1">{RELAY_LICENSE_REQUIRED_MESSAGE}</p>
                </div>
              </div>
              <div class="flex flex-col sm:flex-row items-center gap-2">
                <UpgradeLink
                  destination={state.upgradeDestination()}
                  class={RELAY_PRIMARY_LINK_CLASS}
                  onClick={() => trackUpgradeClicked('settings_relay_panel', 'relay')}
                >
                  Upgrade
                </UpgradeLink>
                <Show when={state.canStartTrial()}>
                  <button
                    type="button"
                    onClick={state.handleStartTrial}
                    disabled={state.startingTrial()}
                    class={RELAY_INLINE_ACTION_CLASS}
                  >
                    {UPGRADE_TRIAL_LABEL}
                  </button>
                </Show>
              </div>
            </div>
          </Card>
        </Show>
      </SettingsPanel>
    );
  }

  return (
    <SettingsPanel
      title="Remote Access"
      description={RELAY_SETTINGS_DESCRIPTION}
      icon={<RadioTower size={20} strokeWidth={2} />}
    >
      <Show
        when={!state.loading()}
        fallback={<div class="text-sm ">{getSettingsConfigurationLoadingState().text}</div>}
      >
        <Show when={!state.canManage()}>
          <Card
            tone="info"
            padding="md"
            class={RELAY_READONLY_NOTICE_CLASS}
          >
            Remote access settings are read-only for this account.
          </Card>
        </Show>

        <Card tone="info" padding="md">
          <p class={RELAY_INFO_TITLE_CLASS}>{RELAY_PAIRING_AVAILABILITY_TITLE}</p>
          <p class={RELAY_INFO_MESSAGE_CLASS}>{RELAY_PAIRING_AVAILABILITY_MESSAGE}</p>
        </Card>

        {/* Connection Status */}
        <Card padding="md">
          <div class="flex items-center gap-3">
            <StatusDot
              variant={state.connectionPresentation().variant}
              size="md"
              pulse={state.connectionPresentation().pulse}
            />
            <div class="flex-1">
              <p class="text-sm font-medium text-base-content">{state.connectionPresentation().label}</p>
              <Show when={state.status()?.instance_id}>
                <p class="text-xs text-muted mt-0.5">Instance: {state.status()!.instance_id}</p>
              </Show>
              <Show when={state.status()?.connected && state.status()!.active_channels > 0}>
                <p class="text-xs text-muted">
                  {state.status()!.active_channels} active{' '}
                  {state.status()!.active_channels === 1 ? 'channel' : 'channels'}
                </p>
              </Show>
            </div>
          </div>
          <Show when={state.status()?.last_error}>
            <div class={RELAY_LAST_ERROR_CLASS}>
              {state.status()!.last_error}
            </div>
          </Show>
        </Card>

        {/* Enable/Disable Toggle */}
        <div class={formField}>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <label class={labelClass()}>Enable Remote Access</label>
              <p class={formHelpText}>{RELAY_ENABLE_HELP_TEXT}</p>
            </div>
            <Toggle
              checked={state.config()?.enabled ?? false}
              onChange={(e) => void state.handleToggleEnabled(e.currentTarget.checked)}
              disabled={!state.canManage() || state.saving()}
              containerClass="self-end sm:self-auto"
            />
          </div>
        </div>

        {/* Server URL */}
        <div class={formField}>
          <label class={labelClass()}>Relay Server URL</label>
          <div class="flex flex-col gap-2 sm:flex-row">
            <input
              type="text"
              class={controlClass()}
              value={state.serverUrl()}
              onInput={(e) => state.setServerUrl(e.currentTarget.value)}
              placeholder="wss://relay.example.com/ws/instance"
              disabled={!state.canManage() || state.saving()}
            />
            <Show when={state.canManage() && state.serverUrl() !== state.config()?.server_url}>
              <button
                class={`${RELAY_PRIMARY_BUTTON_CLASS} sm:self-auto self-end`}
                onClick={() => void state.handleSaveServerUrl()}
                disabled={state.saving()}
              >
                Save
              </button>
            </Show>
          </div>
          <p class={formHelpText}>
            The WebSocket URL of the relay server. Only change this for self-hosted relay servers.
          </p>
        </div>

        {/* Identity Fingerprint */}
        <Show when={state.config()?.identity_fingerprint}>
          <div class={formField}>
            <label class={labelClass()}>Instance Fingerprint</label>
            <code class="block text-xs font-mono text-base-content bg-surface-alt rounded px-3 py-2 select-all break-all">
              {state.config()!.identity_fingerprint}
            </code>
            <p class={formHelpText}>
              This fingerprint uniquely identifies your Pulse instance. Mobile clients verify this
              fingerprint to prevent man-in-the-middle attacks.
            </p>
          </div>
        </Show>

        <Show when={state.canShowPairing()}>
          <RelayPairingSection
            canManage={state.canManage()}
            pairingLoading={state.pairingLoading()}
            pairingPayload={state.pairingPayload()}
            pairingQRCode={state.pairingQRCode()}
            saving={state.saving()}
            showPairing={state.showPairing()}
            onCopyPairingPayload={() => void state.handleCopyPairingPayload()}
            onHidePairing={() => void state.handleHidePairing()}
            onPairNewDevice={() => void state.handlePairNewDevice()}
          />
        </Show>
      </Show>
    </SettingsPanel>
  );
};
