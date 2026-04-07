import { createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { createStore } from 'solid-js/store';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  hasFeature,
  runtimeCapabilitiesLoaded,
} from '@/stores/license';
import {
  commercialPosture,
  getUpgradeActionDestination,
} from '@/stores/licenseCommercial';
import { loadRuntimeCapabilities } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import {
  getSSOCopySuccessMessage,
  getSSOConnectionTestErrorMessage,
  getSSOConnectionTestFailureMessage,
  getSSOConnectionTestSuccessMessage,
  getSSOMetadataFetchErrorMessage,
  getSSOMetadataUrlRequiredMessage,
  getSSOProviderDeleteErrorMessage,
  getSSOProviderDeleteSuccessMessage,
  getSSOProviderDetailsLoadErrorMessage,
  getSSOProviderSaveErrorMessage,
  getSSOProviderSaveSuccessMessage,
  getSSOProviderToggleErrorMessage,
  getSSOProviderToggleSuccessMessage,
  getSSOProvidersLoadErrorMessage,
  getSSOTestResultPresentation,
} from '@/utils/ssoProviderPresentation';
import { runStartProTrialAction } from '@/utils/trialStartAction';
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
}

export const useSSOProvidersState = (props: SSOProvidersPanelProps) => {
  const [providers, setProviders] = createSignal<SSOProvider[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [showModal, setShowModal] = createSignal(false);
  const [editingProvider, setEditingProvider] = createSignal<SSOProvider | null>(null);
  const [form, setForm] = createStore<ProviderForm>(createEmptyProviderForm());
  const [advancedOpen, setAdvancedOpen] = createSignal(false);
  const [deleteConfirm, setDeleteConfirm] = createSignal<string | null>(null);
  const [publicUrl, setPublicUrl] = createSignal('');
  const [showSamlUpsell, setShowSamlUpsell] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [testResult, setTestResult] = createSignal<SSOProviderTestResult | null>(null);
  const [showMetadataPreview, setShowMetadataPreview] = createSignal(false);
  const [metadataPreview, setMetadataPreview] = createSignal<MetadataPreview | null>(null);
  const [loadingPreview, setLoadingPreview] = createSignal(false);
  const [startingTrial, setStartingTrial] = createSignal(false);

  const hasAdvancedSSO = createMemo(() => hasFeature('advanced_sso'));
  const canManage = () => props.canManage !== false;
  const canStartTrial = () => commercialPosture()?.trial_eligible !== false;

  const handleStartTrial = async () => {
    if (startingTrial()) {
      return;
    }
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        showSuccess: notificationStore.success,
        showError: notificationStore.error,
      });
    } finally {
      setStartingTrial(false);
    }
  };

  createEffect((wasBannerVisible) => {
    const isBannerVisible = runtimeCapabilitiesLoaded() && !hasAdvancedSSO() && !loading();
    if (isBannerVisible && !wasBannerVisible) {
      trackPaywallViewed('advanced_sso', 'settings_sso_providers_banner');
    }
    return isBannerVisible;
  }, false);

  createEffect((wasUpsellVisible) => {
    const isUpsellVisible = showSamlUpsell();
    if (isUpsellVisible && !wasUpsellVisible) {
      trackPaywallViewed('advanced_sso', 'settings_sso_providers_add_saml_gate');
    }
    return isUpsellVisible;
  }, false);

  const loadProviders = async () => {
    setLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/sso/providers');
      if (!response.ok) {
        throw new Error(`Failed to load SSO providers (${response.status})`);
      }
      const data = (await response.json()) as SSOProvidersResponse;
      setProviders(data.providers || []);

      const statusResponse = await apiFetch('/api/security/status');
      if (statusResponse.ok) {
        const status = (await statusResponse.json()) as { publicUrl?: string };
        setPublicUrl(status.publicUrl || window.location.origin);
      }
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to load providers:', error);
      notificationStore.error(getSSOProvidersLoadErrorMessage());
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadRuntimeCapabilities();
    void loadProviders();
  });

  const openAddModal = (type: 'oidc' | 'saml') => {
    if (!canManage()) {
      return;
    }
    setEditingProvider(null);
    setForm(createEmptyProviderForm());
    setForm('type', type);
    setAdvancedOpen(false);
    setShowModal(true);
  };

  const openEditModal = async (provider: SSOProvider) => {
    if (!canManage()) {
      return;
    }
    setEditingProvider(provider);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch(`/api/security/sso/providers/${provider.id}`);
      if (!response.ok) {
        throw new Error('Failed to load provider details');
      }
      setForm(mapProviderDetailsToForm(await response.json()));
      setAdvancedOpen(false);
      setShowModal(true);
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to load provider for editing:', error);
      notificationStore.error(getSSOProviderDetailsLoadErrorMessage());
    }
  };

  const handleSave = async (event?: Event) => {
    event?.preventDefault();
    if (!canManage()) {
      return;
    }
    setSaving(true);

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const isEdit = Boolean(editingProvider());
      const response = await apiFetch(
        isEdit ? `/api/security/sso/providers/${editingProvider()!.id}` : '/api/security/sso/providers',
        {
          method: isEdit ? 'PUT' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(buildProviderPayload(form)),
        },
      );

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || `Failed to save provider (${response.status})`);
      }

      notificationStore.success(getSSOProviderSaveSuccessMessage(isEdit));
      setShowModal(false);
      await loadProviders();
      props.onConfigUpdated?.();
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to save provider:', error);
      notificationStore.error(getSSOProviderSaveErrorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (providerId: string) => {
    if (!canManage()) {
      return;
    }
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch(`/api/security/sso/providers/${providerId}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error(`Failed to delete provider (${response.status})`);
      }

      notificationStore.success(getSSOProviderDeleteSuccessMessage());
      setDeleteConfirm(null);
      await loadProviders();
      props.onConfigUpdated?.();
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to delete provider:', error);
      notificationStore.error(getSSOProviderDeleteErrorMessage());
    }
  };

  const handleToggleEnabled = async (provider: SSOProvider) => {
    if (!canManage()) {
      return;
    }
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch(`/api/security/sso/providers/${provider.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...provider, enabled: !provider.enabled }),
      });

      if (!response.ok) {
        throw new Error(`Failed to update provider (${response.status})`);
      }

      notificationStore.success(getSSOProviderToggleSuccessMessage(!provider.enabled));
      await loadProviders();
      props.onConfigUpdated?.();
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to toggle provider:', error);
      notificationStore.error(getSSOProviderToggleErrorMessage());
    }
  };

  const copyToClipboard = (text: string, label: string) => {
    void navigator.clipboard.writeText(text);
    notificationStore.success(getSSOCopySuccessMessage(label));
  };

  const testResultPresentation = createMemo(() =>
    testResult() ? getSSOTestResultPresentation(Boolean(testResult()?.success)) : null,
  );

  const testConnection = async () => {
    setTesting(true);
    setTestResult(null);

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/sso/providers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(buildProviderTestPayload(form)),
      });

      const result = (await response.json()) as SSOProviderTestResult;
      setTestResult(result);

      if (result.success) {
        notificationStore.success(getSSOConnectionTestSuccessMessage());
      } else {
        notificationStore.error(getSSOConnectionTestFailureMessage(result.message));
      }
    } catch (error) {
      logger.error('[SSOProvidersPanel] Test connection error:', error);
      setTestResult({
        success: false,
        message: getSSOConnectionTestErrorMessage(),
        error: String(error),
      });
      notificationStore.error(getSSOConnectionTestErrorMessage());
    } finally {
      setTesting(false);
    }
  };

  const canTest = () => canTestProviderForm(form);

  const fetchMetadataPreview = async () => {
    if (!form.samlIdpMetadataUrl.trim()) {
      notificationStore.error(getSSOMetadataUrlRequiredMessage());
      return;
    }

    setLoadingPreview(true);
    setMetadataPreview(null);

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/sso/providers/metadata/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(buildMetadataPreviewPayload(form)),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || 'Failed to fetch metadata');
      }

      setMetadataPreview((await response.json()) as MetadataPreview);
      setShowMetadataPreview(true);
    } catch (error) {
      logger.error('[SSOProvidersPanel] Metadata preview error:', error);
      notificationStore.error(getSSOMetadataFetchErrorMessage(error));
    } finally {
      setLoadingPreview(false);
    }
  };

  return {
    providers,
    loading,
    saving,
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
    showSamlUpsell,
    setShowSamlUpsell,
    testing,
    testResult,
    setTestResult,
    showMetadataPreview,
    setShowMetadataPreview,
    metadataPreview,
    loadingPreview,
    hasAdvancedSSO,
    canManage,
    startingTrial,
    canStartTrial,
    handleStartTrial,
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
    getUpgradeActionDestination,
    runtimeCapabilitiesLoaded,
    trackUpgradeClicked,
  };
};
