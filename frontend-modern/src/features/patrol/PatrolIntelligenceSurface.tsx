import { usePatrolIntelligenceState } from './usePatrolIntelligenceState';
import { PatrolIntelligenceHeader } from './PatrolIntelligenceHeader';
import { PatrolIntelligenceBanners } from './PatrolIntelligenceBanners';
import { PatrolIntelligenceWorkspace } from './PatrolIntelligenceWorkspace';
import { PatrolAttentionWorkbench } from './PatrolAttentionWorkbench';

export function PatrolIntelligenceSurface() {
  const state = usePatrolIntelligenceState();

  return (
    <div class="space-y-6">
      <PatrolIntelligenceHeader state={state} />
      <PatrolIntelligenceBanners state={state} />
      <PatrolAttentionWorkbench />

      <details class="rounded-lg border border-border bg-surface">
        <summary class="cursor-pointer px-4 py-3 text-sm font-medium text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-5">
          Patrol checks, investigations, and run history
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
