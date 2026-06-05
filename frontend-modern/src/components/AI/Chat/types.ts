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
  output: string;
  success: boolean;
}

export interface PendingTool {
  id: string;
  name: string;
  input: string;
  rawInput?: string;
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
  | 'tool'
  | 'content'
  | 'pending_tool'
  | 'approval'
  | 'question';

export interface StreamDisplayEvent {
  type: StreamEventType;
  thinking?: string;
  tool?: ToolExecution;
  pendingTool?: PendingTool;
  content?: string;
  toolId?: string; // Used to match pending_tool with completed tool
  approval?: PendingApproval; // For approval_needed events
  question?: PendingQuestion; // For question events
}

export interface WorkflowStatus {
  phase?: string;
  message: string;
  state?: string;
  tool?: string;
  startedAt?: number;
}

export interface ChatMessageRequestContext {
  mentions?: ChatMention[];
  findingId?: string;
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
  model?: string;
  tokens?: { input: number; output: number };
  toolCalls?: ToolExecution[];
  isStreaming?: boolean;
  workflowStatus?: WorkflowStatus;
  pendingTools?: PendingTool[];
  pendingApprovals?: PendingApproval[];
  pendingQuestions?: PendingQuestion[];
}

export interface ModelRouteRecoveryOption {
  id: string;
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
