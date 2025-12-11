import { Component, Show, createSignal, For, createEffect, createMemo, onMount } from 'solid-js';
import { marked } from 'marked';
import { AIAPI } from '@/api/ai';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { aiChatStore } from '@/stores/aiChat';
import { useWebSocket } from '@/App';
import { GuestNotes } from './GuestNotes';
import type {
  AIToolExecution,
  AIStreamEvent,
  AIStreamToolStartData,
  AIStreamToolEndData,
  AIStreamCompleteData,
  AIStreamApprovalNeededData,
  ModelInfo,
} from '@/types/ai';

// Provider display names for grouped model selection
const PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic (Claude)',
  openai: 'OpenAI (GPT)',
  deepseek: 'DeepSeek',
  ollama: 'Ollama (Local)',
};

// Parse provider from model ID (format: "provider:model-name")
function getProviderFromModelId(modelId: string): string {
  const colonIndex = modelId.indexOf(':');
  if (colonIndex > 0) {
    return modelId.substring(0, colonIndex);
  }
  // Default detection for models without prefix
  if (modelId.includes('claude') || modelId.includes('opus') || modelId.includes('sonnet') || modelId.includes('haiku')) {
    return 'anthropic';
  }
  if (modelId.includes('gpt') || modelId.includes('o1') || modelId.includes('o3')) {
    return 'openai';
  }
  if (modelId.includes('deepseek')) {
    return 'deepseek';
  }
  return 'ollama';
}

// Group models by provider for grouped rendering
function groupModelsByProvider(models: ModelInfo[]): Map<string, ModelInfo[]> {
  const grouped = new Map<string, ModelInfo[]>();

  for (const model of models) {
    const provider = getProviderFromModelId(model.id);
    const existing = grouped.get(provider) || [];
    existing.push(model);
    grouped.set(provider, existing);
  }

  return grouped;
}

// Configure marked for safe rendering
marked.setOptions({
  breaks: true, // Convert \n to <br>
  gfm: true, // GitHub Flavored Markdown
});

// Helper to render markdown safely
const renderMarkdown = (content: string): string => {
  try {
    return marked.parse(content) as string;
  } catch {
    return content;
  }
};

// Helper to sanitize thinking/reasoning content for display
// Removes raw network errors with IP addresses that are not user-friendly
const sanitizeThinking = (content: string): string => {
  // Replace raw TCP connection details like "write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout"
  // with friendlier messages
  let sanitized = content.replace(
    /write tcp [\d.:]+->[\d.:]+: i\/o timeout/g,
    'connection timed out'
  );
  sanitized = sanitized.replace(
    /read tcp [\d.:]+: i\/o timeout/g,
    'connection timed out'
  );
  sanitized = sanitized.replace(
    /dial tcp [\d.:]+: connection refused/g,
    'connection refused'
  );
  // Replace "failed to send command: <raw error>" patterns
  sanitized = sanitized.replace(
    /failed to send command: write tcp [\d.:->\s]+/g,
    'failed to send command: connection error'
  );
  return sanitized;
};

// In-progress tool execution (before completion)
interface PendingTool {
  name: string;
  input: string;
}

// Command awaiting user approval
interface PendingApproval {
  command: string;
  toolId: string;
  toolName: string;
  runOnHost: boolean;
  targetHost?: string; // Explicit host for command routing
  isExecuting?: boolean;
}


// Unified event type for chronological display
interface StreamDisplayEvent {
  type: 'thinking' | 'tool';
  thinking?: string;
  tool?: AIToolExecution;
}

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  thinking?: string; // DeepSeek reasoning/thinking content (accumulated)
  thinkingChunks?: string[]; // Thinking split into sequential blocks for display
  streamEvents?: StreamDisplayEvent[]; // All events in chronological order
  timestamp: Date;
  model?: string;
  tokens?: { input: number; output: number };
  toolCalls?: AIToolExecution[];
  // Streaming state
  isStreaming?: boolean;
  pendingTools?: PendingTool[];
  pendingApprovals?: PendingApproval[];
}

interface AIChatProps {
  onClose: () => void;
}

// Extract guest name from context if available
const getGuestName = (context?: Record<string, unknown>): string | undefined => {
  if (!context) return undefined;
  if (typeof context.guestName === 'string') return context.guestName;
  if (typeof context.name === 'string') return context.name;
  return undefined;
};

export const AIChat: Component<AIChatProps> = (props) => {
  // Read all context from store for proper SolidJS reactivity
  const isOpen = () => aiChatStore.isOpen;
  const context = () => aiChatStore.context;
  const targetType = () => context().targetType;
  const targetId = () => context().targetId;
  const contextData = () => context().context;
  const initialPrompt = () => context().initialPrompt;
  const findingId = () => context().findingId; // For resolving patrol findings

  // Access WebSocket state for listing available resources
  const wsContext = useWebSocket();

  // Context picker state
  const [showContextPicker, setShowContextPicker] = createSignal(false);
  const [contextSearch, setContextSearch] = createSignal('');

  // Build a list of all available resources for the context picker
  const availableResources = createMemo(() => {
    const resources: Array<{
      id: string;
      type: 'vm' | 'container' | 'node' | 'host' | 'docker';
      name: string;
      status: string;
      node?: string;
      data: Record<string, unknown>;
    }> = [];

    // Add VMs
    for (const vm of wsContext.state.vms || []) {
      resources.push({
        id: `${vm.node}-${vm.vmid}`,
        type: 'vm',
        name: vm.name || `VM ${vm.vmid}`,
        status: vm.status,
        node: vm.node,
        data: {
          guest_id: `${vm.node}-${vm.vmid}`,
          guest_name: vm.name,
          guest_vmid: vm.vmid,
          guest_type: 'qemu',
          guest_node: vm.node,
          guest_status: vm.status,
          cpu: vm.cpu,
          mem: vm.memory?.used,
          maxmem: vm.memory?.total,
          disk: vm.disk?.used,
          maxdisk: vm.disk?.total,
        },
      });
    }

    // Add containers
    for (const ct of wsContext.state.containers || []) {
      resources.push({
        id: `${ct.node}-${ct.vmid}`,
        type: 'container',
        name: ct.name || `CT ${ct.vmid}`,
        status: ct.status,
        node: ct.node,
        data: {
          guest_id: `${ct.node}-${ct.vmid}`,
          guest_name: ct.name,
          guest_vmid: ct.vmid,
          guest_type: 'lxc',
          guest_node: ct.node,
          guest_status: ct.status,
          cpu: ct.cpu,
          mem: ct.memory?.used,
          maxmem: ct.memory?.total,
          disk: ct.disk?.used,
          maxdisk: ct.disk?.total,
        },
      });
    }

    // Add Proxmox nodes
    for (const node of wsContext.state.nodes || []) {
      resources.push({
        id: `node-${node.name}`,
        type: 'node',
        name: node.name,
        status: node.status,
        data: {
          node_name: node.name,
          node_status: node.status,
          cpu: node.cpu,
          mem: node.memory?.used,
          maxmem: node.memory?.total,
          disk: node.disk?.used,
          maxdisk: node.disk?.total,
        },
      });
    }

    // Add host agents
    for (const host of wsContext.state.hosts || []) {
      resources.push({
        id: `host-${host.hostname}`,
        type: 'host',
        name: host.hostname,
        status: host.status === 'online' ? 'online' : 'offline',
        data: {
          host_name: host.hostname,
          host_platform: host.platform,
          host_version: host.agentVersion,
          connected: host.status === 'online',
        },
      });
    }

    return resources;
  });

  // Filtered resources based on search
  const filteredResources = createMemo(() => {
    const search = contextSearch().toLowerCase();
    if (!search) return availableResources();
    return availableResources().filter(
      (r) =>
        r.name.toLowerCase().includes(search) ||
        r.type.toLowerCase().includes(search) ||
        (r.node && r.node.toLowerCase().includes(search))
    );
  });

  // Add a resource to context
  const addResourceToContext = (resource: ReturnType<typeof availableResources>[number]) => {
    aiChatStore.addContextItem(resource.type, resource.id, resource.name, resource.data);
    setShowContextPicker(false);
    setContextSearch('');
  };

  // Initialize messages from store (for persistence across navigation)
  const [messages, setMessagesLocal] = createSignal<Message[]>(
    aiChatStore.messages as Message[] || []
  );
  const [input, setInput] = createSignal('');
  const [isLoading, setIsLoading] = createSignal(false);
  const [queuedMessage, setQueuedMessage] = createSignal<string | null>(null);

  // Model selection
  const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);
  const [selectedModel, setSelectedModel] = createSignal<string>(''); // Empty = use default
  const [showModelSelector, setShowModelSelector] = createSignal(false);

  let messagesEndRef: HTMLDivElement | undefined;
  let inputRef: HTMLTextAreaElement | undefined;
  let abortControllerRef: AbortController | null = null;

  // Fetch available models on mount using the dynamic API
  onMount(async () => {
    try {
      const result = await AIAPI.getModels();
      if (result.models && result.models.length > 0) {
        setAvailableModels(result.models.map(m => ({
          id: m.id,
          name: m.name || m.id,
          description: m.description,
        })));
      }
    } catch (e) {
      // Silently fail - models will just not be selectable
    }
  });

  // Wrapper to sync messages to global store
  const setMessages = (updater: Message[] | ((prev: Message[]) => Message[])) => {
    setMessagesLocal((prev) => {
      const newMsgs = typeof updater === 'function' ? updater(prev) : updater;
      // Sync to global store for persistence (debounce or defer to avoid too many updates)
      setTimeout(() => aiChatStore.setMessages(newMsgs as any), 0);
      return newMsgs;
    });
  };

  // Auto-scroll to bottom when new messages arrive
  createEffect(() => {
    if (messages().length > 0 && messagesEndRef) {
      messagesEndRef.scrollIntoView({ behavior: 'smooth' });
    }
  });

  // Focus input when drawer opens and register with store for keyboard shortcuts
  createEffect(() => {
    if (isOpen() && inputRef) {
      setTimeout(() => inputRef?.focus(), 100);
      aiChatStore.registerInput(inputRef);
    } else {
      aiChatStore.registerInput(null);
    }
  });

  // Handle initial prompt if provided
  createEffect(() => {
    if (initialPrompt() && isOpen()) {
      setInput(initialPrompt()!);
    }
  });

  // Auto-send queued message when AI finishes processing
  createEffect(() => {
    const loading = isLoading();
    const queued = queuedMessage();

    // When loading finishes and we have a queued message, send it
    if (!loading && queued) {
      logger.info('[AIChat] AI finished, auto-sending queued message', { prompt: queued.substring(0, 50) });
      setQueuedMessage(null);
      // Small delay to let the UI update first
      setTimeout(() => {
        handleSubmit(undefined, queued);
      }, 100);
    }
  });

  const generateId = () => Math.random().toString(36).substring(2, 9);

  // Stop/cancel the current AI request
  const handleStop = () => {
    if (abortControllerRef) {
      abortControllerRef.abort();
      abortControllerRef = null;
    }
    // Mark any streaming message as stopped
    setMessages((prev) =>
      prev.map((msg) =>
        msg.isStreaming
          ? { ...msg, isStreaming: false, content: msg.content || '(Stopped by user)' }
          : msg
      )
    );
    // Clear queued message when stopping - user likely doesn't want it sent
    setQueuedMessage(null);
    setIsLoading(false);
  };

  const handleSubmit = async (e?: Event, forcePrompt?: string) => {
    e?.preventDefault();
    const prompt = forcePrompt || input().trim();
    if (!prompt) return;

    // If AI is currently working, queue this message for later
    if (isLoading() && !forcePrompt) {
      setQueuedMessage(prompt);
      setInput('');
      logger.info('[AIChat] Message queued while AI is working', { prompt: prompt.substring(0, 50) });
      return;
    }

    // IMPORTANT: Capture the current messages BEFORE adding new ones to avoid race conditions
    // SolidJS batches updates, so messages() may not be updated synchronously
    const previousMessages = messages();

    // Build conversation history from previous messages (before we add new ones)
    // Include tool call outputs so the AI remembers what commands were run
    const history = previousMessages
      .filter((m) => !m.isStreaming) // Only include completed messages
      .filter((m) => m.content || (m.toolCalls && m.toolCalls.length > 0)) // Must have content or tool calls
      .map((m) => {
        let content = m.content || '';

        // For assistant messages, prepend tool call outputs so AI has full context
        if (m.role === 'assistant' && m.toolCalls && m.toolCalls.length > 0) {
          const toolSummary = m.toolCalls
            .map((tc) => `Command: ${tc.input}\nOutput: ${tc.output}`)
            .join('\n\n');
          content = toolSummary + (content ? '\n\n' + content : '');
        }

        return {
          role: m.role,
          content,
        };
      })
      .filter((m) => m.content); // Filter out any empty messages

    // Add user message
    const userMessage: Message = {
      id: generateId(),
      role: 'user',
      content: prompt,
      timestamp: new Date(),
    };
    setMessages((prev) => [...prev, userMessage]);
    setInput('');
    setIsLoading(true);

    // Create abort controller for this request
    abortControllerRef = new AbortController();

    // Create a streaming assistant message
    const assistantId = generateId();
    const streamingMessage: Message = {
      id: assistantId,
      role: 'assistant',
      content: '',
      timestamp: new Date(),
      isStreaming: true,
      pendingTools: [],
      pendingApprovals: [],
      toolCalls: [],
      thinkingChunks: [],
      streamEvents: [],
    };
    setMessages((prev) => [...prev, streamingMessage]);

    // Safety timeout - clear streaming state if we don't get any completion event
    // This prevents the UI from getting stuck in a streaming state
    let lastEventTime = Date.now();
    const SAFETY_TIMEOUT_MS = 120000; // 2 minutes

    const safetyCheckInterval = setInterval(() => {
      const timeSinceLastEvent = Date.now() - lastEventTime;
      if (timeSinceLastEvent > SAFETY_TIMEOUT_MS) {
        logger.warn('[AIChat] Safety timeout - forcing stream completion', { seconds: SAFETY_TIMEOUT_MS / 1000 });
        clearInterval(safetyCheckInterval);
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === assistantId && msg.isStreaming
              ? { ...msg, isStreaming: false, content: msg.content || '(Request timed out - no response received)' }
              : msg
          )
        );
        setIsLoading(false);
        if (abortControllerRef) {
          abortControllerRef.abort();
          abortControllerRef = null;
        }
      }
    }, 10000); // Check every 10 seconds

    try {
      await AIAPI.executeStream(
        {
          prompt,
          target_type: targetType(),
          target_id: targetId(),
          context: contextData(),
          history: history.length > 0 ? history : undefined,
          finding_id: findingId(), // Pass finding ID so AI can resolve it when fixed
          model: selectedModel() || undefined, // Use selected model or default
        },
        (event: AIStreamEvent) => {
          lastEventTime = Date.now(); // Update last event time
          logger.debug('[AIChat] Received event', { type: event.type, event });
          // Update the streaming message based on event type
          setMessages((prev) =>
            prev.map((msg) => {
              if (msg.id !== assistantId) return msg;

              switch (event.type) {
                case 'tool_start': {
                  const data = event.data as AIStreamToolStartData;
                  return {
                    ...msg,
                    pendingTools: [...(msg.pendingTools || []), { name: data.name, input: data.input }],
                  };
                }
                case 'tool_end': {
                  const data = event.data as AIStreamToolEndData;
                  // Remove one pending tool with matching name
                  const pendingTools = msg.pendingTools || [];
                  const matchingIndex = pendingTools.findIndex((t) => t.name === data.name);
                  const updatedPending = matchingIndex >= 0
                    ? [...pendingTools.slice(0, matchingIndex), ...pendingTools.slice(matchingIndex + 1)]
                    : pendingTools;
                  // Use input directly from tool_end event (authoritative)
                  const newToolCall: AIToolExecution = {
                    name: data.name,
                    input: data.input,
                    output: data.output,
                    success: data.success,
                  };
                  // Add to both toolCalls and streamEvents for chronological display
                  const events = msg.streamEvents || [];
                  return {
                    ...msg,
                    pendingTools: updatedPending,
                    toolCalls: [...(msg.toolCalls || []), newToolCall],
                    streamEvents: [...events, { type: 'tool' as const, tool: newToolCall }],
                  };
                }
                case 'thinking': {
                  const chunk = event.data as string;
                  if (!chunk.trim()) return msg; // Skip empty chunks
                  // Each thinking event is a new chunk - add to both arrays
                  const chunks = msg.thinkingChunks || [];
                  const events = msg.streamEvents || [];
                  return {
                    ...msg,
                    thinking: (msg.thinking || '') + chunk,
                    thinkingChunks: [...chunks, chunk.trim()],
                    streamEvents: [...events, { type: 'thinking' as const, thinking: chunk.trim() }],
                  };
                }
                case 'content': {
                  const content = event.data as string;
                  // Append content rather than replace - this allows intermediate AI responses
                  // during tool execution to accumulate, showing the user the full conversation flow
                  const existingContent = msg.content || '';
                  const separator = existingContent && !existingContent.endsWith('\n') ? '\n\n' : '';
                  return {
                    ...msg,
                    content: existingContent + separator + content,
                  };
                }
                case 'complete': {
                  // Complete event has flat structure (model, input_tokens at top level, not under data)
                  const completeEvent = event as unknown as AIStreamCompleteData & { type: string };
                  return {
                    ...msg,
                    isStreaming: false,
                    pendingTools: [],
                    model: completeEvent.model,
                    tokens: {
                      input: completeEvent.input_tokens,
                      output: completeEvent.output_tokens,
                    },
                    // Use tool_calls from complete if we missed any
                    toolCalls: msg.toolCalls?.length ? msg.toolCalls : completeEvent.tool_calls,
                  };
                }
                case 'done': {
                  return {
                    ...msg,
                    isStreaming: false,
                    pendingTools: [],
                  };
                }
                case 'error': {
                  const errorMsg = event.data as string;
                  return {
                    ...msg,
                    isStreaming: false,
                    pendingTools: [],
                    content: `Error: ${errorMsg}`,
                  };
                }
                case 'processing': {
                  // Show processing status for multi-iteration calls
                  const status = event.data as string;
                  logger.debug('[AIChat] Processing', status);
                  // Add as a pending tool for visual feedback
                  return {
                    ...msg,
                    pendingTools: [{ name: 'processing', input: status }],
                  };
                }
                case 'approval_needed': {
                  const data = event.data as AIStreamApprovalNeededData;
                  return {
                    ...msg,
                    pendingApprovals: [...(msg.pendingApprovals || []), {
                      command: data.command,
                      toolId: data.tool_id,
                      toolName: data.tool_name,
                      runOnHost: data.run_on_host ?? false, // Default to false if undefined
                      targetHost: data.target_host, // Pass through the explicit routing target
                    }],
                  };
                }

                default:
                  return msg;
              }
            })
          );
        },
        abortControllerRef?.signal
      );
    } catch (error) {
      // Don't show error for user-initiated abort
      if (error instanceof Error && error.name === 'AbortError') {
        logger.debug('[AIChat] Request aborted by user');
        return;
      }
      logger.error('[AIChat] Execute failed:', error);
      const errorMessage = error instanceof Error ? error.message : 'Failed to get AI response';
      notificationStore.error(errorMessage);

      // Update the streaming message to show error
      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantId
            ? { ...msg, isStreaming: false, content: `Error: ${errorMessage}` }
            : msg
        )
      );
    } finally {
      clearInterval(safetyCheckInterval);
      abortControllerRef = null;
      setIsLoading(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const clearChat = () => {
    setMessages([]);
    aiChatStore.clearConversation();
  };

  // Execute an approved command
  const executeApprovedCommand = async (messageId: string, approval: PendingApproval) => {
    // Mark as executing
    setMessages((prev) =>
      prev.map((m) =>
        m.id === messageId
          ? {
            ...m,
            pendingApprovals: m.pendingApprovals?.map((a) =>
              a.toolId === approval.toolId ? { ...a, isExecuting: true } : a
            ),
          }
          : m
      )
    );

    try {
      // Extract VMID from context if available
      const vmid = contextData()?.vmid as string | undefined;

      const result = await AIAPI.runCommand({
        command: approval.command,
        target_type: targetType() || '',
        target_id: targetId() || '',
        run_on_host: approval.runOnHost,
        vmid,
        target_host: approval.targetHost, // Pass through the explicit routing target
      });


      // Move from pending approvals to completed tool calls
      const currentMessages = messages();
      const targetMessage = currentMessages.find((m) => m.id === messageId);
      const pendingCount = targetMessage?.pendingApprovals?.length || 0;
      const remainingAfterThis = (targetMessage?.pendingApprovals?.filter((a) => a.toolId !== approval.toolId) || []).length;

      logger.info('[AIChat] Approval processed', {
        messageId,
        toolId: approval.toolId,
        pendingCount,
        remainingAfterThis,
        pendingApprovals: targetMessage?.pendingApprovals?.map(a => a.toolId)
      });

      setMessages((prev) =>
        prev.map((m) => {
          if (m.id !== messageId) return m;

          const newToolCall: AIToolExecution = {
            name: approval.toolName,
            input: approval.command,
            output: result.output || result.error || '',
            success: result.success,
          };

          const remainingApprovals = m.pendingApprovals?.filter((a) => a.toolId !== approval.toolId) || [];

          return {
            ...m,
            pendingApprovals: remainingApprovals,
            toolCalls: [...(m.toolCalls || []), newToolCall],
            // Clear the stale "I need approval" content after the last approval is processed
            // The tool output will show the result instead
            content: remainingApprovals.length === 0 ? '' : m.content,
          };
        })
      );

      // No toast for success - the tool output shows the result inline
      // Only show error toast for failures since they might need attention
      if (!result.success && result.error) {
        notificationStore.error(result.error);
      }

      // After the last approval is processed, automatically continue the conversation
      // This lets the AI analyze the command output and provide a summary
      if (remainingAfterThis === 0) {
        logger.info('[AIChat] Last approval processed, triggering auto-continuation');
        // Small delay to let the UI update first
        setTimeout(async () => {
          logger.info('[AIChat] Starting auto-continuation');
          setIsLoading(true);

          // Build history including the just-executed command
          const currentMsgs = messages();
          logger.debug('[AIChat] Building history for continuation', { messageCount: currentMsgs.length });

          const historyForContinuation = currentMsgs
            .filter((m) => !m.isStreaming)
            .filter((m) => m.content || (m.toolCalls && m.toolCalls.length > 0))
            .map((m) => {
              let content = m.content || '';
              if (m.role === 'assistant' && m.toolCalls && m.toolCalls.length > 0) {
                const toolSummary = m.toolCalls
                  .map((tc) => `Command: ${tc.input}\nOutput: ${tc.output}`)
                  .join('\n\n');
                content = toolSummary + (content ? '\n\n' + content : '');
              }
              return { role: m.role, content };
            })
            .filter((m) => m.content);

          logger.debug('[AIChat] History for continuation built', { historyLength: historyForContinuation.length });

          // Add a hidden continuation prompt - the AI will see it but user won't
          const continuationPrompt = 'Continue analyzing the command output above and provide a summary.';

          // Create the streaming assistant response message (no visible user message)
          // Show "Analyzing..." as initial content so user sees inline feedback
          const assistantId = generateId();
          const streamingMessage: Message = {
            id: assistantId,
            role: 'assistant',
            content: '*Analyzing results...*',
            timestamp: new Date(),
            isStreaming: true,
            pendingTools: [],
            pendingApprovals: [],
            toolCalls: [],
          };
          setMessages((prev) => [...prev, streamingMessage]);

          try {
            logger.info('[AIChat] Calling executeStream for continuation');
            await AIAPI.executeStream(
              {
                prompt: continuationPrompt,
                target_type: targetType(),
                target_id: targetId(),
                context: contextData(),
                history: historyForContinuation,
              },
              (event: AIStreamEvent) => {
                logger.debug('[AIChat] Continuation event received', { type: event.type });
                setMessages((prev) =>
                  prev.map((msg) => {
                    if (msg.id !== assistantId) return msg;
                    switch (event.type) {
                      case 'content': {
                        // Append content, but filter out the initial placeholder
                        const content = event.data as string;
                        const existingContent = msg.content === '*Analyzing results...*' ? '' : (msg.content || '');
                        const separator = existingContent && !existingContent.endsWith('\n') ? '\n\n' : '';
                        return { ...msg, content: existingContent + separator + content, isStreaming: false };
                      }
                      case 'done':
                        return { ...msg, isStreaming: false };
                      case 'error':
                        return { ...msg, content: `Error: ${event.data}`, isStreaming: false };
                      case 'thinking':
                        // Ignore thinking events for now
                        return msg;
                      case 'processing':
                        // Ignore processing events
                        return msg;
                      case 'tool_start': {
                        const data = event.data as { name: string; input: string };
                        return {
                          ...msg,
                          pendingTools: [...(msg.pendingTools || []), { name: data.name, input: data.input }],
                        };
                      }
                      case 'tool_end': {
                        const data = event.data as { name: string; input: string; output: string; success: boolean };
                        const pendingTools = msg.pendingTools || [];
                        const matchingIndex = pendingTools.findIndex((t) => t.name === data.name);
                        const updatedPending = matchingIndex >= 0
                          ? [...pendingTools.slice(0, matchingIndex), ...pendingTools.slice(matchingIndex + 1)]
                          : pendingTools;
                        return {
                          ...msg,
                          pendingTools: updatedPending,
                          toolCalls: [...(msg.toolCalls || []), {
                            name: data.name,
                            input: data.input,
                            output: data.output,
                            success: data.success,
                          }],
                        };
                      }
                      case 'approval_needed': {
                        const data = event.data as AIStreamApprovalNeededData;
                        logger.info('[AIChat] Approval needed in continuation', { command: data.command });
                        return {
                          ...msg,
                          pendingApprovals: [...(msg.pendingApprovals || []), {
                            command: data.command,
                            toolId: data.tool_id,
                            toolName: data.tool_name,
                            runOnHost: data.run_on_host,
                            targetHost: data.target_host,
                          }],
                          isStreaming: false, // Stop streaming when approval is needed
                        };
                      }
                      default:
                        logger.debug('[AIChat] Unhandled continuation event', { type: event.type, event });
                        return msg;
                    }
                  })
                );
              }
            );
            logger.info('[AIChat] Continuation executeStream completed');
          } catch (err) {
            logger.error('[AIChat] Failed to continue after approval:', err);
            setMessages((prev) =>
              prev.map((msg) =>
                msg.id === assistantId
                  ? { ...msg, content: 'Failed to analyze results.', isStreaming: false }
                  : msg
              )
            );
          } finally {
            setIsLoading(false);
          }
        }, 200);
      } else {
        logger.debug('[AIChat] Approvals remaining, not triggering continuation', { remainingAfterThis });
      }
    } catch (error) {
      logger.error('[AIChat] Failed to execute approved command:', error);
      const errorMsg = error instanceof Error ? error.message : 'Failed to execute command';
      notificationStore.error(errorMsg);

      // Mark as no longer executing
      setMessages((prev) =>
        prev.map((m) =>
          m.id === messageId
            ? {
              ...m,
              pendingApprovals: m.pendingApprovals?.map((a) =>
                a.toolId === approval.toolId ? { ...a, isExecuting: false } : a
              ),
            }
            : m
        )
      );
    }
  };

  // Panel renders as flex child, width controlled by isOpen state
  return (
    <div
      class={`flex-shrink-0 h-full bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex flex-col transition-all duration-300 overflow-hidden ${isOpen() ? 'w-[420px]' : 'w-0 border-l-0'
        }`}
    >
      <Show when={isOpen()}>
        {/* Header */}
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-purple-50 to-pink-50 dark:from-purple-900/20 dark:to-pink-900/20">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-purple-100 dark:bg-purple-900/40 rounded-lg">
              <svg
                class="w-5 h-5 text-purple-600 dark:text-purple-300"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5"
                />
              </svg>
            </div>
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                <Show when={getGuestName(contextData())} fallback="AI Assistant">
                  Ask AI about {getGuestName(contextData())}
                </Show>
              </h2>
              <Show when={targetType()}>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {targetType() === 'vm' ? 'Virtual Machine' : targetType() === 'container' ? 'LXC Container' : targetType()}
                </p>
              </Show>
            </div>
          </div>
          <div class="flex items-center gap-2">
            {/* Model selector dropdown */}
            <Show when={availableModels().length > 0}>
              <div class="relative">
                <button
                  onClick={() => setShowModelSelector(!showModelSelector())}
                  class="flex items-center gap-1 px-2 py-1 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded border border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 bg-white dark:bg-gray-800"
                  title="Select AI model"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                  <span class="max-w-[80px] truncate">
                    {selectedModel() || 'Default'}
                  </span>
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showModelSelector()}>
                  <div class="absolute right-0 top-full mt-1 w-64 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 z-50 py-1">
                    <button
                      onClick={() => { setSelectedModel(''); setShowModelSelector(false); }}
                      class={`w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 ${!selectedModel() ? 'bg-blue-50 dark:bg-blue-900/30' : ''}`}
                    >
                      <div class="font-medium text-gray-900 dark:text-gray-100">Default</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">Use configured default model</div>
                    </button>
                    <For each={Array.from(groupModelsByProvider(availableModels()).entries())}>
                      {([provider, models]) => (
                        <>
                          <div class="px-3 py-1.5 text-xs font-semibold text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/50 sticky top-0">
                            {PROVIDER_DISPLAY_NAMES[provider] || provider}
                          </div>
                          <For each={models}>
                            {(model) => (
                              <button
                                onClick={() => { setSelectedModel(model.id); setShowModelSelector(false); }}
                                class={`w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 ${selectedModel() === model.id ? 'bg-blue-50 dark:bg-blue-900/30' : ''}`}
                              >
                                <div class="font-medium text-gray-900 dark:text-gray-100">
                                  {model.name || model.id.split(':').pop()}
                                </div>
                              </button>
                            )}
                          </For>
                        </>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            </Show>
            <button
              onClick={clearChat}
              class="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800"
              title="Clear chat"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
            <button
              onClick={props.onClose}
              class="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800"
              title="Collapse panel"
            >
              {/* Double chevron right - collapse */}
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 5l7 7-7 7M6 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>
        </div>

        {/* Messages Area */}
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <Show when={messages().length === 0}>
            <div class="text-center py-12 text-gray-500 dark:text-gray-400">
              <svg
                class="w-12 h-12 mx-auto mb-4 text-purple-200 dark:text-purple-800"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z"
                />
              </svg>
              <Show when={getGuestName(contextData())} fallback={
                <>
                  <p class="text-sm font-medium mb-2">Start a conversation</p>
                  <p class="text-xs max-w-xs mx-auto">
                    Ask about your infrastructure, diagnose issues, or get remediation suggestions.
                  </p>
                </>
              }>
                <p class="text-sm font-medium mb-2">Ask about {getGuestName(contextData())}</p>
                <p class="text-xs max-w-xs mx-auto mb-4">
                  AI has access to this guest's current metrics and state. Try asking:
                </p>
                <div class="text-xs space-y-2 text-left max-w-xs mx-auto">
                  <button
                    type="button"
                    class="w-full text-left px-3 py-2 rounded-lg bg-purple-50 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 hover:bg-purple-100 dark:hover:bg-purple-900/50 transition-colors"
                    onClick={() => setInput('What is the current status of this guest?')}
                  >
                    "What is the current status of this guest?"
                  </button>
                  <button
                    type="button"
                    class="w-full text-left px-3 py-2 rounded-lg bg-purple-50 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 hover:bg-purple-100 dark:hover:bg-purple-900/50 transition-colors"
                    onClick={() => setInput('Are there any performance concerns?')}
                  >
                    "Are there any performance concerns?"
                  </button>
                  <button
                    type="button"
                    class="w-full text-left px-3 py-2 rounded-lg bg-purple-50 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 hover:bg-purple-100 dark:hover:bg-purple-900/50 transition-colors"
                    onClick={() => setInput('How can I optimize resource usage?')}
                  >
                    "How can I optimize resource usage?"
                  </button>
                </div>
              </Show>
            </div>
          </Show>

          <For each={messages()}>
            {(message) => (
              <div
                class={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                <div
                  class={`max-w-[85%] rounded-lg px-4 py-2 ${message.role === 'user'
                    ? 'bg-purple-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100'
                    }`}
                >
                  {/* Render all events in chronological order - thinking and tools interleaved */}
                  <Show when={message.role === 'assistant' && message.streamEvents && message.streamEvents.length > 0}>
                    <div class="mb-3 space-y-2">
                      <For each={message.streamEvents}>
                        {(evt) => (
                          <Show
                            when={evt.type === 'tool' && evt.tool}
                            fallback={
                              // Thinking chunk
                              <div class="px-2 py-1.5 text-xs bg-blue-50 dark:bg-blue-900/20 text-gray-700 dark:text-gray-300 rounded border-l-2 border-blue-400 whitespace-pre-wrap">
                                {sanitizeThinking(evt.thinking && evt.thinking.length > 500 ? evt.thinking.substring(0, 500) + '...' : evt.thinking || '')}
                              </div>
                            }
                          >
                            {/* Tool call */}
                            <div class="rounded border border-gray-300 dark:border-gray-600 overflow-hidden">
                              <div class={`px-2 py-1 text-xs font-medium flex items-center gap-2 ${evt.tool!.success
                                ? 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200'
                                : 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-200'
                                }`}>
                                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                </svg>
                                <code class="font-mono">{evt.tool!.input}</code>
                              </div>
                              <Show when={evt.tool!.output}>
                                <pre class="px-2 py-1 text-xs font-mono bg-gray-50 dark:bg-gray-900 text-gray-700 dark:text-gray-300 overflow-x-auto max-h-32 overflow-y-auto whitespace-pre-wrap break-words">
                                  {evt.tool!.output.length > 500 ? evt.tool!.output.substring(0, 500) + '...' : evt.tool!.output}
                                </pre>
                              </Show>
                            </div>
                          </Show>
                        )}
                      </For>
                    </div>
                  </Show>

                  {/* Show in-progress tool executions - at the bottom */}
                  <Show when={message.role === 'assistant' && message.pendingTools && message.pendingTools.length > 0}>
                    <div class="mb-3 space-y-2">
                      <For each={message.pendingTools}>
                        {(tool) => (
                          <div class="rounded border border-purple-400 dark:border-purple-600 overflow-hidden">
                            <div class="px-2 py-1 text-xs font-medium flex items-center gap-2 bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-200">
                              <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                              </svg>
                              <code class="font-mono">{tool.input}</code>
                              <span class="text-[10px] text-purple-600 dark:text-purple-400">Running...</span>
                            </div>
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>

                  {/* Show AI's response text AFTER tool calls */}
                  <Show when={message.content}>
                    <div
                      class="text-sm prose prose-sm dark:prose-invert max-w-none prose-pre:bg-gray-800 prose-pre:text-gray-100 prose-code:text-purple-600 dark:prose-code:text-purple-400 prose-code:before:content-none prose-code:after:content-none"
                      innerHTML={renderMarkdown(message.content)}
                    />
                  </Show>

                  {/* Show commands awaiting approval */}
                  <Show when={message.role === 'assistant' && message.pendingApprovals && message.pendingApprovals.length > 0}>
                    <div class="mt-3 space-y-2">
                      <For each={message.pendingApprovals}>
                        {(approval) => (
                          <div class="rounded border border-amber-400 dark:border-amber-600 overflow-hidden">
                            <div class="px-2 py-1.5 text-xs font-medium flex items-center gap-2 bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-200">
                              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                              </svg>
                              <span>Approval Required</span>
                              <Show when={approval.runOnHost}>
                                <span class="px-1 py-0.5 bg-amber-200 dark:bg-amber-800 rounded text-[10px]">HOST</span>
                              </Show>
                            </div>
                            <div class="px-2 py-2 bg-amber-50 dark:bg-amber-900/20">
                              <code class="text-xs font-mono text-gray-800 dark:text-gray-200 block mb-2">{approval.command}</code>
                              <div class="flex gap-2">
                                <button
                                  type="button"
                                  class={`flex-1 px-2 py-1 text-xs font-medium rounded transition-colors ${approval.isExecuting
                                    ? 'bg-green-400 text-white cursor-wait'
                                    : 'bg-green-600 hover:bg-green-700 text-white'
                                    }`}
                                  onClick={() => executeApprovedCommand(message.id, approval)}
                                  disabled={approval.isExecuting}
                                >
                                  {approval.isExecuting ? (
                                    <span class="flex items-center justify-center gap-1">
                                      <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                                      </svg>
                                      Running...
                                    </span>
                                  ) : 'Run'}
                                </button>
                                <button
                                  type="button"
                                  class="flex-1 px-2 py-1 text-xs font-medium bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-200 rounded transition-colors disabled:opacity-50"
                                  onClick={() => {
                                    // Remove from pending approvals
                                    setMessages((prev) =>
                                      prev.map((m) =>
                                        m.id === message.id
                                          ? { ...m, pendingApprovals: m.pendingApprovals?.filter((a) => a.toolId !== approval.toolId) }
                                          : m
                                      )
                                    );
                                  }}
                                  disabled={approval.isExecuting}
                                >
                                  Skip
                                </button>
                              </div>
                            </div>
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>
                  {/* Minimal footer - no model/token info shown */}
                </div>
              </div>
            )}
          </For>

          <div ref={messagesEndRef} />
        </div>

        {/* Processing indicator - sticky above input */}
        <Show when={isLoading()}>
          <div class="px-4 py-2 bg-purple-50 dark:bg-purple-900/20 border-t border-purple-200 dark:border-purple-800 flex items-center gap-2 text-sm text-purple-700 dark:text-purple-300">
            <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
            <span>Analyzing...</span>
          </div>
        </Show>

        {/* Input Area */}
        <div class="border-t border-gray-200 dark:border-gray-700 p-4">
          {/* Context section - always show with Add button */}
          <div class="mb-3 px-3 py-2 bg-gray-50 dark:bg-gray-800/50 rounded-lg border border-gray-200 dark:border-gray-700">
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                <svg class="w-3.5 h-3.5 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span class="font-medium">
                  Context {aiChatStore.contextItems.length > 0 ? `(${aiChatStore.contextItems.length})` : ''}
                </span>
              </div>
              <div class="flex items-center gap-2">
                <Show when={aiChatStore.contextItems.length > 0}>
                  <button
                    type="button"
                    onClick={() => aiChatStore.clearAllContext()}
                    class="text-[10px] text-gray-400 hover:text-red-500 transition-colors"
                    title="Clear all context"
                  >
                    Clear all
                  </button>
                </Show>
              </div>
            </div>

            {/* Context items */}
            <div class="flex flex-wrap gap-1.5">
              <For each={aiChatStore.contextItems}>
                {(item) => (
                  <span class="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-md bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-200">
                    <span class="text-[9px] uppercase text-purple-500 dark:text-purple-400">{item.type}</span>
                    <span class="font-medium">{item.name}</span>
                    <button
                      type="button"
                      onClick={() => aiChatStore.removeContextItem(item.id)}
                      class="ml-0.5 p-0.5 rounded hover:bg-purple-200 dark:hover:bg-purple-800 transition-colors"
                      title="Remove from context"
                    >
                      <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </span>
                )}
              </For>

              {/* Add context button */}
              <div class="relative">
                <button
                  type="button"
                  onClick={() => setShowContextPicker(!showContextPicker())}
                  class="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-md border border-dashed border-gray-300 dark:border-gray-600 text-gray-500 dark:text-gray-400 hover:border-purple-400 hover:text-purple-600 dark:hover:border-purple-500 dark:hover:text-purple-400 transition-colors"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                  </svg>
                  <span>Add</span>
                </button>

                {/* Context picker dropdown */}
                <Show when={showContextPicker()}>
                  <div class="absolute bottom-full left-0 mb-1 w-72 max-h-80 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 overflow-hidden z-50">
                    {/* Search input */}
                    <div class="p-2 border-b border-gray-200 dark:border-gray-700">
                      <input
                        type="text"
                        value={contextSearch()}
                        onInput={(e) => setContextSearch(e.currentTarget.value)}
                        placeholder="Search VMs, containers, hosts..."
                        class="w-full px-2 py-1.5 text-xs rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-purple-500"
                        autofocus
                      />
                    </div>

                    {/* Resource list */}
                    <div class="max-h-56 overflow-y-auto">
                      <Show when={filteredResources().length > 0} fallback={
                        <div class="p-4 text-center text-xs text-gray-500 dark:text-gray-400">
                          No resources found
                        </div>
                      }>
                        <For each={filteredResources()}>
                          {(resource) => {
                            const isAlreadyAdded = () => aiChatStore.hasContextItem(resource.id);
                            return (
                              <button
                                type="button"
                                onClick={() => !isAlreadyAdded() && addResourceToContext(resource)}
                                disabled={isAlreadyAdded()}
                                class={`w-full px-3 py-2 text-left flex items-center gap-2 text-xs transition-colors ${isAlreadyAdded()
                                  ? 'bg-purple-50 dark:bg-purple-900/20 text-gray-400 dark:text-gray-500 cursor-default'
                                  : 'hover:bg-gray-50 dark:hover:bg-gray-700/50 text-gray-700 dark:text-gray-300'
                                  }`}
                              >
                                {/* Type icon */}
                                <span class={`flex-shrink-0 w-5 h-5 rounded flex items-center justify-center text-[9px] font-bold uppercase ${resource.type === 'vm' ? 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-400' :
                                  resource.type === 'container' ? 'bg-green-100 text-green-600 dark:bg-green-900/40 dark:text-green-400' :
                                    resource.type === 'node' ? 'bg-orange-100 text-orange-600 dark:bg-orange-900/40 dark:text-orange-400' :
                                      resource.type === 'host' ? 'bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-400' :
                                        'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
                                  }`}>
                                  {resource.type === 'vm' ? 'VM' :
                                    resource.type === 'container' ? 'CT' :
                                      resource.type === 'node' ? 'N' :
                                        resource.type === 'host' ? 'H' : '?'}
                                </span>

                                {/* Name and details */}
                                <div class="flex-1 min-w-0">
                                  <div class="font-medium truncate">{resource.name}</div>
                                  <Show when={resource.node}>
                                    <div class="text-[10px] text-gray-400">{resource.node}</div>
                                  </Show>
                                </div>

                                {/* Status indicator */}
                                <span class={`flex-shrink-0 w-2 h-2 rounded-full ${resource.status === 'running' || resource.status === 'online' ? 'bg-green-500' :
                                  resource.status === 'stopped' || resource.status === 'offline' ? 'bg-gray-400' :
                                    'bg-yellow-500'
                                  }`} />

                                {/* Check if already added */}
                                <Show when={isAlreadyAdded()}>
                                  <svg class="w-3.5 h-3.5 text-purple-500" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                                  </svg>
                                </Show>
                              </button>
                            );
                          }}
                        </For>
                      </Show>
                    </div>

                    {/* Close button */}
                    <div class="p-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
                      <button
                        type="button"
                        onClick={() => { setShowContextPicker(false); setContextSearch(''); }}
                        class="w-full px-2 py-1 text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                      >
                        Close
                      </button>
                    </div>
                  </div>
                </Show>
              </div>
            </div>

            {/* Empty state hint */}
            <Show when={aiChatStore.contextItems.length === 0}>
              <p class="mt-2 text-[10px] text-gray-400 dark:text-gray-500">
                Add VMs, containers, or hosts to provide context for your questions
              </p>
            </Show>

            {/* Guest Notes - show for first context item */}
            <Show when={aiChatStore.contextItems.length > 0}>
              <GuestNotes
                guestId={`${aiChatStore.contextItems[0].type}-${aiChatStore.contextItems[0].id}`}
                guestName={aiChatStore.contextItems[0].name}
                guestType={aiChatStore.contextItems[0].type}
              />
            </Show>
          </div>
          {/* Queued message indicator */}
          <Show when={queuedMessage()}>
            <div class="flex items-center gap-2 px-3 py-2 mb-2 text-xs rounded-lg bg-amber-50 dark:bg-amber-900/30 border border-amber-200 dark:border-amber-700 text-amber-700 dark:text-amber-300">
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="flex-1 truncate">
                <span class="font-medium">Queued:</span> "{queuedMessage()!.substring(0, 50)}{queuedMessage()!.length > 50 ? '...' : ''}"
              </span>
              <button
                type="button"
                onClick={() => setQueuedMessage(null)}
                class="p-0.5 rounded hover:bg-amber-200 dark:hover:bg-amber-800 transition-colors"
                title="Cancel queued message"
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </Show>
          <form onSubmit={handleSubmit} class="flex gap-2">
            <textarea
              ref={inputRef}
              value={input()}
              onInput={(e) => setInput(e.currentTarget.value)}
              onKeyDown={handleKeyDown}
              placeholder={
                isLoading()
                  ? queuedMessage()
                    ? "Type another message to replace queued..."
                    : "Type to queue your next message..."
                  : aiChatStore.contextItems.length > 0
                    ? `Ask about ${aiChatStore.contextItems.length} item${aiChatStore.contextItems.length > 1 ? 's' : ''} in context...`
                    : "Ask about your infrastructure..."
              }
              rows={2}
              class={`flex-1 px-3 py-2 text-sm rounded-lg border bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:border-transparent resize-none transition-colors ${isLoading()
                ? 'border-amber-300 dark:border-amber-600 focus:ring-amber-500'
                : 'border-gray-300 dark:border-gray-600 focus:ring-purple-500'
                }`}
            />
            <Show
              when={isLoading()}
              fallback={
                <button
                  type="submit"
                  disabled={!input().trim()}
                  class="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors self-end"
                  title="Send message"
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
                    />
                  </svg>
                </button>
              }
            >
              <div class="flex flex-col gap-1 self-end">
                {/* Queue button when AI is working */}
                <button
                  type="submit"
                  disabled={!input().trim()}
                  class="px-4 py-2 bg-amber-500 text-white rounded-lg hover:bg-amber-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  title={queuedMessage() ? "Replace queued message" : "Queue message for when AI finishes"}
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </button>
                {/* Stop button */}
                <button
                  type="button"
                  onClick={handleStop}
                  class="px-4 py-2 bg-red-500 text-white rounded-lg hover:bg-red-600 transition-colors"
                  title="Stop generating"
                >
                  <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <rect x="6" y="6" width="12" height="12" rx="1" />
                  </svg>
                </button>
              </div>
            </Show>
          </form>
          <p class="text-xs text-gray-400 dark:text-gray-500 mt-2">
            {isLoading()
              ? "Type and press Enter to queue your next message"
              : "Press Enter to send, Shift+Enter for new line"
            }
          </p>
        </div>
      </Show>
    </div>
  );
};

export default AIChat;
