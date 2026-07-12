import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { assertAPIResponseOK } from './responseUtils';
import { logger } from '@/utils/logger';
import type { AIChatStreamEvent } from './generated/aiChatEvents';
import type { AgentSurfaceToolContract } from './agentCapabilities';
import { consumeJSONEventStream } from './streaming';
import { maybeRunAIChatDevStreamFixture } from './aiChatDevStreamFixture';

// AI Chat API - Simplified AI interface

export interface ChatSession {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  message_count: number;
  can_redo?: boolean;
  /** Pulse-owned background run (Patrol detection/eval/investigation), not a resumable user chat. */
  system?: boolean;
  handoff_summary?: ChatSessionHandoffSummary;
}

export interface ChatSessionUndoResult {
  success: boolean;
  session_id: string;
  restored_prompt?: string;
  removed_messages?: number;
  can_redo: boolean;
  message?: string;
}

export interface ChatSessionRedoResult {
  success: boolean;
  session_id: string;
  restored_messages?: number;
  can_redo: boolean;
  message?: string;
}

export interface ChatSessionCompactionResult {
  success: boolean;
  status: 'compacted' | 'not_needed' | 'empty' | string;
  message?: string;
  session_id: string;
  summary_message_id?: string;
  original_message_count?: number;
  compacted_message_count?: number;
  compacted_messages?: number;
  kept_recent_messages?: number;
  summary_chars?: number;
}

export interface ListChatSessionsOptions {
  search?: string;
  limit?: number;
}

export type ChatMentionType = 'vm' | 'system-container' | 'app-container' | 'agent' | 'storage';

export interface ChatMention {
  id: string;
  name: string;
  type: ChatMentionType;
  node?: string;
}

export interface ChatHandoffResource {
  id: string;
  name?: string;
  type?: string;
  node?: string;
}

export interface ChatSessionHandoffResource {
  id?: string;
  name?: string;
  type?: string;
  node?: string;
}

export interface ChatSessionHandoffSummary {
  kind?: string;
  finding_id?: string;
  run_id?: string;
  run_type?: string;
  run_status?: string;
  runtime_failure?: boolean;
  has_model_context: boolean;
  resource_count?: number;
  primary_resource?: ChatSessionHandoffResource;
  action_count?: number;
  requires_approval?: boolean;
  last_known_approval_status?: string;
  last_known_action_state?: string;
  last_known_action_risk?: string;
  updated_at?: string;
}

export interface ChatHandoffMetadata {
  kind?: string;
  runId?: string;
  runType?: string;
  runStatus?: string;
  runtimeFailure?: boolean;
}

export interface ChatHandoffAction {
  findingId?: string;
  recordId?: string;
  approvalId?: string;
  approvalStatus?: string;
  approvalRequestedAt?: string;
  approvalExpiresAt?: string;
  approvalDecidedAt?: string;
  approvalConsumed?: boolean;
  actionId?: string;
  actionState?: string;
  actionUpdatedAt?: string;
  actionRequestedBy?: string;
  actionCapability?: string;
  actionApprovalPolicy?: string;
  actionRequiresApproval?: boolean;
  actionPlanExpiresAt?: string;
  actionPlanMessage?: string;
  actionPreflight?: string;
  actionDryRunSummary?: string;
  actionResult?: string;
  fixId?: string;
  description?: string;
  riskLevel?: string;
  destructive?: boolean;
  targetHost?: string;
  targetResourceId?: string;
  targetResourceName?: string;
  targetResourceType?: string;
  targetNode?: string;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  model?: string;
  tool_calls?: ToolCall[];
}

export interface ToolCall {
  name: string;
  input?: string | Record<string, unknown>;
  output?: string;
  success?: boolean;
}

export type StreamEvent = AIChatStreamEvent;

export interface AIChatStreamLifecycle {
  onStreamOpen?: () => void;
}

const AI_CHAT_STREAM_TEXT_PAINT_CHECKPOINT_INTERVAL = 4;
const AI_CHAT_STREAM_TEXT_EVENT_TYPES = new Set(['content', 'thinking']);

// The backend writes an SSE ": heartbeat" comment every 5 seconds while a
// turn is in flight, so a healthy connection never goes quiet for long. If no
// bytes arrive for this window the connection is dead (backend restart,
// half-open socket) and the turn must fail fast instead of leaving the user
// staring at "Generating response" for the old 300s ceiling.
export const AI_CHAT_STREAM_INACTIVITY_TIMEOUT_MS = 30000;

const isAIChatStreamTextEvent = (event: Pick<StreamEvent, 'type'>): boolean =>
  AI_CHAT_STREAM_TEXT_EVENT_TYPES.has(event.type);

export const createAIChatStreamPaintCheckpointPredicate = () => {
  let textEventsSinceCheckpoint = 0;

  return (event: Pick<StreamEvent, 'type'>): boolean => {
    if (!isAIChatStreamTextEvent(event)) {
      textEventsSinceCheckpoint = 0;
      return true;
    }

    textEventsSinceCheckpoint += 1;
    if (
      textEventsSinceCheckpoint === 1 ||
      textEventsSinceCheckpoint >= AI_CHAT_STREAM_TEXT_PAINT_CHECKPOINT_INTERVAL
    ) {
      if (textEventsSinceCheckpoint >= AI_CHAT_STREAM_TEXT_PAINT_CHECKPOINT_INTERVAL) {
        textEventsSinceCheckpoint = 0;
      }
      return true;
    }

    return false;
  };
};

export interface AIStatus {
  running: boolean;
  engine: string;
}

export interface AssistantWorkflowPromptRenderRequest {
  name: string;
  arguments?: Record<string, string>;
}

export const PULSE_OPERATIONS_LOOP_WORKFLOW_PROMPT_NAME = 'pulse_operations_loop';
export const PULSE_PATROL_WORKFLOW_PROMPT_SURFACE = 'pulse_patrol';
export const PULSE_PATROL_CONTROL_WORKFLOW_PROMPT_SURFACE = 'patrol_control';
export const PULSE_PATROL_AUTONOMY_WORKFLOW_PROMPT_SURFACE = 'patrol_autonomy';
export const PULSE_PRO_ACTIVATION_WORKFLOW_PROMPT_SURFACE = 'pulse_pro_activation';

export type AssistantWorkflowPromptActivitySurface =
  | 'pulse_assistant'
  | 'pulse_patrol'
  | 'patrol_control'
  | 'patrol_autonomy'
  | 'pulse_pro_activation';

export interface AssistantWorkflowPromptActivityRequest {
  name: string;
  surface?: AssistantWorkflowPromptActivitySurface;
}

export interface AssistantWorkflowPromptRenderResponse {
  description: string;
  text: string;
}

// AI Agent (build, code, etc.) with specific permissions and model
export interface Agent {
  name: string;
  description?: string;
  mode: 'subagent' | 'primary' | 'all';
  native?: boolean;
  hidden?: boolean;
  color?: string;
  model?: {
    providerID: string;
    modelID: string;
  };
}

export class AIChatAPI {
  private static baseUrl = '/api/ai';

  // Get AI status
  static async getStatus(): Promise<AIStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<AIStatus>;
  }

  // Get the live native Assistant surface tool contract for the current runtime mode.
  static async getAssistantSurfaceTools(): Promise<AgentSurfaceToolContract> {
    return apiFetchJSON(
      `${this.baseUrl}/assistant/surface-tools`,
    ) as Promise<AgentSurfaceToolContract>;
  }

  // Render a shared Pulse Intelligence workflow prompt for the native Assistant surface.
  static async renderWorkflowPrompt(
    request: AssistantWorkflowPromptRenderRequest,
  ): Promise<AssistantWorkflowPromptRenderResponse> {
    return apiFetchJSON(`${this.baseUrl}/workflow-prompts/render`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    }) as Promise<AssistantWorkflowPromptRenderResponse>;
  }

  // Record that a first-party Pulse surface started a shared workflow prompt.
  static async recordWorkflowPromptActivity(
    request: AssistantWorkflowPromptActivityRequest,
  ): Promise<void> {
    await apiFetch(`${this.baseUrl}/workflow-prompts/activity`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    });
  }

  // List all chat sessions. Normalizes a null/non-array payload (e.g. a server
  // error body) to an empty array so callers can rely on array semantics and do
  // not crash on .length/.some()/.map() (#1149).
  static async listSessions(options: ListChatSessionsOptions = {}): Promise<ChatSession[]> {
    const params = new URLSearchParams();
    const search = options.search?.trim();
    if (search) params.set('search', search);
    if (typeof options.limit === 'number' && Number.isFinite(options.limit) && options.limit > 0) {
      params.set('limit', String(Math.floor(options.limit)));
    }
    const suffix = params.toString();
    const value = await apiFetchJSON(`${this.baseUrl}/sessions${suffix ? `?${suffix}` : ''}`);
    return Array.isArray(value) ? (value as ChatSession[]) : [];
  }

  // Create a new session
  static async createSession(): Promise<ChatSession> {
    return apiFetchJSON(`${this.baseUrl}/sessions`, {
      method: 'POST',
    }) as Promise<ChatSession>;
  }

  // Delete a session
  static async deleteSession(sessionId: string): Promise<void> {
    await apiFetch(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}`, {
      method: 'DELETE',
    });
  }

  // Rename a session
  static async renameSession(sessionId: string, title: string): Promise<ChatSession> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    }) as Promise<ChatSession>;
  }

  // Get messages for a session
  static async getMessages(sessionId: string): Promise<ChatMessage[]> {
    return apiFetchJSON(
      `${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/messages`,
    ) as Promise<ChatMessage[]>;
  }

  // Abort a session
  static async abortSession(sessionId: string): Promise<void> {
    await apiFetch(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/abort`, {
      method: 'POST',
    });
  }

  // Approve a pending command
  static async approveCommand(approvalId: string): Promise<{ approved: boolean; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/approvals/${encodeURIComponent(approvalId)}/approve`, {
      method: 'POST',
    }) as Promise<{ approved: boolean; message: string }>;
  }

  // Deny a pending command
  static async denyCommand(
    approvalId: string,
    reason?: string,
  ): Promise<{ denied: boolean; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/approvals/${encodeURIComponent(approvalId)}/deny`, {
      method: 'POST',
      body: JSON.stringify({ reason: reason || 'User skipped' }),
    }) as Promise<{ denied: boolean; message: string }>;
  }

  // Answer a pending question from the AI chat
  static async answerQuestion(
    questionId: string,
    answers: Array<{ id: string; value: string }>,
  ): Promise<void> {
    await apiFetch(`${this.baseUrl}/question/${encodeURIComponent(questionId)}/answer`, {
      method: 'POST',
      body: JSON.stringify({ answers }),
    });
  }

  // ============================================
  // AI Chat Extended Features
  // ============================================

  // List available agents (build, code, etc.)
  static async listAgents(): Promise<Agent[]> {
    return apiFetchJSON(`${this.baseUrl}/agents`) as Promise<Agent[]>;
  }

  // Summarize a session (compress context when nearing limits)
  static async summarizeSession(sessionId: string): Promise<ChatSessionCompactionResult> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/summarize`, {
      method: 'POST',
    }) as Promise<ChatSessionCompactionResult>;
  }

  // Fork a session (create a branch point)
  static async forkSession(sessionId: string): Promise<ChatSession> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/fork`, {
      method: 'POST',
    }) as Promise<ChatSession>;
  }

  // Undo latest chat turn and return the user prompt so the composer can restore it.
  // Retry/regenerate passes expectedPrompt so a stale retry can never remove a
  // turn other than the one being re-run.
  static async undoLastTurn(
    sessionId: string,
    options?: { expectedPrompt?: string },
  ): Promise<ChatSessionUndoResult> {
    const expectedPrompt = options?.expectedPrompt?.trim();
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/undo`, {
      method: 'POST',
      ...(expectedPrompt
        ? {
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ expected_prompt: expectedPrompt }),
          }
        : {}),
    }) as Promise<ChatSessionUndoResult>;
  }

  // Redo the latest chat turn removed by undoLastTurn.
  static async redoLastTurn(sessionId: string): Promise<ChatSessionRedoResult> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/redo`, {
      method: 'POST',
    }) as Promise<ChatSessionRedoResult>;
  }

  // Stream chat - the main chat interface
  static async chat(
    prompt: string,
    sessionId: string | undefined,
    model: string | undefined,
    onEvent: (event: StreamEvent) => void,
    signal?: AbortSignal,
    mentions?: ChatMention[],
    findingId?: string,
    autonomousMode?: boolean,
    handoffContext?: string,
    handoffResources?: ChatHandoffResource[],
    handoffActions?: ChatHandoffAction[],
    handoffMetadata?: ChatHandoffMetadata,
    lifecycle?: AIChatStreamLifecycle,
  ): Promise<void> {
    logger.debug('[AI Chat] Starting chat stream', { prompt: prompt.substring(0, 50) });

    if (await maybeRunAIChatDevStreamFixture({ prompt, model, onEvent, signal })) {
      return;
    }

    const body: Record<string, unknown> = {
      prompt,
      session_id: sessionId,
      model,
    };
    if (mentions && mentions.length > 0) {
      body.mentions = mentions;
    }
    if (findingId) {
      body.finding_id = findingId;
    }
    if (typeof autonomousMode === 'boolean') {
      body.autonomous_mode = autonomousMode;
    }
    if (handoffContext && handoffContext.trim()) {
      body.handoff_context = handoffContext;
    }
    if (handoffResources && handoffResources.length > 0) {
      body.handoff_resources = handoffResources;
    }
    if (handoffActions && handoffActions.length > 0) {
      body.handoff_actions = handoffActions.map(serializeChatHandoffAction);
    }
    if (handoffMetadata) {
      body.handoff_metadata = {
        kind: handoffMetadata.kind,
        run_id: handoffMetadata.runId,
        run_type: handoffMetadata.runType,
        run_status: handoffMetadata.runStatus,
        runtime_failure: handoffMetadata.runtimeFailure,
      };
    }

    const response = await apiFetch(`${this.baseUrl}/chat`, {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      signal,
    });

    await assertAPIResponseOK(response, `Request failed with status ${response.status}`);
    lifecycle?.onStreamOpen?.();

    await consumeJSONEventStream<StreamEvent>(response, {
      onEvent: (event) => {
        logger.debug('[AI Chat] Event', { type: event.type });
        onEvent(event);
        return event.type === 'done' || event.type === 'error';
      },
      onParseError: (line) => {
        logger.error('[AI Chat] Failed to parse event', { line });
      },
      onTrailingParseError: () => {
        logger.warn('[AI Chat] Could not parse remaining buffer');
      },
      onTimeout: () => {
        logger.warn('[AI Chat] Stream timeout');
        throw new Error('Pulse Assistant stream timed out waiting for provider data.');
      },
      onComplete: () => {
        // The backend always terminates a chat stream with an explicit done or
        // error event. A clean close without one means the connection was
        // severed mid-turn (backend restart, proxy drop) — surface an
        // interruption the user can retry instead of pretending the turn
        // finished.
        onEvent({
          type: 'error',
          data: { message: 'Connection to Pulse closed before the response finished.' },
        } as StreamEvent);
      },
      timeoutMs: AI_CHAT_STREAM_INACTIVITY_TIMEOUT_MS,
      yieldBetweenEvents: createAIChatStreamPaintCheckpointPredicate(),
    });
  }
}

function serializeChatHandoffAction(action: ChatHandoffAction): Record<string, unknown> {
  return {
    finding_id: action.findingId,
    record_id: action.recordId,
    approval_id: action.approvalId,
    approval_status: action.approvalStatus,
    approval_requested_at: action.approvalRequestedAt,
    approval_expires_at: action.approvalExpiresAt,
    approval_decided_at: action.approvalDecidedAt,
    approval_consumed: action.approvalConsumed,
    action_id: action.actionId,
    action_state: action.actionState,
    action_updated_at: action.actionUpdatedAt,
    action_requested_by: action.actionRequestedBy,
    action_capability: action.actionCapability,
    action_approval_policy: action.actionApprovalPolicy,
    action_requires_approval: action.actionRequiresApproval,
    action_plan_expires_at: action.actionPlanExpiresAt,
    action_plan_message: action.actionPlanMessage,
    action_preflight: action.actionPreflight,
    action_dry_run_summary: action.actionDryRunSummary,
    action_result: action.actionResult,
    fix_id: action.fixId,
    description: action.description,
    risk_level: action.riskLevel,
    destructive: action.destructive,
    target_host: action.targetHost,
    target_resource_id: action.targetResourceId,
    target_resource_name: action.targetResourceName,
    target_resource_type: action.targetResourceType,
    target_node: action.targetNode,
  };
}
