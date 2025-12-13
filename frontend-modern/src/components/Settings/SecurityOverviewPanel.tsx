import { Component, Show, Accessor } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SecurityPostureSummary } from './SecurityPostureSummary';

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
      <Show when={!props.securityStatusLoading() && props.securityStatus()}>
        <SecurityPostureSummary status={props.securityStatus()!} />
      </Show>

      <Show when={!props.securityStatusLoading() && props.securityStatus()?.hasProxyAuth}>
        <Card
          padding="sm"
          class="border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20"
        >
          <div class="flex flex-col gap-2 text-xs text-blue-800 dark:text-blue-200">
            <div class="flex items-center gap-2">
              <svg
                class="w-4 h-4 flex-shrink-0"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <span class="font-semibold text-blue-900 dark:text-blue-100">
                Proxy authentication detected
              </span>
            </div>
            <p>
              Requests are validated by an upstream proxy. The current proxied user is
              {props.securityStatus()?.proxyAuthUsername
                ? ` ${props.securityStatus()?.proxyAuthUsername}`
                : ' available once a request is received'}
              .
              {props.securityStatus()?.proxyAuthIsAdmin ? ' Admin privileges confirmed.' : ''}
              <Show when={props.securityStatus()?.proxyAuthLogoutURL}>
                {' '}
                <a class="underline font-medium" href={props.securityStatus()?.proxyAuthLogoutURL}>
                  Proxy logout
                </a>
              </Show>
            </p>
            <p>
              Need configuration tips? Review the proxy auth guide in the docs.{' '}
              <a
                class="underline font-medium"
                href="https://github.com/rcourtman/Pulse/blob/main/docs/PROXY_AUTH.md"
                target="_blank"
                rel="noreferrer"
              >
                Read proxy auth guide →
              </a>
            </p>
          </div>
        </Card>
      </Show>
    </div>
  );
};
