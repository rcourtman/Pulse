export type ActionAuditState =
  'planned' | 'pending_approval' | 'approved' | 'rejected' | 'executing' | 'completed' | 'failed';

export type ActionAuditApprovalPolicy = 'none' | 'dry_run_only' | 'admin' | 'mfa' | string;

export interface ActionAuditRequest {
  requestId: string;
  resourceId: string;
  capabilityName: string;
  params?: Record<string, unknown>;
  reason: string;
  requestedBy: string;
}

export type ResourceActionRequest = ActionAuditRequest;

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
  predictedBlastRadius?: string[];
  rollbackAvailable: boolean;
  message?: string;
  plannedAt?: string;
  expiresAt?: string;
  resourceVersion?: string;
  policyVersion?: string;
  planHash?: string;
  preflight?: ActionAuditPreflight;
}

export interface ActionAuditApprovalRecord {
  actor: string;
  method: string;
  timestamp: string;
  outcome: 'approved' | 'rejected' | string;
  reason?: string;
}

// ActionVerificationResult mirrors the Go type that records the outcome of
// the broker's post-dispatch read-after-write check. It is best-effort:
// when no verification command is derivable for the action class, ran is
// false and the rest of the fields are empty rather than fabricated.
export interface ActionVerificationResult {
  ran: boolean;
  command?: string;
  output?: string;
  success: boolean;
  ranAt?: string;
  note?: string;
}

export interface ActionAuditExecutionResult {
  success: boolean;
  output?: string;
  errorMessage?: string;
  verification?: ActionVerificationResult;
}

export type ActionVerificationStatus = 'unknown' | 'verified' | 'unverified' | 'failed' | string;

export type ActionAuditRefusalPrefix =
  'plan_drift:' | 'action_plan_expired:' | 'action_dry_run_only:' | 'resource_remediation_locked:';

export interface ActionVerificationOutcome {
  status: ActionVerificationStatus;
  evidenceSummary?: string;
}

export interface ActionAuditRecord {
  id: string;
  createdAt: string;
  updatedAt: string;
  state: ActionAuditState;
  request: ActionAuditRequest;
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

// PatrolActionReference is the compact investigation projection of the
// canonical action audit. Lifecycle state and proposal parameters remain
// authoritative in the action API; Patrol never reconstructs command fixes.
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
}

export interface ActionAuditListResponse {
  audits: ActionAuditRecord[];
  count: number;
  resourceId?: string;
  available: boolean;
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
