import { Component, For, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Dialog } from '@/components/shared/Dialog';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Shield from 'lucide-solid/icons/shield';
import Copy from 'lucide-solid/icons/copy';
import ExternalLink from 'lucide-solid/icons/external-link';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import Eye from 'lucide-solid/icons/eye';
import X from 'lucide-solid/icons/x';
import { SSOProviderTypeIcon } from './SSOProviderTypeIcon';
import {
  getSSOProviderAddButtonLabel,
  getSSOCertificatePresentation,
  getSSOProviderCardClass,
  getSSOProviderEmptyStateDescription,
  getSSOProviderEmptyStateTitle,
  getSSOProvidersLoadingState,
  getSSOProviderModalTitle,
  getSSOProviderSummary,
  getSSOProviderTypeBadgeClass,
  getSSOProviderTypeLabel,
} from '@/utils/ssoProviderPresentation';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';
import { ALERT_EMAIL_REPLY_TO_PLACEHOLDER } from '@/utils/alertEmailPresentation';
import { useSSOProvidersState } from '@/components/Settings/useSSOProvidersState';

interface SSOProvidersPanelProps {
  onConfigUpdated?: () => void;
  canManage?: boolean;
}

export const SSOProvidersPanel: Component<SSOProvidersPanelProps> = (props) => {
  const {
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
  } = useSSOProvidersState(props);

  return (
    <div class="space-y-6">
      <Show when={showSamlUpsell()}>
        <Dialog
          isOpen={true}
          onClose={() => setShowSamlUpsell(false)}
          panelClass="max-w-lg"
          closeOnBackdrop={false}
          ariaLabel="Add SAML provider"
        >
          <div class="w-full">
            <div class="flex items-center justify-between px-6 py-4 border-b border-border">
              <div>
                <h3 class="text-lg font-semibold text-base-content">Add SAML Provider</h3>
                <p class="text-xs text-muted mt-0.5">Pro feature</p>
              </div>
              <button
                type="button"
                onClick={() => setShowSamlUpsell(false)}
                class="p-1.5 rounded-md hover:text-base-content hover:bg-surface-hover"
                aria-label="Close"
              >
                <X class="w-5 h-5" />
              </button>
            </div>
            <div class="px-6 py-5 space-y-4">
              <p class="text-sm text-muted">SAML 2.0 and multi-provider SSO requires Pro.</p>
              <div class="flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowSamlUpsell(false)}
                  class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md"
                >
                  Not now
                </button>
                <UpgradeLink
                  destination={getUpgradeActionDestination('advanced_sso')}
                  class={getUpgradeActionButtonClass({ mobileFullWidth: false })}
                  onClick={() =>
                    trackUpgradeClicked('settings_sso_providers_add_saml_gate', 'advanced_sso')
                  }
                >
                  {UPGRADE_ACTION_LABEL}
                  <ExternalLink class="w-4 h-4" />
                </UpgradeLink>
                <Show when={canStartTrial()}>
                  <button
                    type="button"
                    onClick={handleStartTrial}
                    disabled={startingTrial()}
                    class={UPGRADE_TRIAL_LINK_CLASS}
                  >
                    {UPGRADE_TRIAL_LABEL}
                  </button>
                </Show>
              </div>
            </div>
          </div>
        </Dialog>
      </Show>

      {/* License banner */}
      <Show when={runtimeCapabilitiesLoaded() && !hasAdvancedSSO() && !loading()}>
        <div class="p-4 bg-surface-alt border border-border rounded-md">
          <div class="flex flex-col sm:flex-row items-center gap-4">
            <div class="flex-1">
              <h4 class="text-base font-semibold text-base-content">Advanced SSO</h4>
              <p class="text-sm text-muted mt-1">
                SAML 2.0 and multi-provider SSO requires Pro. Basic OIDC is available in the free
                tier.
              </p>
            </div>
            <div class="flex flex-col sm:flex-row items-center gap-2">
              <UpgradeLink
                destination={getUpgradeActionDestination('advanced_sso')}
                class={getUpgradeActionButtonClass({ mobileFullWidth: false })}
                onClick={() => trackUpgradeClicked('settings_sso_providers_banner', 'advanced_sso')}
              >
                {UPGRADE_ACTION_LABEL}
                <ExternalLink class="w-4 h-4" />
              </UpgradeLink>
              <Show when={canStartTrial()}>
                <button
                  type="button"
                  onClick={handleStartTrial}
                  disabled={startingTrial()}
                  class={UPGRADE_TRIAL_LINK_CLASS}
                >
                  {UPGRADE_TRIAL_LABEL}
                </button>
              </Show>
            </div>
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
              disabled={!canManage()}
              class="min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-1.5"
            >
              <Plus class="w-4 h-4" />
              {getSSOProviderAddButtonLabel('oidc')}
            </button>
            <Show when={hasAdvancedSSO()}>
              <button
                type="button"
                onClick={() => openAddModal('saml')}
                disabled={!canManage()}
                class="min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors flex items-center gap-1.5"
              >
                <Plus class="w-4 h-4" />
                {getSSOProviderAddButtonLabel('saml')}
              </button>
            </Show>
            <Show when={runtimeCapabilitiesLoaded() && !hasAdvancedSSO()}>
              <button
                type="button"
                onClick={() => setShowSamlUpsell(true)}
                disabled={!canManage()}
                class="min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors flex items-center gap-1.5"
              >
                <Plus class="w-4 h-4" />
                {getSSOProviderAddButtonLabel('saml', true)}
              </button>
            </Show>
          </div>
        }
        bodyClass="space-y-6"
      >
        <Show when={!canManage()}>
          <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
            Single sign-on is read-only for this account. You can review configured providers but
            cannot add, edit, enable, or delete them.
          </div>
        </Show>

        {/* Content */}
        <Show when={loading()}>
          <div class="flex items-center gap-3 text-sm text-muted">
            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
            {getSSOProvidersLoadingState().text}
          </div>
        </Show>

        <Show when={!loading() && providers().length === 0}>
          <div class="text-center py-8 text-muted">
            <Shield class="w-12 h-12 mx-auto mb-3 opacity-40" />
            <p class="text-sm">{getSSOProviderEmptyStateTitle()}</p>
            <p class="text-xs mt-1">{getSSOProviderEmptyStateDescription()}</p>
          </div>
        </Show>

        <Show when={!loading() && providers().length > 0}>
          <div class="space-y-3">
            <For each={providers()}>
              {(provider) => (
                <div
                  class={getSSOProviderCardClass(provider.enabled)}
                >
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div class="flex items-center gap-3 min-w-0">
                      <div class="p-2 rounded-md bg-surface-hover">
                        <SSOProviderTypeIcon type={provider.type} class="w-5 h-5 text-muted" />
                      </div>
                      <div class="min-w-0">
                        <div class="flex items-center gap-2">
                          <span class="font-medium text-base-content truncate">
                            {provider.name}
                          </span>
                          <span class={getSSOProviderTypeBadgeClass()}>
                            {getSSOProviderTypeLabel(provider.type)}
                          </span>
                        </div>
                        <p class="text-xs text-muted truncate">
                          {getSSOProviderSummary(provider)}
                        </p>
                      </div>
                    </div>
                    <div class="flex items-center gap-2 self-end sm:self-auto">
                      <Toggle
                        checked={provider.enabled}
                        onChange={() => handleToggleEnabled(provider)}
                        disabled={!canManage()}
                        containerClass="items-center"
                      />
                      <button
                        type="button"
                        onClick={() => openEditModal(provider)}
                        disabled={!canManage()}
                        class="p-2 text-slate-500 hover:text-blue-600 hover:bg-surface-hover rounded-md transition-colors"
                        title="Edit provider"
                      >
                        <Pencil class="w-4 h-4" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setDeleteConfirm(provider.id)}
                        disabled={!canManage()}
                        class="p-2 text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900 rounded-md transition-colors"
                        title="Delete provider"
                      >
                        <Trash2 class="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  {/* SAML metadata info */}
                  <Show when={provider.type === 'saml' && provider.enabled}>
                    <div class="mt-3 pt-3 border-t border-border-subtle">
                      <div class="flex flex-wrap gap-4 text-xs">
                        <div class="flex items-center gap-1">
                          <span class="text-slate-500">SP Metadata:</span>
                          <button
                            type="button"
                            onClick={() =>
                              copyToClipboard(provider.samlMetadataUrl || '', 'Metadata URL')
                            }
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
        <Dialog
          isOpen={true}
          onClose={() => setShowModal(false)}
          panelClass="max-w-2xl"
          ariaLabel={`${editingProvider() ? 'Edit' : 'Add'} ${getSSOProviderTypeLabel(form.type)} provider`}
        >
          <div class="w-full max-h-[90vh] overflow-y-auto">
            <div class="sticky top-0 bg-surface px-6 py-4 border-b border-border z-10">
              <h3 class="text-lg font-semibold text-base-content">
                {getSSOProviderModalTitle(Boolean(editingProvider()), form.type)}
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
                          class="px-3 py-2 text-sm font-medium bg-surface-hover text-base-content rounded-md hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
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
                        placeholder={
                          editingProvider() ? '•••••••• (leave blank to keep)' : 'Optional for PKCE'
                        }
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
                  <div class="bg-surface-alt border border-border rounded-md p-3">
                    <p class="text-xs text-base-content">
                      <strong>Setup:</strong> Provide either IdP Metadata URL (preferred) or
                      configure SSO URL + Certificate manually. Use the SP Metadata URL below to
                      configure your Identity Provider.
                    </p>
                    <Show when={publicUrl()}>
                      <div class="mt-2 flex items-center gap-2">
                        <span class="text-xs text-muted">SP Metadata:</span>
                        <code class="text-xs bg-surface-hover px-2 py-0.5 rounded">
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
                        class="px-3 py-2 text-sm font-medium bg-surface-hover text-base-content rounded-md hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                        title="Test connection to IdP"
                      >
                        {testing() ? 'Testing...' : 'Test'}
                      </button>
                      <button
                        type="button"
                        onClick={fetchMetadataPreview}
                        disabled={loadingPreview() || !form.samlIdpMetadataUrl.trim()}
                        class="px-3 py-2 text-sm font-medium bg-surface-hover text-base-content rounded-md hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap flex items-center gap-1"
                        title="Preview IdP metadata XML"
                      >
                        <Eye class="w-4 h-4" />
                        {loadingPreview() ? 'Loading...' : 'Preview'}
                      </button>
                    </div>
                    <p class={formHelpText}>URL to fetch IdP metadata (preferred method)</p>
                  </div>

                  <div class="text-center text-xs text-muted">— or configure manually —</div>

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
                      <span class="text-base-content">Allow IdP-initiated SSO</span>
                    </label>
                  </div>
                </div>
              </Show>

              {/* Test result display */}
              <Show when={testResult()}>
                <div
                  class={testResultPresentation()!.panelClass}
                >
                  <div class="flex items-start gap-3">
                    {testResult()?.success ? (
                      <CheckCircle class={testResultPresentation()!.iconClass} />
                    ) : (
                      <XCircle class={testResultPresentation()!.iconClass} />
                    )}
                    <div class="flex-1 min-w-0">
                      <p class={testResultPresentation()!.titleClass}>
                        {testResult()?.message}
                      </p>
                      <Show when={testResult()?.error}>
                        <p class={testResultPresentation()!.errorClass}>
                          {testResult()?.error}
                        </p>
                      </Show>
                      <Show when={testResult()?.success && testResult()?.details}>
                        <dl class="mt-2 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-xs">
                          <Show when={testResult()?.details?.entityId}>
                            <div class="flex gap-2">
                              <dt class="text-muted">Entity ID:</dt>
                              <dd class="text-base-content truncate">
                                {testResult()?.details?.entityId}
                              </dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.ssoUrl}>
                            <div class="flex gap-2">
                              <dt class="text-muted">SSO URL:</dt>
                              <dd class="text-base-content truncate">
                                {testResult()?.details?.ssoUrl}
                              </dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.tokenEndpoint}>
                            <div class="flex gap-2">
                              <dt class="text-muted">Token Endpoint:</dt>
                              <dd class="text-base-content truncate">
                                {testResult()?.details?.tokenEndpoint}
                              </dd>
                            </div>
                          </Show>
                          <Show
                            when={
                              testResult()?.details?.certificates &&
                              testResult()!.details!.certificates!.length > 0
                            }
                          >
                            <div class="col-span-2 mt-1">
                              <dt class="text-muted mb-1">Certificates:</dt>
                              <dd class="space-y-1">
                                <For each={testResult()?.details?.certificates}>
                                  {(cert) => {
                                    const certPresentation = getSSOCertificatePresentation(
                                      cert.isExpired,
                                    );

                                    return (
                                    <div
                                      class={certPresentation.containerClass}
                                    >
                                      <span class="font-medium">{cert.subject}</span>
                                      <span class="mx-1">•</span>
                                      <span>
                                        Expires: {new Date(cert.notAfter).toLocaleDateString()}
                                      </span>
                                      <Show when={cert.isExpired}>
                                        <span class={certPresentation.expiredLabelClass}>
                                          {certPresentation.expiredLabel}
                                        </span>
                                      </Show>
                                    </div>
                                    );
                                  }}
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
                      class="text-slate-400 hover:text-base-content"
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
                  class="text-xs font-semibold text-base-content hover:underline"
                  onClick={() => setAdvancedOpen(!advancedOpen())}
                >
                  {advancedOpen() ? 'Hide' : 'Show'} access restrictions & role mapping
                </button>

                <Show when={advancedOpen()}>
                  <div class="mt-4 space-y-4 p-4 bg-surface-alt rounded-md">
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
                          placeholder={ALERT_EMAIL_REPLY_TO_PLACEHOLDER}
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
              <div class="flex justify-end gap-3 pt-4 border-t border-border">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
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
        </Dialog>
      </Show>

      {/* Delete confirmation modal */}
      <Show when={deleteConfirm()}>
        <Dialog
          isOpen={true}
          onClose={() => setDeleteConfirm(null)}
          panelClass="max-w-md"
          ariaLabel="Delete provider"
        >
          <div class="w-full p-6">
            <h3 class="text-lg font-semibold text-base-content mb-2">Delete Provider?</h3>
            <p class="text-sm text-muted mb-4">
              This will permanently delete this SSO provider. Users will no longer be able to sign
              in using this provider.
            </p>
            <div class="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setDeleteConfirm(null)}
                class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
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
        </Dialog>
      </Show>

      {/* Metadata Preview modal */}
      <Show when={showMetadataPreview() && metadataPreview()}>
        <Dialog
          isOpen={true}
          onClose={() => setShowMetadataPreview(false)}
          panelClass="max-w-4xl"
          ariaLabel="IdP metadata preview"
        >
          <div class="w-full max-h-[90vh] flex flex-col">
            {/* Modal header */}
            <div class="px-6 py-4 border-b border-border flex items-center justify-between flex-shrink-0">
              <h3 class="text-lg font-semibold text-base-content">IdP Metadata Preview</h3>
              <button
                type="button"
                onClick={() => setShowMetadataPreview(false)}
                class="text-slate-400 hover:text-base-content"
              >
                <X class="w-5 h-5" />
              </button>
            </div>

            {/* Parsed info summary */}
            <div class="px-6 py-4 bg-surface-alt border-b border-border flex-shrink-0">
              <h4 class="text-sm font-medium text-base-content mb-3">Parsed Information</h4>
              <dl class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                <div>
                  <dt class="text-muted text-xs uppercase tracking-wide">Entity ID</dt>
                  <dd class="text-base-content font-mono text-xs mt-0.5 break-all">
                    {metadataPreview()?.parsed.entityId || 'N/A'}
                  </dd>
                </div>
                <div>
                  <dt class="text-muted text-xs uppercase tracking-wide">SSO URL</dt>
                  <dd class="text-base-content font-mono text-xs mt-0.5 break-all">
                    {metadataPreview()?.parsed.ssoUrl || 'N/A'}
                  </dd>
                </div>
                <Show when={metadataPreview()?.parsed.sloUrl}>
                  <div>
                    <dt class="text-muted text-xs uppercase tracking-wide">SLO URL</dt>
                    <dd class="text-base-content font-mono text-xs mt-0.5 break-all">
                      {metadataPreview()?.parsed.sloUrl}
                    </dd>
                  </div>
                </Show>
                <Show
                  when={
                    metadataPreview()?.parsed.certificates &&
                    metadataPreview()!.parsed.certificates!.length > 0
                  }
                >
                  <div class="sm:col-span-2">
                    <dt class="text-muted text-xs uppercase tracking-wide mb-1">Certificates</dt>
                    <dd class="space-y-1">
                      <For each={metadataPreview()?.parsed.certificates}>
                        {(cert) => {
                          const certPresentation = getSSOCertificatePresentation(
                            Boolean(cert.isExpired),
                          );

                          return (
                          <div
                            class={certPresentation.containerClass}
                          >
                            <span class="font-medium">{cert.subject}</span>
                            <span class="mx-1">•</span>
                            <span>Expires: {new Date(cert.notAfter).toLocaleDateString()}</span>
                            <Show when={cert.isExpired}>
                              <span class={certPresentation.expiredLabelClass}>
                                {certPresentation.expiredLabel}
                              </span>
                            </Show>
                          </div>
                          );
                        }}
                      </For>
                    </dd>
                  </div>
                </Show>
              </dl>
            </div>

            {/* Raw XML */}
            <div class="flex-1 overflow-auto p-4">
              <div class="flex items-center justify-between mb-2">
                <h4 class="text-sm font-medium text-base-content">Raw XML</h4>
                <button
                  type="button"
                  onClick={() => {
                    if (metadataPreview()?.xml) {
                      copyToClipboard(metadataPreview()!.xml, 'XML');
                    }
                  }}
                  class="px-2 py-1 text-xs font-medium text-muted bg-surface-hover rounded hover:bg-surface-hover flex items-center gap-1"
                >
                  <Copy class="w-3 h-3" />
                  Copy
                </button>
              </div>
              <pre class="text-xs bg-base text-base-content p-4 rounded-md overflow-x-auto whitespace-pre-wrap break-all font-mono">
                {metadataPreview()?.xml}
              </pre>
            </div>

            {/* Modal footer */}
            <div class="px-6 py-4 border-t border-border flex justify-end flex-shrink-0">
              <button
                type="button"
                onClick={() => setShowMetadataPreview(false)}
                class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
              >
                Close
              </button>
            </div>
          </div>
        </Dialog>
      </Show>
    </div>
  );
};

export default SSOProvidersPanel;
