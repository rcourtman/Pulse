import { Component, Show, Accessor, For, createMemo } from 'solid-js';
import { CalloutCard } from '@/components/shared/CalloutCard';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { PROXY_AUTH_DOC_URL, SECURITY_DOC_URL } from '@/utils/docsLinks';
import { SecurityPostureSummary } from './SecurityPostureSummary';
import { settingsTabPath } from './settingsNavigationModel';
import Shield from 'lucide-solid/icons/shield';
import Info from 'lucide-solid/icons/info';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import {
  getSecurityHardeningActions,
  type SecurityHardeningAction,
} from '@/utils/securityScorePresentation';

interface SecurityStatusInfo {
  hasAuthentication: boolean;
  ssoEnabled?: boolean;
  hasProxyAuth?: boolean;
  apiTokenConfigured: boolean;
  exportProtected: boolean;
  unprotectedExportAllowed?: boolean;
  hasHTTPS?: boolean;
  hasAuditLogging: boolean;
  requiresAuth: boolean;
  publicAccess?: boolean;
  isPrivateNetwork?: boolean;
  clientIP?: string;
  proxyAuthUsername?: string;
  proxyAuthIsAdmin?: boolean;
  proxyAuthLogoutURL?: string;
}

interface SecurityOverviewPanelProps {
  securityStatus: Accessor<SecurityStatusInfo | null>;
  securityStatusLoading: Accessor<boolean>;
}

export const SecurityOverviewPanel: Component<SecurityOverviewPanelProps> = (props) => {
  const hardeningActions = createMemo(() =>
    props.securityStatus() ? getSecurityHardeningActions(props.securityStatus()!) : [],
  );
  const criticalHardeningActions = createMemo(
    () => hardeningActions().filter((action) => action.severity === 'critical').length,
  );
  const recommendedHardeningActions = createMemo(
    () => hardeningActions().filter((action) => action.severity === 'recommended').length,
  );
  const hardeningTone = createMemo(() =>
    criticalHardeningActions() > 0 ? 'danger' : 'info',
  );
  const hardeningTitle = createMemo(() =>
    criticalHardeningActions() > 0 ? 'Hardening priorities' : 'Recommended hardening steps',
  );
  const hardeningDescription = createMemo(() => {
    if (criticalHardeningActions() > 0) {
      return `Resolve the ${criticalHardeningActions() === 1 ? 'critical exposure' : `${criticalHardeningActions()} critical exposures`} below before using this Pulse instance for live infrastructure.`;
    }

    return 'Authentication is enabled and this instance appears private, but it still needs a few production hardening steps.';
  });
  const actionLinkFor = (action: SecurityHardeningAction) => {
    switch (action.key) {
      case 'enable-authentication':
        return {
          href: settingsTabPath('security-auth'),
          label: 'Open Authentication',
          external: false,
        } as const;
      case 'protect-exports':
      case 'create-api-token':
        return {
          href: settingsTabPath('api'),
          label: 'Open API Access',
          external: false,
        } as const;
      case 'configure-https':
        return {
          href: SECURITY_DOC_URL,
          label: 'Open security guide',
          external: false,
        } as const;
    }
  };

  return (
    <SettingsPanel
      title="Security Overview"
      description="Review your security posture, authentication boundary, and the next hardening steps for this Pulse instance."
      icon={<Shield class="w-5 h-5" strokeWidth={2} />}
      bodyClass="space-y-6"
    >
      <Show when={props.securityStatusLoading()}>
        <div class="rounded-md border border-border overflow-hidden">
          <div class="bg-surface-alt px-6 py-5 animate-pulse">
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 bg-slate-300 rounded-md"></div>
              <div class="flex-1 space-y-2">
                <div class="h-5 bg-slate-300 rounded w-1/3"></div>
                <div class="h-4 bg-slate-300 rounded w-1/2"></div>
              </div>
              <div class="text-right space-y-2">
                <div class="h-8 bg-slate-300 rounded w-16 ml-auto"></div>
                <div class="h-4 bg-slate-300 rounded w-12 ml-auto"></div>
              </div>
            </div>
          </div>
          <div class="p-6">
            <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              {[1, 2, 3, 4].map(() => (
                <div class="rounded-md border border-border p-4 animate-pulse">
                  <div class="h-4 bg-surface-hover rounded w-2/3 mb-2"></div>
                  <div class="h-3 bg-surface-hover rounded w-1/2"></div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </Show>

      <Show when={!props.securityStatusLoading() && props.securityStatus()}>
        <SecurityPostureSummary status={props.securityStatus()!} embedded />
      </Show>

      <Show when={!props.securityStatusLoading() && hardeningActions().length > 0}>
        <CalloutCard
          tone={hardeningTone()}
          title={hardeningTitle()}
          description={hardeningDescription()}
          icon={
            criticalHardeningActions() > 0 ? (
              <AlertTriangle class="h-5 w-5" />
            ) : (
              <Info class="h-5 w-5" />
            )
          }
          class="space-y-4"
        >
          <div class="grid gap-3 lg:grid-cols-2">
            <For each={hardeningActions()}>
              {(action) => {
                const actionLink = actionLinkFor(action);
                return (
                  <div class="rounded-md border border-border bg-surface px-4 py-3">
                    <div class="flex items-start justify-between gap-3">
                      <div class="min-w-0 flex-1">
                        <div class="flex flex-wrap items-center gap-2">
                          <h3 class="text-sm font-semibold text-base-content">{action.title}</h3>
                          <span
                            class={`inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${
                              action.severity === 'critical'
                                ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                : 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                            }`}
                          >
                            {action.severity === 'critical' ? 'Act now' : 'Recommended'}
                          </span>
                        </div>
                        <p class="mt-2 text-sm text-muted">{action.description}</p>
                      </div>
                    </div>
                    <div class="mt-3 flex flex-wrap items-center gap-3">
                      <a
                        href={actionLink.href}
                        target={actionLink.external ? '_blank' : undefined}
                        rel={actionLink.external ? 'noreferrer' : undefined}
                        class="inline-flex min-h-10 items-center rounded-md border border-border bg-surface-alt px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                      >
                        {actionLink.label}
                      </a>
                    </div>
                  </div>
                );
              }}
            </For>
          </div>
          <Show when={recommendedHardeningActions() > 0}>
            <p class="text-xs text-muted">
              Recommended items improve production readiness even when the current runtime is only
              being used locally.
            </p>
          </Show>
        </CalloutCard>
      </Show>

      <Show when={!props.securityStatusLoading() && props.securityStatus()?.hasProxyAuth}>
        <div class="rounded-md border border-blue-200 dark:border-blue-800 overflow-hidden bg-blue-50/60 dark:bg-blue-950/40">
          <div class="bg-blue-50 dark:bg-blue-900 px-6 py-4 border-b border-blue-200 dark:border-blue-700">
            <div class="flex items-start gap-3">
              <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-md">
                <Shield class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-sm font-semibold text-blue-900 dark:text-blue-100">
                  Proxy Authentication Active
                </p>
                <p class="text-sm text-blue-700 dark:text-blue-300">
                  Requests are validated by an upstream proxy before Pulse applies its local authorization rules.
                </p>
              </div>
            </div>
          </div>
          <div class="p-4 text-sm text-blue-800 dark:text-blue-200 space-y-2">
            <p>
              The current proxied user is
              <span class="font-semibold">
                {props.securityStatus()?.proxyAuthUsername
                  ? ` ${props.securityStatus()?.proxyAuthUsername}`
                  : ' available once a request is received'}
              </span>
              .
              {props.securityStatus()?.proxyAuthIsAdmin && (
                <span class="ml-1 inline-flex items-center px-2 py-0.5 text-xs font-medium rounded-full bg-blue-100 text-blue-700 dark:bg-blue-800 dark:text-blue-200">
                  Admin
                </span>
              )}
            </p>
            <div class="flex flex-wrap items-center gap-3 pt-2">
              <Show when={props.securityStatus()?.proxyAuthLogoutURL}>
                <a
                  href={props.securityStatus()?.proxyAuthLogoutURL}
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md border border-blue-300 dark:border-blue-600 bg-blue-50 dark:bg-blue-900 text-blue-700 dark:text-blue-200 hover:bg-blue-100 dark:hover:bg-blue-800 transition-colors"
                >
                  <svg
                    class="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                    />
                  </svg>
                  Proxy Logout
                </a>
              </Show>
              <a
                href={PROXY_AUTH_DOC_URL}
                target="_blank"
                rel="noreferrer"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md text-blue-600 dark:text-blue-300 hover:underline"
              >
                Read proxy auth guide →
              </a>
            </div>
          </div>
        </div>
      </Show>

      <Show when={!props.securityStatusLoading() && props.securityStatus()}>
        <div class="rounded-md border border-border p-4 sm:p-6 space-y-4">
          <div class="flex items-start gap-3">
            <div class="p-2 rounded-md border border-border bg-surface-alt">
              <Info class="w-5 h-5" strokeWidth={2} />
            </div>
            <div class="min-w-0 flex-1">
              <h3 class="text-sm font-semibold text-base-content">Security best practices</h3>
              <p class="text-sm text-muted">
                Recommended hardening actions for production deployments.
              </p>
            </div>
          </div>
          <ul class="space-y-1.5 list-disc list-inside text-sm text-muted">
            <li>Enable HTTPS via a reverse proxy for encrypted connections</li>
            <li>Use strong, unique passwords and rotate credentials regularly</li>
            <li>Consider SSO/OIDC for centralized team authentication</li>
            <li>Review API token scopes and remove unused tokens</li>
          </ul>
        </div>
      </Show>
    </SettingsPanel>
  );
};
