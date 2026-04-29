export type ActionAuditState =
  | 'planned'
  | 'pending_approval'
  | 'approved'
  | 'rejected'
  | 'executing'
  | 'completed'
  | 'failed';

export type ActionAuditApprovalPolicy = 'none' | 'dry_run_only' | 'admin' | 'mfa' | string;

export interface ActionAuditRequest {
  requestId: string;
  resourceId: string;
  capabilityName: string;
  params?: Record<string, unknown>;
  reason: string;
  requestedBy: string;
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

export interface ActionAuditExecutionResult {
  success: boolean;
  output?: string;
  errorMessage?: string;
}

export interface ActionAuditRecord {
  id: string;
  createdAt: string;
  updatedAt: string;
  state: ActionAuditState;
  request: ActionAuditRequest;
  plan: ActionAuditPlan;
  approvals?: ActionAuditApprovalRecord[];
  result?: ActionAuditExecutionResult;
}

export interface ActionAuditListResponse {
  audits: ActionAuditRecord[];
  count: number;
  resourceId?: string;
  available: boolean;
}
