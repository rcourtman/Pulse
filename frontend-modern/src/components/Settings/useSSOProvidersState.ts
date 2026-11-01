import { batch, createEffect, createMemo, createSignal, on, onCleanup, onMount } from 'solid-js';
import { apiFetch, apiErrorFromResponse, getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { createStore } from 'solid-js/store';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  getSSOCopySuccessMessage,
  getSSOCopyErrorMessage,
  getSSOConnectionTestErrorMessage,
  getSSOMetadataFetchErrorMessage,
  getSSOMetadataUrlRequiredMessage,
  getSSOProviderDeleteErrorMessage,
  getSSOProviderDeleteSuccessMessage,
  getSSOProviderDetailsLoadErrorMessage,
  getSSOProviderSaveErrorMessage,
  getSSOProviderSaveSuccessMessage,
  getSSOProviderToggleErrorMessage,
  getSSOProviderToggleSuccessMessage,
  getSSOTestResultPresentation,
} from '@/utils/ssoProviderPresentation';
import type {
  MetadataPreview,
  ProviderForm,
  SSOProvider,
  SSOProvidersResponse,
  SSOProviderTestResult,
} from '@/components/Settings/ssoProvidersModel';
import {
  buildMetadataPreviewPayload,
  buildProviderPayload,
  buildProviderTestPayload,
  canTestProviderForm,
  createEmptyProviderForm,
  mapProviderDetailsToForm,
} from '@/components/Settings/ssoProvidersModel';

interface SSOProvidersPanelProps {
  onConfigUpdated?: () => void;
  canManage?: boolean;
  currentUser?: string;
}

export const useSSOProvidersState = (props: SSOProvidersPanelProps) => {
  const [providers, setProviders] = createSignal<SSOProvider[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [editorFeedback, setEditorFeedback] = createSignal<{
    tone: 'success' | 'error';
    message: string;
  } | null>(null);
  const [showModal, setShowModalValue] = createSignal(false);
  const [editingProvider, setEditingProvider] = createSignal<SSOProvider | null>(null);
  const [form, setForm] = createStore<ProviderForm>(createEmptyProviderForm());
  const [advancedOpen, setAdvancedOpen] = createSignal(false);
  const [deleteConfirm, setDeleteConfirmValue] = createSignal<string | null>(null);
  const [publicUrl, setPublicUrl] = createSignal('');
  const [testing, setTesting] = createSignal(false);
  const [testResult, setTestResult] = createSignal<SSOProviderTestResult | null>(null);
  const [showMetadataPreview, setShowMetadataPreviewValue] = createSignal(false);
  const [metadataPreview, setMetadataPreview] = createSignal<MetadataPreview | null>(null);
  const [loadingPreview, setLoadingPreview] = createSignal(false);

  const [loadError, setLoadError] = createSignal(false);
  const [providersLoaded, setProvidersLoaded] = createSignal(false);
  const [loadingDetails, setLoadingDetails] = createSignal(false);
  const [mutating, setMutating] = createSignal(false);
  const managementAllowed = createMemo(() => props.canManage !== false);
  const authenticatedUser = createMemo(() => props.currentUser);
  const currentOrg = () => getOrgID() || 'default';
  let selectedOrg = currentOrg();
  let selectedUser = authenticatedUser();
  let selectedManagement = managementAllowed();
  let disposed = false;
  let contextVersion = 0;
  let editorVersion = 0;
  let readController: AbortController | undefined;
  let detailController: AbortController | undefined;
  let testController: AbortController | undefined;
  let previewController: AbortController | undefined;
  let mutationController: AbortController | undefined;

  const contextIsCurrent = () =>
    !disposed &&
    selectedOrg === currentOrg() &&
    selectedUser === authenticatedUser() &&
    selectedManagement === managementAllowed();
  const canManage = () => contextIsCurrent() && managementAllowed();
  const canEdit = () =>
    canManage() && providersLoaded() && !loading() && !loadError() && !mutating();
  const isCurrentProvider = (provider: SSOProvider) => providers().includes(provider);
  const requestOptions = (controller: AbortController) => ({
    signal: controller.signal,
    expectedOrgID: selectedOrg,
    retry: false,
  });

  const clearPreviewAndTest = () => {
    testController?.abort();
    previewController?.abort();
    testController = previewController = undefined;
    batch(() => {
      setTesting(false);
      setTestResult(null);
      setLoadingPreview(false);
      setMetadataPreview(null);
      setShowMetadataPreviewValue(false);
    });
  };
  const setShowMetadataPreview = (open: boolean) => {
    setShowMetadataPreviewValue(open);
    if (!open) setMetadataPreview(null);
  };
  const clearEditor = () => {
    editorVersion += 1;
    detailController?.abort();
    detailController = undefined;
    clearPreviewAndTest();
    batch(() => {
      setLoadingDetails(false);
      setShowModalValue(false);
      setEditingProvider(null);
      setEditorFeedback(null);
      setForm(createEmptyProviderForm());
      setAdvancedOpen(false);
    });
  };
  const setShowModal = (open: boolean) => {
    if (!open && !mutating()) clearEditor();
  };
  const setDeleteConfirm = (id: string | null) => {
    if (mutating()) return;
    if (id === null || (canEdit() && providers().some((provider) => provider.id === id))) {
      clearEditor();
      setDeleteConfirmValue(id);
    }
  };
  // A test/preview describes exactly the submitted draft, not later edits in
  // the same dialog. Closing/replacing that dialog also releases its payload.
  createEffect(
    on(
      () => JSON.stringify(form),
      () => {
        clearPreviewAndTest();
        setEditorFeedback(null);
      },
      { defer: true },
    ),
  );

  const loadProviders = async () => {
    if (!contextIsCurrent() || readController) return;
    const version = contextVersion;
    const controller = new AbortController();
    const isCurrent = () =>
      contextIsCurrent() && version === contextVersion && readController === controller;
    readController = controller;
    setLoading(true);
    setLoadError(false);
    setProvidersLoaded(false);
    setProviders([]);
    try {
      const response = await apiFetch('/api/security/sso/providers', requestOptions(controller));
      if (!isCurrent()) return;
      if (!response.ok) throw new Error('Provider read refused');
      const data = (await response.json()) as SSOProvidersResponse;
      if (!isCurrent()) return;
      const ids = new Set<string>();
      // A missing or malformed list is unknown, not an empty configuration.
      if (
        !data ||
        !Array.isArray(data.providers) ||
        data.providers.some((provider) => {
          if (
            !provider ||
            typeof provider.id !== 'string' ||
            !provider.id.trim() ||
            ids.has(provider.id) ||
            typeof provider.name !== 'string' ||
            !['oidc', 'saml'].includes(provider.type) ||
            typeof provider.enabled !== 'boolean'
          )
            return true;
          ids.add(provider.id);
          return false;
        })
      )
        throw new Error('Invalid provider inventory');
      setProviders(data.providers);
      setProvidersLoaded(true);
      setPublicUrl(window.location.origin);
      // The optional public URL is not evidence that a provider read failed.
      try {
        const statusResponse = await apiFetch('/api/security/status', requestOptions(controller));
        if (statusResponse.ok) {
          const status = (await statusResponse.json()) as { publicUrl?: string };
          if (isCurrent() && typeof status.publicUrl === 'string' && status.publicUrl)
            setPublicUrl(status.publicUrl);
        }
      } catch {
        /* Keep the current origin when optional status is unavailable. */
      }
    } catch {
      if (!isCurrent()) return;
      setLoadError(true);
      logger.error('[SSOProvidersPanel] Failed to load providers.');
    } finally {
      if (isCurrent()) {
        readController = undefined;
        setLoading(false);
      }
    }
  };

  const resetContext = () => {
    contextVersion += 1;
    readController?.abort();
    mutationController?.abort();
    readController = mutationController = undefined;
    selectedOrg = currentOrg();
    selectedUser = authenticatedUser();
    selectedManagement = managementAllowed();
    batch(() => {
      clearEditor();
      setProviders([]);
      setProvidersLoaded(false);
      setPublicUrl('');
      setDeleteConfirmValue(null);
      setSaving(false);
      setMutating(false);
    });
    void loadProviders();
  };
  createEffect(on([managementAllowed, authenticatedUser], resetContext, { defer: true }));
  const unsubscribeOrg = eventBus.on('org_switched', resetContext);
  onMount(() => void loadProviders());
  onCleanup(() => {
    disposed = true;
    contextVersion += 1;
    readController?.abort();
    mutationController?.abort();
    clearEditor();
    setProviders([]);
    unsubscribeOrg();
  });

  const openAddModal = (type: 'oidc' | 'saml') => {
    if (!canEdit()) return;
    clearEditor();
    setDeleteConfirmValue(null);
    setForm('type', type);
    setShowModalValue(true);
  };

  const openEditModal = async (provider: SSOProvider) => {
    if (!canEdit() || !isCurrentProvider(provider)) return;
    clearEditor();
    setDeleteConfirmValue(null);
    const version = editorVersion;
    const context = contextVersion;
    const controller = new AbortController();
    detailController = controller;
    const isCurrent = () =>
      canEdit() &&
      version === editorVersion &&
      context === contextVersion &&
      detailController === controller;
    setLoadingDetails(true);
    try {
      const response = await apiFetch(
        `/api/security/sso/providers/${encodeURIComponent(provider.id)}`,
        requestOptions(controller),
      );
      if (!isCurrent()) return;
      if (!response.ok) throw new Error('Provider details refused');
      const data = await response.json();
      if (!isCurrent()) return;
      if (!data || data.id !== provider.id || data.type !== provider.type)
        throw new Error('Mismatched provider details');
      setForm(mapProviderDetailsToForm(data));
      setEditingProvider(provider);
      setShowModalValue(true);
    } catch {
      if (!isCurrent()) return;
      logger.error('[SSOProvidersPanel] Failed to load provider details.');
      notificationStore.error(getSSOProviderDetailsLoadErrorMessage());
    } finally {
      if (detailController === controller) {
        detailController = undefined;
        setLoadingDetails(false);
      }
    }
  };

  const beginMutation = () => {
    if (!canEdit()) return;
    const context = contextVersion;
    const controller = new AbortController();
    mutationController = controller;
    setMutating(true);
    return {
      options: requestOptions(controller),
      isCurrent: () =>
        contextIsCurrent() && context === contextVersion && mutationController === controller,
      finish: () => {
        if (mutationController !== controller) return;
        mutationController = undefined;
        setMutating(false);
        setSaving(false);
      },
    };
  };

  const handleSave = async (event?: Event) => {
    event?.preventDefault();
    if (!showModal() || testing() || loadingPreview()) return;
    const mutation = beginMutation();
    if (!mutation) return;
    const isEdit = Boolean(editingProvider());
    const providerId = editingProvider()?.id;
    const payload = buildProviderPayload(form);
    setEditorFeedback(null);
    setSaving(true);
    try {
      const response = await apiFetch(
        isEdit
          ? `/api/security/sso/providers/${encodeURIComponent(providerId!)}`
          : '/api/security/sso/providers',
        {
          ...mutation.options,
          method: isEdit ? 'PUT' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        },
      );
      if (!mutation.isCurrent()) return;
      if (!response.ok) {
        const error = await apiErrorFromResponse(response);
        if (!mutation.isCurrent()) return;
        setEditorFeedback({ tone: 'error', message: getSSOProviderSaveErrorMessage(error) });
        logger.error('[SSOProvidersPanel] Failed to save provider.', { status: response.status });
        return;
      }
      notificationStore.success(getSSOProviderSaveSuccessMessage(isEdit));
      clearEditor();
      await loadProviders();
      if (mutation.isCurrent()) props.onConfigUpdated?.();
    } catch {
      if (!mutation.isCurrent()) return;
      // A lost response cannot establish whether the server saved the change.
      logger.error('[SSOProvidersPanel] Failed to save provider.');
      setEditorFeedback({ tone: 'error', message: getSSOProviderSaveErrorMessage() });
    } finally {
      mutation.finish();
    }
  };

  const handleDelete = async (providerId: string) => {
    if (
      deleteConfirm() !== providerId ||
      !providers().some((provider) => provider.id === providerId)
    )
      return;
    const mutation = beginMutation();
    if (!mutation) return;
    try {
      const response = await apiFetch(
        `/api/security/sso/providers/${encodeURIComponent(providerId)}`,
        {
          ...mutation.options,
          method: 'DELETE',
        },
      );
      if (!mutation.isCurrent()) return;
      if (!response.ok) throw new Error('Provider deletion refused');
      notificationStore.success(getSSOProviderDeleteSuccessMessage());
      setDeleteConfirmValue(null);
      await loadProviders();
      if (mutation.isCurrent()) props.onConfigUpdated?.();
    } catch {
      if (!mutation.isCurrent()) return;
      logger.error('[SSOProvidersPanel] Failed to delete provider.');
      notificationStore.error(getSSOProviderDeleteErrorMessage());
    } finally {
      mutation.finish();
    }
  };

  const handleToggleEnabled = async (provider: SSOProvider) => {
    if (!isCurrentProvider(provider)) return;
    const mutation = beginMutation();
    if (!mutation) return;
    clearEditor();
    try {
      // Only writable fields; omitted nested configuration and secret are preserved.
      const response = await apiFetch(
        `/api/security/sso/providers/${encodeURIComponent(provider.id)}`,
        {
          ...mutation.options,
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            id: provider.id,
            name: provider.name,
            type: provider.type,
            enabled: !provider.enabled,
            displayName: provider.displayName,
            iconUrl: provider.iconUrl,
            priority: provider.priority,
            allowedGroups: provider.allowedGroups ?? [],
            allowedDomains: provider.allowedDomains ?? [],
            allowedEmails: provider.allowedEmails ?? [],
            groupsClaim: provider.groupsClaim,
            groupRoleMappings: provider.groupRoleMappings,
          }),
        },
      );
      if (!mutation.isCurrent()) return;
      if (!response.ok) throw new Error('Provider update refused');
      notificationStore.success(getSSOProviderToggleSuccessMessage(!provider.enabled));
      await loadProviders();
      if (mutation.isCurrent()) props.onConfigUpdated?.();
    } catch {
      if (!mutation.isCurrent()) return;
      logger.error('[SSOProvidersPanel] Failed to toggle provider.');
      notificationStore.error(getSSOProviderToggleErrorMessage());
    } finally {
      mutation.finish();
    }
  };

  const copyToClipboard = async (text: string, label: string) => {
    if (!contextIsCurrent()) return;
    const context = contextVersion;
    const editor = editorVersion;
    const isCurrent = () =>
      contextIsCurrent() && context === contextVersion && editor === editorVersion;
    try {
      await navigator.clipboard.writeText(text);
      if (!isCurrent()) return;
      const message = getSSOCopySuccessMessage(label);
      if (showModal()) {
        setEditorFeedback({ tone: 'success', message });
      } else {
        notificationStore.success(message);
      }
    } catch {
      if (!isCurrent()) return;
      const message = getSSOCopyErrorMessage(label);
      if (showModal()) {
        setEditorFeedback({ tone: 'error', message });
      } else {
        notificationStore.error(message);
      }
    }
  };

  const testResultPresentation = createMemo(() =>
    testResult() ? getSSOTestResultPresentation(Boolean(testResult()?.success)) : null,
  );

  const testConnection = async () => {
    if (!canEdit() || !showModal() || !canTest() || testing() || loadingPreview()) return;
    const controller = new AbortController();
    testController = controller;
    const version = editorVersion;
    const context = contextVersion;
    const payload = JSON.stringify(buildProviderTestPayload(form));
    const isCurrent = () =>
      canEdit() &&
      showModal() &&
      version === editorVersion &&
      context === contextVersion &&
      testController === controller &&
      payload === JSON.stringify(buildProviderTestPayload(form));
    setEditorFeedback(null);
    setTesting(true);
    setTestResult(null);
    try {
      const response = await apiFetch('/api/security/sso/providers/test', {
        ...requestOptions(controller),
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      });
      if (!isCurrent()) return;
      if (!response.ok) throw new Error('Connection test refused');
      const result = (await response.json()) as SSOProviderTestResult;
      if (isCurrent()) setTestResult(result);
    } catch {
      if (!isCurrent()) return;
      logger.error('[SSOProvidersPanel] Test connection failed.');
      setTestResult({ success: false, message: getSSOConnectionTestErrorMessage() });
    } finally {
      if (testController === controller) {
        testController = undefined;
        setTesting(false);
      }
    }
  };

  const canTest = () => canTestProviderForm(form);

  const fetchMetadataPreview = async () => {
    if (!canEdit() || !showModal() || testing() || loadingPreview()) {
      return;
    }
    setEditorFeedback(null);
    if (!form.samlIdpMetadataUrl.trim()) {
      setEditorFeedback({ tone: 'error', message: getSSOMetadataUrlRequiredMessage() });
      return;
    }

    const controller = new AbortController();
    previewController = controller;
    const version = editorVersion;
    const context = contextVersion;
    const payload = JSON.stringify(buildMetadataPreviewPayload(form));
    const isCurrent = () =>
      canEdit() &&
      showModal() &&
      version === editorVersion &&
      context === contextVersion &&
      previewController === controller &&
      payload === JSON.stringify(buildMetadataPreviewPayload(form));
    setLoadingPreview(true);
    setMetadataPreview(null);

    try {
      const response = await apiFetch('/api/security/sso/providers/metadata/preview', {
        ...requestOptions(controller),
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      });

      if (!isCurrent()) return;
      if (!response.ok) {
        const error = await apiErrorFromResponse(response);
        if (!isCurrent()) return;
        setEditorFeedback({ tone: 'error', message: getSSOMetadataFetchErrorMessage(error) });
        logger.error('[SSOProvidersPanel] Metadata preview failed.', { status: response.status });
        return;
      }

      const preview = (await response.json()) as MetadataPreview;
      if (!isCurrent()) return;
      setMetadataPreview(preview);
      setShowMetadataPreview(true);
    } catch {
      if (!isCurrent()) return;
      logger.error('[SSOProvidersPanel] Metadata preview failed.');
      setEditorFeedback({ tone: 'error', message: getSSOMetadataFetchErrorMessage() });
    } finally {
      if (previewController === controller) {
        previewController = undefined;
        setLoadingPreview(false);
      }
    }
  };

  return {
    providers,
    loading,
    loadError,
    loadProviders,
    loadingDetails,
    canEdit,
    mutating,
    saving,
    editorFeedback,
    showModal,
    setShowModal,
    editingProvider,
    form,
    setForm,
    advancedOpen,
    setAdvancedOpen,
    deleteConfirm,
    setDeleteConfirm,
    publicUrl,
    testing,
    testResult,
    setTestResult,
    showMetadataPreview,
    setShowMetadataPreview,
    metadataPreview,
    loadingPreview,
    canManage,
    openAddModal,
    openEditModal,
    handleSave,
    handleDelete,
    handleToggleEnabled,
    copyToClipboard,
    testResultPresentation,
    testConnection,
    canTest,
    fetchMetadataPreview,
  };
};
