import type { Component } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import CheckCircle2Icon from 'lucide-solid/icons/check-circle-2';
import CircleSlashIcon from 'lucide-solid/icons/circle-slash';
import Clock3Icon from 'lucide-solid/icons/clock-3';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import MinusCircleIcon from 'lucide-solid/icons/minus-circle';
import type { DiscoveryReadinessPresentation } from '@/utils/resourceDiscoveryReadiness';

interface DiscoveryReadinessBadgeProps {
  presentation?: DiscoveryReadinessPresentation | null;
  compact?: boolean;
  class?: string;
}

const toneClasses: Record<DiscoveryReadinessPresentation['tone'], string> = {
  success:
    'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300',
  warning:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-300',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300',
  danger:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300',
  muted:
    'border-border bg-surface-alt text-muted dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300',
};

const iconForState = (state: DiscoveryReadinessPresentation['state']): Component<{ class?: string }> => {
  switch (state) {
    case 'fresh':
      return CheckCircle2Icon;
    case 'stale':
      return Clock3Icon;
    case 'running':
      return Loader2Icon;
    case 'failed':
    case 'unavailable':
      return AlertTriangleIcon;
    case 'unsupported':
      return CircleSlashIcon;
    case 'missing':
    case 'unknown':
    default:
      return MinusCircleIcon;
  }
};

export const DiscoveryReadinessBadge: Component<DiscoveryReadinessBadgeProps> = (props) => {
  const presentation = () => props.presentation;
  const label = () => (props.compact ? presentation()?.shortLabel : presentation()?.label);
  const Icon = () => iconForState(presentation()?.state ?? 'unknown');

  return (
    <span
      class={[
        'inline-flex h-6 max-w-full items-center gap-1 rounded border px-1.5 text-[10px] font-medium leading-none whitespace-nowrap',
        presentation() ? toneClasses[presentation()!.tone] : toneClasses.muted,
        props.class,
      ]
        .filter(Boolean)
        .join(' ')}
      title={presentation()?.title}
      aria-label={presentation()?.title ?? presentation()?.label}
    >
      <Dynamic
        component={Icon()}
        class={`h-3.5 w-3.5 shrink-0 ${presentation()?.state === 'running' ? 'animate-spin' : ''}`}
        aria-hidden="true"
      />
      <span class="truncate">{label()}</span>
    </span>
  );
};
