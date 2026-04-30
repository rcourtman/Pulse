import { createEffect, createSignal, onMount } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { showSuccess, showWarning } from '@/utils/toast';
import { hasFeature } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { loadRuntimeCapabilities } from '@/stores/license';
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

  const canManage = () => canManageOverride !== false;
  const showUpgradePrompts = () => !presentationPolicyHidesUpgradePrompts();
  const isAuditLoggingEnabled = () => hasFeature('audit_logging');
  const upgradeDestination = () => getUpgradeActionDestination('audit_logging');

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

  createEffect(() => {
    if (hasFeature('audit_logging')) {
      void fetchWebhooks();
    } else {
      setLoading(false);
    }
  });

  return {
    canManage,
    handleAddWebhook,
    handleRemoveWebhook,
    isAuditLoggingEnabled,
    loading,
    newUrl,
    saving,
    setNewUrl,
    showUpgradePrompts,
    upgradeDestination,
    webhookUrls,
  };
};
