// Chat component types

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
  isExecuting?: boolean;
  approvalId?: string; // ID of the approval record for API calls
}

// Question from Pulse AI
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

export interface ExploreStatus {
  phase: string;
  message: string;
  model?: string;
  outcome?: string;
}

// Unified event for chronological display
export type StreamEventType = 'thinking' | 'explore_status' | 'tool' | 'content' | 'pending_tool' | 'approval' | 'question';

export interface StreamDisplayEvent {
  type: StreamEventType;
  thinking?: string;
  exploreStatus?: ExploreStatus;
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
