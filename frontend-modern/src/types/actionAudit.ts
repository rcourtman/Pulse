export type ActionAuditState =
  | 'planned'
  | 'pending_approval'
  | 'approved'
  | 'rejected'
  | 'expired'
  | 'executing'
  | 'completed'
  | 'failed';

export type ActionAuditApprovalPolicy = 'none' | 'dry_run_only' | 'admin' | 'mfa';

export type ActionActorKind = 'user' | 'api_token' | 'service' | 'policy';

export interface ActionActor {
  subjectId: string;
  kind: ActionActorKind;
  credentialId: string;
  orgId: string;
}

export interface ActionAuditRequest {
  requestId: string;
  resourceId: string;
  capabilityName: string;
  params?: Record<string, unknown>;
  reason: string;
  requestedBy: string;
  actor?: ActionActor;
}

export interface ActionResourceReference {
  id: string;
  name: string;
  type: string;
}

export type ResourceActionRequest = Omit<ActionAuditRequest, 'actor'>;

export interface ActionApprovalRequirement {
  version: number;
  floor: ActionAuditApprovalPolicy;
  quorum: number;
  disallowRequester: boolean;
}

export type ActionPolicyDecisionStatus = 'resolved' | 'legacy_unknown';
export type ActionPolicyAuthorityKind =
  'capability_registry' | 'tenant_patrol_policy' | 'resource_operator_policy';
export type ActionPolicyAuthorityStatus = 'consulted' | 'unavailable' | 'not_found';
export type ActionPolicyReasonCode =
  | 'capability_approval_none'
  | 'capability_approval_admin'
  | 'capability_approval_mfa'
  | 'capability_dry_run_only'
  | 'capability_auto_never'
  | 'capability_auto_low_risk'
  | 'capability_auto_elevated'
  | 'tenant_policy_unavailable'
  | 'tenant_emergency_stop'
  | 'tenant_mode_monitor'
  | 'tenant_mode_assisted'
  | 'tenant_mode_full'
  | 'tenant_mode_unknown'
  | 'tenant_full_mode_locked'
  | 'tenant_full_mode_unlocked'
  | 'resource_policy_unavailable'
  | 'resource_policy_missing'
  | 'resource_never_auto_remediate'
  | 'resource_capability_allowed'
  | 'resource_capability_not_allowed'
  | 'resource_window_open'
  | 'resource_window_closed';

export interface ActionPolicyDecisionScope {
  orgId: string;
  resourceId: string;
  capabilityName: string;
}

export interface ActionPolicyAuthorityFactor {
  kind: ActionPolicyAuthorityKind;
  sourceId: string;
  revision?: string;
  status: ActionPolicyAuthorityStatus;
  scope: ActionPolicyDecisionScope;
  approvalFloor?: ActionAuditApprovalPolicy;
  reasonCodes: ActionPolicyReasonCode[];
}

export interface ActionPolicyDecisionProvenance {
  version: number;
  status: ActionPolicyDecisionStatus;
  decisionId?: string;
  actionId?: string;
  scope: ActionPolicyDecisionScope;
  authorities: ActionPolicyAuthorityFactor[];
  approvalRequirement: ActionApprovalRequirement;
  planningAllowed: boolean;
  requiresApproval: boolean;
}

export interface ActionAuditPreflight {
  target?: string;
  currentState?: string;
  intendedChange?: string;
  dryRunAvailable: boolean;
  dryRunSummary?: string;
  safetyChecks?: string[];
  verificationSteps?: string[];
  generatedAt?: string;
}

export interface ActionAuditPlan {
  actionId: string;
  requestId: string;
  allowed: boolean;
  requiresApproval: boolean;
  approvalPolicy: ActionAuditApprovalPolicy;
  approvalRequirement?: ActionApprovalRequirement;
  predictedBlastRadius?: string[];
  rollbackAvailable: boolean;
  message?: string;
  plannedAt?: string;
  expiresAt?: string;
  resourceVersion?: string;
  policyVersion?: string;
  policyDecision?: ActionPolicyDecisionProvenance;
  planHash?: string;
  preflight?: ActionAuditPreflight;
}

export interface ActionAuditApprovalRecord {
  actor: string;
  method: string;
  timestamp: string;
  outcome: 'approved' | 'rejected';
  reason?: string;
}

export interface ActionVerificationResult {
  ran: boolean;
  command?: string;
  output?: string;
  success: boolean;
  ranAt?: string;
  note?: string;
}

export type ActionExecutionStatus = 'not_run' | 'succeeded' | 'failed' | 'inconclusive';
export type ActionVerificationTruthStatus =
  'not_attempted' | 'confirmed' | 'contradicted' | 'inconclusive';
export type ActionEvidenceClass = 'none' | 'agent_attested' | 'independent';

export interface ActionEvidenceRef {
  id: string;
  kind: string;
  digest: string;
}

export interface ActionEvidence {
  version: number;
  id: string;
  observerId: string;
  observerKind: string;
  observerTrustDomain: string;
  executorTrustDomain: string;
  method: string;
  subjectId: string;
  observedAt: string;
  receivedAt: string;
  reasonCode?: string;
  summary?: string;
  refs?: ActionEvidenceRef[];
  digest: string;
}

export interface ActionExecutionTruth {
  status: ActionExecutionStatus;
  reasonCode?: string;
  summary?: string;
}

export interface ActionVerificationTruth {
  status: ActionVerificationTruthStatus;
  evidenceClass: ActionEvidenceClass;
  reasonCode?: string;
  summary?: string;
  evidence?: ActionEvidence[];
}

export type ActionCompensationSupport = 'unavailable' | 'declared';
export type ActionCompensationStatus =
  | 'not_available'
  | 'not_needed'
  | 'not_attempted'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'inconclusive';

export interface ActionRestoredState {
  subjectId: string;
  expectedDigest: string;
  observedDigest: string;
  observedAt: string;
}

export interface ActionCompensationTruth {
  support: ActionCompensationSupport;
  strategy?: string;
  trigger?: string;
  status: ActionCompensationStatus;
  reasonCode?: string;
  summary?: string;
  attemptId?: string;
  stepId?: string;
  startedAt?: string;
  completedAt?: string;
  evidence?: ActionEvidence[];
  execution?: ActionExecutionTruth;
  verification?: ActionVerificationTruth;
  restoredState?: ActionRestoredState;
}

export interface ActionResultV2 {
  version: 2;
  execution: ActionExecutionTruth;
  verification: ActionVerificationTruth;
  compensation: ActionCompensationTruth;
}

export interface ActionAuditExecutionResult {
  success: boolean;
  output?: string;
  errorMessage?: string;
  verification?: ActionVerificationResult;
  actionResultV2?: ActionResultV2;
}

export type ActionVerificationStatus = 'unknown' | 'verified' | 'unverified' | 'failed';

export type ActionAuditRefusalPrefix =
  | 'plan_drift:'
  | 'action_plan_expired:'
  | 'action_dry_run_only:'
  | 'resource_remediation_locked:'
  | 'policy_authorization_expired:'
  | 'policy_authorization_invalid:'
  | 'policy_authorization_revoked:'
  | 'action_emergency_stop:'
  | 'action_replan_required:';

export interface ActionVerificationOutcome {
  status: ActionVerificationStatus;
  evidenceSummary?: string;
}

export interface ActionAuditRecord {
  id: string;
  createdAt: string;
  updatedAt: string;
  state: ActionAuditState;
  decisionRevision?: number;
  request: ActionAuditRequest;
  /** Read-time display metadata from the canonical resource API; never part of plan identity. */
  resource?: ActionResourceReference;
  /** Read-time auto-authorization class of the planned capability; presentation only. */
  capabilityAutoAuthorization?: 'never' | 'low_risk' | 'elevated';
  /** Read-time names for plan.predictedBlastRadius IDs; entries may carry an empty name. */
  blastRadius?: ActionResourceReference[];
  plan: ActionAuditPlan;
  origin?: ActionAuditOrigin;
  approvals?: ActionAuditApprovalRecord[];
  result?: ActionAuditExecutionResult;
  verification?: ActionVerificationResult;
  verificationOutcome?: ActionVerificationOutcome;
}

export interface ActionAuditOrigin {
  surface: string;
  findingId?: string;
  investigationId?: string;
  proposalId?: string;
}

export interface PatrolActionReference {
  action_id: string;
  proposal_id?: string;
  resource_id: string;
  capability_name: string;
  state: ActionAuditState;
  plan: ActionAuditPlan;
}

export interface PendingActionsResponse {
  actions: ActionAuditRecord[];
  count: number;
  readOnly?: boolean;
}

export type ActionInboxView = 'pending' | 'settled';
export interface ActionInboxResponse extends PendingActionsResponse {
  view: ActionInboxView;
}

export interface ActionAuditListResponse {
  audits: ActionAuditRecord[];
  count: number;
  resourceId?: string;
  available: boolean;
}

export interface ActionLifecycleEvent {
  actionId: string;
  timestamp: string;
  kind: 'transition' | 'decision' | 'legacy';
  state: ActionAuditState;
  decisionRevision?: number;
  decision?: ActionAuditApprovalRecord;
  actor?: string;
  message?: string;
}

export type ActionDispatchState = 'queued' | 'claimed' | 'receipt_pending' | 'receipt_recorded';
export interface ActionDispatchAttempt {
  id: string;
  actionId: string;
  state: ActionDispatchState;
  createdAt: string;
  updatedAt: string;
  leaseOwner?: string;
  leaseExpiresAt?: string;
  dispatchCount: number;
}

export interface ActionDispatchReceipt {
  attemptId: string;
  actionId: string;
  transportRequestId: string;
  receivedAt: string;
}

export interface ActionDetailResponse {
  audit: ActionAuditRecord;
  events: ActionLifecycleEvent[];
  attempt?: ActionDispatchAttempt;
  receipt?: ActionDispatchReceipt;
  readOnly?: boolean;
}

export interface ActionDecisionResponse {
  actionId: string;
  state: ActionAuditState;
  approval: ActionAuditApprovalRecord;
  audit: ActionAuditRecord;
}

export interface ActionExecutionResponse {
  actionId: string;
  state: ActionAuditState;
  result?: ActionAuditExecutionResult;
  audit: ActionAuditRecord;
}
