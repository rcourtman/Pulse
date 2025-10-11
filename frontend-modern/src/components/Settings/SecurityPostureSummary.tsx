import { Component, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

interface SecurityPostureSummaryProps {
  status: {
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
  };
}

export const SecurityPostureSummary: Component<SecurityPostureSummaryProps> = (props) => {
  const items = () => [
    {
      key: 'password',
      label: 'Password login',
      enabled: props.status.hasAuthentication,
      description: props.status.hasAuthentication ? 'Active' : 'Not configured',
    },
    {
      key: 'oidc',
      label: 'Single sign-on',
      enabled: Boolean(props.status.oidcEnabled),
      description: props.status.oidcEnabled ? 'OIDC configured' : 'Not configured',
    },
    {
      key: 'proxy',
      label: 'Proxy auth',
      enabled: Boolean(props.status.hasProxyAuth),
      description: props.status.hasProxyAuth ? 'Active' : 'Not configured',
    },
    {
      key: 'token',
      label: 'API token',
      enabled: props.status.apiTokenConfigured,
      description: props.status.apiTokenConfigured ? 'Active' : 'Not configured',
    },
    {
      key: 'export',
      label: 'Export protection',
      enabled: props.status.exportProtected && !props.status.unprotectedExportAllowed,
      description: props.status.unprotectedExportAllowed
        ? 'Unprotected'
        : 'Token + passphrase required',
    },
    {
      key: 'https',
      label: 'HTTPS',
      enabled: Boolean(props.status.hasHTTPS),
      description: props.status.hasHTTPS ? 'Encrypted' : 'HTTP only',
    },
    {
      key: 'audit',
      label: 'Audit log',
      enabled: props.status.hasAuditLogging,
      description: props.status.hasAuditLogging ? 'Active' : 'Not enabled',
    },
  ];

  const badgeClasses = (enabled: boolean) =>
    enabled
      ? 'inline-flex items-center gap-1 px-2.5 py-1 text-xs font-semibold rounded-full bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
      : 'inline-flex items-center gap-1 px-2.5 py-1 text-xs font-semibold rounded-full bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-200';

  return (
    <Card padding="md" class="border border-gray-200 dark:border-gray-700">
      <div class="flex flex-col gap-4">
        <div class="flex flex-col md:flex-row md:items-start md:justify-between gap-3">
          <SectionHeader title="Security posture" size="sm" class="flex-1" />
          <div class="flex items-center gap-2">
            <span
              class={`${
                props.status.publicAccess && !props.status.isPrivateNetwork
                  ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
                  : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
              } px-3 py-1 text-xs font-semibold rounded-full`}
            >
              {props.status.publicAccess && !props.status.isPrivateNetwork
                ? 'Public network access'
                : 'Private network access'}
            </span>
            <Show when={props.status.clientIP}>
              <span class="hidden md:inline-flex items-center px-3 py-1 text-xs font-medium rounded-full bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                IP: {props.status.clientIP}
              </span>
            </Show>
          </div>
        </div>


        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          <For each={items()}>
            {(item) => (
              <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3 bg-white dark:bg-gray-800">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                    {item.label}
                  </span>
                  <span class={badgeClasses(item.enabled)}>
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d={item.enabled ? 'M5 13l4 4L19 7' : 'M6 18L18 6M6 6l12 12'}
                      />
                    </svg>
                    {item.enabled ? 'On' : 'Off'}
                  </span>
                </div>
                <p class="text-xs text-gray-600 dark:text-gray-400 leading-relaxed">
                  {item.description}
                </p>
              </div>
            )}
          </For>
        </div>
      </div>
    </Card>
  );
};
