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

  const fetchWebhooks = async () => {
    if (disposed) return;
    ensureCurrentContext();
    if (!canRead() || !isAuditLoggingEnabled() || readController || saveController) return;
    const reader = context();
    const controller = new AbortController();
    readController = controller;
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
    } catch {
      if (isCurrent()) setActionError(getAuditWebhookSaveErrorMessage());
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
    try {
      new URL(url);
    } catch {
      setActionError(getAuditWebhookInvalidUrlMessage());
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
    // A same-reader refresh retains accepted destinations/draft on failure.
    void fetchWebhooks();
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
    hasLoadedConfiguration,
    newUrl,
    openRemoveWebhook,
    pendingRemoval,
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
