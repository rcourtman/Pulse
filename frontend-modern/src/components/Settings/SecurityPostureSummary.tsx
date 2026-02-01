import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import Shield from 'lucide-solid/icons/shield';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';

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
      critical: true, // Critical security feature
    },
    {
      key: 'oidc',
      label: 'Single sign-on',
      enabled: Boolean(props.status.oidcEnabled),
      description: props.status.oidcEnabled ? 'OIDC configured' : 'Not configured',
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
    const criticalItems = list.filter(i => i.critical);
    const enabledCritical = criticalItems.filter(i => i.enabled).length;
    const allItems = list.filter(i => i.enabled).length;

    // Weight critical items more heavily
    const criticalWeight = 0.7;
    const optionalWeight = 0.3;

    const criticalScore = criticalItems.length > 0 ? (enabledCritical / criticalItems.length) * criticalWeight : 0;
    const optionalScore = list.length > 0 ? (allItems / list.length) * optionalWeight : 0;

    return Math.round((criticalScore + optionalScore) * 100);
  });

  const scoreColor = () => {
    const score = securityScore();
    if (score >= 80) return 'bg-green-500';
    if (score >= 50) return 'bg-amber-500';
    return 'bg-red-500';
  };

  const scoreLabel = () => {
    const score = securityScore();
    if (score >= 80) return 'Strong';
    if (score >= 50) return 'Moderate';
    return 'Weak';
  };

  const ScoreIcon = () => {
    const score = securityScore();
    if (score >= 80) return ShieldCheck;
    if (score >= 50) return Shield;
    return ShieldAlert;
  };

  return (
    <Card
      padding="none"
      class="overflow-hidden border border-gray-200 dark:border-gray-700"
      border={false}
    >
      {/* Header with Security Score */}
      <div class={`${scoreColor()} px-6 py-5`}>
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <div class="p-3 bg-white/20 rounded-xl backdrop-blur-sm">
              {(() => {
                const Icon = ScoreIcon();
                return <Icon class="w-6 h-6 text-white" />;
              })()}
            </div>
            <div>
              <h2 class="text-lg font-bold text-white">Security Posture</h2>
              <p class="text-sm text-white/80">
                {props.status.publicAccess && !props.status.isPrivateNetwork
                  ? 'Public network access detected'
                  : 'Private network access'}
              </p>
            </div>
          </div>
          <div class="text-right">
            <div class="text-3xl font-bold text-white">{securityScore()}%</div>
            <div class="text-sm text-white/80">{scoreLabel()}</div>
          </div>
        </div>
      </div>

      {/* Security Items Grid */}
      <div class="p-6">
        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <For each={items()}>
            {(item) => (
              <div class={`rounded-xl border p-4 transition-all ${item.enabled
                  ? 'border-green-200 dark:border-green-800 bg-green-50/50 dark:bg-green-900/20'
                  : item.critical
                    ? 'border-red-200 dark:border-red-800 bg-red-50/50 dark:bg-red-900/20'
                    : 'border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-800/50'
                }`}>
                <div class="flex items-center justify-between mb-2">
                  <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                    {item.label}
                  </span>
                  <Show when={item.enabled} fallback={
                    <XCircle class={`w-5 h-5 ${item.critical ? 'text-red-500' : 'text-gray-400 dark:text-gray-500'}`} />
                  }>
                    <CheckCircle class="w-5 h-5 text-green-500" />
                  </Show>
                </div>
                <div class="flex items-center justify-between">
                  <p class="text-xs text-gray-600 dark:text-gray-400">
                    {item.description}
                  </p>
                  <Show when={item.critical && !item.enabled}>
                    <span class="text-[10px] font-medium text-red-600 dark:text-red-400 uppercase">
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
          <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end">
            <span class="inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-full bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300">
              Your IP: {props.status.clientIP}
            </span>
          </div>
        </Show>
      </div>
    </Card>
  );
};
