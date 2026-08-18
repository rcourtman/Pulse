import { createSignal, Show } from 'solid-js';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import ClipboardCheckIcon from 'lucide-solid/icons/clipboard-check';
import HistoryIcon from 'lucide-solid/icons/history';
import { ButtonLink } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { actionInboxStore } from '@/stores/actionInbox';
import { usePatrolIntelligenceState } from './usePatrolIntelligenceState';
import { PatrolIntelligenceHeader } from './PatrolIntelligenceHeader';
import { PatrolIntelligenceBanners } from './PatrolIntelligenceBanners';
import { PatrolIntelligenceWorkspace } from './PatrolIntelligenceWorkspace';
import { PatrolAttentionWorkbench } from './PatrolAttentionWorkbench';
import { PatrolObjectivesPanel } from './PatrolObjectivesPanel';
import { PatrolRecentWorkPanel } from './PatrolRecentWorkPanel';

export function PatrolIntelligenceSurface() {
  const state = usePatrolIntelligenceState();
  const [findingsOpen, setFindingsOpen] = createSignal(false);
  let findingsPanel: HTMLDetailsElement | undefined;
  const openWorkCount = () => aiIntelligenceStore.patrolOpenWorkCount;
  const openFindings = () => {
    setFindingsOpen(true);
    queueMicrotask(() => {
      findingsPanel?.scrollIntoView?.({ block: 'start' });
      findingsPanel?.focus?.({ preventScroll: true });
    });
  };

  return (
    <div class="space-y-4 lg:space-y-5">
      <PatrolIntelligenceHeader state={state} />
      <PatrolIntelligenceBanners state={state} />
      <PatrolAttentionWorkbench
        autonomyLevel={state.autonomyLevel()}
        autonomyLocked={state.autoFixLocked()}
        pendingActionCount={actionInboxStore.pendingActionCount}
        onOpenFindings={openFindings}
      />

      <div class="grid min-w-0 gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(20rem,0.85fr)]">
        <PatrolObjectivesPanel />
        <PatrolRecentWorkPanel />
      </div>

      <section
        aria-labelledby="patrol-continuity-title"
        class="rounded-lg border border-border bg-surface"
      >
        <div class="border-b border-border px-4 py-4 sm:px-5">
          <h2 id="patrol-continuity-title" class="text-base font-semibold text-base-content">
            Review and history
          </h2>
          <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
            Follow pending decisions, audit completed operations, or inspect Patrol's underlying
            records.
          </p>
        </div>
        <div class="grid divide-y divide-border sm:grid-cols-2 sm:divide-x sm:divide-y-0">
          <ButtonLink
            href="/actions"
            variant="ghost"
            size="md"
            class="group h-auto min-h-20 w-full justify-between rounded-none px-4 py-3 text-left sm:px-5"
          >
            <span class="flex min-w-0 items-start gap-3">
              <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-alt text-muted">
                <ClipboardCheckIcon class="h-5 w-5" aria-hidden="true" />
              </span>
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-2 text-sm font-semibold text-base-content">
                  Actions and approvals
                  <Show when={actionInboxStore.pendingActionCount > 0}>
                    <MetadataBadge
                      tone="warning"
                      size="xs"
                      shape="rounded"
                      aria-label={`${actionInboxStore.pendingActionCount} ${actionInboxStore.pendingActionCount === 1 ? 'operation needs' : 'operations need'} review`}
                    >
                      {actionInboxStore.pendingActionCount} waiting
                    </MetadataBadge>
                  </Show>
                </span>
                <span class="mt-1 block text-xs font-normal leading-5 text-muted">
                  Review governed actions and their complete audit trail.
                </span>
              </span>
            </span>
            <ArrowRightIcon
              class="h-4 w-4 shrink-0 text-muted transition-transform group-hover:translate-x-0.5"
              aria-hidden="true"
            />
          </ButtonLink>

          <button
            type="button"
            class="group flex min-h-20 w-full items-center justify-between gap-3 px-4 py-3 text-left hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-5"
            aria-expanded={findingsOpen()}
            aria-controls="patrol-operational-records"
            onClick={() => setFindingsOpen((open) => !open)}
          >
            <span class="flex min-w-0 items-start gap-3">
              <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-alt text-muted">
                <HistoryIcon class="h-5 w-5" aria-hidden="true" />
              </span>
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-2 text-sm font-semibold text-base-content">
                  Findings and run records
                  <Show when={openWorkCount() > 0}>
                    <MetadataBadge
                      tone="info"
                      size="xs"
                      shape="rounded"
                      aria-label={`${openWorkCount()} open patrol ${openWorkCount() === 1 ? 'finding' : 'findings'}`}
                    >
                      {openWorkCount()} open
                    </MetadataBadge>
                  </Show>
                </span>
                <span class="mt-1 block text-xs font-normal leading-5 text-muted">
                  Inspect finding detail, check results, and historical runs.
                </span>
              </span>
            </span>
            <ArrowRightIcon
              class={`h-4 w-4 shrink-0 text-muted transition-transform ${findingsOpen() ? 'rotate-90' : ''}`}
              aria-hidden="true"
            />
          </button>
        </div>
      </section>

      <details
        ref={findingsPanel}
        id="patrol-operational-records"
        tabindex="-1"
        class="rounded-lg border border-border bg-surface"
        open={findingsOpen()}
        onToggle={(event) => setFindingsOpen(event.currentTarget.open)}
      >
        <summary class="sr-only">Findings and run records</summary>
        <div
          class={`space-y-4 border-t border-border p-4 sm:p-5 ${!state.patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}
        >
          <PatrolIntelligenceWorkspace state={state} />
        </div>
      </details>
    </div>
  );
}

export default PatrolIntelligenceSurface;
