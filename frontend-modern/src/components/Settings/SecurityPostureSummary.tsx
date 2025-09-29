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
      label: 'Password authentication',
      enabled: props.status.hasAuthentication,
      description: props.status.hasAuthentication
        ? 'Login required for the UI.'
        : 'Disabled or not configured.',
    },
    {
      key: 'oidc',
      label: 'Single sign-on (OIDC)',
      enabled: Boolean(props.status.oidcEnabled),
      description: props.status.oidcEnabled
        ? 'OIDC login available.'
        : 'Add your identity provider to enable it.',
    },
    {
      key: 'proxy',
      label: 'Proxy authentication',
      enabled: Boolean(props.status.hasProxyAuth),
      description: props.status.hasProxyAuth
        ? 'Requests validated by upstream proxy.'
        : 'Optional reverse-proxy auth.',
    },
    {
      key: 'token',
      label: 'API token',
      enabled: props.status.apiTokenConfigured,
      description: props.status.apiTokenConfigured
        ? 'Automation available via token.'
        : 'Generate a token for scripts.',
    },
    {
      key: 'export',
      label: 'Export protection',
      enabled: props.status.exportProtected && !props.status.unprotectedExportAllowed,
      description: props.status.unprotectedExportAllowed
        ? 'Exports can bypass token checks.'
        : 'Exports require token + passphrase.',
    },
    {
      key: 'https',
      label: 'HTTPS',
      enabled: Boolean(props.status.hasHTTPS),
      description: props.status.hasHTTPS
        ? 'Connection is encrypted.'
        : 'Serving over HTTP.',
    },
    {
      key: 'audit',
      label: 'Audit logging',
      enabled: props.status.hasAuditLogging,
      description: props.status.hasAuditLogging
        ? 'Auth events logged for review.'
        : 'Enable PULSE_AUDIT_LOG for trails.',
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
          <SectionHeader
            title="Security posture"
            description="Snapshot of authentication and hardening features"
            size="sm"
            class="flex-1"
          />
          <div class="flex items-center gap-2">
            <span
              class={`${
                props.status.publicAccess && !props.status.isPrivateNetwork
                  ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
                  : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
              } px-3 py-1 text-xs font-semibold rounded-full`}
            >
              {props.status.publicAccess && !props.status.isPrivateNetwork ? 'Public network access' : 'Private network access'}
            </span>
            <Show when={props.status.clientIP}>
              <span class="hidden md:inline-flex items-center px-3 py-1 text-xs font-medium rounded-full bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                IP: {props.status.clientIP}
              </span>
            </Show>
          </div>
        </div>

        <Show when={props.status.requiresAuth}>
          <div class="flex items-start gap-2 p-3 rounded-lg bg-green-50 text-xs text-green-700 dark:bg-green-900/30 dark:text-green-300 border border-green-200 dark:border-green-800">
            <svg class="w-4 h-4 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            <span>
              Authentication is required for this instance. Keep at least one trusted login path enabled before disabling password auth.
            </span>
          </div>
        </Show>

        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          <For each={items()}>
            {(item) => (
              <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3 bg-white dark:bg-gray-800">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">{item.label}</span>
                  <span class={badgeClasses(item.enabled)}>
                    <svg
                      class="w-3.5 h-3.5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
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
                <p class="text-xs text-gray-600 dark:text-gray-400 leading-relaxed">{item.description}</p>
              </div>
            )}
          </For>
        </div>
      </div>
    </Card>
  );
};
