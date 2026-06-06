import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { assertAPIResponseOK } from './responseUtils';
import { logger } from '@/utils/logger';
import type { AIChatStreamEvent } from './generated/aiChatEvents';
import { consumeJSONEventStream } from './streaming';

// AI Chat API - Simplified AI interface

export interface ChatSession {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  message_count: number;
  handoff_summary?: ChatSessionHandoffSummary;
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

export interface AIStatus {
  running: boolean;
  engine: string;
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

// File change from a session
export interface FileChange {
  path: string;
  status: 'added' | 'modified' | 'deleted';
  added: number;
  removed: number;
}

// Session diff showing all file changes
export interface SessionDiff {
  files: FileChange[];
  summary?: string;
}

export class AIChatAPI {
  private static baseUrl = '/api/ai';

  // Get AI status
  static async getStatus(): Promise<AIStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<AIStatus>;
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
  static async summarizeSession(
    sessionId: string,
  ): Promise<{ success: boolean; message?: string }> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/summarize`, {
      method: 'POST',
    }) as Promise<{ success: boolean; message?: string }>;
  }

  // Get file changes/diff for a session
  static async getSessionDiff(sessionId: string): Promise<SessionDiff> {
    return apiFetchJSON(
      `${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/diff`,
    ) as Promise<SessionDiff>;
  }

  // Fork a session (create a branch point)
  static async forkSession(sessionId: string): Promise<ChatSession> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/fork`, {
      method: 'POST',
    }) as Promise<ChatSession>;
  }

  // Revert session changes
  static async revertSession(sessionId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/revert`, {
      method: 'POST',
    }) as Promise<{ success: boolean }>;
  }

  // Unrevert session changes (redo)
  static async unrevertSession(sessionId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${encodeURIComponent(sessionId)}/unrevert`, {
      method: 'POST',
    }) as Promise<{ success: boolean }>;
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
  ): Promise<void> {
    logger.debug('[AI Chat] Starting chat stream', { prompt: prompt.substring(0, 50) });

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
      },
      onComplete: () => {
        onEvent({ type: 'done' });
      },
      yieldBetweenEvents: (event) => event.type !== 'content' && event.type !== 'thinking',
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
