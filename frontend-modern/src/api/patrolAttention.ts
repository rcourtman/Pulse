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

/**
 * AttentionFlapping summarises an item whose lifecycle keeps switching between
 * open and resolved. Present only once the shared flapping threshold (four
 * transitions inside 24 hours) is crossed; the full transition list stays in
 * the detail timeline.
 */
export interface AttentionFlapping {
  transitionCount: number;
  windowHours: number;
  firstTransitionAt: string;
  lastTransitionAt: string;
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
  flapping?: AttentionFlapping;
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

export interface AttentionEvidenceResponse {
  evidence: EvidenceEnvelope;
  freshness: EvidenceFreshness;
  retained: boolean;
}

export interface PatrolWorkReceipt {
  actionId: string;
  resourceId: string;
  resourceName: string;
  resourceType?: string;
  capabilityName: string;
  verifiedAt: string;
  evidenceClass: 'none' | 'agent_attested' | 'independent';
  originSurface: 'patrol' | 'operational_trust_attention';
  findingId?: string;
  operationalRecordId?: string;
}

export interface PatrolWorkReceiptListResponse {
  data: PatrolWorkReceipt[];
  count: number;
  limit: number;
}

export interface AttentionMutationResponse {
  success: boolean;
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

export async function getPatrolWorkReceipts(limit = 6): Promise<PatrolWorkReceiptListResponse> {
  const search = new URLSearchParams({ limit: String(limit) });
  return apiFetchJSON<PatrolWorkReceiptListResponse>(
    `/api/ai/patrol/attention/receipts?${search.toString()}`,
  );
}

export async function getPatrolAttentionDetail(itemId: string): Promise<AttentionItemDetail> {
  return apiFetchJSON<AttentionItemDetail>(
    `/api/ai/patrol/attention/${encodeURIComponent(itemId)}`,
  );
}

export async function getPatrolAttentionEvidence(
  itemId: string,
  evidenceId: string,
): Promise<AttentionEvidenceResponse> {
  return apiFetchJSON<AttentionEvidenceResponse>(
    `/api/ai/patrol/attention/${encodeURIComponent(itemId)}/evidence/${encodeURIComponent(evidenceId)}`,
  );
}

async function mutatePatrolAttention(
  itemId: string,
  mutation: 'acknowledge' | 'unacknowledge' | 'suppress' | 'unsuppress',
  body = '{}',
): Promise<AttentionMutationResponse> {
  return apiFetchJSON<AttentionMutationResponse>(
    `/api/ai/patrol/attention/${encodeURIComponent(itemId)}/${mutation}`,
    {
      method: 'POST',
      body,
    },
  );
}

export async function acknowledgePatrolAttention(
  itemId: string,
): Promise<AttentionMutationResponse> {
  return mutatePatrolAttention(itemId, 'acknowledge');
}

export async function unacknowledgePatrolAttention(
  itemId: string,
): Promise<AttentionMutationResponse> {
  return mutatePatrolAttention(itemId, 'unacknowledge');
}

export async function suppressPatrolAttention(
  itemId: string,
  reason: string,
  expiresAt: string,
): Promise<AttentionMutationResponse> {
  return mutatePatrolAttention(
    itemId,
    'suppress',
    JSON.stringify({
      reason,
      expiresAt,
    }),
  );
}

export async function unsuppressPatrolAttention(
  itemId: string,
): Promise<AttentionMutationResponse> {
  return mutatePatrolAttention(itemId, 'unsuppress');
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
