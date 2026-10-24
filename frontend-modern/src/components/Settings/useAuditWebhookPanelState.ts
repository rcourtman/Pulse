import { createEffect, createSignal, onMount } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { showSuccess, showWarning } from '@/utils/toast';
import { getRuntimeCapabilityBlock, hasFeature } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { loadRuntimeCapabilities } from '@/stores/license';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';
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
  const auditLoggingRuntimeBlock = () => getRuntimeCapabilityBlock('audit_logging');
  const paidRuntimeRequired = () => auditLoggingRuntimeBlock()?.reason === 'paid_runtime_required';
  const showUpgradePrompts = () =>
    !paidRuntimeRequired() && !presentationPolicyHidesUpgradePrompts();
  const showFeatureGateAction = () => paidRuntimeRequired() || showUpgradePrompts();
  const isAuditLoggingEnabled = () => hasFeature('audit_logging');
  const upgradeDestination = () =>
    paidRuntimeRequired()
      ? resolveUpgradeDestination(auditLoggingRuntimeBlock()?.action_url)
      : getUpgradeActionDestination('audit_logging');
  const upgradeActionLabel = () => (paidRuntimeRequired() ? 'Download Pulse Pro' : 'View plans');

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
      return true;
    } catch (error) {
      logger.error('[AuditWebhookPanel] Failed to save audit webhooks', error);
      showWarning(getAuditWebhookSaveErrorMessage());
      return false;
    } finally {
      setSaving(false);
    }
  };

  const handleAddWebhook = async () => {
    if (!canManage() || saving()) return;
    const draft = newUrl();
    const url = draft.trim();
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

    const saved = await saveWebhooks([...webhookUrls(), url]);
    // Leave failed entries available for correction/retry, and do not erase
    // a different draft typed while the submitted entry was saving.
    if (saved && newUrl() === draft) setNewUrl('');
  };

  const handleRemoveWebhook = async (url: string) => {
    if (!canManage() || saving()) return;
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
    showFeatureGateAction,
    upgradeDestination,
    upgradeActionLabel,
    paidRuntimeRequired,
    webhookUrls,
  };
};
