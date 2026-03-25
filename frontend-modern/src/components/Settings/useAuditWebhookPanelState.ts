import { createEffect, createSignal, onMount } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { showSuccess, showWarning } from '@/utils/toast';
import {
  entitlements,
  getUpgradeActionUrlOrFallback,
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import {
  getProTrialStartedMessage,
  getTrialStartErrorMessage,
} from '@/utils/upgradePresentation';
import {
  getAuditWebhookDuplicateUrlMessage,
  getAuditWebhookInvalidUrlMessage,
  getAuditWebhookSaveErrorMessage,
  getAuditWebhookSaveSuccessMessage,
} from '@/utils/auditWebhookPresentation';

export const useAuditWebhookPanelState = (canManageOverride?: boolean) => {
  const [webhookUrls, setWebhookUrls] = createSignal<string[]>([]);
  const [newUrl, setNewUrl] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [loading, setLoading] = createSignal(true);
  const [startingTrial, setStartingTrial] = createSignal(false);

  const canManage = () => canManageOverride !== false;
  const canStartTrial = () => entitlements()?.trial_eligible !== false;
  const isAuditLoggingEnabled = () => hasFeature('audit_logging');
  const upgradeActionUrl = () => getUpgradeActionUrlOrFallback('audit_logging');

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        window.location.href = result.actionUrl;
        return;
      }
      showSuccess(getProTrialStartedMessage());
    } catch (error) {
      showWarning(getTrialStartErrorMessage(error));
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
    loadLicenseStatus();
  });

  createEffect((wasPaywallVisible: boolean) => {
    const isPaywallVisible = licenseLoaded() && !hasFeature('audit_logging');
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
    upgradeActionUrl,
    webhookUrls,
  };
};
