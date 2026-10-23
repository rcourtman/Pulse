import { apiFetch } from '@/utils/apiClient';

export const PULSE_OPERATIONS_LOOP_WORKFLOW_PROMPT_NAME = 'pulse_operations_loop';
export const PULSE_PATROL_CONTROL_WORKFLOW_PROMPT_SURFACE = 'patrol_control';

export type WorkflowPromptActivitySurface =
  'pulse_patrol' | 'patrol_control' | 'patrol_autonomy' | 'pulse_pro_activation';

export interface WorkflowPromptActivityRequest {
  name: string;
  surface?: WorkflowPromptActivitySurface;
}

// Record that a first-party Pulse surface started a shared workflow prompt.
export async function recordWorkflowPromptActivity(
  request: WorkflowPromptActivityRequest,
): Promise<void> {
  await apiFetch('/api/ai/workflow-prompts/activity', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
}
