import {
  Component,
  Show,
  Accessor,
  Setter,
  createEffect,
  createMemo,
  on,
  onCleanup,
} from 'solid-js';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { eventBus } from '@/stores/events';
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
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import type { VersionInfo } from '@/api/updates';

interface SecurityStatusInfo {
  hasAuthentication: boolean;
  currentUsername?: string;
  apiTokenHint?: string;
  tokenScopes?: string[];
  apiTokenConfigured?: boolean;
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
  // A pending read keeps the last admitted snapshot. Do not unmount the
  // credential form and silently replace a custom draft with generated values.
  const showAuthenticationControls = () =>
    props.securityStatus()?.hasAuthentication || props.securityStatus()?.apiTokenConfigured;
  const credentialOwner = createMemo(() =>
    JSON.stringify([
      props.securityStatus()?.currentUsername,
      props.securityStatus()?.apiTokenHint,
      props.securityStatus()?.tokenScopes?.slice().sort(),
      props.securityStatus()?.authUsername,
      props.canManage,
    ]),
  );
  const closeCredentialForms = () => {
    props.setShowQuickSecuritySetup(false);
    props.setShowQuickSecurityWizard(false);
  };
  // Preservation is local to the same reader, not a transfer to another user,
  // credential, permission set or organisation. A failed read also clears it.
  createEffect(on(credentialOwner, closeCredentialForms, { defer: true }));
  const unsubscribeOrg = eventBus.on('org_switched', closeCredentialForms);
  onCleanup(unsubscribeOrg);

  return (
    <SettingsPanel
      title="Authentication"
      action={
        <Show when={props.securityStatusLoading()}>
          <span role="status" aria-live="polite" class="flex items-center gap-2 text-xs text-muted">
            <LoadingSpinner size="sm" tone="muted" />
            {props.securityStatus()
              ? 'Refreshing authentication status…'
              : 'Checking authentication status…'}
          </span>
        </Show>
      }
      noPadding={showAuthenticationControls()}
      bodyClass={showAuthenticationControls() ? 'divide-y divide-border' : 'space-y-6'}
    >
      <Show when={!props.securityStatusLoading() && !props.securityStatus()}>
        <InlineNotice tone="warning" role="status">
          Authentication status is unavailable. Check settings access before making changes.
        </InlineNotice>
      </Show>

      <Show when={props.securityStatus()?.hasAuthentication === false}>
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
            <Button
              variant="warning"
              size="settingsActionXs"
              class="w-full sm:w-auto"
              onClick={() => props.setShowQuickSecuritySetup(!props.showQuickSecuritySetup())}
              disabled={!props.canManage || props.securityStatusLoading()}
            >
              {SECURITY_AUTH_SETUP_LABEL}
            </Button>
          </div>

          <Show when={props.canManage && props.showQuickSecuritySetup()}>
            <fieldset
              disabled={props.securityStatusLoading()}
              class="min-w-0 border-t border-amber-200 pt-4 dark:border-amber-700"
            >
              <QuickSecuritySetup
                onConfigured={() => {
                  props.setShowQuickSecuritySetup(false);
                  props.loadSecurityStatus();
                }}
              />
            </fieldset>
          </Show>
        </CalloutCard>
      </Show>

      <Show when={showAuthenticationControls()}>
        <fieldset disabled={props.securityStatusLoading()} class="min-w-0">
          <div class="p-4 sm:p-6 flex flex-col gap-3 sm:gap-4">
            <Show when={!props.canManage}>
              <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/25 dark:text-blue-200">
                {SECURITY_AUTH_SETTINGS_READ_ONLY_MESSAGE}
              </div>
            </Show>
            <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <Button
                variant="primary"
                size="settingsAction"
                class="w-full sm:w-auto"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  props.setShowPasswordModal(true);
                }}
                disabled={!props.canManage}
              >
                Change password
              </Button>
              <Button
                variant="secondary"
                size="settingsAction"
                class="w-full sm:w-auto"
                onClick={() => props.setShowQuickSecurityWizard(!props.showQuickSecurityWizard())}
                disabled={!props.canManage}
              >
                Rotate credentials
              </Button>
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
        </fieldset>
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

            <div class="space-y-1">
              <p class="text-xs text-base-content">{restartInstruction().label}</p>
              <Show when={restartInstruction().command}>
                {(command) => (
                  <code class="block text-xs bg-surface-hover p-2 rounded-sm mt-1">
                    {command()}
                  </code>
                )}
              </Show>
            </div>
          </div>

          <div class="rounded-sm border border-green-200 bg-green-50 p-2 dark:border-green-800 dark:bg-green-900/25">
            <p class="text-xs text-green-700 dark:text-green-300">
              <strong>Tip:</strong> {SECURITY_AUTH_RESTART_TIP.replace(/^Tip:\s*/, '')}
            </p>
          </div>
        </CalloutCard>
      </Show>
    </SettingsPanel>
  );
};
