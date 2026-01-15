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
}

// Unified event for chronological display
export type StreamEventType = 'thinking' | 'tool' | 'content' | 'pending_tool';

export interface StreamDisplayEvent {
  type: StreamEventType;
  thinking?: string;
  tool?: ToolExecution;
  pendingTool?: PendingTool;
  content?: string;
  toolId?: string; // Used to match pending_tool with completed tool
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
}

export interface ModelInfo {
  id: string;
  name: string;
  description?: string;
}

export interface ChatContextItem {
  id: string;
  type: string;
  name: string;
  status: string;
  node?: string;
  data: Record<string, unknown>;
}

// Stream event types from backend
export type StreamEventKind =
  | 'content'
  | 'thinking'
  | 'tool_start'
  | 'tool_end'
  | 'approval_needed'
  | 'processing'
  | 'complete'
  | 'done'
  | 'error';

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
}

export interface StreamCompleteData {
  model: string;
  input_tokens: number;
  output_tokens: number;
  tool_calls?: ToolExecution[];
}
