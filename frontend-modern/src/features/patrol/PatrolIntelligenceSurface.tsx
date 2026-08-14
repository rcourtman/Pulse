import { createSignal, Show } from 'solid-js';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
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
