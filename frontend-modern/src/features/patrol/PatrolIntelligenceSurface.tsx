import { createSignal, Show } from 'solid-js';
import ListChecksIcon from 'lucide-solid/icons/list-checks';
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
  let findingsSummary: HTMLElement | undefined;
  const openWorkCount = () => aiIntelligenceStore.patrolOpenWorkCount;
  const openFindings = () => {
    setFindingsOpen(true);
    queueMicrotask(() => {
      findingsSummary?.scrollIntoView?.({ block: 'start' });
      findingsSummary?.focus?.({ preventScroll: true });
    });
  };

  return (
    <div class="space-y-6">
      <PatrolIntelligenceHeader state={state} />
      <PatrolIntelligenceBanners state={state} />
      <PatrolObjectivesPanel />
      <PatrolAttentionWorkbench
        autonomyLevel={state.autonomyLevel()}
        autonomyLocked={state.autoFixLocked()}
        onOpenFindings={openFindings}
      />

      <section
        aria-labelledby="patrol-activity-history-title"
        class="flex flex-col gap-4 rounded-lg border border-border bg-surface px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-5"
      >
        <div class="flex min-w-0 items-start gap-3">
          <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-alt text-muted">
            <ListChecksIcon class="h-5 w-5" aria-hidden="true" />
          </span>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2
                id="patrol-activity-history-title"
                class="text-base font-semibold text-base-content"
              >
                Activity history
              </h2>
              <Show when={actionInboxStore.pendingActionCount > 0}>
                <MetadataBadge
                  tone="warning"
                  size="sm"
                  shape="rounded"
                  aria-label={`${actionInboxStore.pendingActionCount} ${actionInboxStore.pendingActionCount === 1 ? 'operation needs' : 'operations need'} review`}
                >
                  {actionInboxStore.pendingActionCount} to review
                </MetadataBadge>
              </Show>
            </div>
            <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
              Audit every governed operation, including work started elsewhere in Pulse.
            </p>
          </div>
        </div>
        <ButtonLink
          href="/actions"
          variant="secondary"
          size="sm"
          class="w-full shrink-0 justify-center sm:w-auto"
        >
          Open activity history
        </ButtonLink>
      </section>

      <PatrolRecentWorkPanel />

      <details
        class="rounded-lg border border-border bg-surface"
        open={findingsOpen()}
        onToggle={(event) => setFindingsOpen(event.currentTarget.open)}
      >
        <summary
          ref={findingsSummary}
          class="cursor-pointer px-4 py-3 text-sm font-medium text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-5"
        >
          <span class="inline-flex flex-wrap items-center gap-2">
            Operational records and run history
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
        </summary>
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
