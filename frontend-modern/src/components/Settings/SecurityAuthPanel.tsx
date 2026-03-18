import { Component, Show, Accessor, Setter } from 'solid-js';
import { CalloutCard } from '@/components/shared/CalloutCard';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import {
  getSecurityAuthRestartInstruction,
  SECURITY_AUTH_DISABLED_MESSAGE,
  SECURITY_AUTH_DISABLED_PANEL_TITLE,
  SECURITY_AUTH_DISABLED_READ_ONLY_MESSAGE,
  SECURITY_AUTH_RESTART_FOOTER,
  SECURITY_AUTH_RESTART_REQUIRED_MESSAGE,
  SECURITY_AUTH_RESTART_REQUIRED_TITLE,
  SECURITY_AUTH_RESTART_TIP,
  SECURITY_AUTH_SETTINGS_READ_ONLY_MESSAGE,
  SECURITY_AUTH_SETUP_LABEL,
} from '@/utils/securityAuthPresentation';
import { QuickSecuritySetup } from './QuickSecuritySetup';
import Lock from 'lucide-solid/icons/lock';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
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
}

interface SecurityAuthPanelProps {
  securityStatus: Accessor<SecurityStatusInfo | null>;
  securityStatusLoading: Accessor<boolean>;
  versionInfo: Accessor<VersionInfo | null>;
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
  canManage: boolean;
}

export const SecurityAuthPanel: Component<SecurityAuthPanelProps> = (props) => {
  const restartInstruction = () =>
    getSecurityAuthRestartInstruction(props.versionInfo()?.deploymentType);
  const showAuthenticationControls = () =>
    !props.securityStatusLoading() &&
    (props.securityStatus()?.hasAuthentication || props.securityStatus()?.apiTokenConfigured);

  return (
    <SettingsPanel
      title="Authentication"
      description="Manage password-based authentication, login visibility, and credential rotation."
      icon={<Lock class="w-5 h-5" strokeWidth={2} />}
      noPadding={showAuthenticationControls()}
      bodyClass={showAuthenticationControls() ? 'divide-y divide-border' : 'space-y-6'}
    >
      <Show when={!props.securityStatus()?.hasAuthentication}>
        <CalloutCard
          tone="warning"
          title={SECURITY_AUTH_DISABLED_PANEL_TITLE}
          description={SECURITY_AUTH_DISABLED_MESSAGE}
          icon={<AlertTriangle class="h-5 w-5" />}
          class="space-y-4"
        >
          <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
            <Show when={!props.canManage}>
              <p class="text-xs text-amber-700 dark:text-amber-300">
                {SECURITY_AUTH_DISABLED_READ_ONLY_MESSAGE}
              </p>
            </Show>
            <button
              type="button"
              onClick={() => props.setShowQuickSecuritySetup(!props.showQuickSecuritySetup())}
              disabled={!props.canManage}
              class="w-full sm:w-auto px-3 py-2 text-xs font-medium rounded-md border border-amber-300 text-amber-800 bg-amber-100 hover:bg-amber-200 transition-colors dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900 dark:hover:bg-amber-800"
            >
              {SECURITY_AUTH_SETUP_LABEL}
            </button>
          </div>

          <Show when={props.canManage && props.showQuickSecuritySetup()}>
            <div class="border-t border-amber-200 pt-4 dark:border-amber-700">
              <QuickSecuritySetup
                onConfigured={() => {
                  props.setShowQuickSecuritySetup(false);
                  props.loadSecurityStatus();
                }}
              />
            </div>
          </Show>
        </CalloutCard>
      </Show>

      <Show when={showAuthenticationControls()}>
        <div>
          <div class="p-4 sm:p-6 flex flex-col gap-3 sm:gap-4">
            <Show when={!props.canManage}>
              <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
                {SECURITY_AUTH_SETTINGS_READ_ONLY_MESSAGE}
              </div>
            </Show>
            <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <button
                type="button"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  props.setShowPasswordModal(true);
                }}
                disabled={!props.canManage}
                class="w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
              >
                Change password
              </button>
              <button
                type="button"
                onClick={() => props.setShowQuickSecurityWizard(!props.showQuickSecurityWizard())}
                disabled={!props.canManage}
                class="w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors"
              >
                Rotate credentials
              </button>
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
              disabled={
                !props.canManage || props.hideLocalLoginLocked() || props.savingHideLocalLogin()
              }
              locked={props.hideLocalLoginLocked()}
              lockedMessage="This setting is managed by the PULSE_AUTH_HIDE_LOCAL_LOGIN environment variable"
            />
          </div>

          <Show when={props.canManage && props.showQuickSecurityWizard()}>
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
        </div>
      </Show>

      <Show when={props.securityStatus()?.configuredButPendingRestart}>
        <CalloutCard
          tone="warning"
          title={SECURITY_AUTH_RESTART_REQUIRED_TITLE}
          description={
            <>
              <p>{SECURITY_AUTH_RESTART_REQUIRED_MESSAGE}</p>
              <p class="mt-2">{SECURITY_AUTH_RESTART_FOOTER}</p>
            </>
          }
          icon={<AlertTriangle class="h-5 w-5" />}
          class="space-y-4"
        >
          <div class="rounded-md border border-amber-200 bg-surface p-3 dark:border-amber-700">
            <p class="text-xs font-semibold text-base-content mb-2">How to restart Pulse:</p>

            <Show when={props.versionInfo()?.deploymentType === 'proxmoxve'}>
              <div class="space-y-2">
                <p class="text-xs text-base-content">
                  Type <code class="px-1 py-0.5 bg-surface-hover rounded">update</code> in your
                  ProxmoxVE console
                </p>
                <p class="text-xs text-muted italic">
                  {restartInstruction().secondaryLabel}{' '}
                  <code class="text-xs">{restartInstruction().command}</code>
                </p>
              </div>
            </Show>

            <Show when={props.versionInfo()?.deploymentType === 'docker'}>
              <div class="space-y-1">
                <p class="text-xs text-base-content">{restartInstruction().label}</p>
                <code class="block text-xs bg-surface-hover p-2 rounded mt-1">
                  {restartInstruction().command}
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
                <p class="text-xs text-base-content">{restartInstruction().label}</p>
                <code class="block text-xs bg-surface-hover p-2 rounded mt-1">
                  {restartInstruction().command}
                </code>
              </div>
            </Show>

            <Show when={props.versionInfo()?.deploymentType === 'development'}>
              <div class="space-y-1">
                <p class="text-xs text-base-content">{restartInstruction().label}</p>
                <code class="block text-xs bg-surface-hover p-2 rounded mt-1">
                  {restartInstruction().command}
                </code>
              </div>
            </Show>

            <Show when={!props.versionInfo()?.deploymentType}>
              <div class="space-y-1">
                <p class="text-xs text-base-content">{restartInstruction().label}</p>
              </div>
            </Show>
          </div>

          <div class="rounded border border-green-200 bg-green-50 p-2 dark:border-green-800 dark:bg-green-900">
            <p class="text-xs text-green-700 dark:text-green-300">
              <strong>Tip:</strong> {SECURITY_AUTH_RESTART_TIP.replace(/^Tip:\s*/, '')}
            </p>
          </div>
        </CalloutCard>
      </Show>
    </SettingsPanel>
  );
};
