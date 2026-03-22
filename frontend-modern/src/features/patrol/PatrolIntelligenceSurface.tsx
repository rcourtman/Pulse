import { usePatrolIntelligenceState } from './usePatrolIntelligenceState';
import { PatrolIntelligenceHeader } from './PatrolIntelligenceHeader';
import { PatrolIntelligenceBanners } from './PatrolIntelligenceBanners';
import { PatrolIntelligenceSummary } from './PatrolIntelligenceSummary';
import { PatrolIntelligenceWorkspace } from './PatrolIntelligenceWorkspace';

export function PatrolIntelligenceSurface() {
  const state = usePatrolIntelligenceState();

  return (
    <div class="h-full flex flex-col bg-base">
      <PatrolIntelligenceHeader state={state} />
      <PatrolIntelligenceBanners state={state} />

      <div
        class={`flex-1 overflow-auto p-4 transition-opacity ${!state.patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <div class="space-y-4">
          <PatrolIntelligenceSummary state={state} />
          <PatrolIntelligenceWorkspace state={state} />
        </div>
      </div>
    </div>
  );
}

export default PatrolIntelligenceSurface;
