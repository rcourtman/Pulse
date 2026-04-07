import { createEffect, createSignal, onMount } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { showSuccess, showWarning } from '@/utils/toast';
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
import {
  getAuditWebhookDuplicateUrlMessage,
  getAuditWebhookInvalidUrlMessage,
  getAuditWebhookSaveErrorMessage,
  getAuditWebhookSaveSuccessMessage,
} from '@/utils/auditWebhookPresentation';
import { runStartProTrialAction } from '@/utils/trialStartAction';

export const useAuditWebhookPanelState = (canManageOverride?: boolean) => {
  const [webhookUrls, setWebhookUrls] = createSignal<string[]>([]);
  const [newUrl, setNewUrl] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [loading, setLoading] = createSignal(true);
  const [startingTrial, setStartingTrial] = createSignal(false);

  const canManage = () => canManageOverride !== false;
  const canStartTrial = () => canOfferCommercialTrial();
  const isAuditLoggingEnabled = () => hasFeature('audit_logging');
  const upgradeDestination = () => getUpgradeActionDestination('audit_logging');

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        showSuccess,
        showError: showWarning,
      });
    } finally {
      setStartingTrial(false);
    }
  };

  const fetchWebhooks = async () => {
    try {
      const data = await apiFetchJSON<{ urls: string[] }>('/api/admin/webhooks/audit');
      setWebhookUrls(data.urls || []);
    } catch (error) {
      logger.error('[AuditWebhookPanel] Failed to fetch audit webhooks', error);
    } finally {
      setLoading(false);
    }
  };

  const saveWebhooks = async (urls: string[]) => {
    setSaving(true);
    try {
      await apiFetchJSON('/api/admin/webhooks/audit', {
        method: 'POST',
        body: JSON.stringify({ urls }),
      });
      setWebhookUrls(urls);
      showSuccess(getAuditWebhookSaveSuccessMessage());
    } catch (error) {
      logger.error('[AuditWebhookPanel] Failed to save audit webhooks', error);
      showWarning(getAuditWebhookSaveErrorMessage());
    } finally {
      setSaving(false);
    }
  };

  const handleAddWebhook = async () => {
    if (!canManage()) return;
    const url = newUrl().trim();
    if (!url) return;

    try {
      new URL(url);
    } catch {
      showWarning(getAuditWebhookInvalidUrlMessage());
      return;
    }

    if (webhookUrls().includes(url)) {
      showWarning(getAuditWebhookDuplicateUrlMessage());
      return;
    }

    await saveWebhooks([...webhookUrls(), url]);
    setNewUrl('');
  };

  const handleRemoveWebhook = async (url: string) => {
    if (!canManage()) return;
    await saveWebhooks(webhookUrls().filter((existingUrl) => existingUrl !== url));
  };

  onMount(() => {
    loadRuntimeCapabilities();
  });

  createEffect((wasPaywallVisible: boolean) => {
    const isPaywallVisible = runtimeCapabilitiesLoaded() && !hasFeature('audit_logging');
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('audit_logging', 'settings_audit_webhook_panel');
    }
    return isPaywallVisible;
  }, false);

  createEffect(() => {
    if (hasFeature('audit_logging')) {
      void fetchWebhooks();
    } else {
      setLoading(false);
    }
  });

  return {
    canManage,
    canStartTrial,
    handleAddWebhook,
    handleRemoveWebhook,
    handleStartTrial,
    isAuditLoggingEnabled,
    loading,
    newUrl,
    saving,
    setNewUrl,
    startingTrial,
    upgradeDestination,
    webhookUrls,
  };
};
