import { apiFetchJSON } from '@/utils/apiClient';

const ALERT_INTENT_POLICIES_PATH = '/api/alerts/intent-policies';

export type AlertIntentSignal =
  '*' | 'state.offline' | 'incident.availability' | `metric.${string}`;

export interface BackupOfflineIntentPolicy {
  enabled: boolean;
  postGraceSeconds?: number;
  maxDeferralSeconds?: number;
}

export interface AlertIntentRule {
  graceSeconds?: number;
  honorOperatorState?: boolean;
  backupOffline?: BackupOfflineIntentPolicy;
}

export interface AlertIntentPolicyDocument {
  schemaVersion: number;
  revision: number;
  updatedAt?: string;
  defaults?: Record<string, AlertIntentRule>;
  resourceTypes?: Record<string, Record<string, AlertIntentRule>>;
  resources?: Record<string, Record<string, AlertIntentRule>>;
}

export interface AlertIntentPolicyPreviewRequest {
  resourceId: string;
  resourceType: string;
  signal: AlertIntentSignal;
  conditionActive: boolean;
  firstMatchedAt?: string;
  backupActive?: boolean;
  backupObservedAt?: string;
}

export interface AlertIntentPolicyPreview {
  resourceId: string;
  resourceType: string;
  signal: string;
  status: 'clear' | 'expected_transient' | 'pending_grace' | 'would_activate';
  reason: string;
  effective: {
    graceSeconds: number;
    honorOperatorState: boolean;
    backupOffline?: BackupOfflineIntentPolicy;
    sources: Record<string, string>;
    explicit: boolean;
  };
  firstMatchedAt?: string;
  eligibleAt?: string;
  hardCapAt?: string;
  remainingSeconds?: number;
  contexts: Array<{
    kind: string;
    active: boolean;
    evidence?: string;
    observedAt?: string;
    expiresAt?: string;
  }>;
  warnings: string[];
}

export class AlertIntentPoliciesAPI {
  static async get(): Promise<AlertIntentPolicyDocument> {
    return apiFetchJSON(ALERT_INTENT_POLICIES_PATH);
  }

  static async update(document: AlertIntentPolicyDocument): Promise<AlertIntentPolicyDocument> {
    return apiFetchJSON(ALERT_INTENT_POLICIES_PATH, {
      method: 'PUT',
      body: JSON.stringify(document),
    });
  }

  static async preview(
    request: AlertIntentPolicyPreviewRequest,
  ): Promise<AlertIntentPolicyPreview> {
    return apiFetchJSON(`${ALERT_INTENT_POLICIES_PATH}/preview`, {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }
}
