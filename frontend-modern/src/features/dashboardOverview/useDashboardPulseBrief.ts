import { createEffect, createMemo, type Accessor } from 'solid-js';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import { aiRuntimeSettings, loadAIRuntimeSettings } from '@/stores/aiRuntimeState';
import { hasFeature } from '@/stores/license';
import type { DashboardEstateSummary } from './estateSummaryModel';
import { buildDashboardPulseBrief, type DashboardPulseBrief } from './dashboardPulseBriefModel';

interface UseDashboardPulseBriefInput {
  estate: Accessor<DashboardEstateSummary>;
  overview: Accessor<DashboardOverview>;
  storageCapacityPercent: Accessor<number>;
  recovery: Accessor<DashboardRecoverySummary>;
  pendingApprovalCount: Accessor<number>;
  patrolFindingCount: Accessor<number>;
}

export function useDashboardPulseBrief(
  input: UseDashboardPulseBriefInput,
): Accessor<DashboardPulseBrief | null> {
  createEffect(() => {
    if (!hasFeature('ai_patrol')) return;
    void loadAIRuntimeSettings().catch(() => undefined);
  });

  return createMemo(() => {
    const settings = aiRuntimeSettings();
    if (!hasFeature('ai_patrol') || !settings?.enabled || !settings.configured) {
      return null;
    }

    return buildDashboardPulseBrief({
      estate: input.estate(),
      overview: input.overview(),
      storageCapacityPercent: input.storageCapacityPercent(),
      recovery: input.recovery(),
      pendingApprovalCount: input.pendingApprovalCount(),
      patrolFindingCount: input.patrolFindingCount(),
    });
  });
}
