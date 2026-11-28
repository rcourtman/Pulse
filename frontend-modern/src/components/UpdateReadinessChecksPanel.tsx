import { For, Show, createMemo } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import type { UpdateReadiness, UpdateReadinessCheck } from '@/api/updates';

const STATUS_PRIORITY: Record<UpdateReadinessCheck['status'], number> = {
  blocked: 0,
  warning: 1,
  pass: 2,
};

const PANEL_TONE: Record<UpdateReadiness['status'], string> = {
  blocked:
    'border-red-200 bg-red-50 text-red-900 dark:border-red-800 dark:bg-red-950/40 dark:text-red-100',
  attention:
    'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100',
  ready:
    'border-green-200 bg-green-50 text-green-900 dark:border-green-800 dark:bg-green-950/40 dark:text-green-100',
};

const CHECK_TONE: Record<UpdateReadinessCheck['status'], string> = {
  blocked: 'text-red-800 dark:text-red-200',
  warning: 'text-amber-900 dark:text-amber-200',
  pass: 'text-green-800 dark:text-green-200',
};

const CHECK_BADGE: Record<UpdateReadinessCheck['status'], { label: string; className: string }> = {
  blocked: {
    label: 'Blocked',
    className: 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-200',
  },
  warning: {
    label: 'Warning',
    className: 'bg-amber-100 text-amber-900 dark:bg-amber-900/50 dark:text-amber-200',
  },
  pass: {
    label: 'Pass',
    className: 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-200',
  },
};

export interface UpdateReadinessSummaryCounts {
  blocked: number;
  warnings: number;
  isBlocked: boolean;
  hasWarnings: boolean;
}

export function getUpdateReadinessCounts(
  readiness?: UpdateReadiness | null,
): UpdateReadinessSummaryCounts {
  if (!readiness) {
    return { blocked: 0, warnings: 0, isBlocked: false, hasWarnings: false };
  }
  let blocked = 0;
  let warnings = 0;
  for (const check of readiness.checks) {
    if (check.status === 'blocked') blocked += 1;
    else if (check.status === 'warning') warnings += 1;
  }
  return {
    blocked,
    warnings,
    isBlocked: readiness.status === 'blocked' || blocked > 0,
    hasWarnings: readiness.status === 'attention' || warnings > 0,
  };
}

function ReadinessIcon(props: { status: UpdateReadinessCheck['status'] }) {
  if (props.status === 'blocked') {
    return <CircleAlertIcon class="h-4 w-4" aria-hidden="true" />;
  }
  if (props.status === 'warning') {
    return <AlertTriangleIcon class="h-4 w-4" aria-hidden="true" />;
  }
  return <CheckCircleIcon class="h-4 w-4" aria-hidden="true" />;
}

export function UpdateReadinessChecksPanel(props: { readiness: UpdateReadiness; class?: string }) {
  const orderedChecks = createMemo(() =>
    [...props.readiness.checks].sort(
      (left, right) => STATUS_PRIORITY[left.status] - STATUS_PRIORITY[right.status],
    ),
  );

  return (
    <section
      data-testid="update-readiness-panel"
      role={props.readiness.status === 'blocked' ? 'alert' : 'region'}
      aria-label="Upgrade checks"
      class={`rounded-md border p-3.5 ${PANEL_TONE[props.readiness.status]} ${props.class ?? ''}`.trim()}
    >
      <div class="flex flex-col gap-0.5">
        <p class="text-sm font-semibold">Upgrade checks</p>
        <p class="text-xs leading-relaxed opacity-95">{props.readiness.summary}</p>
      </div>
      <Show when={orderedChecks().length > 0}>
        <ul class="mt-3 divide-y divide-black/10 dark:divide-white/10">
          <For each={orderedChecks()}>
            {(check) => (
              <li class="py-2.5 first:pt-0 last:pb-0">
                <div class={`flex items-start gap-2 ${CHECK_TONE[check.status]}`}>
                  <span class="mt-0.5 shrink-0">
                    <ReadinessIcon status={check.status} />
                  </span>
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-1.5">
                      <span class="text-xs font-semibold sm:text-sm">{check.title}</span>
                      <span
                        class={`rounded-sm px-1.5 py-0.5 text-[11px] font-medium ${CHECK_BADGE[check.status].className}`}
                      >
                        {CHECK_BADGE[check.status].label}
                      </span>
                    </div>
                    <p class="mt-0.5 text-xs leading-relaxed opacity-95">{check.summary}</p>
                    <Show when={check.details?.length}>
                      <ul class="mt-1.5 list-disc space-y-1 pl-4 text-xs leading-relaxed opacity-90">
                        <For each={check.details ?? []}>
                          {(detail) => <li class="break-words">{detail}</li>}
                        </For>
                      </ul>
                    </Show>
                  </div>
                </div>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </section>
  );
}
