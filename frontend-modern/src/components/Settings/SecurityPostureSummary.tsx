import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import CheckCircle from 'lucide-solid/icons/check-circle';
import ChevronRight from 'lucide-solid/icons/chevron-right';
import XCircle from 'lucide-solid/icons/x-circle';
import { PROXY_AUTH_DOC_URL, SECURITY_DOC_URL } from '@/utils/docsLinks';
import { settingsTabPath } from './settingsNavigationModel';
import {
  getSecurityFeatureCardPresentation,
  getSecurityPostureItems,
  getSecurityScoreIconComponent,
  getSecurityScorePresentation,
  type SecurityPostureItem,
  type SecurityPostureStatus,
} from '@/utils/securityScorePresentation';

interface SecurityPostureSummaryProps {
  status: SecurityPostureStatus;
  embedded?: boolean;
}

const CORE_POSTURE_KEYS = new Set<SecurityPostureItem['key']>([
  'authentication',
  'export',
  'https',
  'token',
]);

const postureLinkFor = (key: SecurityPostureItem['key']) => {
  switch (key) {
    case 'authentication':
      return { href: settingsTabPath('security-auth'), external: false } as const;
    case 'oidc':
      return { href: settingsTabPath('security-sso'), external: false } as const;
    case 'proxy':
      return { href: PROXY_AUTH_DOC_URL, external: true } as const;
    case 'token':
    case 'export':
      return { href: settingsTabPath('api'), external: false } as const;
    case 'https':
      return { href: SECURITY_DOC_URL, external: true } as const;
    case 'audit':
      return { href: settingsTabPath('security-audit'), external: false } as const;
  }
};

const getPlainNetworkAccessSubtitle = (status: SecurityPostureStatus): string => {
  if (status.clientIP?.trim() && status.publicAccess !== status.isPrivateNetwork) {
    if (status.isPrivateNetwork === true) return 'Accessed from a private network';
    if (status.publicAccess === true) return 'Accessed from a non-private network';
  }
  return 'Network exposure unavailable';
};

export const SecurityPostureSummary: Component<SecurityPostureSummaryProps> = (props) => {
  const items = createMemo(() => getSecurityPostureItems(props.status));
  const coreItems = createMemo(() => items().filter((item) => CORE_POSTURE_KEYS.has(item.key)));
  const optionalItems = createMemo(() =>
    items().filter((item) => !CORE_POSTURE_KEYS.has(item.key)),
  );

  // Evaluate 6 independent security dimensions so mutually alternative
  // sign-in methods (SSO vs proxy auth) do not penalize healthy installs, and
  // cap the score in Weak whenever a critical control is missing.
  const postureMetrics = createMemo(() => {
    const status = props.status;
    const list = items();
    const hasMissingCritical = list.some((item) => item.critical && !item.enabled);
    const dimensions = [
      status.hasAuthentication,
      status.exportProtected && !status.unprotectedExportAllowed,
      Boolean(status.hasHTTPS),
      status.apiTokenConfigured,
      status.hasAuditLogging,
      Boolean(status.ssoEnabled || status.hasProxyAuth),
    ];
    const activeCount = dimensions.filter(Boolean).length;
    const rawScore = Math.round((activeCount / dimensions.length) * 100);
    const score = hasMissingCritical ? Math.min(rawScore, 45) : rawScore;
    return { activeCount, totalCount: dimensions.length, score };
  });

  const securityScore = () => postureMetrics().score;
  const scorePresentation = createMemo(() => getSecurityScorePresentation(securityScore()));
  const scoreIcon = createMemo(() => getSecurityScoreIconComponent(securityScore()));

  const renderPostureCard = (item: SecurityPostureItem) => {
    const card = getSecurityFeatureCardPresentation({
      enabled: item.enabled,
      critical: item.critical,
    });
    const destination = postureLinkFor(item.key);
    return (
      <a
        href={destination.href}
        target={destination.external ? '_blank' : undefined}
        rel={destination.external ? 'noreferrer' : undefined}
        class={`group block rounded-md border p-4 transition-all hover:border-blue-400 focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 ${card.cardClassName}`}
      >
        <div class="flex items-center justify-between gap-2 mb-2">
          <span class="text-sm font-semibold text-base-content group-hover:text-blue-700 dark:group-hover:text-blue-300">
            {item.label}
          </span>
          <div class="flex items-center gap-1 shrink-0">
            <Show
              when={item.enabled}
              fallback={<XCircle class={`w-5 h-5 ${card.iconClassName}`} aria-hidden="true" />}
            >
              <CheckCircle
                class="w-5 h-5 text-emerald-500 dark:text-emerald-400"
                aria-hidden="true"
              />
            </Show>
            <ChevronRight
              class="w-4 h-4 text-muted opacity-60 transition-transform group-hover:translate-x-0.5 group-hover:opacity-100"
              aria-hidden="true"
            />
          </div>
        </div>
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs text-muted">{item.description}</p>
          <Show when={item.critical && !item.enabled}>
            <span class={`text-[10px] font-medium uppercase ${card.criticalLabelClassName}`}>
              Critical
            </span>
          </Show>
        </div>
      </a>
    );
  };

  return (
    <Card
      padding="none"
      class={`overflow-hidden border border-border ${props.embedded ? 'rounded-md bg-surface' : ''}`.trim()}
      border={false}
    >
      {/* Header with Security Score */}
      <div
        class={`px-4 py-4 sm:px-6 sm:py-5 ${scorePresentation().tone.headerBg} ${scorePresentation().tone.headerBorder}`}
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-3 sm:gap-4 min-w-0">
            <div class={`p-3 rounded-md shrink-0 ${scorePresentation().tone.iconWrap}`}>
              {(() => {
                const Icon = scoreIcon();
                return <Icon class={`w-6 h-6 ${scorePresentation().tone.icon}`} />;
              })()}
            </div>
            <div class="min-w-0">
              <h2 class="text-lg font-semibold text-base-content">Security Posture</h2>
              <p class={`text-sm ${scorePresentation().tone.subtitle}`}>
                {getPlainNetworkAccessSubtitle(props.status)}
              </p>
            </div>
          </div>
          <div class="text-right shrink-0">
            <div class={`text-3xl font-semibold ${scorePresentation().tone.score}`}>
              {securityScore()}%
            </div>
            <div class="mt-1 flex items-center justify-end gap-2">
              <span class="text-xs text-muted">
                {postureMetrics().activeCount} of {postureMetrics().totalCount} checks active
              </span>
              <span
                class={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${scorePresentation().tone.badge}`}
              >
                {scorePresentation().label}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Security Items Grids (4 core + 3 team/compliance, zero orphan slots) */}
      <div class="p-4 sm:p-6 space-y-4">
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <For each={coreItems()}>{(item) => renderPostureCard(item)}</For>
        </div>

        <div class="grid gap-3 sm:grid-cols-3">
          <For each={optionalItems()}>{(item) => renderPostureCard(item)}</For>
        </div>

        {/* Client IP Badge */}
        <Show when={props.status.clientIP}>
          <div class="pt-4 border-t border-border flex items-center justify-end">
            <span class="inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-full bg-surface-alt text-muted">
              Your IP: {props.status.clientIP}
            </span>
          </div>
        </Show>
      </div>
    </Card>
  );
};
