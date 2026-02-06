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
import { logger } from '@/utils/logger';

export const RelaySettingsPanel: Component = () => {
  const [config, setConfig] = createSignal<RelayConfig | null>(null);
  const [status, setStatus] = createSignal<RelayStatus | null>(null);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [serverUrl, setServerUrl] = createSignal('');

  let statusInterval: ReturnType<typeof setInterval> | undefined;

  const loadConfig = async () => {
    try {
      const cfg = await RelayAPI.getConfig();
      setConfig(cfg);
      setServerUrl(cfg.server_url);
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
        description="Connect to the Pulse relay for mobile app access."
        icon={<RadioTower size={20} strokeWidth={2} />}
      >
        <Show when={!loading()} fallback={<div class="text-sm text-gray-500">Loading...</div>}>
          <Card tone="info" padding="md">
            <div class="flex items-start gap-3">
              <RadioTower size={20} class="text-blue-500 mt-0.5 flex-shrink-0" strokeWidth={2} />
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  Pulse Pro Required
                </p>
                <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                  Remote access via Pulse Relay requires a Pulse Pro license. Upgrade to access your infrastructure from anywhere.
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
      description="Connect to the Pulse relay for mobile app access to your infrastructure."
      icon={<RadioTower size={20} strokeWidth={2} />}
    >
      <Show when={!loading()} fallback={<div class="text-sm text-gray-500">Loading configuration...</div>}>
        {/* Connection Status */}
        <Card padding="md">
          <div class="flex items-center gap-3">
            <StatusDot
              variant={connectionStatusVariant()}
              size="md"
              pulse={config()?.enabled && status()?.connected}
            />
            <div class="flex-1">
              <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                {connectionStatusText()}
              </p>
              <Show when={status()?.instance_id}>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                  Instance: {status()!.instance_id}
                </p>
              </Show>
              <Show when={status()?.connected && (status()!.active_channels > 0)}>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {status()!.active_channels} active {status()!.active_channels === 1 ? 'channel' : 'channels'}
                </p>
              </Show>
            </div>
          </div>
          <Show when={status()?.last_error}>
            <div class="mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 rounded px-2 py-1">
              {status()!.last_error}
            </div>
          </Show>
        </Card>

        {/* Enable/Disable Toggle */}
        <div class={formField}>
          <div class="flex items-center justify-between">
            <div>
              <label class={labelClass}>Enable Remote Access</label>
              <p class={formHelpText}>
                Connect this Pulse instance to the relay server for mobile app access.
              </p>
            </div>
            <Toggle
              checked={config()?.enabled ?? false}
              onChange={(e) => void handleToggleEnabled(e.checked)}
              disabled={saving()}
            />
          </div>
        </div>

        {/* Server URL */}
        <div class={formField}>
          <label class={labelClass}>Relay Server URL</label>
          <div class="flex gap-2">
            <input
              type="text"
              class={controlClass}
              value={serverUrl()}
              onInput={(e) => setServerUrl(e.currentTarget.value)}
              placeholder="wss://relay.pulserelay.pro/ws/instance"
              disabled={saving()}
            />
            <Show when={serverUrl() !== config()?.server_url}>
              <button
                class="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50"
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
            <label class={labelClass}>Instance Fingerprint</label>
            <code class="block text-xs font-mono text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 rounded px-3 py-2 select-all break-all">
              {config()!.identity_fingerprint}
            </code>
            <p class={formHelpText}>
              This fingerprint uniquely identifies your Pulse instance. The mobile app will verify this fingerprint to prevent man-in-the-middle attacks.
            </p>
          </div>
        </Show>
      </Show>
    </SettingsPanel>
  );
};
