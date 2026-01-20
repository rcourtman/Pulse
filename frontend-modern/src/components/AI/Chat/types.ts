// Chat component types

export interface ToolExecution {
  name: string;
  input: string;
  output: string;
  success: boolean;
}

export interface PendingTool {
  name: string;
  input: string;
}

export interface PendingApproval {
  command: string;
  toolId: string;
  toolName: string;
  runOnHost: boolean;
  targetHost?: string;
  isExecuting?: boolean;
  approvalId?: string; // ID of the approval record for API calls
}

// Question from Pulse AI
export interface QuestionOption {
  label: string;
  value: string;
}

export interface Question {
  id: string;
  type: 'text' | 'select';
  question: string;
  options?: QuestionOption[];
}

export interface PendingQuestion {
  questionId: string;
  sessionId: string;
  questions: Question[];
  isAnswering?: boolean;
}

// Unified event for chronological display
export type StreamEventType = 'thinking' | 'tool' | 'content' | 'pending_tool' | 'approval' | 'question';

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

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  thinking?: string;
  thinkingChunks?: string[];
  streamEvents?: StreamDisplayEvent[];
  timestamp: Date;
  model?: string;
  tokens?: { input: number; output: number };
  toolCalls?: ToolExecution[];
  isStreaming?: boolean;
  pendingTools?: PendingTool[];
  pendingApprovals?: PendingApproval[];
  pendingQuestions?: PendingQuestion[];
}

export interface ModelInfo {
  id: string;
  name: string;
  description?: string;
  notable?: boolean;
}

// Stream event types from backend
export type StreamEventKind =
  | 'content'
  | 'thinking'
  | 'tool_start'
  | 'tool_end'
  | 'approval_needed'
  | 'question'
  | 'processing'
  | 'complete'
  | 'done'
  | 'error';

export interface StreamQuestionData {
  question_id: string;
  session_id: string;
  questions: Array<{
    id: string;
    type: 'text' | 'select';
    question: string;
    options?: Array<{ label: string; value: string }>;
  }>;
}

export interface StreamToolStartData {
  name: string;
  input: string;
}

export interface StreamToolEndData {
  name: string;
  input: string;
  output: string;
  success: boolean;
}

export interface StreamApprovalNeededData {
  command: string;
  tool_id: string;
  tool_name: string;
  run_on_host: boolean;
  target_host?: string;
  approval_id?: string;
}

export interface StreamCompleteData {
  model: string;
  input_tokens: number;
  output_tokens: number;
  tool_calls?: ToolExecution[];
}
