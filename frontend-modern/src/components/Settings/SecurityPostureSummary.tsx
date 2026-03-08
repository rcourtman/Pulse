import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import Shield from 'lucide-solid/icons/shield';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import { getSecurityScorePresentation } from '@/utils/securityScorePresentation';

interface SecurityPostureSummaryProps {
  status: {
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
  };
}

export const SecurityPostureSummary: Component<SecurityPostureSummaryProps> = (props) => {
  const items = () => [
    {
      key: 'password',
      label: 'Password login',
      enabled: props.status.hasAuthentication,
      description: props.status.hasAuthentication ? 'Active' : 'Not configured',
      critical: true, // Critical security feature
    },
    {
      key: 'oidc',
      label: 'Single sign-on',
      enabled: Boolean(props.status.ssoEnabled),
      description: props.status.ssoEnabled ? 'Provider configured' : 'Not configured',
      critical: false,
    },
    {
      key: 'proxy',
      label: 'Proxy auth',
      enabled: Boolean(props.status.hasProxyAuth),
      description: props.status.hasProxyAuth ? 'Active' : 'Not configured',
      critical: false,
    },
    {
      key: 'token',
      label: 'API token',
      enabled: props.status.apiTokenConfigured,
      description: props.status.apiTokenConfigured ? 'Active' : 'Not configured',
      critical: false,
    },
    {
      key: 'export',
      label: 'Export protection',
      enabled: props.status.exportProtected && !props.status.unprotectedExportAllowed,
      description: props.status.unprotectedExportAllowed
        ? 'Unprotected'
        : 'Token + passphrase required',
      critical: true,
    },
    {
      key: 'https',
      label: 'HTTPS',
      enabled: Boolean(props.status.hasHTTPS),
      description: props.status.hasHTTPS ? 'Encrypted' : 'HTTP only',
      critical: true,
    },
    {
      key: 'audit',
      label: 'Audit log',
      enabled: props.status.hasAuditLogging,
      description: props.status.hasAuditLogging ? 'Active' : 'Not enabled',
      critical: false,
    },
  ];

  // Calculate security score
  const securityScore = createMemo(() => {
    const list = items();
    const criticalItems = list.filter((i) => i.critical);
    const enabledCritical = criticalItems.filter((i) => i.enabled).length;
    const allItems = list.filter((i) => i.enabled).length;

    // Weight critical items more heavily
    const criticalWeight = 0.7;
    const optionalWeight = 0.3;

    const criticalScore =
      criticalItems.length > 0 ? (enabledCritical / criticalItems.length) * criticalWeight : 0;
    const optionalScore = list.length > 0 ? (allItems / list.length) * optionalWeight : 0;

    return Math.round((criticalScore + optionalScore) * 100);
  });

  const scorePresentation = createMemo(() => getSecurityScorePresentation(securityScore()));

  const ScoreIcon = () => {
    switch (scorePresentation().icon) {
      case 'shield-check':
        return ShieldCheck;
      case 'shield':
        return Shield;
      default:
        return ShieldAlert;
    }
  };

  return (
    <Card padding="none" class="overflow-hidden border border-border" border={false}>
      {/* Header with Security Score */}
      <div
        class={`px-6 py-5 ${scorePresentation().tone.headerBg} ${scorePresentation().tone.headerBorder}`}
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <div class={`p-3 rounded-md ${scorePresentation().tone.iconWrap}`}>
              {(() => {
                const Icon = ScoreIcon();
                return <Icon class={`w-6 h-6 ${scorePresentation().tone.icon}`} />;
              })()}
            </div>
            <div>
              <h2 class="text-lg font-semibold text-base-content">Security Posture</h2>
              <p class={`text-sm ${scorePresentation().tone.subtitle}`}>
                {props.status.publicAccess && !props.status.isPrivateNetwork
                  ? 'Public network access detected'
                  : 'Private network access'}
              </p>
            </div>
          </div>
          <div class="text-right">
            <div class={`text-3xl font-semibold ${scorePresentation().tone.score}`}>
              {securityScore()}%
            </div>
            <div
              class={`mt-1 inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${scorePresentation().tone.badge}`}
            >
              {scorePresentation().label}
            </div>
          </div>
        </div>
      </div>

      {/* Security Items Grid */}
      <div class="p-6">
        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <For each={items()}>
            {(item) => (
              <div
                class={`rounded-md border p-4 transition-all ${
                  item.enabled
                    ? 'border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950'
                    : item.critical
                      ? 'border-rose-200 dark:border-rose-800 bg-rose-50 dark:bg-rose-950'
                      : 'border-border bg-surface-alt'
                }`}
              >
                <div class="flex items-center justify-between mb-2">
                  <span class="text-sm font-semibold text-base-content">{item.label}</span>
                  <Show
                    when={item.enabled}
                    fallback={
                      <XCircle
                        class={`w-5 h-5 ${item.critical ? 'text-rose-500 dark:text-rose-400' : 'text-muted'}`}
                      />
                    }
                  >
                    <CheckCircle class="w-5 h-5 text-emerald-500 dark:text-emerald-400" />
                  </Show>
                </div>
                <div class="flex items-center justify-between">
                  <p class="text-xs text-muted">{item.description}</p>
                  <Show when={item.critical && !item.enabled}>
                    <span class="text-[10px] font-medium text-rose-600 dark:text-rose-400 uppercase">
                      Critical
                    </span>
                  </Show>
                </div>
              </div>
            )}
          </For>
        </div>

        {/* Client IP Badge */}
        <Show when={props.status.clientIP}>
          <div class="mt-4 pt-4 border-t border-border flex items-center justify-end">
            <span class="inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-full bg-surface-alt text-muted">
              Your IP: {props.status.clientIP}
            </span>
          </div>
        </Show>
      </div>
    </Card>
  );
};
