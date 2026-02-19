import { Component, Show, Accessor } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { SecurityPostureSummary } from './SecurityPostureSummary';
import Shield from 'lucide-solid/icons/shield';
import Info from 'lucide-solid/icons/info';

interface SecurityStatusInfo {
  hasAuthentication: boolean;
  oidcEnabled?: boolean;
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
  return (
    <div class="space-y-6">
      {/* Loading State */}
      <Show when={props.securityStatusLoading()}>
        <Card
          padding="none"
          class="overflow-hidden border border-slate-200 dark:border-slate-700"
          border={false}
        >
          <div class="bg-slate-100 dark:bg-slate-800 px-6 py-5 animate-pulse">
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 bg-slate-300 dark:bg-slate-600 rounded-md"></div>
              <div class="flex-1 space-y-2">
                <div class="h-5 bg-slate-300 dark:bg-slate-600 rounded w-1/3"></div>
                <div class="h-4 bg-slate-300 dark:bg-slate-600 rounded w-1/2"></div>
              </div>
              <div class="text-right space-y-2">
                <div class="h-8 bg-slate-300 dark:bg-slate-600 rounded w-16 ml-auto"></div>
                <div class="h-4 bg-slate-300 dark:bg-slate-600 rounded w-12 ml-auto"></div>
              </div>
            </div>
          </div>
          <div class="p-6">
            <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              {[1, 2, 3, 4].map(() => (
                <div class="rounded-md border border-slate-200 dark:border-slate-700 p-4 animate-pulse">
                  <div class="h-4 bg-slate-200 dark:bg-slate-700 rounded w-2/3 mb-2"></div>
                  <div class="h-3 bg-slate-200 dark:bg-slate-700 rounded w-1/2"></div>
                </div>
              ))}
            </div>
          </div>
        </Card>
      </Show>

      {/* Security Summary */}
      <Show when={!props.securityStatusLoading() && props.securityStatus()}>
        <SecurityPostureSummary status={props.securityStatus()!} />
      </Show>

      {/* Proxy Auth Notice */}
      <Show when={!props.securityStatusLoading() && props.securityStatus()?.hasProxyAuth}>
        <Card
          padding="none"
          class="overflow-hidden border border-blue-200 dark:border-blue-800"
          border={false}
        >
          <div class="bg-blue-50 dark:bg-blue-900/20 px-6 py-4 border-b border-blue-200 dark:border-blue-700">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-blue-100 dark:bg-blue-900/50 rounded-md">
                <Shield class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
              </div>
              <SectionHeader
                title="Proxy Authentication Active"
                description="Requests are validated by an upstream proxy"
                size="sm"
                class="flex-1"
              />
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
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md border border-blue-300 dark:border-blue-600 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-200 hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                  </svg>
                  Proxy Logout
                </a>
              </Show>
              <a
                href="https://github.com/rcourtman/Pulse/blob/main/docs/PROXY_AUTH.md"
                target="_blank"
                rel="noreferrer"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md text-blue-600 dark:text-blue-300 hover:underline"
              >
                Read proxy auth guide →
              </a>
            </div>
          </div>
        </Card>
      </Show>

      {/* Security Tips Card */}
      <Show when={!props.securityStatusLoading() && props.securityStatus()}>
        <Card
          padding="md"
          class="border border-slate-200 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-800"
          border={false}
        >
          <div class="flex items-start gap-3">
            <div class="p-1.5 bg-slate-100 dark:bg-slate-700 rounded-md flex-shrink-0">
              <Info class="w-4 h-4 text-slate-500 dark:text-slate-400" />
            </div>
            <div class="text-xs text-slate-600 dark:text-slate-400">
              <p class="font-medium text-slate-700 dark:text-slate-300 mb-1">Security Best Practices</p>
              <ul class="space-y-0.5 list-disc list-inside">
                <li>Enable HTTPS via a reverse proxy for encrypted connections</li>
                <li>Use strong, unique passwords and rotate credentials regularly</li>
                <li>Consider SSO/OIDC for centralized team authentication</li>
                <li>Review API token scopes and remove unused tokens</li>
              </ul>
            </div>
          </div>
        </Card>
      </Show>
    </div>
  );
};
