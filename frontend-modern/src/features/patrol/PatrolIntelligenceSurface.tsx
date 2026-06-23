import { usePatrolIntelligenceState } from './usePatrolIntelligenceState';
import { PatrolIntelligenceHeader } from './PatrolIntelligenceHeader';
import { PatrolIntelligenceBanners } from './PatrolIntelligenceBanners';
import { PatrolIntelligenceWorkspace } from './PatrolIntelligenceWorkspace';

export function PatrolIntelligenceSurface() {
  const state = usePatrolIntelligenceState();

  return (
    <div class="space-y-6">
      <PatrolIntelligenceHeader state={state} />
      <PatrolIntelligenceBanners state={state} />

      <div
        class={`space-y-4 transition-opacity ${!state.patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <PatrolIntelligenceWorkspace state={state} />
      </div>
    </div>
  );
}

export default PatrolIntelligenceSurface;
