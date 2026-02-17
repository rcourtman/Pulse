import { Component, Show, createEffect, createSignal, onMount } from 'solid-js';
import { createStore } from 'solid-js/store';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { getUpgradeActionUrlOrFallback, hasFeature, loadLicenseStatus, licenseLoaded } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import Globe from 'lucide-solid/icons/globe';

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
  groupRoleMappings?: Record<string, string>;
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
    groupRoleMappings: '',
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
        groupRoleMappings: '',
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
      groupRoleMappings: mappingsToString(data.groupRoleMappings),
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

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible = licenseLoaded() && !hasFeature('sso') && !loading();
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('sso', 'settings_oidc_panel');
    }
    return isPaywallVisible;
  }, false);

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
        groupRoleMappings: stringToMappings(form.groupRoleMappings),
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
    <SettingsPanel
      title="Single Sign-On (OIDC)"
      description="Connect Pulse to your identity provider."
      icon={<Globe class="w-5 h-5" />}
      action={
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
      }
      bodyClass="space-y-5"
    >
      <Show when={licenseLoaded() && !hasFeature('sso') && !loading()}>
        <div class="p-5 bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700 rounded-xl">
          <div class="flex flex-col sm:flex-row items-center gap-4">
            <div class="flex-1">
              <h4 class="text-base font-semibold text-gray-900 dark:text-white">Single Sign-On</h4>
              <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                Connect Pulse to your identity provider for seamless team authentication.
              </p>
            </div>
            <a
              href={getUpgradeActionUrlOrFallback('advanced_sso')}
              target="_blank"
              rel="noopener noreferrer"
              class="px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
              onClick={() => trackUpgradeClicked('settings_oidc_panel', 'sso')}
            >
              Upgrade to Pro
            </a>
          </div>
        </div>
      </Show>
      <form class="space-y-5" onSubmit={handleSave}>
        <div class="bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700 rounded-lg p-3 text-xs text-gray-700 dark:text-gray-300">
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
                    class="text-xs text-gray-600 hover:underline dark:text-gray-300"
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
              class="text-xs font-semibold text-gray-700 hover:underline dark:text-gray-300"
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
                <div class={formField}>
                  <label class={labelClass()}>Group role mappings</label>
                  <textarea
                    rows={2}
                    value={form.groupRoleMappings}
                    onInput={(event) => setForm('groupRoleMappings', event.currentTarget.value)}
                    placeholder="admins=admin, operators=operator"
                    class={controlClass('min-h-[70px]')}
                    disabled={isEnvLocked() || saving()}
                  />
                  <p class={formHelpText}>
                    Comma or space separated <code>group=roleId</code> pairs.
                  </p>
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
    </SettingsPanel>
  );
};

export default OIDCPanel;
