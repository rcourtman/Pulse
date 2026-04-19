import { useNavigate } from '@solidjs/router';
import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { ResourceAPI } from '@/api/resources';
import { buildInfrastructureOnboardingPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { useWebSocket } from '@/contexts/appRuntime';
import { DASHBOARD_PATH } from '@/routing/resourceLinks';
import { isHostedModeEnabled } from '@/stores/license';

function dashboardSummaryHasResources(totalResources: number | undefined): boolean {
  return Number(totalResources || 0) > 0;
}

export default function RuntimeHome() {
  const navigate = useNavigate();
  const ws = useWebSocket();
  const [summaryResolved, setSummaryResolved] = createSignal(!isHostedModeEnabled());
  const [summaryHasResources, setSummaryHasResources] = createSignal(false);
  const [summaryFailed, setSummaryFailed] = createSignal(false);

  const hasConnectedInfrastructure = createMemo(() => {
    const items = ws.state?.connectedInfrastructure;
    return Array.isArray(items) && items.length > 0;
  });

  const destination = createMemo<string | null>(() => {
    if (!isHostedModeEnabled()) {
      return DASHBOARD_PATH;
    }
    if (hasConnectedInfrastructure()) {
      return DASHBOARD_PATH;
    }
    if (!summaryResolved()) {
      return null;
    }
    if (summaryFailed()) {
      return DASHBOARD_PATH;
    }
    return summaryHasResources() ? DASHBOARD_PATH : buildInfrastructureOnboardingPath('agent');
  });

  onMount(() => {
    if (!isHostedModeEnabled() || hasConnectedInfrastructure()) {
      setSummaryResolved(true);
      return;
    }

    let cancelled = false;
    void ResourceAPI.getDashboardSummary()
      .then((summary) => {
        if (cancelled) {
          return;
        }
        setSummaryHasResources(dashboardSummaryHasResources(summary?.health?.totalResources));
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setSummaryFailed(true);
      })
      .finally(() => {
        if (cancelled) {
          return;
        }
        setSummaryResolved(true);
      });

    onCleanup(() => {
      cancelled = true;
    });
  });

  createEffect(() => {
    const nextPath = destination();
    if (!nextPath) {
      return;
    }
    navigate(nextPath, { replace: true });
  });

  return <div class="px-4 py-6 text-sm text-muted">Opening workspace...</div>;
}
