import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';

interface AIChatContext {
  targetType?: string;
  targetId?: string;
  context?: Record<string, unknown>;
  initialPrompt?: string;
  findingId?: string; // If opened from AI Insights "Get Help", the finding ID to resolve on success
}

// A single context item that can be accumulated
interface ContextItem {
  id: string; // unique identifier (e.g., "vm-delly-101")
  type: string; // "vm", "container", "storage", "node", etc.
  name: string; // display name
  data: Record<string, unknown>; // the actual context data
  addedAt: Date;
}

// Message type for persisted conversation
interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  model?: string;
  tokens?: { input: number; output: number };
  toolCalls?: Array<{
    name: string;
    input: string;
    output: string;
    success: boolean;
  }>;
}

// Local storage key
const HISTORY_STORAGE_KEY = 'pulse:ai_chat_history';

// Load initial messages from storage
const loadMessagesFromStorage = (): Message[] => {
  try {
    const stored = localStorage.getItem(HISTORY_STORAGE_KEY);
    if (!stored) return [];

    const parsed = JSON.parse(stored);
    // Revive Date objects
    return parsed.map((m: any) => ({
      ...m,
      timestamp: new Date(m.timestamp)
    }));
  } catch (e) {
    logger.error('Failed to load chat history:', e);
    return [];
  }
};

// Global state for the AI chat drawer
const [isAIChatOpen, setIsAIChatOpen] = createSignal(false);
const [aiChatContext, setAIChatContext] = createSignal<AIChatContext>({});
const [contextItems, setContextItems] = createSignal<ContextItem[]>([]);
const [messages, setMessages] = createSignal<Message[]>(loadMessagesFromStorage());
const [aiEnabled, setAiEnabled] = createSignal<boolean | null>(null); // null = not checked yet

// Store reference to AI input for focusing from keyboard shortcuts
let aiInputRef: HTMLTextAreaElement | null = null;

export const aiChatStore = {
  // Check if chat is open
  get isOpen() {
    return isAIChatOpen();
  },

  // Get current context (legacy single-item)
  get context() {
    return aiChatContext();
  },

  // Get all accumulated context items
  get contextItems() {
    return contextItems();
  },

  // Get messages (for persistence)
  get messages() {
    return messages();
  },

  // Get AI enabled state
  get enabled() {
    return aiEnabled();
  },

  // Check if a specific item is in context
  hasContextItem(id: string) {
    return contextItems().some(item => item.id === id);
  },

  // Set AI enabled state (called from settings check)
  setEnabled(enabled: boolean) {
    setAiEnabled(enabled);
  },

  // Set messages (for persistence from AIChat component)
  setMessages(msgs: Message[]) {
    setMessages(msgs);
    try {
      localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(msgs));
    } catch (e) {
      logger.error('Failed to save chat history:', e);
    }
  },

  // Toggle the AI chat panel
  toggle() {
    setIsAIChatOpen(!isAIChatOpen());
  },

  // Open the AI chat with optional context
  open(context?: AIChatContext) {
    if (context) {
      setAIChatContext(context);
    }
    setIsAIChatOpen(true);
  },

  // Close the AI chat
  close() {
    setIsAIChatOpen(false);
    // Keep context and messages for when user reopens
  },

  // Update context without opening (for navigation-based context changes)
  setContext(context: AIChatContext) {
    setAIChatContext(context);
  },

  // Clear single-item context (legacy)
  clearContext() {
    setAIChatContext({});
  },

  // Add an item to the context (accumulative)
  addContextItem(type: string, id: string, name: string, data: Record<string, unknown>) {
    setContextItems(prev => {
      // Don't add duplicates
      if (prev.some(item => item.id === id)) {
        // Update existing item with new data
        return prev.map(item =>
          item.id === id
            ? { ...item, data, addedAt: new Date() }
            : item
        );
      }
      return [...prev, { id, type, name, data, addedAt: new Date() }];
    });
    // Also update legacy context to point to most recently added
    setAIChatContext({
      targetType: type,
      targetId: id,
      context: data,
    });
  },

  // Remove an item from context
  removeContextItem(id: string) {
    setContextItems(prev => prev.filter(item => item.id !== id));
    // Update legacy context if we removed the current one
    const current = aiChatContext();
    if (current.targetId === id) {
      const remaining = contextItems().filter(item => item.id !== id);
      if (remaining.length > 0) {
        const last = remaining[remaining.length - 1];
        setAIChatContext({
          targetType: last.type,
          targetId: last.id,
          context: last.data,
        });
      } else {
        setAIChatContext({});
      }
    }
  },

  // Clear all context items
  clearAllContext() {
    setContextItems([]);
    setAIChatContext({});
  },

  // Clear conversation (start fresh)
  clearConversation() {
    setMessages([]);
    localStorage.removeItem(HISTORY_STORAGE_KEY);
  },

  // Convenience method to update context for a specific target (host, VM, container, etc.)
  // This is called when user selects/views a specific resource
  setTargetContext(targetType: string, targetId: string, additionalContext?: Record<string, unknown>) {
    // Use addContextItem instead of replacing
    const name = (additionalContext?.guestName as string) ||
      (additionalContext?.name as string) ||
      targetId;
    this.addContextItem(targetType, targetId, name, additionalContext || {});
  },

  // Open for a specific target - opens the panel and adds to context
  openForTarget(targetType: string, targetId: string, additionalContext?: Record<string, unknown>) {
    const name = (additionalContext?.guestName as string) ||
      (additionalContext?.name as string) ||
      targetId;
    this.addContextItem(targetType, targetId, name, additionalContext || {});
    setIsAIChatOpen(true);
  },

  // Open with a pre-filled prompt
  openWithPrompt(prompt: string, context?: Omit<AIChatContext, 'initialPrompt'>) {
    setAIChatContext({
      ...context,
      initialPrompt: prompt,
    });
    setIsAIChatOpen(true);
  },

  // Register the AI input element (called by AIChat component)
  registerInput(ref: HTMLTextAreaElement | null) {
    aiInputRef = ref;
  },

  // Focus the AI input (called by keyboard handlers)
  focusInput() {
    if (aiInputRef && isAIChatOpen()) {
      aiInputRef.focus();
      return true;
    }
    return false;
  },
};
