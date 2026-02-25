import { createMemo, createEffect, onCleanup, type Accessor } from 'solid-js';
import type { Alert } from '@/types/api';
import type { UnifiedFinding } from '@/stores/aiIntelligence';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import type { ApprovalRequest } from '@/api/ai';
import { hasFeature } from '@/stores/license';

export interface DashboardActions {
  pendingApprovals: Accessor<ApprovalRequest[]>;
  unackedCriticalAlerts: Accessor<Alert[]>;
  findingsNeedingAttention: Accessor<UnifiedFinding[]>;
  hasAnyActions: Accessor<boolean>;
  totalActionCount: Accessor<number>;
}

const APPROVAL_REFRESH_INTERVAL_MS = 30_000;

export function useDashboardActions(alertsList: Accessor<Alert[]>): DashboardActions {
  const hasPatrol = () => hasFeature('ai_patrol');

  // Load patrol data on mount when feature is enabled
  createEffect(() => {
    if (hasPatrol()) {
      aiIntelligenceStore.loadFindings();
      aiIntelligenceStore.loadPendingApprovals();
    }
  });

  // Refresh approvals every 30s (they have 5-min expiry)
  let refreshInterval: number | undefined;
  createEffect(() => {
    if (refreshInterval) {
      window.clearInterval(refreshInterval);
      refreshInterval = undefined;
    }
    if (hasPatrol()) {
      refreshInterval = window.setInterval(() => {
        aiIntelligenceStore.loadPendingApprovals();
      }, APPROVAL_REFRESH_INTERVAL_MS);
    }
  });
  onCleanup(() => {
    if (refreshInterval) window.clearInterval(refreshInterval);
  });

  const pendingApprovals = createMemo(() => {
    if (!hasPatrol()) return [];
    return aiIntelligenceStore.pendingApprovals.filter((a) => a.status === 'pending');
  });

  const unackedCriticalAlerts = createMemo(() =>
    alertsList().filter((a) => a.level === 'critical' && !a.acknowledged),
  );

  const findingsNeedingAttention = createMemo(() => {
    if (!hasPatrol()) return [];
    return aiIntelligenceStore.findingsNeedingAttention;
  });

  const totalActionCount = createMemo(
    () =>
      pendingApprovals().length +
      unackedCriticalAlerts().length +
      findingsNeedingAttention().length,
  );

  const hasAnyActions = createMemo(() => totalActionCount() > 0);

  return {
    pendingApprovals,
    unackedCriticalAlerts,
    findingsNeedingAttention,
    hasAnyActions,
    totalActionCount,
  };
}
