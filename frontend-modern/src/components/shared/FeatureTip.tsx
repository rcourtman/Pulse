import { Component, Show } from 'solid-js';
import { A } from '@solidjs/router';
import X from 'lucide-solid/icons/x';
import Lightbulb from 'lucide-solid/icons/lightbulb';
import ArrowRight from 'lucide-solid/icons/arrow-right';
import type { FeatureTip as FeatureTipType } from '@/content/features';
import { dismissTip, isTipDismissed } from '@/stores/featureTips';

export interface FeatureTipProps {
  /** The feature tip to display */
  tip: FeatureTipType;

  /** Display variant */
  variant?: 'inline' | 'banner' | 'compact';

  /** Additional CSS classes */
  class?: string;

  /** Called when the tip is dismissed */
  onDismiss?: () => void;
}

export const FeatureTip: Component<FeatureTipProps> = (props) => {
  // Don't render if already dismissed
  if (isTipDismissed(props.tip.id)) {
    return null;
  }

  const variant = () => props.variant ?? 'inline';

  const handleDismiss = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dismissTip(props.tip.id);
    props.onDismiss?.();
  };

  // Inline variant - small, subtle
  if (variant() === 'inline') {
    return (
      <div
        class={`flex items-start gap-2 p-2 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg text-xs ${props.class ?? ''}`}
      >
        <Lightbulb class="w-3.5 h-3.5 text-blue-500 flex-shrink-0 mt-0.5" strokeWidth={2} />
        <div class="flex-1 min-w-0">
          <span class="text-blue-800 dark:text-blue-200">{props.tip.description}</span>
          <Show when={props.tip.action}>
            <A
              href={props.tip.action!.path}
              class="ml-1 text-blue-600 dark:text-blue-400 hover:underline font-medium"
            >
              {props.tip.action!.label}
            </A>
          </Show>
        </div>
        <button
          type="button"
          onClick={handleDismiss}
          class="p-0.5 text-blue-400 hover:text-blue-600 dark:hover:text-blue-300 rounded transition-colors flex-shrink-0"
          aria-label="Dismiss tip"
        >
          <X class="w-3 h-3" strokeWidth={2} />
        </button>
      </div>
    );
  }

  // Compact variant - single line
  if (variant() === 'compact') {
    return (
      <div
        class={`flex items-center gap-2 px-2 py-1 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded text-xs ${props.class ?? ''}`}
      >
        <Lightbulb class="w-3 h-3 text-amber-500 flex-shrink-0" strokeWidth={2} />
        <span class="text-amber-800 dark:text-amber-200 truncate">{props.tip.title}</span>
        <Show when={props.tip.action}>
          <A
            href={props.tip.action!.path}
            class="text-amber-600 dark:text-amber-400 hover:underline flex items-center gap-0.5"
          >
            <ArrowRight class="w-3 h-3" strokeWidth={2} />
          </A>
        </Show>
        <button
          type="button"
          onClick={handleDismiss}
          class="p-0.5 text-amber-400 hover:text-amber-600 dark:hover:text-amber-300 rounded transition-colors flex-shrink-0 ml-auto"
          aria-label="Dismiss tip"
        >
          <X class="w-3 h-3" strokeWidth={2} />
        </button>
      </div>
    );
  }

  // Banner variant - full width, more prominent
  return (
    <div
      class={`flex items-start gap-3 p-3 bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 border border-blue-200 dark:border-blue-800 rounded-lg ${props.class ?? ''}`}
    >
      <div class="p-1.5 bg-blue-100 dark:bg-blue-800 rounded-lg flex-shrink-0">
        <Lightbulb class="w-4 h-4 text-blue-600 dark:text-blue-300" strokeWidth={2} />
      </div>
      <div class="flex-1 min-w-0">
        <div class="text-sm font-medium text-blue-900 dark:text-blue-100">{props.tip.title}</div>
        <p class="text-xs text-blue-700 dark:text-blue-300 mt-0.5">{props.tip.description}</p>
        <Show when={props.tip.action}>
          <A
            href={props.tip.action!.path}
            class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
          >
            {props.tip.action!.label}
            <ArrowRight class="w-3 h-3" strokeWidth={2} />
          </A>
        </Show>
      </div>
      <button
        type="button"
        onClick={handleDismiss}
        class="p-1 text-blue-400 hover:text-blue-600 dark:hover:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-800 rounded transition-colors flex-shrink-0"
        aria-label="Dismiss tip"
      >
        <X class="w-4 h-4" strokeWidth={2} />
      </button>
    </div>
  );
};

export default FeatureTip;
