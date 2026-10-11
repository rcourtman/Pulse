import { apiFetchJSON } from '@/utils/apiClient';

export interface AgentResourceContextFact {
  label: string;
  value: string;
  source?: string;
  trustTier?: string;
  observedAt?: string;
  redacted?: boolean;
}

export interface AgentResourceContextRedaction {
  field: string;
  reason: string;
}

export interface AgentResourceContextSection {
  id: string;
  title: string;
  summary?: string;
  source: string;
  trustTier: string;
  observedAt?: string;
  generatedAt: string;
  facts: AgentResourceContextFact[];
  redactions?: AgentResourceContextRedaction[];
}

export interface AgentResourceFindingSnapshot {
  id: string;
  title: string;
  severity: string;
  category?: string;
  description?: string;
  impact?: string;
  recommendation?: string;
  confidence?: string;
  regressionCount: number;
  previousResolvedFixSummary?: string;
  detectedAt?: string;
  lastSeenAt?: string;
}

export interface AgentResourceApprovalSummary {
  id: string;
  riskLevel: string;
  requestedBy?: string;
  requestedAt: string;
  expiresAt: string;
  command?: string;
  commandRedacted?: boolean;
}

export interface AgentResourceActionSummary {
  id: string;
  capabilityName: string;
  state: string;
  success: boolean;
  errorMessage?: string;
  requestedBy?: string;
  createdAt: string;
  updatedAt: string;
  command?: string;
  commandRedacted?: boolean;
}

export interface AgentResourceContext {
  canonicalId: string;
  resourceType: string;
  resourceName: string;
  technology?: string;
  activeFindings: AgentResourceFindingSnapshot[];
  pendingApprovals: AgentResourceApprovalSummary[];
  recentActions: AgentResourceActionSummary[];
  contextSections: AgentResourceContextSection[];
  generatedAt: string;
}

export const AgentContextAPI = {
  getResourceContext(resourceId: string): Promise<AgentResourceContext> {
    return apiFetchJSON<AgentResourceContext>(
      `/api/agent/resource-context/${encodeURIComponent(resourceId)}`,
    );
  },
};
