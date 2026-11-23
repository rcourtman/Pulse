import { batch, createEffect, createSignal, on, onCleanup, onMount } from 'solid-js';
import { apiFetchJSON, getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { getRuntimeCapabilityBlock, hasFeature, loadRuntimeCapabilities } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';
import {
  getAuditWebhookDuplicateUrlMessage,
  getAuditWebhookInvalidUrlMessage,
  getAuditWebhookSaveErrorMessage,
  getAuditWebhookSaveSuccessMessage,
} from '@/utils/auditWebhookPresentation';

export interface AuditWebhookPanelProps {
  currentUser?: string;
  canRead?: boolean;
  canManage?: boolean;
}

export const useAuditWebhookPanelState = (props: AuditWebhookPanelProps) => {
  const [webhookUrls, setWebhookUrls] = createSignal<string[]>([]);
  const [newUrl, setNewUrl] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal(false);
  const [actionError, setActionError] = createSignal<string | null>(null);
  const [successNotice, setSuccessNotice] = createSignal<string | null>(null);
  const [hasLoadedConfiguration, setHasLoadedConfiguration] = createSignal(false);
  const [unconfirmedChange, setUnconfirmedChange] = createSignal(false);
  const [pendingRemoval, setPendingRemoval] = createSignal<string | null>(null);
  const currentOrg = () => getOrgID() || 'default';
  const canRead = () => props.canRead !== false;
  const canManage = () => canRead() && props.canManage !== false;
  const isAuditLoggingEnabled = () => hasFeature('audit_logging');
  const snapshot = () => ({
    org: currentOrg(),
    user: props.currentUser,
    read: canRead(),
    write: canManage(),
    enabled: isAuditLoggingEnabled(),
  });
  const [context, setContext] = createSignal(snapshot());
  let disposed = false;
  let readController: AbortController | undefined;
  let saveController: AbortController | undefined;
  const contextIsCurrent = () => {
    const current = snapshot();
    const accepted = context();
    return (
      !disposed &&
      current.org === accepted.org &&
      current.user === accepted.user &&
      current.read === accepted.read &&
      current.write === accepted.write &&
      current.enabled === accepted.enabled
    );
  };

  const auditLoggingRuntimeBlock = () => getRuntimeCapabilityBlock('audit_logging');
  const paidRuntimeRequired = () => auditLoggingRuntimeBlock()?.reason === 'paid_runtime_required';
  const showUpgradePrompts = () =>
    !paidRuntimeRequired() && !presentationPolicyHidesUpgradePrompts();
  const showFeatureGateAction = () => paidRuntimeRequired() || showUpgradePrompts();
  const upgradeDestination = () =>
    paidRuntimeRequired()
      ? resolveUpgradeDestination(auditLoggingRuntimeBlock()?.action_url)
      : getUpgradeActionDestination('audit_logging');
  const upgradeActionLabel = () => (paidRuntimeRequired() ? 'Download Pulse Pro' : 'View plans');
  const canEdit = () =>
    contextIsCurrent() &&
    canManage() &&
    isAuditLoggingEnabled() &&
    hasLoadedConfiguration() &&
    !loading() &&
    !loadError() &&
    !unconfirmedChange() &&
    !saving();

  const resetContext = () => {
    readController?.abort();
    saveController?.abort();
    readController = saveController = undefined;
    batch(() => {
      // Replacing the snapshot invalidates old completions even if an ID is reused.
      setContext(snapshot());
      setWebhookUrls([]);
      setHasLoadedConfiguration(false);
      setUnconfirmedChange(false);
      setNewUrl('');
      setPendingRemoval(null);
      setActionError(null);
      setSuccessNotice(null);
      setSaving(false);
      setLoadError(false);
      setLoading(false);
    });
  };
  const ensureCurrentContext = () => {
    if (contextIsCurrent()) return true;
    if (!disposed) {
      resetContext();
      // Storage can change without an org event. Withdraw first; require a read,
      // never turn the interrupted action into a replacement in another org.
      setLoadError(true);
    }
    return false;
  };

  const loadWebhooks = async (restart = false) => {
    if (disposed) return;
    // A stale Restart belongs to the old reader. Withdraw it instead of
    // turning it into a fresh check in a different organization.
    if (!ensureCurrentContext() && restart) return;
    if (!canRead() || !isAuditLoggingEnabled() || saveController) return;
    const previousRead = readController;
    if (restart ? !previousRead : previousRead) return;
    const reader = context();
    const controller = new AbortController();
    readController = controller;
    previousRead?.abort();
    const isCurrent = () =>
      contextIsCurrent() && reader === context() && readController === controller;
    batch(() => {
      setPendingRemoval(null);
      setActionError(null);
      setSuccessNotice(null);
      setLoading(true);
      setLoadError(false);
    });
    try {
      const data = await apiFetchJSON<{ urls: string[] | null }>('/api/admin/webhooks/audit', {
        expectedOrgID: reader.org,
        retry: false,
        signal: controller.signal,
      });
      if (!isCurrent()) return;
      // Only a known empty selection is empty. Unknown configuration must not
      // enable replacing destinations; the server encodes a nil slice as null.
      if (
        !data ||
        (data.urls !== null &&
          (!Array.isArray(data.urls) || data.urls.some((url) => typeof url !== 'string')))
      ) {
        throw new Error('Invalid audit webhook configuration');
      }
      batch(() => {
        setWebhookUrls(data.urls ?? []);
        setHasLoadedConfiguration(true);
        setUnconfirmedChange(false);
      });
    } catch {
      if (isCurrent()) setLoadError(true);
    } finally {
      if (isCurrent()) {
        readController = undefined;
        setLoading(false);
      } else if (!disposed && reader === context() && !contextIsCurrent()) {
        ensureCurrentContext();
      }
    }
  };
  const fetchWebhooks = () => loadWebhooks();
  const restartWebhooksRead = () => loadWebhooks(true);

  const saveWebhooks = async (urls: string[]) => {
    if (!ensureCurrentContext() || !canEdit()) return false;
    const reader = context();
    const controller = new AbortController();
    saveController = controller;
    const isCurrent = () =>
      contextIsCurrent() && reader === context() && saveController === controller;
    batch(() => {
      setSaving(true);
      setActionError(null);
      setSuccessNotice(null);
    });
    try {
      await apiFetchJSON('/api/admin/webhooks/audit', {
        method: 'POST',
        body: JSON.stringify({ urls }),
        expectedOrgID: reader.org,
        retry: false,
        signal: controller.signal,
      });
      if (!isCurrent()) return false;
      batch(() => {
        setWebhookUrls(urls);
        setSuccessNotice(getAuditWebhookSaveSuccessMessage());
      });
      return true;
    } catch (error) {
      if (!isCurrent()) return false;
      const status = (error as { status?: number } | null)?.status;
      const code = (error as { code?: unknown } | null)?.code;
      // The handler validates the whole list before saving. A known admission
      // or validation refusal is different from losing the save's result.
      if (status === 400 || status === 401 || status === 403 || status === 429) {
        if (
          status === 400 &&
          typeof code === 'string' &&
          [
            'invalid_url',
            'invalid_webhook_url',
            'ssrf_blocked',
            'private_network_blocked',
            'webhook_blocked',
          ].includes(code)
        ) {
          setActionError(
            `${getAuditWebhookSaveErrorMessage()}: the destination URL is invalid or blocked by network policy.`,
          );
        } else {
          setActionError(getAuditWebhookSaveErrorMessage());
        }
      } else {
        // Add and Remove both replace the whole list. Do not offer another
        // write based on the old list until the current reader checks it.
        setUnconfirmedChange(true);
      }
      return false;
    } finally {
      if (isCurrent()) {
        saveController = undefined;
        setSaving(false);
      } else if (!disposed && reader === context() && !contextIsCurrent()) {
        ensureCurrentContext();
      }
    }
  };

  const handleAddWebhook = async () => {
    if (!ensureCurrentContext() || !canEdit()) return;
    const draft = newUrl();
    const url = draft.trim();
    if (!url) return;
    setSuccessNotice(null);
    let parsed: URL;
    try {
      parsed = new URL(url);
    } catch {
      setActionError(getAuditWebhookInvalidUrlMessage());
      return;
    }
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
      setActionError('Please enter a valid http:// or https:// URL');
      return;
    }
    if (webhookUrls().includes(url)) {
      setActionError(getAuditWebhookDuplicateUrlMessage());
      return;
    }
    const reader = context();
    const saved = await saveWebhooks([...webhookUrls(), url]);
    // An obsolete completion cannot erase a later reader's matching draft.
    if (saved && contextIsCurrent() && context() === reader && newUrl() === draft) setNewUrl('');
  };

  const openRemoveWebhook = (url: string) => {
    if (!ensureCurrentContext() || !canEdit() || !webhookUrls().includes(url)) return;
    batch(() => {
      setActionError(null);
      setSuccessNotice(null);
      setPendingRemoval(url);
    });
  };
  const closeRemoveWebhook = () => {
    if (saving()) return;
    setPendingRemoval(null);
    setActionError(null);
  };
  const handleRemoveWebhook = async () => {
    if (!ensureCurrentContext() || !canEdit()) return;
    const url = pendingRemoval();
    if (!url || !webhookUrls().includes(url)) return;
    const reader = context();
    const saved = await saveWebhooks(webhookUrls().filter((existingUrl) => existingUrl !== url));
    if (saved && contextIsCurrent() && context() === reader) setPendingRemoval(null);
  };
  const updateNewUrl = (url: string) => {
    if (!ensureCurrentContext() || !canManage() || !isAuditLoggingEnabled()) return;
    setNewUrl(url);
    if (!pendingRemoval() && !saving()) setActionError(null);
  };

  onMount(() => {
    void loadRuntimeCapabilities();
  });
  createEffect(
    on(
      [isAuditLoggingEnabled, canRead, canManage, () => props.currentUser],
      (current, previous) => {
        // A copied status object is not a new reader: keep their current draft.
        if (previous && current.every((value, index) => value === previous[index])) return;
        resetContext();
        void fetchWebhooks();
      },
    ),
  );
  const unsubscribeOrg = eventBus.on('org_switched', () => {
    if (!contextIsCurrent()) resetContext();
    // An incidental refresh doesn't stand in for an explicit saved check.
    if (!unconfirmedChange()) void fetchWebhooks();
  });
  onCleanup(() => {
    disposed = true;
    unsubscribeOrg();
    resetContext();
  });

  return {
    actionError,
    canEdit,
    canManage,
    canRead,
    closeRemoveWebhook,
    fetchWebhooks,
    handleAddWebhook,
    handleRemoveWebhook,
    isAuditLoggingEnabled,
    loading,
    loadError,
    unconfirmedChange: () => contextIsCurrent() && unconfirmedChange(),
    hasLoadedConfiguration,
    newUrl,
    openRemoveWebhook,
    pendingRemoval,
    restartWebhooksRead,
    saving,
    setNewUrl: updateNewUrl,
    showUpgradePrompts,
    showFeatureGateAction,
    successNotice,
    upgradeDestination,
    upgradeActionLabel,
    paidRuntimeRequired,
    webhookUrls,
  };
};
