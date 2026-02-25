import { Component, Show, Accessor, Setter } from 'solid-js';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { QuickSecuritySetup } from './QuickSecuritySetup';
import Lock from 'lucide-solid/icons/lock';
import type { VersionInfo } from '@/api/updates';

interface SecurityStatusInfo {
  hasAuthentication: boolean;
  apiTokenConfigured: boolean;
  authUsername?: string;
  configuredButPendingRestart?: boolean;
  hasProxyAuth?: boolean;
  proxyAuthUsername?: string;
  proxyAuthIsAdmin?: boolean;
  proxyAuthLogoutURL?: string;
  deprecatedDisableAuth?: boolean;
}

interface SecurityAuthPanelProps {
  securityStatus: Accessor<SecurityStatusInfo | null>;
  securityStatusLoading: Accessor<boolean>;
  versionInfo: Accessor<VersionInfo | null>;
  authDisabledByEnv: Accessor<boolean>;
  showQuickSecuritySetup: Accessor<boolean>;
  setShowQuickSecuritySetup: Setter<boolean>;
  showQuickSecurityWizard: Accessor<boolean>;
  setShowQuickSecurityWizard: Setter<boolean>;
  showPasswordModal: Accessor<boolean>;
  setShowPasswordModal: Setter<boolean>;
  hideLocalLogin: Accessor<boolean>;
  hideLocalLoginLocked: () => boolean;
  savingHideLocalLogin: Accessor<boolean>;
  handleHideLocalLoginChange: (enabled: boolean) => Promise<void>;
  loadSecurityStatus: () => Promise<void>;
}

export const SecurityAuthPanel: Component<SecurityAuthPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      {/* Show message when auth is disabled */}
      <Show when={!props.securityStatus()?.hasAuthentication}>
        <Card
          padding="none"
          class="overflow-hidden border border-amber-200 dark:border-amber-800"
          border={false}
        >
          {/* Header */}
          <div class="bg-amber-50 dark:bg-amber-900 px-6 py-4 border-b border-amber-200 dark:border-amber-700">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-amber-100 dark:bg-amber-900 rounded-md">
                <svg
                  class="w-5 h-5 text-amber-600 dark:text-amber-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <SectionHeader title="Authentication disabled" size="sm" class="flex-1" />
              <Show
                when={!props.authDisabledByEnv()}
                fallback={
                  <span class="px-3 py-1.5 text-xs font-semibold rounded-md border border-amber-300 text-amber-800 bg-amber-100 dark:border-amber-700 dark:text-amber-100 dark:bg-amber-900 whitespace-nowrap">
                    Controlled by DISABLE_AUTH
                  </span>
                }
              >
                <button
                  type="button"
                  onClick={() => props.setShowQuickSecuritySetup(!props.showQuickSecuritySetup())}
                  class="px-3 py-1.5 text-xs font-medium rounded-md border border-amber-300 text-amber-800 bg-amber-100 hover:bg-amber-200 transition-colors dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900 dark:hover:bg-amber-800 whitespace-nowrap"
                >
                  Setup
                </button>
              </Show>
            </div>
          </div>

          {/* Content */}
          <div class="p-6">
            <p class="text-sm text-amber-700 dark:text-amber-300 mb-4">
              <Show
                when={props.authDisabledByEnv()}
                fallback={
                  <>
                    Authentication is currently disabled. Set up password authentication to protect
                    your Pulse instance.
                  </>
                }
              >
                Authentication settings are locked by the legacy{' '}
                <code class="font-mono text-xs text-amber-800 dark:text-amber-200">
                  DISABLE_AUTH
                </code>{' '}
                environment variable. Remove it from your deployment and restart Pulse before
                enabling security from this page.
              </Show>
            </p>

            <Show when={props.showQuickSecuritySetup() && !props.authDisabledByEnv()}>
              <QuickSecuritySetup
                onConfigured={() => {
                  props.setShowQuickSecuritySetup(false);
                  props.loadSecurityStatus();
                }}
              />
            </Show>
          </div>
        </Card>
      </Show>

      {/* Authentication */}
      <Show
        when={
          !props.securityStatusLoading() &&
          (props.securityStatus()?.hasAuthentication || props.securityStatus()?.apiTokenConfigured)
        }
      >
        <SettingsPanel
          title="Authentication"
          description="Manage password authentication and credential rotation."
          icon={<Lock class="w-5 h-5" strokeWidth={2} />}
          noPadding
          bodyClass="divide-y divide-border"
        >
          {/* Content */}
          <div class="p-4 sm:p-6 flex flex-col gap-3 sm:gap-4">
            <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <button
                type="button"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  props.setShowPasswordModal(true);
                }}
                class="w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
              >
                Change password
              </button>
              <Show
                when={!props.authDisabledByEnv()}
                fallback={
                  <span class="w-full sm:w-auto min-h-10 sm:min-h-10 inline-flex items-center justify-center px-4 py-2.5 text-sm font-semibold border border-amber-300 text-amber-800 bg-amber-50 dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900 rounded-md">
                    Remove DISABLE_AUTH to rotate credentials
                  </span>
                }
              >
                <button
                  type="button"
                  onClick={() => props.setShowQuickSecurityWizard(!props.showQuickSecurityWizard())}
                  class="w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors"
                >
                  Rotate credentials
                </button>
              </Show>
            </div>
            <div class="text-xs text-muted">
              <span class="font-medium text-base-content">User:</span>{' '}
              {props.securityStatus()?.authUsername || 'Not configured'}
            </div>
          </div>

          <div class="p-4 sm:p-6">
            <Toggle
              label="Hide local login form"
              description="Hide the username/password form on the login page. Users will only see SSO options unless ?show_local=true is used."
              checked={props.hideLocalLogin()}
              onChange={(e: ToggleChangeEvent) =>
                props.handleHideLocalLoginChange(e.currentTarget.checked)
              }
              disabled={props.hideLocalLoginLocked() || props.savingHideLocalLogin()}
              locked={props.hideLocalLoginLocked()}
              lockedMessage="This setting is managed by the PULSE_AUTH_HIDE_LOCAL_LOGIN environment variable"
            />
          </div>

          <Show when={!props.authDisabledByEnv() && props.showQuickSecurityWizard()}>
            <div class="p-4 sm:p-6">
              <QuickSecuritySetup
                mode="rotate"
                defaultUsername={props.securityStatus()?.authUsername || 'admin'}
                onConfigured={() => {
                  props.setShowQuickSecurityWizard(false);
                  props.loadSecurityStatus();
                }}
              />
            </div>
          </Show>
        </SettingsPanel>
      </Show>

      {/* Show pending restart message if configured but not loaded */}
      <Show when={props.securityStatus()?.configuredButPendingRestart}>
        <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-4">
          <div class="flex items-start space-x-3">
            <div class="flex-shrink-0">
              <svg
                class="h-6 w-6 text-amber-600 dark:text-amber-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <div class="flex-1">
              <h4 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                Security Configured - Restart Required
              </h4>
              <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                Security settings have been configured but the service needs to be restarted to
                activate them.
              </p>
              <p class="text-xs text-amber-600 dark:text-amber-400 mt-2">
                After restarting, you'll need to log in with your saved credentials.
              </p>

              <div class="mt-4 bg-surface rounded-md p-3 border border-amber-200 dark:border-amber-700">
                <p class="text-xs font-semibold text-base-content mb-2">How to restart Pulse:</p>

                <Show when={props.versionInfo()?.deploymentType === 'proxmoxve'}>
                  <div class="space-y-2">
                    <p class="text-xs text-base-content">
                      Type <code class="px-1 py-0.5 bg-surface-hover rounded">update</code> in your
                      ProxmoxVE console
                    </p>
                    <p class="text-xs text-muted italic">
                      Or restart manually with: <code class="text-xs">systemctl restart pulse</code>
                    </p>
                  </div>
                </Show>

                <Show when={props.versionInfo()?.deploymentType === 'docker'}>
                  <div class="space-y-1">
                    <p class="text-xs text-base-content">Restart your Docker container:</p>
                    <code class="block text-xs bg-surface-hover p-2 rounded mt-1">
                      docker restart pulse
                    </code>
                  </div>
                </Show>

                <Show
                  when={
                    props.versionInfo()?.deploymentType === 'systemd' ||
                    props.versionInfo()?.deploymentType === 'manual'
                  }
                >
                  <div class="space-y-1">
                    <p class="text-xs text-base-content">Restart the service:</p>
                    <code class="block text-xs bg-surface-hover p-2 rounded mt-1">
                      sudo systemctl restart pulse
                    </code>
                  </div>
                </Show>

                <Show when={props.versionInfo()?.deploymentType === 'development'}>
                  <div class="space-y-1">
                    <p class="text-xs text-base-content">Restart the development server:</p>
                    <code class="block text-xs bg-surface-hover p-2 rounded mt-1">
                      sudo systemctl restart pulse-hot-dev
                    </code>
                  </div>
                </Show>

                <Show when={!props.versionInfo()?.deploymentType}>
                  <div class="space-y-1">
                    <p class="text-xs text-base-content">
                      Restart Pulse using your deployment method
                    </p>
                  </div>
                </Show>
              </div>

              <div class="mt-3 p-2 bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded">
                <p class="text-xs text-green-700 dark:text-green-300">
                  <strong>Tip:</strong> Make sure you've saved your credentials before restarting!
                </p>
              </div>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
