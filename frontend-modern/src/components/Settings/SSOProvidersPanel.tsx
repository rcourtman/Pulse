import { Component, Show, For, createSignal, onMount, createMemo, createEffect } from 'solid-js';
import { createStore } from 'solid-js/store';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { getUpgradeActionUrlOrFallback, hasFeature, loadLicenseStatus, licenseLoaded } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Shield from 'lucide-solid/icons/shield';
import Key from 'lucide-solid/icons/key';
import Globe from 'lucide-solid/icons/globe';
import Copy from 'lucide-solid/icons/copy';
import ExternalLink from 'lucide-solid/icons/external-link';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import Eye from 'lucide-solid/icons/eye';
import X from 'lucide-solid/icons/x';

// Types
interface SSOProvider {
  id: string;
  name: string;
  type: 'oidc' | 'saml';
  enabled: boolean;
  displayName?: string;
  iconUrl?: string;
  priority: number;
  oidcIssuerUrl?: string;
  oidcClientId?: string;
  oidcClientSecretSet?: boolean;
  samlIdpEntityId?: string;
  samlSpEntityId?: string;
  samlMetadataUrl?: string;
  samlAcsUrl?: string;
  allowedGroups?: string[];
  allowedDomains?: string[];
  allowedEmails?: string[];
}

interface SSOProvidersResponse {
  providers: SSOProvider[];
  defaultProviderId?: string;
  allowMultipleProviders: boolean;
}

// Test connection response
interface TestResult {
  success: boolean;
  message: string;
  error?: string;
  details?: {
    type: string;
    entityId?: string;
    ssoUrl?: string;
    sloUrl?: string;
    tokenEndpoint?: string;
    userinfoEndpoint?: string;
    certificates?: Array<{
      subject: string;
      issuer: string;
      notBefore: string;
      notAfter: string;
      isExpired: boolean;
    }>;
  };
}

// Metadata preview response
interface MetadataPreview {
  xml: string;
  parsed: {
    entityId: string;
    ssoUrl?: string;
    sloUrl?: string;
    certificates?: Array<{
      subject: string;
      notAfter: string;
      isExpired?: boolean;
    }>;
    nameIdFormats?: string[];
  };
}

// Provider form for creating/editing
interface ProviderForm {
  id: string;
  name: string;
  type: 'oidc' | 'saml';
  enabled: boolean;
  displayName: string;
  priority: number;
  // OIDC fields
  oidcIssuerUrl: string;
  oidcClientId: string;
  oidcClientSecret: string;
  oidcRedirectUrl: string;
  oidcLogoutUrl: string;
  oidcScopes: string;
  // SAML fields
  samlIdpMetadataUrl: string;
  samlIdpMetadataXml: string;
  samlIdpSsoUrl: string;
  samlIdpEntityId: string;
  samlIdpCertificate: string;
  samlSpEntityId: string;
  samlSignRequests: boolean;
  samlAllowIdpInitiated: boolean;
  samlUsernameAttr: string;
  samlEmailAttr: string;
  samlGroupsAttr: string;
  // Common
  allowedGroups: string;
  allowedDomains: string;
  allowedEmails: string;
  groupRoleMappings: string;
}

const emptyForm = (): ProviderForm => ({
  id: '',
  name: '',
  type: 'oidc',
  enabled: true,
  displayName: '',
  priority: 0,
  oidcIssuerUrl: '',
  oidcClientId: '',
  oidcClientSecret: '',
  oidcRedirectUrl: '',
  oidcLogoutUrl: '',
  oidcScopes: 'openid profile email',
  samlIdpMetadataUrl: '',
  samlIdpMetadataXml: '',
  samlIdpSsoUrl: '',
  samlIdpEntityId: '',
  samlIdpCertificate: '',
  samlSpEntityId: '',
  samlSignRequests: false,
  samlAllowIdpInitiated: false,
  samlUsernameAttr: '',
  samlEmailAttr: 'email',
  samlGroupsAttr: '',
  allowedGroups: '',
  allowedDomains: '',
  allowedEmails: '',
  groupRoleMappings: '',
});

const listToString = (values?: string[]) => (values && values.length > 0 ? values.join(', ') : '');
const splitList = (input: string) =>
  input
    .split(/[,\s]+/)
    .map((v) => v.trim())
    .filter(Boolean);

const mappingsToString = (mappings?: Record<string, string>) =>
  mappings ? Object.entries(mappings).map(([k, v]) => `${k}=${v}`).join(', ') : '';

const stringToMappings = (input: string) => {
  const result: Record<string, string> = {};
  splitList(input).forEach(pair => {
    const [k, v] = pair.split('=').map(s => s.trim());
    if (k && v) result[k] = v;
  });
  return result;
};

interface Props {
  onConfigUpdated?: () => void;
}

export const SSOProvidersPanel: Component<Props> = (props) => {
  const [providers, setProviders] = createSignal<SSOProvider[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [showModal, setShowModal] = createSignal(false);
  const [editingProvider, setEditingProvider] = createSignal<SSOProvider | null>(null);
  const [form, setForm] = createStore<ProviderForm>(emptyForm());
  const [advancedOpen, setAdvancedOpen] = createSignal(false);
  const [deleteConfirm, setDeleteConfirm] = createSignal<string | null>(null);
  const [publicUrl, setPublicUrl] = createSignal<string>('');

  const [showSamlUpsell, setShowSamlUpsell] = createSignal(false);

  // Test connection state
  const [testing, setTesting] = createSignal(false);
  const [testResult, setTestResult] = createSignal<TestResult | null>(null);

  // Metadata preview state
  const [showMetadataPreview, setShowMetadataPreview] = createSignal(false);
  const [metadataPreview, setMetadataPreview] = createSignal<MetadataPreview | null>(null);
  const [loadingPreview, setLoadingPreview] = createSignal(false);

  const hasAdvancedSSO = createMemo(() => hasFeature('advanced_sso'));

  createEffect((wasBannerVisible) => {
    const isBannerVisible = licenseLoaded() && !hasAdvancedSSO() && !loading();
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

      // Also get public URL for metadata display
      const statusResp = await apiFetch('/api/security/status');
      if (statusResp.ok) {
        const status = await statusResp.json();
        setPublicUrl(status.publicUrl || window.location.origin);
      }
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to load providers:', error);
      notificationStore.error('Failed to load SSO providers');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadLicenseStatus();
    loadProviders();
  });

  const openAddModal = (type: 'oidc' | 'saml') => {
    setEditingProvider(null);
    setForm(emptyForm());
    setForm('type', type);
    setAdvancedOpen(false);
    setShowModal(true);
  };

  const openEditModal = async (provider: SSOProvider) => {
    setEditingProvider(provider);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch(`/api/security/sso/providers/${provider.id}`);
      if (!response.ok) throw new Error('Failed to load provider details');
      const full = await response.json();

      setForm({
        id: full.id,
        name: full.name,
        type: full.type,
        enabled: full.enabled,
        displayName: full.displayName || '',
        priority: full.priority || 0,
        oidcIssuerUrl: full.oidc?.issuerUrl || '',
        oidcClientId: full.oidc?.clientId || '',
        oidcClientSecret: '',
        oidcRedirectUrl: full.oidc?.redirectUrl || '',
        oidcLogoutUrl: full.oidc?.logoutUrl || '',
        oidcScopes: full.oidc?.scopes?.join(' ') || 'openid profile email',
        samlIdpMetadataUrl: full.saml?.idpMetadataUrl || '',
        samlIdpMetadataXml: full.saml?.idpMetadataXml || '',
        samlIdpSsoUrl: full.saml?.idpSsoUrl || '',
        samlIdpEntityId: full.saml?.idpEntityId || '',
        samlIdpCertificate: full.saml?.idpCertificate || '',
        samlSpEntityId: full.saml?.spEntityId || '',
        samlSignRequests: full.saml?.signRequests || false,
        samlAllowIdpInitiated: full.saml?.allowIdpInitiated || false,
        samlUsernameAttr: full.saml?.usernameAttr || '',
        samlEmailAttr: full.saml?.emailAttr || 'email',
        samlGroupsAttr: full.saml?.groupsAttr || full.groupsClaim || '',
        allowedGroups: listToString(full.allowedGroups),
        allowedDomains: listToString(full.allowedDomains),
        allowedEmails: listToString(full.allowedEmails),
        groupRoleMappings: mappingsToString(full.groupRoleMappings),
      });
      setAdvancedOpen(false);
      setShowModal(true);
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to load provider for editing:', error);
      notificationStore.error('Failed to load provider details');
    }
  };

  const handleSave = async (e?: Event) => {
    e?.preventDefault();
    setSaving(true);

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const isEdit = !!editingProvider();

      // Build payload based on type
      const payload: Record<string, unknown> = {
        id: form.id || undefined,
        name: form.name.trim(),
        type: form.type,
        enabled: form.enabled,
        displayName: form.displayName.trim() || undefined,
        priority: form.priority,
        allowedGroups: splitList(form.allowedGroups),
        allowedDomains: splitList(form.allowedDomains),
        allowedEmails: splitList(form.allowedEmails),
        groupRoleMappings: stringToMappings(form.groupRoleMappings),
      };

      if (form.type === 'oidc') {
        payload.oidc = {
          issuerUrl: form.oidcIssuerUrl.trim(),
          clientId: form.oidcClientId.trim(),
          clientSecret: form.oidcClientSecret.trim() || undefined,
          redirectUrl: form.oidcRedirectUrl.trim() || undefined,
          logoutUrl: form.oidcLogoutUrl.trim() || undefined,
          scopes: splitList(form.oidcScopes),
        };
        payload.groupsClaim = form.samlGroupsAttr.trim() || undefined;
      } else {
        payload.saml = {
          idpMetadataUrl: form.samlIdpMetadataUrl.trim() || undefined,
          idpMetadataXml: form.samlIdpMetadataXml.trim() || undefined,
          idpSsoUrl: form.samlIdpSsoUrl.trim() || undefined,
          idpEntityId: form.samlIdpEntityId.trim() || undefined,
          idpCertificate: form.samlIdpCertificate.trim() || undefined,
          spEntityId: form.samlSpEntityId.trim() || undefined,
          signRequests: form.samlSignRequests,
          allowIdpInitiated: form.samlAllowIdpInitiated,
          usernameAttr: form.samlUsernameAttr.trim() || undefined,
          emailAttr: form.samlEmailAttr.trim() || undefined,
          groupsAttr: form.samlGroupsAttr.trim() || undefined,
        };
        payload.groupsClaim = form.samlGroupsAttr.trim() || undefined;
      }

      const url = isEdit
        ? `/api/security/sso/providers/${editingProvider()!.id}`
        : '/api/security/sso/providers';

      const response = await apiFetch(url, {
        method: isEdit ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errText = await response.text();
        throw new Error(errText || `Failed to save provider (${response.status})`);
      }

      notificationStore.success(isEdit ? 'Provider updated' : 'Provider created');
      setShowModal(false);
      loadProviders();
      props.onConfigUpdated?.();
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to save provider:', error);
      notificationStore.error(`Failed to save provider: ${error}`);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (providerId: string) => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch(`/api/security/sso/providers/${providerId}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error(`Failed to delete provider (${response.status})`);
      }

      notificationStore.success('Provider deleted');
      setDeleteConfirm(null);
      loadProviders();
      props.onConfigUpdated?.();
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to delete provider:', error);
      notificationStore.error('Failed to delete provider');
    }
  };

  const handleToggleEnabled = async (provider: SSOProvider) => {
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

      notificationStore.success(provider.enabled ? 'Provider disabled' : 'Provider enabled');
      loadProviders();
      props.onConfigUpdated?.();
    } catch (error) {
      logger.error('[SSOProvidersPanel] Failed to toggle provider:', error);
      notificationStore.error('Failed to update provider');
    }
  };

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    notificationStore.success(`${label} copied to clipboard`);
  };

  // Test connection for current form configuration
  const testConnection = async () => {
    setTesting(true);
    setTestResult(null);

    try {
      const { apiFetch } = await import('@/utils/apiClient');

      const payload: Record<string, unknown> = {
        type: form.type,
      };

      if (form.type === 'oidc') {
        payload.oidc = {
          issuerUrl: form.oidcIssuerUrl.trim(),
          clientId: form.oidcClientId.trim() || undefined,
        };
      } else {
        payload.saml = {
          idpMetadataUrl: form.samlIdpMetadataUrl.trim() || undefined,
          idpMetadataXml: form.samlIdpMetadataXml.trim() || undefined,
          idpSsoUrl: form.samlIdpSsoUrl.trim() || undefined,
          idpCertificate: form.samlIdpCertificate.trim() || undefined,
        };
      }

      const response = await apiFetch('/api/security/sso/providers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      const result = (await response.json()) as TestResult;
      setTestResult(result);

      if (result.success) {
        notificationStore.success('Connection test successful');
      } else {
        notificationStore.error(`Connection test failed: ${result.message}`);
      }
    } catch (error) {
      logger.error('[SSOProvidersPanel] Test connection error:', error);
      setTestResult({
        success: false,
        message: 'Failed to test connection',
        error: String(error),
      });
      notificationStore.error('Failed to test connection');
    } finally {
      setTesting(false);
    }
  };

  // Check if we have enough info to test
  const canTest = () => {
    if (form.type === 'oidc') {
      return !!form.oidcIssuerUrl.trim();
    }
    return !!(
      form.samlIdpMetadataUrl.trim() ||
      form.samlIdpMetadataXml.trim() ||
      form.samlIdpSsoUrl.trim()
    );
  };

  // Fetch and preview metadata
  const fetchMetadataPreview = async () => {
    if (!form.samlIdpMetadataUrl.trim()) {
      notificationStore.error('Please enter an IdP Metadata URL');
      return;
    }

    setLoadingPreview(true);
    setMetadataPreview(null);

    try {
      const { apiFetch } = await import('@/utils/apiClient');

      const response = await apiFetch('/api/security/sso/providers/metadata/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'saml',
          metadataUrl: form.samlIdpMetadataUrl.trim(),
        }),
      });

      if (!response.ok) {
        const err = await response.text();
        throw new Error(err || 'Failed to fetch metadata');
      }

      const preview = (await response.json()) as MetadataPreview;
      setMetadataPreview(preview);
      setShowMetadataPreview(true);
    } catch (error) {
      logger.error('[SSOProvidersPanel] Metadata preview error:', error);
      notificationStore.error(`Failed to fetch metadata: ${error}`);
    } finally {
      setLoadingPreview(false);
    }
  };

  return (
    <div class="space-y-6">
      <Show when={showSamlUpsell()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black opacity-50">
          <div class="w-full max-w-lg bg-white dark:bg-slate-900 rounded-md shadow-sm border border-slate-200 dark:border-slate-700 mx-4">
            <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-700">
              <div>
                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">Add SAML Provider</h3>
                <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Pro feature</p>
              </div>
              <button
                type="button"
                onClick={() => setShowSamlUpsell(false)}
                class="p-1.5 rounded-md text-slate-500 hover:text-slate-700 hover:bg-slate-100 dark:hover:text-slate-300 dark:hover:bg-slate-800"
                aria-label="Close"
              >
                <X class="w-5 h-5" />
              </button>
            </div>
            <div class="px-6 py-5 space-y-4">
              <p class="text-sm text-slate-600 dark:text-slate-300">
                SAML 2.0 and multi-provider SSO requires Pro.
              </p>
              <div class="flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowSamlUpsell(false)}
                  class="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-md hover:bg-slate-50 dark:hover:bg-slate-600"
                >
                  Not now
                </button>
                <a
                  href={getUpgradeActionUrlOrFallback('advanced_sso')}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="px-4 py-2 text-sm font-semibold bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors inline-flex items-center gap-2"
                  onClick={() => trackUpgradeClicked('settings_sso_providers_add_saml_gate', 'advanced_sso')}
                >
                  Upgrade to Pro
                  <ExternalLink class="w-4 h-4" />
                </a>
              </div>
            </div>
          </div>
        </div>
      </Show>

      {/* License banner */}
      <Show when={licenseLoaded() && !hasAdvancedSSO() && !loading()}>
        <div class="p-4 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md">
          <div class="flex flex-col sm:flex-row items-center gap-4">
            <div class="flex-1">
              <h4 class="text-base font-semibold text-slate-900 dark:text-white">Advanced SSO</h4>
              <p class="text-sm text-slate-600 dark:text-slate-400 mt-1">
                SAML 2.0 and multi-provider SSO requires Pro. Basic OIDC is available in the free tier.
              </p>
            </div>
            <a
              href={getUpgradeActionUrlOrFallback('advanced_sso')}
              target="_blank"
              rel="noopener noreferrer"
              class="px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-2"
              onClick={() => trackUpgradeClicked('settings_sso_providers_banner', 'advanced_sso')}
            >
              Upgrade to Pro
              <ExternalLink class="w-4 h-4" />
            </a>
          </div>
        </div>
      </Show>

      {/* Main panel */}
      <SettingsPanel
        title="Single Sign-On Providers"
        description="Configure OIDC and SAML identity providers."
        icon={<Shield class="w-5 h-5" strokeWidth={2} />}
        action={
          <div class="flex flex-wrap justify-end gap-2">
            <button
              type="button"
              onClick={() => openAddModal('oidc')}
              class="min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-1.5"
            >
              <Plus class="w-4 h-4" />
              Add OIDC
            </button>
            <Show when={hasAdvancedSSO()}>
              <button
                type="button"
                onClick={() => openAddModal('saml')}
                class="min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-200 rounded-md hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5"
              >
                <Plus class="w-4 h-4" />
                Add SAML
              </button>
            </Show>
            <Show when={licenseLoaded() && !hasAdvancedSSO()}>
              <button
                type="button"
                onClick={() => setShowSamlUpsell(true)}
                class="min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-200 rounded-md hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5"
              >
                <Plus class="w-4 h-4" />
                Add SAML (Pro)
              </button>
            </Show>
          </div>
        }
        bodyClass="space-y-6"
      >
        {/* Content */}
        <Show when={loading()}>
          <div class="flex items-center gap-3 text-sm text-slate-600 dark:text-slate-300">
            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
            Loading SSO providers...
          </div>
        </Show>

        <Show when={!loading() && providers().length === 0}>
          <div class="text-center py-8 text-slate-500 dark:text-slate-400">
            <Shield class="w-12 h-12 mx-auto mb-3 opacity-40" />
            <p class="text-sm">No SSO providers configured</p>
            <p class="text-xs mt-1">Click "Add OIDC" or "Add SAML" to get started</p>
          </div>
        </Show>

        <Show when={!loading() && providers().length > 0}>
          <div class="space-y-3">
            <For each={providers()}>
              {(provider) => (
                <div class={`p-4 rounded-md border ${provider.enabled ? 'bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700' : 'bg-slate-50 dark:bg-slate-800 border-slate-200 dark:border-slate-700 opacity-60'}`}>
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div class="flex items-center gap-3 min-w-0">
                      <div class="p-2 rounded-md bg-slate-100 dark:bg-slate-700">
                        {provider.type === 'oidc' ? (
                          <Globe class="w-5 h-5 text-slate-600 dark:text-slate-300" />
                        ) : (
                          <Key class="w-5 h-5 text-slate-600 dark:text-slate-300" />
                        )}
                      </div>
                      <div class="min-w-0">
                        <div class="flex items-center gap-2">
                          <span class="font-medium text-slate-900 dark:text-white truncate">
                            {provider.name}
                          </span>
                          <span class="px-1.5 py-0.5 text-xs font-medium rounded bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-300">
                            {provider.type.toUpperCase()}
                          </span>
                        </div>
                        <p class="text-xs text-slate-500 dark:text-slate-400 truncate">
                          {provider.type === 'oidc'
                            ? provider.oidcIssuerUrl
                            : provider.samlIdpEntityId || provider.samlMetadataUrl}
                        </p>
                      </div>
                    </div>
                    <div class="flex items-center gap-2 self-end sm:self-auto">
                      <Toggle
                        checked={provider.enabled}
                        onChange={() => handleToggleEnabled(provider)}
                        containerClass="items-center"
                      />
                      <button
                        type="button"
                        onClick={() => openEditModal(provider)}
                        class="p-2 text-slate-500 hover:text-blue-600 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-md transition-colors"
                        title="Edit provider"
                      >
                        <Pencil class="w-4 h-4" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setDeleteConfirm(provider.id)}
                        class="p-2 text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900 rounded-md transition-colors"
                        title="Delete provider"
                      >
                        <Trash2 class="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  {/* SAML metadata info */}
                  <Show when={provider.type === 'saml' && provider.enabled}>
                    <div class="mt-3 pt-3 border-t border-slate-100 dark:border-slate-700">
                      <div class="flex flex-wrap gap-4 text-xs">
                        <div class="flex items-center gap-1">
                          <span class="text-slate-500">SP Metadata:</span>
                          <button
                            type="button"
                            onClick={() => copyToClipboard(provider.samlMetadataUrl || '', 'Metadata URL')}
                            class="text-blue-600 hover:underline flex items-center gap-1"
                          >
                            {provider.samlMetadataUrl}
                            <Copy class="w-3 h-3" />
                          </button>
                        </div>
                        <div class="flex items-center gap-1">
                          <span class="text-slate-500">ACS URL:</span>
                          <button
                            type="button"
                            onClick={() => copyToClipboard(provider.samlAcsUrl || '', 'ACS URL')}
                            class="text-blue-600 hover:underline flex items-center gap-1"
                          >
                            {provider.samlAcsUrl}
                            <Copy class="w-3 h-3" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </Show>
                </div>
              )}
            </For>
          </div>
        </Show>
      </SettingsPanel>

      {/* Add/Edit Modal */}
      <Show when={showModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black opacity-50 p-4" onClick={() => setShowModal(false)}>
          <div
            class="bg-white dark:bg-slate-800 rounded-md shadow-sm max-w-2xl w-full max-h-[90vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            <div class="sticky top-0 bg-white dark:bg-slate-800 px-6 py-4 border-b border-slate-200 dark:border-slate-700 z-10">
              <h3 class="text-lg font-semibold text-slate-900 dark:text-white">
                {editingProvider() ? 'Edit' : 'Add'} {form.type.toUpperCase()} Provider
              </h3>
            </div>

            <form class="p-6 space-y-4" onSubmit={handleSave}>
              {/* Common fields */}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class={formField}>
                  <label class={labelClass()}>Provider Name</label>
                  <input
                    type="text"
                    value={form.name}
                    onInput={(e) => setForm('name', e.currentTarget.value)}
                    placeholder="e.g., Corporate SSO"
                    class={controlClass()}
                    required
                  />
                  <p class={formHelpText}>Display name for this provider</p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Display Name (Button)</label>
                  <input
                    type="text"
                    value={form.displayName}
                    onInput={(e) => setForm('displayName', e.currentTarget.value)}
                    placeholder="Sign in with SSO"
                    class={controlClass()}
                  />
                  <p class={formHelpText}>Text shown on login button</p>
                </div>
              </div>

              {/* OIDC-specific fields */}
              <Show when={form.type === 'oidc'}>
                <div class="space-y-4">
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div class={formField}>
                      <label class={labelClass()}>Issuer URL</label>
                      <div class="flex gap-2">
                        <input
                          type="url"
                          value={form.oidcIssuerUrl}
                          onInput={(e) => setForm('oidcIssuerUrl', e.currentTarget.value)}
                          placeholder="https://login.example.com/realms/pulse"
                          class={controlClass() + ' flex-1'}
                          required
                        />
                        <button
                          type="button"
                          onClick={testConnection}
                          disabled={testing() || !canTest()}
                          class="px-3 py-2 text-sm font-medium bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-md hover:bg-slate-200 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                          title="Test connection to IdP"
                        >
                          {testing() ? 'Testing...' : 'Test'}
                        </button>
                      </div>
                    </div>
                    <div class={formField}>
                      <label class={labelClass()}>Client ID</label>
                      <input
                        type="text"
                        value={form.oidcClientId}
                        onInput={(e) => setForm('oidcClientId', e.currentTarget.value)}
                        placeholder="pulse-client"
                        class={controlClass()}
                        required
                      />
                    </div>
                    <div class={formField}>
                      <label class={labelClass()}>Client Secret</label>
                      <input
                        type="password"
                        value={form.oidcClientSecret}
                        onInput={(e) => setForm('oidcClientSecret', e.currentTarget.value)}
                        placeholder={editingProvider() ? '•••••••• (leave blank to keep)' : 'Optional for PKCE'}
                        class={controlClass()}
                      />
                    </div>
                    <div class={formField}>
                      <label class={labelClass()}>Redirect URL</label>
                      <input
                        type="url"
                        value={form.oidcRedirectUrl}
                        onInput={(e) => setForm('oidcRedirectUrl', e.currentTarget.value)}
                        placeholder="Auto-detected if empty"
                        class={controlClass()}
                      />
                    </div>
                  </div>
                </div>
              </Show>

              {/* SAML-specific fields */}
              <Show when={form.type === 'saml'}>
                <div class="space-y-4">
                  <div class="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md p-3">
                    <p class="text-xs text-slate-700 dark:text-slate-300">
                      <strong>Setup:</strong> Provide either IdP Metadata URL (preferred) or configure SSO URL + Certificate manually.
                      Use the SP Metadata URL below to configure your Identity Provider.
                    </p>
                    <Show when={publicUrl()}>
                      <div class="mt-2 flex items-center gap-2">
                        <span class="text-xs text-slate-600 dark:text-slate-300">SP Metadata:</span>
                        <code class="text-xs bg-slate-100 dark:bg-slate-700 px-2 py-0.5 rounded">
                          {publicUrl()}/api/saml/{form.id || '{id}'}/metadata
                        </code>
                      </div>
                    </Show>
                  </div>

                  <div class={formField}>
                    <label class={labelClass()}>IdP Metadata URL</label>
                    <div class="flex gap-2">
                      <input
                        type="url"
                        value={form.samlIdpMetadataUrl}
                        onInput={(e) => setForm('samlIdpMetadataUrl', e.currentTarget.value)}
                        placeholder="https://idp.example.com/metadata"
                        class={controlClass() + ' flex-1'}
                      />
                      <button
                        type="button"
                        onClick={testConnection}
                        disabled={testing() || !canTest()}
                        class="px-3 py-2 text-sm font-medium bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-md hover:bg-slate-200 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                        title="Test connection to IdP"
                      >
                        {testing() ? 'Testing...' : 'Test'}
                      </button>
                      <button
                        type="button"
                        onClick={fetchMetadataPreview}
                        disabled={loadingPreview() || !form.samlIdpMetadataUrl.trim()}
                        class="px-3 py-2 text-sm font-medium bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-md hover:bg-slate-200 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap flex items-center gap-1"
                        title="Preview IdP metadata XML"
                      >
                        <Eye class="w-4 h-4" />
                        {loadingPreview() ? 'Loading...' : 'Preview'}
                      </button>
                    </div>
                    <p class={formHelpText}>URL to fetch IdP metadata (preferred method)</p>
                  </div>

                  <div class="text-center text-xs text-slate-500 dark:text-slate-400">— or configure manually —</div>

                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div class={formField}>
                      <label class={labelClass()}>IdP SSO URL</label>
                      <input
                        type="url"
                        value={form.samlIdpSsoUrl}
                        onInput={(e) => setForm('samlIdpSsoUrl', e.currentTarget.value)}
                        placeholder="https://idp.example.com/sso"
                        class={controlClass()}
                      />
                    </div>
                    <div class={formField}>
                      <label class={labelClass()}>IdP Entity ID</label>
                      <input
                        type="text"
                        value={form.samlIdpEntityId}
                        onInput={(e) => setForm('samlIdpEntityId', e.currentTarget.value)}
                        placeholder="https://idp.example.com"
                        class={controlClass()}
                      />
                    </div>
                  </div>

                  <div class={formField}>
                    <label class={labelClass()}>IdP Certificate (PEM)</label>
                    <textarea
                      rows={4}
                      value={form.samlIdpCertificate}
                      onInput={(e) => setForm('samlIdpCertificate', e.currentTarget.value)}
                      placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                      class={controlClass('font-mono text-xs')}
                    />
                  </div>

                  <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div class={formField}>
                      <label class={labelClass()}>Username Attribute</label>
                      <input
                        type="text"
                        value={form.samlUsernameAttr}
                        onInput={(e) => setForm('samlUsernameAttr', e.currentTarget.value)}
                        placeholder="NameID (default)"
                        class={controlClass()}
                      />
                    </div>
                    <div class={formField}>
                      <label class={labelClass()}>Email Attribute</label>
                      <input
                        type="text"
                        value={form.samlEmailAttr}
                        onInput={(e) => setForm('samlEmailAttr', e.currentTarget.value)}
                        placeholder="email"
                        class={controlClass()}
                      />
                    </div>
                    <div class={formField}>
                      <label class={labelClass()}>Groups Attribute</label>
                      <input
                        type="text"
                        value={form.samlGroupsAttr}
                        onInput={(e) => setForm('samlGroupsAttr', e.currentTarget.value)}
                        placeholder="groups"
                        class={controlClass()}
                      />
                    </div>
                  </div>

                  <div class="flex gap-4">
                    <label class="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={form.samlAllowIdpInitiated}
                        onChange={(e) => setForm('samlAllowIdpInitiated', e.currentTarget.checked)}
                        class="rounded border-slate-300"
                      />
                      <span class="text-slate-700 dark:text-slate-300">Allow IdP-initiated SSO</span>
                    </label>
                  </div>
                </div>
              </Show>

              {/* Test result display */}
              <Show when={testResult()}>
                <div
                  class={`p-4 rounded-md border ${testResult()?.success
                    ? 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800'
                    : 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800'
                    }`}
                >
                  <div class="flex items-start gap-3">
                    {testResult()?.success ? (
                      <CheckCircle class="w-5 h-5 text-emerald-500 dark:text-emerald-400 flex-shrink-0 mt-0.5" />
                    ) : (
                      <XCircle class="w-5 h-5 text-rose-500 dark:text-rose-400 flex-shrink-0 mt-0.5" />
                    )}
                    <div class="flex-1 min-w-0">
                      <p
                        class={`text-sm font-medium ${testResult()?.success
                          ? 'text-green-800 dark:text-green-200'
                          : 'text-red-800 dark:text-red-200'
                          }`}
                      >
                        {testResult()?.message}
                      </p>
                      <Show when={testResult()?.error}>
                        <p class="text-xs text-red-600 dark:text-red-400 mt-1">{testResult()?.error}</p>
                      </Show>
                      <Show when={testResult()?.success && testResult()?.details}>
                        <dl class="mt-2 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-xs">
                          <Show when={testResult()?.details?.entityId}>
                            <div class="flex gap-2">
                              <dt class="text-slate-500 dark:text-slate-400">Entity ID:</dt>
                              <dd class="text-slate-700 dark:text-slate-300 truncate">{testResult()?.details?.entityId}</dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.ssoUrl}>
                            <div class="flex gap-2">
                              <dt class="text-slate-500 dark:text-slate-400">SSO URL:</dt>
                              <dd class="text-slate-700 dark:text-slate-300 truncate">{testResult()?.details?.ssoUrl}</dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.tokenEndpoint}>
                            <div class="flex gap-2">
                              <dt class="text-slate-500 dark:text-slate-400">Token Endpoint:</dt>
                              <dd class="text-slate-700 dark:text-slate-300 truncate">{testResult()?.details?.tokenEndpoint}</dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.certificates && testResult()!.details!.certificates!.length > 0}>
                            <div class="col-span-2 mt-1">
                              <dt class="text-slate-500 dark:text-slate-400 mb-1">Certificates:</dt>
                              <dd class="space-y-1">
                                <For each={testResult()?.details?.certificates}>
                                  {(cert) => (
                                    <div class={`text-xs px-2 py-1 rounded ${cert.isExpired ? 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300' : 'bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300'}`}>
                                      <span class="font-medium">{cert.subject}</span>
                                      <span class="mx-1">•</span>
                                      <span>Expires: {new Date(cert.notAfter).toLocaleDateString()}</span>
                                      <Show when={cert.isExpired}>
                                        <span class="ml-1 text-red-600 dark:text-red-400 font-medium">(Expired!)</span>
                                      </Show>
                                    </div>
                                  )}
                                </For>
                              </dd>
                            </div>
                          </Show>
                        </dl>
                      </Show>
                    </div>
                    <button
                      type="button"
                      onClick={() => setTestResult(null)}
                      class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                    >
                      <X class="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </Show>

              {/* Advanced options (collapsed by default) */}
              <div class="pt-2">
                <button
                  type="button"
                  class="text-xs font-semibold text-slate-700 hover:underline dark:text-slate-300"
                  onClick={() => setAdvancedOpen(!advancedOpen())}
                >
                  {advancedOpen() ? 'Hide' : 'Show'} access restrictions & role mapping
                </button>

                <Show when={advancedOpen()}>
                  <div class="mt-4 space-y-4 p-4 bg-slate-50 dark:bg-slate-800 rounded-md">
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div class={formField}>
                        <label class={labelClass()}>Allowed Groups</label>
                        <textarea
                          rows={2}
                          value={form.allowedGroups}
                          onInput={(e) => setForm('allowedGroups', e.currentTarget.value)}
                          placeholder="admin, sso-users"
                          class={controlClass('min-h-[60px]')}
                        />
                        <p class={formHelpText}>Comma-separated. Empty allows all.</p>
                      </div>
                      <div class={formField}>
                        <label class={labelClass()}>Allowed Domains</label>
                        <textarea
                          rows={2}
                          value={form.allowedDomains}
                          onInput={(e) => setForm('allowedDomains', e.currentTarget.value)}
                          placeholder="example.com, corp.io"
                          class={controlClass('min-h-[60px]')}
                        />
                        <p class={formHelpText}>Email domains (without @)</p>
                      </div>
                      <div class={formField}>
                        <label class={labelClass()}>Allowed Emails</label>
                        <textarea
                          rows={2}
                          value={form.allowedEmails}
                          onInput={(e) => setForm('allowedEmails', e.currentTarget.value)}
                          placeholder="admin@example.com"
                          class={controlClass('min-h-[60px]')}
                        />
                      </div>
                      <div class={formField}>
                        <label class={labelClass()}>Group Role Mappings</label>
                        <textarea
                          rows={2}
                          value={form.groupRoleMappings}
                          onInput={(e) => setForm('groupRoleMappings', e.currentTarget.value)}
                          placeholder="admins=admin, ops=operator"
                          class={controlClass('min-h-[60px]')}
                        />
                        <p class={formHelpText}>Format: group=roleId</p>
                      </div>
                    </div>
                  </div>
                </Show>
              </div>

              {/* Actions */}
              <div class="flex justify-end gap-3 pt-4 border-t border-slate-200 dark:border-slate-700">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  class="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-600 rounded-md hover:bg-slate-50 dark:hover:bg-slate-700"
                  disabled={saving()}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  class="px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
                  disabled={saving()}
                >
                  {saving() ? 'Saving...' : editingProvider() ? 'Save Changes' : 'Create Provider'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </Show>

      {/* Delete confirmation modal */}
      <Show when={deleteConfirm()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black opacity-50 p-4" onClick={() => setDeleteConfirm(null)}>
          <div
            class="bg-white dark:bg-slate-800 rounded-md shadow-sm max-w-md w-full p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 class="text-lg font-semibold text-slate-900 dark:text-white mb-2">Delete Provider?</h3>
            <p class="text-sm text-slate-600 dark:text-slate-400 mb-4">
              This will permanently delete this SSO provider. Users will no longer be able to sign in using this provider.
            </p>
            <div class="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setDeleteConfirm(null)}
                class="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-600 rounded-md hover:bg-slate-50 dark:hover:bg-slate-700"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => handleDelete(deleteConfirm()!)}
                class="px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700"
              >
                Delete Provider
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* Metadata Preview modal */}
      <Show when={showMetadataPreview() && metadataPreview()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black opacity-50 p-4" onClick={() => setShowMetadataPreview(false)}>
          <div
            class="bg-white dark:bg-slate-800 rounded-md shadow-sm max-w-4xl w-full max-h-[90vh] flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal header */}
            <div class="px-6 py-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between flex-shrink-0">
              <h3 class="text-lg font-semibold text-slate-900 dark:text-white">IdP Metadata Preview</h3>
              <button
                type="button"
                onClick={() => setShowMetadataPreview(false)}
                class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
              >
                <X class="w-5 h-5" />
              </button>
            </div>

            {/* Parsed info summary */}
            <div class="px-6 py-4 bg-slate-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 flex-shrink-0">
              <h4 class="text-sm font-medium text-slate-900 dark:text-white mb-3">Parsed Information</h4>
              <dl class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                <div>
                  <dt class="text-slate-500 dark:text-slate-400 text-xs uppercase tracking-wide">Entity ID</dt>
                  <dd class="text-slate-900 dark:text-white font-mono text-xs mt-0.5 break-all">
                    {metadataPreview()?.parsed.entityId || 'N/A'}
                  </dd>
                </div>
                <div>
                  <dt class="text-slate-500 dark:text-slate-400 text-xs uppercase tracking-wide">SSO URL</dt>
                  <dd class="text-slate-900 dark:text-white font-mono text-xs mt-0.5 break-all">
                    {metadataPreview()?.parsed.ssoUrl || 'N/A'}
                  </dd>
                </div>
                <Show when={metadataPreview()?.parsed.sloUrl}>
                  <div>
                    <dt class="text-slate-500 dark:text-slate-400 text-xs uppercase tracking-wide">SLO URL</dt>
                    <dd class="text-slate-900 dark:text-white font-mono text-xs mt-0.5 break-all">
                      {metadataPreview()?.parsed.sloUrl}
                    </dd>
                  </div>
                </Show>
                <Show when={metadataPreview()?.parsed.certificates && metadataPreview()!.parsed.certificates!.length > 0}>
                  <div class="sm:col-span-2">
                    <dt class="text-slate-500 dark:text-slate-400 text-xs uppercase tracking-wide mb-1">Certificates</dt>
                    <dd class="space-y-1">
                      <For each={metadataPreview()?.parsed.certificates}>
                        {(cert) => (
                          <div class={`text-xs px-2 py-1 rounded ${cert.isExpired ? 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300' : 'bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300'}`}>
                            <span class="font-medium">{cert.subject}</span>
                            <span class="mx-1">•</span>
                            <span>Expires: {new Date(cert.notAfter).toLocaleDateString()}</span>
                            <Show when={cert.isExpired}>
                              <span class="ml-1 text-red-600 dark:text-red-400 font-medium">(Expired!)</span>
                            </Show>
                          </div>
                        )}
                      </For>
                    </dd>
                  </div>
                </Show>
              </dl>
            </div>

            {/* Raw XML */}
            <div class="flex-1 overflow-auto p-4">
              <div class="flex items-center justify-between mb-2">
                <h4 class="text-sm font-medium text-slate-900 dark:text-white">Raw XML</h4>
                <button
                  type="button"
                  onClick={() => {
                    if (metadataPreview()?.xml) {
                      navigator.clipboard.writeText(metadataPreview()!.xml);
                      notificationStore.success('XML copied to clipboard');
                    }
                  }}
                  class="px-2 py-1 text-xs font-medium text-slate-600 dark:text-slate-300 bg-slate-100 dark:bg-slate-700 rounded hover:bg-slate-200 dark:hover:bg-slate-600 flex items-center gap-1"
                >
                  <Copy class="w-3 h-3" />
                  Copy
                </button>
              </div>
              <pre class="text-xs bg-slate-900 text-slate-100 p-4 rounded-md overflow-x-auto whitespace-pre-wrap break-all font-mono">
                {metadataPreview()?.xml}
              </pre>
            </div>

            {/* Modal footer */}
            <div class="px-6 py-4 border-t border-slate-200 dark:border-slate-700 flex justify-end flex-shrink-0">
              <button
                type="button"
                onClick={() => setShowMetadataPreview(false)}
                class="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-600 rounded-md hover:bg-slate-50 dark:hover:bg-slate-700"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default SSOProvidersPanel;
