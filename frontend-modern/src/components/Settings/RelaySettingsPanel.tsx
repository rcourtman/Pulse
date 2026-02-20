import { Component, createSignal, onMount, onCleanup, Show } from 'solid-js';
import RadioTower from 'lucide-solid/icons/radio-tower';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { StatusDot } from '@/components/shared/StatusDot';
import { Toggle } from '@/components/shared/Toggle';
import { Card } from '@/components/shared/Card';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { hasFeature, loadLicenseStatus } from '@/stores/license';
import { showSuccess, showError } from '@/utils/toast';
import { RelayAPI, type RelayConfig, type RelayStatus } from '@/api/relay';
import { OnboardingAPI, type OnboardingQRResponse } from '@/api/onboarding';
import { logger } from '@/utils/logger';
import QRCode from 'qrcode';

export const RelaySettingsPanel: Component = () => {
  const [config, setConfig] = createSignal<RelayConfig | null>(null);
  const [status, setStatus] = createSignal<RelayStatus | null>(null);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [serverUrl, setServerUrl] = createSignal('');
  const [showPairing, setShowPairing] = createSignal(false);
  const [pairingLoading, setPairingLoading] = createSignal(false);
  const [pairingPayload, setPairingPayload] = createSignal<OnboardingQRResponse | null>(null);
  const [pairingQRCode, setPairingQRCode] = createSignal<string | null>(null);

  let statusInterval: ReturnType<typeof setInterval> | undefined;

  const resetPairingState = () => {
    setShowPairing(false);
    setPairingLoading(false);
    setPairingPayload(null);
    setPairingQRCode(null);
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
    setShowPairing(true);
    setPairingLoading(true);
    try {
      const payload = await OnboardingAPI.getQRPayload();
      const qrCodeDataUrl = await QRCode.toDataURL(payload.deep_link, {
        width: 256,
        margin: 2,
      });
      setPairingPayload(payload);
      setPairingQRCode(qrCodeDataUrl);
    } catch (err) {
      setPairingPayload(null);
      setPairingQRCode(null);
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

  const connectionStatusVariant = () => {
    const cfg = config();
    const st = status();
    if (!cfg?.enabled) return 'muted' as const;
    if (st?.connected) return 'success' as const;
    return 'danger' as const;
  };

  const connectionStatusText = () => {
    const cfg = config();
    const st = status();
    if (!cfg?.enabled) return 'Not enabled';
    if (st?.connected) return 'Connected';
    return 'Disconnected';
  };

  // Pro feature gate
  if (!hasFeature('relay')) {
    return (
      <SettingsPanel
        title="Remote Access"
        description="Configure Pulse relay connectivity for secure remote access."
        icon={<RadioTower size={20} strokeWidth={2} />}
      >
        <Show when={!loading()} fallback={<div class="text-sm text-slate-500">Loading...</div>}>
          <Card tone="info" padding="md">
            <div class="flex items-start gap-3">
              <RadioTower size={20} class="text-blue-500 mt-0.5 flex-shrink-0" strokeWidth={2} />
              <div>
                <p class="text-sm font-medium text-slate-900 dark:text-slate-100">
                  Pro Required
                </p>
                <p class="text-sm text-slate-600 dark:text-slate-400 mt-1">
                  Remote access via Pulse Relay requires a Pro license. Upgrade to access your infrastructure from anywhere.
                </p>
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
      description="Configure Pulse relay connectivity for secure remote access."
      icon={<RadioTower size={20} strokeWidth={2} />}
    >
      <Show when={!loading()} fallback={<div class="text-sm text-slate-500">Loading configuration...</div>}>
        {/* Connection Status */}
        <Card padding="md">
          <div class="flex items-center gap-3">
            <StatusDot
              variant={connectionStatusVariant()}
              size="md"
              pulse={config()?.enabled && status()?.connected}
            />
            <div class="flex-1">
              <p class="text-sm font-medium text-slate-900 dark:text-slate-100">
                {connectionStatusText()}
              </p>
              <Show when={status()?.instance_id}>
                <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Instance: {status()!.instance_id}
                </p>
              </Show>
              <Show when={status()?.connected && (status()!.active_channels > 0)}>
                <p class="text-xs text-slate-500 dark:text-slate-400">
                  {status()!.active_channels} active {status()!.active_channels === 1 ? 'channel' : 'channels'}
                </p>
              </Show>
            </div>
          </div>
          <Show when={status()?.last_error}>
            <div class="mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900 rounded px-2 py-1">
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
                Connect this Pulse instance to the relay server for mobile app access.
              </p>
            </div>
            <Toggle
              checked={config()?.enabled ?? false}
              onChange={(e) => void handleToggleEnabled(e.currentTarget.checked)}
              disabled={saving()}
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
              disabled={saving()}
            />
            <Show when={serverUrl() !== config()?.server_url}>
              <button
                class="min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50 sm:self-auto self-end"
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
            <code class="block text-xs font-mono text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 rounded px-3 py-2 select-all break-all">
              {config()!.identity_fingerprint}
            </code>
            <p class={formHelpText}>
              This fingerprint uniquely identifies your Pulse instance. The mobile app will verify this fingerprint to prevent man-in-the-middle attacks.
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
                    class="min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50"
                    onClick={() => void handlePairNewDevice()}
                    disabled={saving() || pairingLoading()}
                  >
                    {pairingLoading()
                      ? 'Generating QR code...'
                      : showPairing()
                        ? 'Refresh QR Code'
                        : 'Pair New Device'}
                  </button>
                  <Show when={showPairing() && pairingPayload()}>
                    <button
                      class="min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-md disabled:opacity-50"
                      onClick={() => void handleCopyPairingPayload()}
                      disabled={pairingLoading()}
                    >
                      Copy Payload
                    </button>
                  </Show>
                </div>

                <p class={formHelpText}>
                  Generate a QR code and scan it from the Pulse mobile app to pair a new device.
                </p>

                <Show when={showPairing()}>
                  <div class="space-y-3">
                    <Show when={pairingLoading()}>
                      <p class="text-sm text-slate-600 dark:text-slate-300">
                        Preparing pairing payload...
                      </p>
                    </Show>

                    <Show when={!pairingLoading() && pairingQRCode()}>
                      <img
                        src={pairingQRCode()!}
                        alt="Pulse mobile pairing QR code"
                        width="256"
                        height="256"
                        class="rounded-md border border-slate-200 dark:border-slate-700 bg-white p-2"
                      />
                    </Show>

                    <Show when={pairingPayload()?.deep_link}>
                      <code class="block text-xs font-mono text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 rounded px-3 py-2 select-all break-all">
                        {pairingPayload()!.deep_link}
                      </code>
                    </Show>

                    <Show when={(pairingPayload()?.diagnostics?.length ?? 0) > 0}>
                      <div class="space-y-2">
                        <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">
                          Diagnostics
                        </p>
                        {(pairingPayload()?.diagnostics ?? []).map((diagnostic) => (
                          <div
                            class={`rounded px-2 py-1 text-xs ${
                              diagnostic.severity === 'error'
                                ? 'bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-300'
                                : 'bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300'
                            }`}
                          >
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
