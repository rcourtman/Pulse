import { apiFetchJSON } from '@/utils/apiClient';
import type {
  ActionAuditPlan,
  ActionDecisionResponse,
  ActionExecutionResponse,
  ResourceActionRequest,
} from '@/types/actionAudit';

export type ActionDecisionOutcome = 'approved' | 'rejected';

export class ResourceActionsAPI {
  static async planAction(request: ResourceActionRequest): Promise<ActionAuditPlan> {
    return apiFetchJSON<ActionAuditPlan>('/api/actions/plan', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }

  static async decideAction(
    actionId: string,
    outcome: ActionDecisionOutcome,
    reason?: string,
  ): Promise<ActionDecisionResponse> {
    return apiFetchJSON<ActionDecisionResponse>(
      `/api/actions/${encodeURIComponent(actionId)}/decision`,
      {
        method: 'POST',
        body: JSON.stringify({
          outcome,
          ...(reason ? { reason } : {}),
        }),
      },
    );
  }

  static async executeAction(actionId: string, reason?: string): Promise<ActionExecutionResponse> {
    return apiFetchJSON<ActionExecutionResponse>(
      `/api/actions/${encodeURIComponent(actionId)}/execute`,
      {
        method: 'POST',
        body: JSON.stringify(reason ? { reason } : {}),
      },
    );
  }
}
