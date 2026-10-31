import { batch, createEffect, createSignal, on, onCleanup, onMount } from 'solid-js';
import { apiFetchJSON, getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
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
  const [loadError, setLoadError] = createSignal(false);
  const [hasLoadedConfiguration, setHasLoadedConfiguration] = createSignal(false);
  const currentOrg = () => getOrgID() || 'default';
  let selectedOrg = currentOrg();
  let request = 0;
  let disposed = false;
  let readController: AbortController | undefined;
  let saveController: AbortController | undefined;

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
  const canEdit = () =>
    canManage() &&
    isAuditLoggingEnabled() &&
    hasLoadedConfiguration() &&
    !loading() &&
    !loadError() &&
    selectedOrg === currentOrg();

  const invalidateRequests = () => {
    request += 1;
    readController?.abort();
    readController = undefined;
    saveController?.abort();
  };

  const selectCurrentOrg = () => {
    if (selectedOrg === currentOrg()) return;
    invalidateRequests();
    selectedOrg = currentOrg();
    batch(() => {
      setWebhookUrls([]);
      setHasLoadedConfiguration(false);
      setNewUrl('');
      setLoadError(true);
      setLoading(false);
    });
  };

  const fetchWebhooks = async () => {
    selectCurrentOrg();
    if (disposed || !isAuditLoggingEnabled() || readController || saving()) return;
    const org = selectedOrg;
    const thisRequest = ++request;
    const isCurrent = () =>
      !disposed && thisRequest === request && org === currentOrg() && isAuditLoggingEnabled();
    readController = new AbortController();
    setLoading(true);
    setLoadError(false);
    try {
      const data = await apiFetchJSON<{ urls: string[] | null }>('/api/admin/webhooks/audit', {
        expectedOrgID: org,
        retry: false,
        signal: readController.signal,
      });
      if (!isCurrent()) return;
      // The server can encode a nil slice as null. Only an explicit empty
      // selection is empty; an unexpected payload must not enable replacement.
      if (
        !data ||
        (data.urls !== null &&
          (!Array.isArray(data.urls) || data.urls.some((url) => typeof url !== 'string')))
      ) {
        throw new Error('Invalid audit webhook configuration');
      }
      setWebhookUrls(data.urls ?? []);
      setHasLoadedConfiguration(true);
    } catch (error) {
      if (!isCurrent()) return;
      setLoadError(true);
      logger.error('[AuditWebhookPanel] Failed to fetch audit webhooks', error);
    } finally {
      if (!disposed && thisRequest === request) {
        readController = undefined;
        setLoading(false);
        selectCurrentOrg();
      }
    }
  };

  const saveWebhooks = async (urls: string[]) => {
    selectCurrentOrg();
    if (!canEdit() || saving() || disposed) return false;
    const org = selectedOrg;
    const thisRequest = request;
    const isCurrent = () =>
      !disposed && thisRequest === request && org === currentOrg() && isAuditLoggingEnabled();
    saveController = new AbortController();
    setSaving(true);
    try {
      await apiFetchJSON('/api/admin/webhooks/audit', {
        method: 'POST',
        body: JSON.stringify({ urls }),
        expectedOrgID: org,
        retry: false,
        signal: saveController.signal,
      });
      if (!isCurrent()) return false;
      setWebhookUrls(urls);
      showSuccess(getAuditWebhookSaveSuccessMessage());
      return true;
    } catch (error) {
      if (!isCurrent()) return false;
      logger.error('[AuditWebhookPanel] Failed to save audit webhooks', error);
      showWarning(getAuditWebhookSaveErrorMessage());
      return false;
    } finally {
      saveController = undefined;
      if (!disposed) {
        setSaving(false);
        selectCurrentOrg();
      }
    }
  };

  const handleAddWebhook = async () => {
    selectCurrentOrg();
    if (!canEdit() || saving() || disposed) return;
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
    selectCurrentOrg();
    if (!canEdit() || saving() || disposed || !webhookUrls().includes(url)) return;
    await saveWebhooks(webhookUrls().filter((existingUrl) => existingUrl !== url));
  };

  onMount(() => {
    loadRuntimeCapabilities();
  });

  const unsubscribeOrg = eventBus.on('org_switched', () => {
    selectCurrentOrg();
    // Read the new selection, never replay an interrupted save.
    void fetchWebhooks();
  });

  createEffect(
    on(isAuditLoggingEnabled, (enabled) => {
      if (enabled) {
        void fetchWebhooks();
      } else {
        invalidateRequests();
        // A capability refresh is not an empty configuration. Keep this
        // selection's last accepted data hidden behind the feature gate.
        // An interrupted save may have reached the server. Require a read
        // before another replacement, even when the selection is unchanged.
        if (saving()) setLoadError(true);
        setLoading(false);
      }
    }),
  );

  onCleanup(() => {
    disposed = true;
    invalidateRequests();
    unsubscribeOrg();
  });

  return {
    canEdit,
    canManage,
    fetchWebhooks,
    handleAddWebhook,
    handleRemoveWebhook,
    isAuditLoggingEnabled,
    loading,
    loadError,
    hasLoadedConfiguration,
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
