import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import {
  hasFeature,
  runtimeCapabilitiesLoaded,
} from '@/stores/license';
import {
  canOfferCommercialTrial,
  getUpgradeActionDestination,
} from '@/stores/licenseCommercial';
import { loadRuntimeCapabilities } from '@/stores/license';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import { showError, showSuccess } from '@/utils/toast';
import { RelayAPI, type RelayConfig, type RelayStatus } from '@/api/relay';
import { OnboardingAPI, type OnboardingQRResponse } from '@/api/onboarding';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { logger } from '@/utils/logger';
import { getRelayConnectionPresentation } from '@/utils/relayPresentation';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import QRCode from 'qrcode';

export interface RelaySettingsPanelProps {
  canManage?: boolean;
}

export function useRelaySettingsPanelState(props: RelaySettingsPanelProps) {
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
  const canStartTrial = () => canOfferCommercialTrial();
  const relayEnabled = () => hasFeature('relay');
  const upgradeDestination = () => getUpgradeActionDestination('relay');
  const connectionPresentation = createMemo(() =>
    getRelayConnectionPresentation(config(), status()),
  );
  const canShowPairing = createMemo(() => Boolean(config()?.enabled && status()?.connected));

  createEffect((wasPaywallVisible: boolean) => {
    const isPaywallVisible = runtimeCapabilitiesLoaded() && !relayEnabled();
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
    } catch (error) {
      logger.warn('[RelaySettings] Failed to clean up relay pairing token', error);
    }
  };

  const getPairingTokenRecord = async (tokenID: string): Promise<APITokenRecord | null> => {
    try {
      return await SecurityAPI.getToken(tokenID);
    } catch (error) {
      const status = (error as { status?: number } | null)?.status;
      if (status === 404) {
        return null;
      }
      throw error;
    }
  };

  const deletePairingTokenIfUnused = async (tokenID: string | null) => {
    if (!tokenID) return;
    try {
      const record = await getPairingTokenRecord(tokenID);
      if (!record || record.lastUsedAt) {
        return;
      }
      await deletePairingToken(tokenID);
    } catch (error) {
      logger.warn('[RelaySettings] Failed to inspect relay pairing token lifecycle', error);
    }
  };

  const loadConfig = async () => {
    try {
      const nextConfig = await RelayAPI.getConfig();
      setConfig(nextConfig);
      setServerUrl(nextConfig.server_url);
      if (!nextConfig.enabled) {
        await deletePairingTokenIfUnused(pairingTokenId());
        resetPairingState();
      }
    } catch (error) {
      logger.error('[RelaySettings] Failed to load config', error);
    }
  };

  const loadStatus = async () => {
    try {
      const nextStatus = await RelayAPI.getStatus();
      setStatus(nextStatus);
    } catch (error) {
      logger.error('[RelaySettings] Failed to load status', error);
    }
  };

  const stopStatusPolling = () => {
    if (statusInterval !== undefined) {
      clearInterval(statusInterval);
      statusInterval = undefined;
    }
  };

  const startStatusPolling = () => {
    stopStatusPolling();
    void loadStatus();
    statusInterval = setInterval(() => void loadStatus(), 5000);
  };

  onMount(async () => {
    await loadRuntimeCapabilities();
    if (!relayEnabled()) {
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
    void deletePairingTokenIfUnused(pairingTokenId());
  });

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        successMessage: 'Remote access trial started',
        showSuccess,
        showError,
      });
    } finally {
      setStartingTrial(false);
    }
  };

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
        await deletePairingTokenIfUnused(pairingTokenId());
        resetPairingState();
        showSuccess('Remote access disabled');
      }
    } catch (error) {
      showError('Failed to update relay configuration');
      logger.error('[RelaySettings] Failed to toggle', error);
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
    } catch (error) {
      showError('Failed to update server URL');
      logger.error('[RelaySettings] Failed to save server URL', error);
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
      const createdToken = await SecurityAPI.createRelayMobileAccessToken();
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
      if (previousTokenId && previousTokenId !== createdTokenId) {
        await deletePairingTokenIfUnused(previousTokenId);
      }
    } catch (error) {
      await deletePairingToken(createdTokenId);
      setPairingPayload(previousPayload);
      setPairingQRCode(previousQRCode);
      setPairingTokenId(previousTokenId);
      showError('Failed to generate pairing QR code');
      logger.error('[RelaySettings] Failed to generate onboarding QR', error);
    } finally {
      setPairingLoading(false);
    }
  };

  const handleHidePairing = async () => {
    await deletePairingTokenIfUnused(pairingTokenId());
    resetPairingState();
  };

  const handleCopyPairingPayload = async () => {
    const payload = pairingPayload();
    if (!payload) return;

    try {
      await navigator.clipboard.writeText(JSON.stringify(payload, null, 2));
      showSuccess('Pairing payload copied');
    } catch (error) {
      showError('Failed to copy pairing payload');
      logger.error('[RelaySettings] Failed to copy pairing payload', error);
    }
  };

  return {
    canManage,
    canShowPairing,
    canStartTrial,
    config,
    connectionPresentation,
    handleCopyPairingPayload,
    handleHidePairing,
    handlePairNewDevice,
    handleSaveServerUrl,
    handleStartTrial,
    handleToggleEnabled,
    loading,
    pairingLoading,
    pairingPayload,
    pairingQRCode,
    relayEnabled,
    saving,
    serverUrl,
    setServerUrl,
    showPairing,
    startingTrial,
    status,
    upgradeDestination,
  };
}
