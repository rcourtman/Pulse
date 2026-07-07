import { Component, For, Show } from 'solid-js';
import { ActionIconButton, Button, CopyValueButton } from '@/components/shared/Button';
import { EmptyState } from '@/components/shared/EmptyState';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { Dialog } from '@/components/shared/Dialog';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Shield from 'lucide-solid/icons/shield';
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
  } = useSSOProvidersState(props);

  return (
    <div class="space-y-6">
      {/* Main panel */}
      <SettingsPanel
        title="Single Sign-On Providers"
        action={
          <div class="flex flex-wrap justify-end gap-2">
            <Button
              type="button"
              onClick={() => openAddModal('oidc')}
              disabled={!canManage()}
              variant="primary"
              size="settingsAction"
              class="gap-1.5"
            >
              <Plus class="w-4 h-4" />
              {getSSOProviderAddButtonLabel('oidc')}
            </Button>
            <Button
              type="button"
              onClick={() => openAddModal('saml')}
              disabled={!canManage()}
              variant="outline"
              size="settingsAction"
              class="gap-1.5"
            >
              <Plus class="w-4 h-4" />
              {getSSOProviderAddButtonLabel('saml')}
            </Button>
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
            <LoadingSpinner size="md" tone="current" />
            {getSSOProvidersLoadingState().text}
          </div>
        </Show>

        <Show when={!loading() && providers().length === 0}>
          <EmptyState
            variant="panel"
            icon={<Shield class="h-5 w-5" />}
            title={getSSOProviderEmptyStateTitle()}
            description={getSSOProviderEmptyStateDescription()}
          />
        </Show>

        <Show when={!loading() && providers().length > 0}>
          <div class="space-y-3">
            <For each={providers()}>
              {(provider) => (
                <div class={getSSOProviderCardClass(provider.enabled)}>
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div class="flex items-center gap-3 min-w-0">
                      <div class="p-2 rounded-md bg-surface-hover">
                        <SSOProviderTypeIcon type={provider.type} class="w-5 h-5 text-muted" />
                      </div>
                      <div class="min-w-0">
                        <div class="flex items-center gap-2">
                          <span
                            class="font-medium text-base-content truncate"
                            title={provider.name}
                          >
                            {provider.name}
                          </span>
                          <span class={getSSOProviderTypeBadgeClass()}>
                            {getSSOProviderTypeLabel(provider.type)}
                          </span>
                        </div>
                        <p
                          class="text-xs text-muted truncate"
                          title={getSSOProviderSummary(provider)}
                        >
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
                      <ActionIconButton
                        type="button"
                        onClick={() => openEditModal(provider)}
                        disabled={!canManage()}
                        label="Edit provider"
                        tone="accent"
                        size="md"
                      >
                        <Pencil class="w-4 h-4" />
                      </ActionIconButton>
                      <ActionIconButton
                        type="button"
                        onClick={() => setDeleteConfirm(provider.id)}
                        disabled={!canManage()}
                        label="Delete provider"
                        tone="danger"
                        size="md"
                      >
                        <Trash2 class="w-4 h-4" />
                      </ActionIconButton>
                    </div>
                  </div>

                  {/* SAML metadata info */}
                  <Show when={provider.type === 'saml' && provider.enabled}>
                    <div class="mt-3 pt-3 border-t border-border-subtle">
                      <div class="flex flex-wrap gap-4 text-xs">
                        <div class="flex items-center gap-1">
                          <span class="text-slate-500">SP Metadata:</span>
                          <CopyValueButton
                            value={provider.samlMetadataUrl}
                            onCopyValue={(value) => copyToClipboard(value, 'Metadata URL')}
                            label="Copy SP metadata URL"
                            variant="accent"
                            size="chip"
                            class="max-w-[18rem] text-xs"
                          >
                            <span class="min-w-0 truncate">{provider.samlMetadataUrl}</span>
                          </CopyValueButton>
                        </div>
                        <div class="flex items-center gap-1">
                          <span class="text-slate-500">ACS URL:</span>
                          <CopyValueButton
                            value={provider.samlAcsUrl}
                            onCopyValue={(value) => copyToClipboard(value, 'ACS URL')}
                            label="Copy ACS URL"
                            variant="accent"
                            size="chip"
                            class="max-w-[18rem] text-xs"
                          >
                            <span class="min-w-0 truncate">{provider.samlAcsUrl}</span>
                          </CopyValueButton>
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
                        <Button
                          type="button"
                          onClick={testConnection}
                          disabled={testing() || !canTest()}
                          variant="secondary"
                          size="mdCompact"
                          class="whitespace-nowrap"
                          title="Test connection to IdP"
                        >
                          {testing() ? 'Testing...' : 'Test'}
                        </Button>
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

                  <div class={formField}>
                    <label class={labelClass()}>Scopes</label>
                    <input
                      type="text"
                      value={form.oidcScopes}
                      onInput={(e) => setForm('oidcScopes', e.currentTarget.value)}
                      placeholder="openid profile email"
                      class={controlClass()}
                    />
                    <p class={formHelpText}>
                      Space-separated scopes for the sign-in request. Some IdPs only return group
                      claims when an extra scope (e.g. groups) is requested here.
                    </p>
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
                      <div class="mt-2 flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-2">
                        <span class="text-xs text-muted">SP Metadata:</span>
                        <code class="inline-block max-w-full break-all rounded bg-surface-hover px-2 py-0.5 text-xs">
                          {publicUrl()}/api/saml/{form.id || '{id}'}/metadata
                        </code>
                      </div>
                    </Show>
                  </div>

                  <div class={formField}>
                    <label class={labelClass()}>IdP Metadata URL</label>
                    <div class="flex flex-col gap-2 sm:flex-row">
                      <input
                        type="url"
                        value={form.samlIdpMetadataUrl}
                        onInput={(e) => setForm('samlIdpMetadataUrl', e.currentTarget.value)}
                        placeholder="https://idp.example.com/metadata"
                        class={controlClass() + ' min-w-0 flex-1'}
                      />
                      <div class="flex gap-2">
                        <Button
                          type="button"
                          onClick={testConnection}
                          disabled={testing() || !canTest()}
                          variant="secondary"
                          size="mdCompact"
                          class="flex-1 whitespace-nowrap sm:flex-none"
                          title="Test connection to IdP"
                        >
                          {testing() ? 'Testing...' : 'Test'}
                        </Button>
                        <Button
                          type="button"
                          onClick={fetchMetadataPreview}
                          disabled={loadingPreview() || !form.samlIdpMetadataUrl.trim()}
                          variant="secondary"
                          size="mdCompact"
                          class="flex-1 gap-1 whitespace-nowrap sm:flex-none"
                          title="Preview IdP metadata XML"
                        >
                          <Eye class="w-4 h-4" />
                          {loadingPreview() ? 'Loading...' : 'Preview'}
                        </Button>
                      </div>
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

                  <FormTextarea
                    label="IdP Certificate (PEM)"
                    rows={4}
                    value={form.samlIdpCertificate}
                    onInput={(e) => setForm('samlIdpCertificate', e.currentTarget.value)}
                    placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                    textareaBaseClass={controlClass('font-mono text-xs')}
                  />

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
                        value={form.groupsClaim}
                        onInput={(e) => setForm('groupsClaim', e.currentTarget.value)}
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
                <div class={testResultPresentation()!.panelClass}>
                  <div class="flex items-start gap-3">
                    {testResult()?.success ? (
                      <CheckCircle class={testResultPresentation()!.iconClass} />
                    ) : (
                      <XCircle class={testResultPresentation()!.iconClass} />
                    )}
                    <div class="flex-1 min-w-0">
                      <p class={testResultPresentation()!.titleClass}>{testResult()?.message}</p>
                      <Show when={testResult()?.error}>
                        <p class={testResultPresentation()!.errorClass}>{testResult()?.error}</p>
                      </Show>
                      <Show when={testResult()?.success && testResult()?.details}>
                        <dl class="mt-2 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-xs">
                          <Show when={testResult()?.details?.entityId}>
                            <div class="flex gap-2">
                              <dt class="text-muted">Entity ID:</dt>
                              <dd
                                class="text-base-content truncate"
                                title={testResult()?.details?.entityId}
                              >
                                {testResult()?.details?.entityId}
                              </dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.ssoUrl}>
                            <div class="flex gap-2">
                              <dt class="text-muted">SSO URL:</dt>
                              <dd
                                class="text-base-content truncate"
                                title={testResult()?.details?.ssoUrl}
                              >
                                {testResult()?.details?.ssoUrl}
                              </dd>
                            </div>
                          </Show>
                          <Show when={testResult()?.details?.tokenEndpoint}>
                            <div class="flex gap-2">
                              <dt class="text-muted">Token Endpoint:</dt>
                              <dd
                                class="text-base-content truncate"
                                title={testResult()?.details?.tokenEndpoint}
                              >
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
                                      <div class={certPresentation.containerClass}>
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
                    <ActionIconButton
                      type="button"
                      onClick={() => setTestResult(null)}
                      label="Dismiss test result"
                      tone="muted"
                      size="sm"
                    >
                      <X class="w-4 h-4" />
                    </ActionIconButton>
                  </div>
                </div>
              </Show>

              {/* Advanced options (collapsed by default) */}
              <div class="pt-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  class="justify-start font-semibold hover:underline"
                  onClick={() => setAdvancedOpen(!advancedOpen())}
                >
                  {advancedOpen() ? 'Hide' : 'Show'} access restrictions & role mapping
                </Button>

                <Show when={advancedOpen()}>
                  <div class="mt-4 space-y-4 p-4 bg-surface-alt rounded-md">
                    <Show when={form.type === 'oidc'}>
                      <div class={formField}>
                        <label class={labelClass()}>Groups Claim</label>
                        <input
                          type="text"
                          value={form.groupsClaim}
                          onInput={(e) => setForm('groupsClaim', e.currentTarget.value)}
                          placeholder="groups"
                          class={controlClass()}
                        />
                        <p class={formHelpText}>
                          Claim used for OIDC allowed groups and role mappings. If your IdP only
                          returns groups when asked, add the matching scope in the Scopes field
                          above.
                        </p>
                      </div>
                    </Show>
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <FormTextarea
                        label="Allowed Groups"
                        rows={2}
                        value={form.allowedGroups}
                        onInput={(e) => setForm('allowedGroups', e.currentTarget.value)}
                        placeholder="admin, sso-users"
                        textareaBaseClass={controlClass('min-h-[60px]')}
                        help="Comma-separated. Empty allows all."
                      />
                      <FormTextarea
                        label="Allowed Domains"
                        rows={2}
                        value={form.allowedDomains}
                        onInput={(e) => setForm('allowedDomains', e.currentTarget.value)}
                        placeholder="example.com, corp.io"
                        textareaBaseClass={controlClass('min-h-[60px]')}
                        help="Email domains (without @)"
                      />
                      <FormTextarea
                        label="Allowed Emails"
                        rows={2}
                        value={form.allowedEmails}
                        onInput={(e) => setForm('allowedEmails', e.currentTarget.value)}
                        placeholder={ALERT_EMAIL_REPLY_TO_PLACEHOLDER}
                        textareaBaseClass={controlClass('min-h-[60px]')}
                      />
                      <FormTextarea
                        label="Group Role Mappings"
                        rows={2}
                        value={form.groupRoleMappings}
                        onInput={(e) => setForm('groupRoleMappings', e.currentTarget.value)}
                        placeholder="admins=admin, ops=operator"
                        textareaBaseClass={controlClass('min-h-[60px]')}
                        help="Format: group=roleId"
                      />
                    </div>
                  </div>
                </Show>
              </div>

              {/* Actions */}
              <div class="flex justify-end gap-3 pt-4 border-t border-border">
                <Button
                  type="button"
                  onClick={() => setShowModal(false)}
                  variant="outline"
                  size="md"
                  disabled={saving()}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  variant="primary"
                  size="md"
                  isLoading={saving()}
                  disabled={saving()}
                >
                  {saving() ? 'Saving...' : editingProvider() ? 'Save Changes' : 'Create Provider'}
                </Button>
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
              <Button
                type="button"
                onClick={() => setDeleteConfirm(null)}
                variant="outline"
                size="md"
              >
                Cancel
              </Button>
              <Button
                type="button"
                onClick={() => handleDelete(deleteConfirm()!)}
                variant="danger"
                size="md"
              >
                Delete Provider
              </Button>
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
              <ActionIconButton
                type="button"
                onClick={() => setShowMetadataPreview(false)}
                label="Close metadata preview"
                tone="muted"
                size="md"
              >
                <X class="w-5 h-5" />
              </ActionIconButton>
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
                            <div class={certPresentation.containerClass}>
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
                <CopyValueButton
                  value={metadataPreview()?.xml}
                  onCopyValue={(value) => copyToClipboard(value, 'XML')}
                  label="Copy metadata XML"
                  variant="ghost"
                  size="chip"
                  class="text-xs text-muted"
                >
                  <span>Copy</span>
                </CopyValueButton>
              </div>
              <pre class="text-xs bg-base text-base-content p-4 rounded-md overflow-x-auto whitespace-pre-wrap break-all font-mono">
                {metadataPreview()?.xml}
              </pre>
            </div>

            {/* Modal footer */}
            <div class="px-6 py-4 border-t border-border flex justify-end flex-shrink-0">
              <Button
                type="button"
                onClick={() => setShowMetadataPreview(false)}
                variant="outline"
                size="md"
              >
                Close
              </Button>
            </div>
          </div>
        </Dialog>
      </Show>
    </div>
  );
};

export default SSOProvidersPanel;
