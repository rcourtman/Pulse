// Chat component types
import type {
  ChatHandoffAction,
  ChatHandoffMetadata,
  ChatHandoffResource,
  ChatMention,
} from '@/api/aiChat';

export interface ToolExecution {
  name: string;
  input: string;
  rawInput?: string;
  output: string;
  success: boolean;
}

export interface PendingTool {
  id: string;
  name: string;
  input: string;
  rawInput?: string;
  status?: 'pending' | 'running' | 'waiting';
  progress?: string;
  startedAt?: number;
  // When execution actually began (first 'running' phase). startedAt marks
  // the model starting to stream tool arguments, which can run minutes ahead
  // of execution on slow routes — durations must not bill that to the tool.
  runningAt?: number;
  updatedAt?: number;
}

export interface ToolCancellation {
  id: string;
  name: string;
  input: string;
  rawInput?: string;
  reason?: string;
}

export interface PendingApproval {
  command: string;
  toolId: string;
  toolName: string;
  runOnHost: boolean;
  targetHost?: string;
  targetType?: string;
  targetId?: string;
  risk?: string;
  description?: string;
  auditId?: string;
  plan?: ApprovalPlan;
  contextConfidence?: ApprovalContextConfidence;
  preflight?: ApprovalPreflight;
  isExecuting?: boolean;
  approvalId?: string; // ID of the approval record for API calls
}

export interface ApprovalPlan {
  action_id?: string;
  request_id?: string;
  summary?: string;
  requires_approval: boolean;
  approval_policy?: string;
  blast_radius?: string;
  rollback_available: boolean;
  plan_hash?: string;
  expires_at?: string;
}

export interface ApprovalContextConfidence {
  level?: string;
  summary?: string;
  evidence?: string[];
}

export interface ApprovalPreflight {
  target?: string;
  current_state?: string;
  intended_change?: string;
  dry_run_available: boolean;
  dry_run_summary?: string;
  safety_checks?: string[];
  verification_steps?: string[];
  generated_at?: string;
}

// Question from Pulse Assistant
export interface QuestionOption {
  label: string;
  value: string;
  description?: string;
}

export interface Question {
  id: string;
  type: 'text' | 'select';
  question: string;
  header?: string;
  options?: QuestionOption[];
}

export interface PendingQuestion {
  questionId: string;
  questions: Question[];
  isAnswering?: boolean;
}

// Unified event for chronological display
export type StreamEventType =
  | 'thinking'
  | 'workflow_status'
  | 'tool'
  | 'content'
  | 'pending_tool'
  | 'tool_cancel'
  | 'model_switch'
  | 'approval'
  | 'question';

export interface StreamDisplayEvent {
  type: StreamEventType;
  thinking?: string;
  workflowStatus?: WorkflowStatus;
  startedAt?: number;
  updatedAt?: number;
  tool?: ToolExecution;
  pendingTool?: PendingTool;
  toolCancel?: ToolCancellation;
  // Execution start carried from the pending tool onto the completed row so
  // the duration stamp reflects execution, not model arg-streaming.
  runningAt?: number;
  content?: string;
  model?: string;
  failedModel?: string;
  modelEvent?: 'selected' | 'switch';
  settleUntil?: number;
  toolId?: string; // Used to match pending_tool with completed tool
  approval?: PendingApproval; // For approval_needed events
  question?: PendingQuestion; // For question events
}

export interface WorkflowStatus {
  phase?: string;
  message: string;
  state?: string;
  tool?: string;
  provider?: string;
  model?: string;
  attempt?: number;
  maxAttempts?: number;
  retryAfterMs?: number;
  startedAt?: number;
}

export interface ChatMessageRequestContext {
  mentions?: ChatMention[];
  findingId?: string;
  model?: string;
  autonomousMode?: boolean;
  handoffContext?: string;
  handoffResources?: ChatHandoffResource[];
  handoffActions?: ChatHandoffAction[];
  handoffMetadata?: ChatHandoffMetadata;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  delivery?: 'sent' | 'queued';
  request?: ChatMessageRequestContext;
  interruption?: 'stopped' | 'replaced';
  // Clean, user-facing error for a failed turn. Rendered as a distinct error
  // block (not as answer content) so partial streamed content is preserved and
  // the failure is unmistakable and recoverable.
  error?: string;
  thinking?: string;
  thinkingChunks?: string[];
  streamEvents?: StreamDisplayEvent[];
  timestamp: Date;
  completedAt?: Date;
  model?: string;
  tokens?: { input: number; output: number; contextLimit?: number; sessionCostUsd?: number };
  toolCalls?: ToolExecution[];
  isStreaming?: boolean;
  workflowStatus?: WorkflowStatus;
  workflowStatusHistory?: WorkflowStatus[];
  pendingTools?: PendingTool[];
  pendingApprovals?: PendingApproval[];
  pendingQuestions?: PendingQuestion[];
}

export interface ModelRouteRecoveryOption {
  id: string;
  kind: 'same-model-route' | 'alternate-model-route';
  label: string;
  provider: string;
  providerLabel: string;
}

export interface ModelInfo {
  id: string;
  name: string;
  description?: string;
  notable?: boolean;
  provider?: string;
}
