import { For } from 'solid-js';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { Card } from '@/components/shared/Card';
import { AI_PATROL_PATH } from '@/routing/resourceLinks';
import type { DashboardPulseBrief, DashboardPulseBriefTone } from './dashboardPulseBriefModel';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import MessageCircleIcon from 'lucide-solid/icons/message-circle';

interface PulseBriefPanelProps {
  brief: DashboardPulseBrief;
  onAskAssistant: () => void;
  /**
   * When true, the panel stacks its content, actions, and evidence vertically
   * instead of flowing the actions to the right of the narrative. Use when
   * placing the Brief in a narrow column alongside another header card.
   */
  compact?: boolean;
  class?: string;
}

const toneClass: Record<DashboardPulseBriefTone, string> = {
  healthy: 'border-l-emerald-500',
  attention: 'border-l-amber-500',
  critical: 'border-l-red-500',
};

const badgeClass: Record<DashboardPulseBriefTone, string> = {
  healthy: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  attention: 'bg-amber-50 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  critical: 'bg-red-50 text-red-700 dark:bg-red-900/40 dark:text-red-300',
};

export function PulseBriefPanel(props: PulseBriefPanelProps) {
  return (
    <Card
      padding="none"
      class={`overflow-hidden border-l-[3px] ${toneClass[props.brief.tone]} ${props.class ?? ''}`.trim()}
      data-testid="dashboard-pulse-brief"
    >
      <div
        class={`flex h-full flex-col gap-3 px-4 py-3 ${
          props.compact ? '' : 'lg:flex-row lg:items-start lg:justify-between'
        }`}
      >
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span class="inline-flex h-7 w-7 items-center justify-center rounded-md bg-cyan-50 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300">
              <PulsePatrolLogo class="h-4 w-4" title="Pulse Brief" />
            </span>
            <div class="min-w-0">
              <h2 class="text-sm font-semibold text-base-content">Pulse Brief</h2>
              <p class="text-[11px] font-medium uppercase tracking-wide text-muted">
                Patrol + Assistant
              </p>
            </div>
            <span
              class={`rounded px-2 py-0.5 text-[11px] font-medium ${badgeClass[props.brief.tone]}`}
            >
              {props.brief.title}
            </span>
          </div>

          <p class={`mt-2 text-sm leading-6 text-base-content ${props.compact ? '' : 'max-w-5xl'}`}>
            {props.brief.body}
          </p>

          <div class="mt-2 flex flex-wrap gap-1.5">
            <For each={props.brief.evidence}>
              {(item) => (
                <span class="rounded border border-border-subtle bg-base px-2 py-0.5 text-[11px] text-muted">
                  {item}
                </span>
              )}
            </For>
          </div>
        </div>

        <div
          class={`flex shrink-0 flex-wrap items-center gap-2 ${
            props.compact ? '' : 'lg:justify-end'
          }`}
        >
          <button
            type="button"
            onClick={() => props.onAskAssistant()}
            class="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-2.5 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
          >
            <MessageCircleIcon class="h-3.5 w-3.5" aria-hidden="true" />
            Ask Assistant
          </button>
          <a
            href={AI_PATROL_PATH}
            class="inline-flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1.5 text-xs font-medium text-base-content hover:bg-surface-hover"
          >
            Open Patrol
            <ArrowRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
          </a>
        </div>
      </div>
    </Card>
  );
}
