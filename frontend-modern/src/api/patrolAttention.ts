import { apiFetchJSON } from '@/utils/apiClient';
import type {
  EvidenceCompleteness,
  EvidenceEnvelope,
  EvidenceFreshness,
  LifecycleTransition,
  OperationalRecord,
  OperationalSeverity,
  OperationalState,
} from '@/types/operationalTrust';
import type { ProtectionPosture } from '@/types/recovery';
import type { ActionAuditPlan } from '@/types/actionAudit';

export type AttentionFilter =
  'active' | 'open' | 'acknowledged' | 'suppressed' | 'stale_unknown' | 'resolved' | 'all';

export type AttentionVerificationState =
  'not_available' | 'pending' | 'succeeded' | 'failed' | 'unknown';

export interface AttentionActionOffer {
  actionId?: string;
  targetResourceId: string;
  capability: string;
  kind: string;
  label: string;
  mode: 'plan' | 'dry-run' | 'execute';
  risk: string;
  approval: 'not-required' | 'required' | 'granted' | 'denied';
  eligibility: 'eligible' | 'ineligible' | 'unknown';
  reasons: string[];
  evidenceIds: string[];
  expectedPostcondition: string;
  verificationPolicy: string;
  requiresApproval: boolean;
}

export interface AttentionResource {
  resourceId: string;
}

export interface AttentionItem {
  id: string;
  operationalRecordId: string;
  subjectResourceId: string;
  subjectResourceName: string;
  subjectResourceType?: string;
  kind: string;
  title: string;
  plainLanguageSummary: string;
  severity: OperationalSeverity;
  state: OperationalState;
  firstObservedAt: string;
  lastObservedAt: string;
  evidenceFreshness: EvidenceFreshness;
  evidenceCompleteness: EvidenceCompleteness;
  impact?: string;
  protectionPosture?: ProtectionPosture;
  relatedResources: AttentionResource[];
  recommendedNextStep?: string;
  availableActions: AttentionActionOffer[];
  verificationState: AttentionVerificationState;
}

export interface AttentionItemDetail {
  item: AttentionItem;
  operationalRecord: OperationalRecord;
  timeline: LifecycleTransition[];
  evidence: EvidenceEnvelope[];
}

export interface AttentionSummary {
  activeCount: number;
  openCount: number;
  acknowledgedCount: number;
  suppressedCount: number;
  uncertainCount: number;
  resolvedCount: number;
  calm: boolean;
  coverageState: 'current' | 'partial' | 'unavailable';
  evaluatedAt: string;
}

export interface AttentionListResponse {
  data: AttentionItem[];
  summary: AttentionSummary;
  meta: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
  };
}

export async function getPatrolAttention(
  filter: AttentionFilter = 'active',
  page = 1,
  limit = 50,
): Promise<AttentionListResponse> {
  const search = new URLSearchParams({
    filter,
    page: String(page),
    limit: String(limit),
  });
  return apiFetchJSON<AttentionListResponse>(`/api/ai/patrol/attention?${search.toString()}`);
}

export async function getPatrolAttentionSummary(): Promise<AttentionSummary> {
  return apiFetchJSON<AttentionSummary>('/api/ai/patrol/attention/summary');
}

export async function getPatrolAttentionDetail(itemId: string): Promise<AttentionItemDetail> {
  return apiFetchJSON<AttentionItemDetail>(
    `/api/ai/patrol/attention/${encodeURIComponent(itemId)}`,
  );
}

export async function planPatrolAttentionAction(
  itemId: string,
  capability: string,
): Promise<ActionAuditPlan> {
  return apiFetchJSON<ActionAuditPlan>(
    `/api/ai/patrol/attention/${encodeURIComponent(itemId)}/actions/${encodeURIComponent(capability)}/plan`,
    {
      method: 'POST',
      body: '{}',
    },
  );
}
