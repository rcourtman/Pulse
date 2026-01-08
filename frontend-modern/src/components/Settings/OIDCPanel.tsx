import { Component, Show, createSignal, onMount } from 'solid-js';
import { createStore } from 'solid-js/store';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { isEnterprise, loadLicenseStatus } from '@/stores/license';
import Sparkles from 'lucide-solid/icons/sparkles';
import ExternalLink from 'lucide-solid/icons/external-link';

interface OIDCConfigResponse {
  enabled: boolean;
  issuerUrl: string;
  clientId: string;
  redirectUrl: string;
  logoutUrl: string;
  scopes: string[];
  usernameClaim: string;
  emailClaim: string;
  groupsClaim: string;
  allowedGroups: string[];
  allowedDomains: string[];
  allowedEmails: string[];
  caBundle?: string;
  clientSecretSet: boolean;
  envOverrides?: Record<string, boolean>;
  defaultRedirect: string;
}

const listToString = (values?: string[]) => (values && values.length > 0 ? values.join(', ') : '');
const splitList = (input: string) =>
  input
    .split(/[,\s]+/)
    .map((v) => v.trim())
    .filter(Boolean);

interface Props {
  onConfigUpdated?: (config: OIDCConfigResponse) => void;
}

export const OIDCPanel: Component<Props> = (props) => {
  const [config, setConfig] = createSignal<OIDCConfigResponse | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [advancedOpen, setAdvancedOpen] = createSignal(false);

  const [form, setForm] = createStore({
    enabled: false,
    issuerUrl: '',
    clientId: '',
    redirectUrl: '',
    logoutUrl: '',
    scopes: '',
    usernameClaim: 'preferred_username',
    emailClaim: 'email',
    groupsClaim: '',
    allowedGroups: '',
    allowedDomains: '',
    allowedEmails: '',
    caBundle: '',
    clientSecret: '',
    clearSecret: false,
  });

  const isEnvLocked = () => {
    const env = config()?.envOverrides;
    return env ? Object.keys(env).length > 0 : false;
  };

  const resetForm = (data: OIDCConfigResponse | null) => {
    if (!data) {
      setForm({
        enabled: false,
        issuerUrl: '',
        clientId: '',
        redirectUrl: '',
        logoutUrl: '',
        scopes: '',
        usernameClaim: 'preferred_username',
        emailClaim: 'email',
        groupsClaim: '',
        allowedGroups: '',
        allowedDomains: '',
        allowedEmails: '',
        caBundle: '',
        clientSecret: '',
        clearSecret: false,
      });
      return;
    }

    setForm({
      enabled: data.enabled,
      issuerUrl: data.issuerUrl ?? '',
      clientId: data.clientId ?? '',
      redirectUrl: data.redirectUrl || data.defaultRedirect || '',
      logoutUrl: data.logoutUrl ?? '',
      scopes: data.scopes?.join(' ') ?? 'openid profile email',
      usernameClaim: data.usernameClaim || 'preferred_username',
      emailClaim: data.emailClaim || 'email',
      groupsClaim: data.groupsClaim ?? '',
      allowedGroups: listToString(data.allowedGroups),
      allowedDomains: listToString(data.allowedDomains),
      allowedEmails: listToString(data.allowedEmails),
      caBundle: data.caBundle || '',
      clientSecret: '',
      clearSecret: false,
    });
  };

  const loadConfig = async () => {
    setLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/oidc');
      if (!response.ok) {
        throw new Error(`Failed to load OIDC settings (${response.status})`);
      }
      const data = (await response.json()) as OIDCConfigResponse;
      setConfig(data);
      resetForm(data);
      props.onConfigUpdated?.(data);
    } catch (error) {
      logger.error('[OIDCPanel] Failed to load config:', error);
      notificationStore.error('Failed to load OIDC settings');
      setConfig(null);
      resetForm(null);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadLicenseStatus();
    loadConfig();
  });

  const handleSave = async (event?: Event) => {
    event?.preventDefault();
    if (isEnvLocked()) {
      return;
    }

    setSaving(true);
    try {
      const payload: Record<string, unknown> = {
        enabled: form.enabled,
        issuerUrl: form.issuerUrl.trim(),
        clientId: form.clientId.trim(),
        redirectUrl: form.redirectUrl.trim(),
        logoutUrl: form.logoutUrl.trim(),
        scopes: splitList(form.scopes),
        usernameClaim: form.usernameClaim.trim(),
        emailClaim: form.emailClaim.trim(),
        groupsClaim: form.groupsClaim.trim(),
        allowedGroups: splitList(form.allowedGroups),
        allowedDomains: splitList(form.allowedDomains),
        allowedEmails: splitList(form.allowedEmails),
        caBundle: form.caBundle.trim(),
      };

      if (form.clientSecret.trim() !== '') {
        payload.clientSecret = form.clientSecret.trim();
      } else if (form.clearSecret) {
        payload.clearClientSecret = true;
      }

      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/oidc', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || `Failed to save OIDC settings (${response.status})`);
      }

      const updated = (await response.json()) as OIDCConfigResponse;
      setConfig(updated);
      resetForm(updated);
      notificationStore.success('OIDC settings updated');
      props.onConfigUpdated?.(updated);
    } catch (error) {
      logger.error('[OIDCPanel] Failed to save config:', error);
      notificationStore.error('Failed to save OIDC settings');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      padding="none"
      class="overflow-hidden border border-gray-200 dark:border-gray-700"
      border={false}
    >
      <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-300"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
                d="M21 12c0 4.97-4.03 9-9 9m9-9c0-4.97-4.03-9-9-9m9 9H3m9 9c-4.97 0-9-4.03-9-9m9 9c-1.5-1.35-3-4.5-3-9s1.5-7.65 3-9m0 18c1.5-1.35 3-4.5 3-9s-1.5-7.65-3-9"
              />
            </svg>
          </div>
          <SectionHeader
            title="Single sign-on (OIDC)"
            description="Connect Pulse to your identity provider"
            size="sm"
            class="flex-1"
          />
          <Show when={isEnterprise()}>
            <span class="px-2 py-0.5 text-xs font-bold uppercase tracking-wider bg-gradient-to-r from-indigo-500 to-purple-500 text-white rounded-md shadow-sm">
              Enterprise
            </span>
          </Show>
          <Show when={!isEnterprise()}>
            <span class="px-2 py-0.5 text-xs font-bold uppercase tracking-wider bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400 rounded-md">
              Enterprise
            </span>
          </Show>
          <Toggle
            checked={form.enabled}
            onChange={async (event) => {
              const newValue = event.currentTarget.checked;
              setForm('enabled', newValue);

              // Auto-save when DISABLING (safe operation)
              // When ENABLING, require full form save to ensure issuerUrl and clientId are set
              if (!newValue && config()?.enabled) {
                try {
                  const { apiFetch } = await import('@/utils/apiClient');
                  const response = await apiFetch('/api/security/oidc', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ enabled: false }),
                  });
                  if (!response.ok) throw new Error('Failed to disable OIDC');
                  const updated = (await response.json()) as OIDCConfigResponse;
                  setConfig(updated);
                  notificationStore.success('OIDC disabled');
                  props.onConfigUpdated?.(updated);
                } catch (error) {
                  setForm('enabled', true); // Revert
                  logger.error('[OIDCPanel] Failed to disable OIDC:', error);
                  notificationStore.error('Failed to disable OIDC');
                }
              } else if (newValue && !config()?.enabled) {
                // Show hint that save is required
                notificationStore.info('Configure issuer URL and client ID, then click Save', 3000);
              }
            }}
            disabled={isEnvLocked() || loading() || saving()}
            containerClass="items-center gap-2"
            label={
              <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
                {form.enabled ? 'Enabled' : 'Disabled'}
              </span>
            }
          />
        </div>
      </div>
      <Show when={!isEnterprise() && !loading()}>
        <div class="mx-6 mt-6 p-4 bg-gradient-to-br from-indigo-50 to-purple-50 dark:from-indigo-900/20 dark:to-purple-900/20 border border-indigo-100 dark:border-indigo-800/50 rounded-xl">
          <div class="flex items-start gap-4">
            <div class="p-2 bg-indigo-100 dark:bg-indigo-900/50 rounded-lg">
              <Sparkles class="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
            </div>
            <div class="flex-1">
              <h4 class="text-sm font-bold text-indigo-900 dark:text-indigo-100">Enterprise Feature</h4>
              <p class="text-xs text-indigo-800/80 dark:text-indigo-200/80 mt-1">
                OIDC integration is an Enterprise-only feature. Upgrade your license to enable seamless Single Sign-On for your team.
              </p>
              <div class="mt-3">
                <a
                  href="https://pulse.sh/pro"
                  target="_blank"
                  class="inline-flex items-center gap-1.5 text-xs font-semibold text-indigo-600 dark:text-indigo-400 hover:underline"
                >
                  Learn about Enterprise
                  <ExternalLink class="w-3 h-3" />
                </a>
              </div>
            </div>
          </div>
        </div>
      </Show>
      <form class="p-6 space-y-5" onSubmit={handleSave}>
        <div class="bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 rounded-lg p-3 text-xs text-blue-800 dark:text-blue-200">
          <ol class="space-y-1 list-decimal pl-4">
            <li>Set PUBLIC_URL environment variable</li>
            <li>Register client with your IdP using redirect URL below</li>
            <li>Enter issuer and client ID (client secret optional for PKCE)</li>
            <li>Save and test with SSO button on login page</li>
          </ol>
        </div>
        <Show when={loading()}>
          <div class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-300">
            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
            Loading OIDC settings...
          </div>
        </Show>

        <Show when={!loading()}>
          <Show when={isEnvLocked()}>
            <div class="bg-amber-50 dark:bg-amber-900/30 border border-amber-200 dark:border-amber-700 rounded p-3 text-xs text-amber-800 dark:text-amber-200">
              <strong>Managed by environment variables:</strong> OIDC settings are currently defined
              through environment variables. Edit the deployment configuration to make changes.
            </div>
          </Show>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class={formField}>
              <label class={labelClass()}>Issuer URL</label>
              <input
                type="url"
                value={form.issuerUrl}
                onInput={(event) => setForm('issuerUrl', event.currentTarget.value)}
                placeholder="https://login.example.com/realms/pulse"
                class={controlClass()}
                disabled={isEnvLocked() || saving()}
                required
              />
              <p class={formHelpText}>Base issuer URL from your OIDC provider configuration.</p>
            </div>
            <div class={formField}>
              <label class={labelClass()}>Client ID</label>
              <input
                type="text"
                value={form.clientId}
                onInput={(event) => setForm('clientId', event.currentTarget.value)}
                placeholder="pulse-client"
                class={controlClass()}
                disabled={isEnvLocked() || saving()}
                required
              />
            </div>
            <div class={formField}>
              <div class="flex items-center justify-between">
                <label class={labelClass('mb-0')}>Client secret</label>
                <Show when={config()?.clientSecretSet}>
                  <button
                    type="button"
                    class="text-xs text-blue-600 hover:underline dark:text-blue-300"
                    onClick={() => {
                      if (!isEnvLocked() && !saving()) {
                        setForm('clientSecret', '');
                        setForm('clearSecret', true);
                        notificationStore.info('Client secret will be cleared on save', 2500);
                      }
                    }}
                    disabled={isEnvLocked() || saving()}
                  >
                    Clear stored secret
                  </button>
                </Show>
              </div>
              <input
                type="password"
                value={form.clientSecret}
                onInput={(event) => {
                  setForm('clientSecret', event.currentTarget.value);
                  if (event.currentTarget.value.trim() !== '') {
                    setForm('clearSecret', false);
                  }
                }}
                placeholder={
                  config()?.clientSecretSet
                    ? '•••••••• (leave blank to keep existing)'
                    : 'Enter client secret'
                }
                class={controlClass()}
                disabled={isEnvLocked() || saving()}
              />
              <p class={formHelpText}>
                Optional - Leave blank if your provider supports PKCE (Zitadel, Authentik, etc). Otherwise, enter your client secret.
              </p>
            </div>
            <div class={formField}>
              <label class={labelClass()}>Redirect URL</label>
              <input
                type="url"
                value={form.redirectUrl}
                onInput={(event) => setForm('redirectUrl', event.currentTarget.value)}
                placeholder={config()?.defaultRedirect || ''}
                class={controlClass()}
                disabled={isEnvLocked() || saving()}
              />
              <p class={formHelpText}>
                {config()?.defaultRedirect && config()?.defaultRedirect.trim() !== ''
                  ? `Optional - Leave blank to auto-detect from request headers (supports reverse proxies). Detected URL: ${config()?.defaultRedirect}`
                  : 'Optional - Leave blank to auto-detect from request headers. For best results, set PUBLIC_URL environment variable.'}
              </p>
            </div>
            <div class={formField}>
              <label class={labelClass()}>Logout URL</label>
              <input
                type="url"
                value={form.logoutUrl}
                onInput={(event) => setForm('logoutUrl', event.currentTarget.value)}
                placeholder="https://auth.example.com/application/o/pulse/end-session/"
                class={controlClass()}
                disabled={isEnvLocked() || saving()}
              />
              <p class={formHelpText}>
                Optional - OIDC end-session URL for proper logout (e.g., Authentik's end-session endpoint). Leave blank to use local logout only.
              </p>
            </div>
          </div>

          <div class="space-y-4">
            <button
              type="button"
              class="text-xs font-semibold text-blue-600 hover:underline dark:text-blue-300"
              onClick={() => setAdvancedOpen(!advancedOpen())}
            >
              {advancedOpen() ? 'Hide advanced OIDC options' : 'Show advanced OIDC options'}
            </button>

            <Show when={advancedOpen()}>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class={formField}>
                  <label class={labelClass()}>Scopes</label>
                  <input
                    type="text"
                    value={form.scopes}
                    onInput={(event) => setForm('scopes', event.currentTarget.value)}
                    placeholder="openid profile email"
                    class={controlClass()}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>Space-separated list of scopes requested during login.</p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>CA bundle path</label>
                  <input
                    type="text"
                    value={form.caBundle}
                    onInput={(event) => setForm('caBundle', event.currentTarget.value)}
                    placeholder="/etc/ssl/certs/oidc-ca.pem"
                    class={controlClass()}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>
                    Optional path to a PEM bundle containing your IdP&apos;s root certificates. Mount
                    the file into the container; leave blank to use the system trust store.
                  </p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Username claim</label>
                  <input
                    type="text"
                    value={form.usernameClaim}
                    onInput={(event) => setForm('usernameClaim', event.currentTarget.value)}
                    class={controlClass()}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>
                    Claim used to populate the Pulse username (default: preferred_username).
                  </p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Email claim</label>
                  <input
                    type="text"
                    value={form.emailClaim}
                    onInput={(event) => setForm('emailClaim', event.currentTarget.value)}
                    class={controlClass()}
                    disabled={isEnvLocked() || saving()}
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Groups claim</label>
                  <input
                    type="text"
                    value={form.groupsClaim}
                    onInput={(event) => setForm('groupsClaim', event.currentTarget.value)}
                    class={controlClass()}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>
                    Optional claim that lists group memberships. Used for group restrictions.
                  </p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Allowed groups</label>
                  <textarea
                    rows={2}
                    value={form.allowedGroups}
                    onInput={(event) => setForm('allowedGroups', event.currentTarget.value)}
                    placeholder="admin, sso-admins"
                    class={controlClass('min-h-[70px]')}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>
                    Comma or space separated values. Leave empty to allow any group.
                  </p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Allowed domains</label>
                  <textarea
                    rows={2}
                    value={form.allowedDomains}
                    onInput={(event) => setForm('allowedDomains', event.currentTarget.value)}
                    placeholder="example.com, partner.io"
                    class={controlClass('min-h-[70px]')}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>
                    Restrict access to email domains (without @). Leave empty to allow all.
                  </p>
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Allowed email addresses</label>
                  <textarea
                    rows={2}
                    value={form.allowedEmails}
                    onInput={(event) => setForm('allowedEmails', event.currentTarget.value)}
                    placeholder="admin@example.com"
                    class={controlClass('min-h-[70px]')}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>Optional allowlist of specific emails.</p>
                </div>
              </div>
            </Show>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3 pt-4">
            <Show when={config()?.defaultRedirect}>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                Redirect URL registered with your IdP must match Pulse:{' '}
                {config()?.defaultRedirect}
              </div>
            </Show>
            <div class="flex gap-3">
              <button
                type="button"
                class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                onClick={() => resetForm(config())}
                disabled={saving() || loading()}
              >
                Reset
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={saving() || loading() || isEnvLocked()}
              >
                {saving() ? 'Saving…' : 'Save changes'}
              </button>
            </div>
          </div>
        </Show>
      </form>
    </Card>
  );
};

export default OIDCPanel;
