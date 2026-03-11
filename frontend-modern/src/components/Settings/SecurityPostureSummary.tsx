import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import {
  getSecurityFeatureCardPresentation,
  getSecurityNetworkAccessSubtitle,
  getSecurityPostureItems,
  getSecurityScoreIconComponent,
  getSecurityScorePresentation,
  type SecurityPostureStatus,
} from '@/utils/securityScorePresentation';

interface SecurityPostureSummaryProps {
  status: SecurityPostureStatus;
}

export const SecurityPostureSummary: Component<SecurityPostureSummaryProps> = (props) => {
  const items = () => getSecurityPostureItems(props.status);

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
  const scoreIcon = createMemo(() => getSecurityScoreIconComponent(securityScore()));

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
                const Icon = scoreIcon();
                return <Icon class={`w-6 h-6 ${scorePresentation().tone.icon}`} />;
              })()}
            </div>
            <div>
              <h2 class="text-lg font-semibold text-base-content">Security Posture</h2>
              <p class={`text-sm ${scorePresentation().tone.subtitle}`}>
                {getSecurityNetworkAccessSubtitle(props.status)}
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
            {(item) => {
              const card = getSecurityFeatureCardPresentation({
                enabled: item.enabled,
                critical: item.critical,
              });
              return (
                <div class={`rounded-md border p-4 transition-all ${card.cardClassName}`}>
                  <div class="flex items-center justify-between mb-2">
                    <span class="text-sm font-semibold text-base-content">{item.label}</span>
                    <Show
                      when={item.enabled}
                      fallback={<XCircle class={`w-5 h-5 ${card.iconClassName}`} />}
                    >
                      <CheckCircle class="w-5 h-5 text-emerald-500 dark:text-emerald-400" />
                    </Show>
                  </div>
                  <div class="flex items-center justify-between">
                    <p class="text-xs text-muted">{item.description}</p>
                    <Show when={item.critical && !item.enabled}>
                      <span class={`text-[10px] font-medium uppercase ${card.criticalLabelClassName}`}>
                        Critical
                      </span>
                    </Show>
                  </div>
                </div>
              );
            }}
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
