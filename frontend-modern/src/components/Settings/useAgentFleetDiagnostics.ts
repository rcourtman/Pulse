import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { AgentDiagnosticsAPI, type AgentFleetDiagnosticsResponse } from '@/api/agentDiagnostics';

const AGENT_DIAGNOSTICS_QUERY_KEY = 'settings-agent-fleet-diagnostics';
const POLL_INTERVAL_MS = 15000;

const EMPTY_DIAGNOSTICS: AgentFleetDiagnosticsResponse = {
  schemaVersion: 0,
  generatedAt: 0,
  summary: {
    total: 0,
    healthy: 0,
    warning: 0,
    critical: 0,
    removed: 0,
  },
  agents: [],
};

export const useAgentFleetDiagnostics = (enabled: () => boolean) => {
  const query = createNonSuspendingQuery<AgentFleetDiagnosticsResponse, string>({
    source: () => (enabled() ? AGENT_DIAGNOSTICS_QUERY_KEY : null),
    fetcher: () => AgentDiagnosticsAPI.getFleetDiagnostics(),
    initialValue: EMPTY_DIAGNOSTICS,
    cacheKey: (key) => key,
    pollMs: POLL_INTERVAL_MS,
  });

  return {
    data: query.value,
    error: query.error,
    loading: query.loading,
    reload: query.refetch,
    resolvedOnce: query.resolvedOnce,
  };
};
