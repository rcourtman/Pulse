import { Component, createSignal, createEffect, onMount, onCleanup, Show } from 'solid-js';
import RadioTower from 'lucide-solid/icons/radio-tower';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { StatusDot } from '@/components/shared/StatusDot';
import { Toggle } from '@/components/shared/Toggle';
import { Card } from '@/components/shared/Card';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import {
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
  getUpgradeActionUrlOrFallback,
  startProTrial,
  entitlements,
} from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { showSuccess, showError } from '@/utils/toast';
import { RelayAPI, type RelayConfig, type RelayStatus } from '@/api/relay';
import { OnboardingAPI, type OnboardingQRResponse } from '@/api/onboarding';
import { SecurityAPI } from '@/api/security';
import { logger } from '@/utils/logger';
import { getSettingsConfigurationLoadingState } from '@/utils/settingsShellPresentation';
import {
  getRelayConnectionPresentation,
  getRelayDiagnosticClass,
  RELAY_BETA_MESSAGE_CLASS,
  RELAY_BETA_TITLE_CLASS,
  RELAY_CODE_BLOCK_CLASS,
  RELAY_DIAGNOSTICS_TITLE_CLASS,
  RELAY_DIAGNOSTICS_WRAP_CLASS,
  RELAY_INLINE_ACTION_CLASS,
  RELAY_LAST_ERROR_CLASS,
  RELAY_PRIMARY_BUTTON_CLASS,
  RELAY_PRIMARY_LINK_CLASS,
  RELAY_QR_IMAGE_CLASS,
  RELAY_READONLY_NOTICE_CLASS,
  RELAY_SECONDARY_BUTTON_CLASS,
} from '@/utils/relayPresentation';
import QRCode from 'qrcode';

interface RelaySettingsPanelProps {
  canManage?: boolean;
}

function buildRelayPairingTokenName(now: Date): string {
  return `Relay mobile device ${now.toISOString()}`;
}

export const RelaySettingsPanel: Component<RelaySettingsPanelProps> = (props) => {
  const [config, setConfig] = createSignal<RelayConfig | null>(null);
  const [status, setStatus] = createSignal<RelayStatus | null>(null);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [serverUrl, setServerUrl] = createSignal('');
  const [showPairing, setShowPairing] = createSignal(false);
  const [pairingLoading, setPairingLoading] = createSignal(false);
  const [pairingPayload, setPairingPayload] = createSignal<OnboardingQRResponse | null>(null);
  const [pairingQRCode, setPairingQRCode] = createSignal<string | null>(null);
  const [pairingTokenId, setPairingTokenId] = createSignal<string | null>(null);
  const [startingTrial, setStartingTrial] = createSignal(false);
  const canManage = () => props.canManage !== false;

  const canStartTrial = () => entitlements()?.trial_eligible !== false;

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        window.location.href = result.actionUrl;
        return;
      }
      showSuccess('Remote access trial started');
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        showError('Trial already used');
      } else {
        showError(err instanceof Error ? err.message : 'Failed to start trial');
      }
    } finally {
      setStartingTrial(false);
    }
  };

  createEffect((wasPaywallVisible: boolean) => {
    const isPaywallVisible = licenseLoaded() && !hasFeature('relay');
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('relay', 'settings_relay_panel');
    }
    return isPaywallVisible;
  }, false);

  let statusInterval: ReturnType<typeof setInterval> | undefined;

  const resetPairingState = () => {
    setShowPairing(false);
    setPairingLoading(false);
    setPairingPayload(null);
    setPairingQRCode(null);
    setPairingTokenId(null);
  };

  const deletePairingToken = async (tokenID: string | null) => {
    if (!tokenID) return;
    try {
      await SecurityAPI.deleteToken(tokenID);
    } catch (err) {
      logger.warn('[RelaySettings] Failed to clean up relay pairing token', err);
    }
  };

  const loadConfig = async () => {
    try {
      const cfg = await RelayAPI.getConfig();
      setConfig(cfg);
      setServerUrl(cfg.server_url);
      if (!cfg.enabled) {
        resetPairingState();
      }
    } catch (err) {
      logger.error('[RelaySettings] Failed to load config', err);
    }
  };

  const loadStatus = async () => {
    try {
      const st = await RelayAPI.getStatus();
      setStatus(st);
    } catch (err) {
      logger.error('[RelaySettings] Failed to load status', err);
    }
  };

  const startStatusPolling = () => {
    stopStatusPolling();
    void loadStatus();
    statusInterval = setInterval(() => void loadStatus(), 5000);
  };

  const stopStatusPolling = () => {
    if (statusInterval !== undefined) {
      clearInterval(statusInterval);
      statusInterval = undefined;
    }
  };

  onMount(async () => {
    await loadLicenseStatus();
    if (!hasFeature('relay')) {
      setLoading(false);
      return;
    }
    await loadConfig();
    if (config()?.enabled) {
      startStatusPolling();
    }
    setLoading(false);
  });

  onCleanup(() => {
    stopStatusPolling();
  });

  const handleToggleEnabled = async (enabled: boolean) => {
    if (!canManage()) return;
    setSaving(true);
    try {
      await RelayAPI.updateConfig({ enabled });
      await loadConfig();
      if (enabled) {
        startStatusPolling();
        showSuccess('Remote access enabled');
      } else {
        stopStatusPolling();
        setStatus(null);
        resetPairingState();
        showSuccess('Remote access disabled');
      }
    } catch (err) {
      showError('Failed to update relay configuration');
      logger.error('[RelaySettings] Failed to toggle', err);
    } finally {
      setSaving(false);
    }
  };

  const handleSaveServerUrl = async () => {
    if (!canManage()) return;
    const url = serverUrl().trim();
    if (!url) return;
    setSaving(true);
    try {
      await RelayAPI.updateConfig({ server_url: url });
      await loadConfig();
      showSuccess('Server URL updated');
    } catch (err) {
      showError('Failed to update server URL');
      logger.error('[RelaySettings] Failed to save server URL', err);
    } finally {
      setSaving(false);
    }
  };

  const handlePairNewDevice = async () => {
    if (!canManage()) return;
    const previousPayload = pairingPayload();
    const previousQRCode = pairingQRCode();
    const previousTokenId = pairingTokenId();
    setShowPairing(true);
    setPairingLoading(true);
    let createdTokenId: string | null = null;
    try {
      const createdToken = await SecurityAPI.createToken(buildRelayPairingTokenName(new Date()));
      if (!createdToken.token) {
        throw new Error('Failed to generate relay device token');
      }
      createdTokenId = createdToken.record.id;

      const payload = await OnboardingAPI.getQRPayload(createdToken.token);
      if (!payload.auth_token) {
        throw new Error('Relay pairing payload is missing auth_token');
      }
      const qrCodeDataUrl = await QRCode.toDataURL(payload.deep_link, {
        width: 256,
        margin: 2,
      });
      setPairingPayload(payload);
      setPairingQRCode(qrCodeDataUrl);
      setPairingTokenId(createdTokenId);
      if (previousTokenId && previousTokenId != createdTokenId) {
        await deletePairingToken(previousTokenId);
      }
    } catch (err) {
      await deletePairingToken(createdTokenId);
      setPairingPayload(previousPayload);
      setPairingQRCode(previousQRCode);
      setPairingTokenId(previousTokenId);
      showError('Failed to generate pairing QR code');
      logger.error('[RelaySettings] Failed to generate onboarding QR', err);
    } finally {
      setPairingLoading(false);
    }
  };

  const handleCopyPairingPayload = async () => {
    const payload = pairingPayload();
    if (!payload) return;

    try {
      await navigator.clipboard.writeText(JSON.stringify(payload, null, 2));
      showSuccess('Pairing payload copied');
    } catch (err) {
      showError('Failed to copy pairing payload');
      logger.error('[RelaySettings] Failed to copy pairing payload', err);
    }
  };

  const connectionPresentation = () => getRelayConnectionPresentation(config(), status());

  // Pro feature gate
  if (!hasFeature('relay')) {
    return (
      <SettingsPanel
        title="Remote Access"
        description="Configure Pulse relay connectivity for secure remote access (mobile rollout coming soon)."
        icon={<RadioTower size={20} strokeWidth={2} />}
      >
        <Show when={!loading()} fallback={<div class="text-sm ">Loading...</div>}>
          <Card tone="info" padding="md">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex items-start gap-3 flex-1">
                <RadioTower size={20} class="text-blue-500 mt-0.5 flex-shrink-0" strokeWidth={2} />
                <div>
                  <p class="text-sm font-medium text-base-content">Remote Access (Relay)</p>
                  <p class="text-sm text-muted mt-1">
                    Remote access via Pulse Relay requires a Relay license or above. Mobile app
                    public rollout is coming soon.
                  </p>
                </div>
              </div>
              <div class="flex flex-col sm:flex-row items-center gap-2">
                <a
                  href={getUpgradeActionUrlOrFallback('relay')}
                  target="_blank"
                  rel="noopener noreferrer"
                  class={RELAY_PRIMARY_LINK_CLASS}
                  onClick={() => trackUpgradeClicked('settings_relay_panel', 'relay')}
                >
                  Upgrade
                </a>
                <Show when={canStartTrial()}>
                  <button
                    type="button"
                    onClick={handleStartTrial}
                    disabled={startingTrial()}
                    class={RELAY_INLINE_ACTION_CLASS}
                  >
                    Start free trial
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
      description="Configure Pulse relay connectivity for secure remote access (mobile rollout coming soon)."
      icon={<RadioTower size={20} strokeWidth={2} />}
    >
      <Show
        when={!loading()}
        fallback={<div class="text-sm ">{getSettingsConfigurationLoadingState().text}</div>}
      >
        <Show when={!canManage()}>
          <Card
            tone="info"
            padding="md"
            class={RELAY_READONLY_NOTICE_CLASS}
          >
            Remote access settings are read-only for this account.
          </Card>
        </Show>

        <Card tone="info" padding="md">
          <p class={RELAY_BETA_TITLE_CLASS}>Pulse Mobile rollout is coming soon</p>
          <p class={RELAY_BETA_MESSAGE_CLASS}>
            Relay infrastructure is available now. Pairing and remote sessions are currently
            intended for staged beta access.
          </p>
        </Card>

        {/* Connection Status */}
        <Card padding="md">
          <div class="flex items-center gap-3">
            <StatusDot
              variant={connectionPresentation().variant}
              size="md"
              pulse={connectionPresentation().pulse}
            />
            <div class="flex-1">
              <p class="text-sm font-medium text-base-content">{connectionPresentation().label}</p>
              <Show when={status()?.instance_id}>
                <p class="text-xs text-muted mt-0.5">Instance: {status()!.instance_id}</p>
              </Show>
              <Show when={status()?.connected && status()!.active_channels > 0}>
                <p class="text-xs text-muted">
                  {status()!.active_channels} active{' '}
                  {status()!.active_channels === 1 ? 'channel' : 'channels'}
                </p>
              </Show>
            </div>
          </div>
          <Show when={status()?.last_error}>
            <div class={RELAY_LAST_ERROR_CLASS}>
              {status()!.last_error}
            </div>
          </Show>
        </Card>

        {/* Enable/Disable Toggle */}
        <div class={formField}>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <label class={labelClass()}>Enable Remote Access</label>
              <p class={formHelpText}>
                Connect this Pulse instance to the relay server for secure remote access and mobile
                beta readiness.
              </p>
            </div>
            <Toggle
              checked={config()?.enabled ?? false}
              onChange={(e) => void handleToggleEnabled(e.currentTarget.checked)}
              disabled={!canManage() || saving()}
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
              value={serverUrl()}
              onInput={(e) => setServerUrl(e.currentTarget.value)}
              placeholder="wss://relay.example.com/ws/instance"
              disabled={!canManage() || saving()}
            />
            <Show when={canManage() && serverUrl() !== config()?.server_url}>
              <button
                class={`${RELAY_PRIMARY_BUTTON_CLASS} sm:self-auto self-end`}
                onClick={() => void handleSaveServerUrl()}
                disabled={saving()}
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
        <Show when={config()?.identity_fingerprint}>
          <div class={formField}>
            <label class={labelClass()}>Instance Fingerprint</label>
            <code class="block text-xs font-mono text-base-content bg-surface-alt rounded px-3 py-2 select-all break-all">
              {config()!.identity_fingerprint}
            </code>
            <p class={formHelpText}>
              This fingerprint uniquely identifies your Pulse instance. Mobile clients verify this
              fingerprint to prevent man-in-the-middle attacks.
            </p>
          </div>
        </Show>

        <Show when={config()?.enabled && status()?.connected}>
          <div class={formField}>
            <label class={labelClass()}>Pair Mobile Device</label>
            <Card tone="muted" padding="md">
              <div class="space-y-3">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    class={RELAY_PRIMARY_BUTTON_CLASS}
                    onClick={() => void handlePairNewDevice()}
                    disabled={!canManage() || saving() || pairingLoading()}
                  >
                    {pairingLoading()
                      ? 'Generating QR code...'
                      : showPairing()
                        ? 'Refresh QR Code'
                        : 'Pair New Device'}
                  </button>
                  <Show when={showPairing() && pairingPayload()}>
                    <button
                      class={RELAY_SECONDARY_BUTTON_CLASS}
                      onClick={() => void handleCopyPairingPayload()}
                      disabled={!canManage() || pairingLoading()}
                    >
                      Copy Payload
                    </button>
                  </Show>
                </div>

                <p class={formHelpText}>
                  Generate a QR code for pairing during staged mobile beta rollout.
                </p>

                <Show when={showPairing()}>
                  <div class="space-y-3">
                    <Show when={pairingLoading()}>
                      <p class="text-sm text-muted">Preparing pairing payload...</p>
                    </Show>

                    <Show when={!pairingLoading() && pairingQRCode()}>
                      <img
                        src={pairingQRCode()!}
                        alt="Pulse mobile pairing QR code"
                        width="256"
                        height="256"
                        class={RELAY_QR_IMAGE_CLASS}
                      />
                    </Show>

                    <Show when={pairingPayload()?.deep_link}>
                      <code class={RELAY_CODE_BLOCK_CLASS}>
                        {pairingPayload()!.deep_link}
                      </code>
                    </Show>

                    <Show when={(pairingPayload()?.diagnostics?.length ?? 0) > 0}>
                      <div class={RELAY_DIAGNOSTICS_WRAP_CLASS}>
                        <p class={RELAY_DIAGNOSTICS_TITLE_CLASS}>Diagnostics</p>
                        {(pairingPayload()?.diagnostics ?? []).map((diagnostic) => (
                          <div class={getRelayDiagnosticClass(diagnostic.severity)}>
                            <p class="font-medium">{diagnostic.message}</p>
                            <p class="mt-0.5 font-mono">
                              {diagnostic.code}
                              <Show when={diagnostic.field}> | field: {diagnostic.field}</Show>
                            </p>
                          </div>
                        ))}
                      </div>
                    </Show>
                  </div>
                </Show>
              </div>
            </Card>
          </div>
        </Show>
      </Show>
    </SettingsPanel>
  );
};
