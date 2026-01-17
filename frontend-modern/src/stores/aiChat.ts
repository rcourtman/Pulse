import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';
// NOTE: AIAPI import removed - session management is handled by OpenCode's embedded UI
import type { AIChatSessionSummary } from '@/types/ai';

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

// Local storage keys
const HISTORY_STORAGE_KEY = 'pulse:ai_chat_history';
const SESSION_ID_KEY = 'pulse:ai_chat_session_id';

// Generate a unique session ID
const generateSessionId = (): string => {
  return `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
};

// Load session ID from storage or generate new one
const loadOrCreateSessionId = (): string => {
  try {
    const stored = localStorage.getItem(SESSION_ID_KEY);
    if (stored) return stored;
  } catch (e) {
    logger.error('Failed to load session ID:', e);
  }
  const newId = generateSessionId();
  try {
    localStorage.setItem(SESSION_ID_KEY, newId);
  } catch (e) {
    logger.error('Failed to save session ID:', e);
  }
  return newId;
};

// Load initial messages from local storage (fallback/cache)
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

// Save messages to local storage (cache)
const saveMessagesToStorage = (msgs: Message[]) => {
  try {
    localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(msgs));
  } catch (e) {
    logger.error('Failed to save chat history:', e);
  }
};

// Global state for the AI chat drawer
const [isAIChatOpen, setIsAIChatOpen] = createSignal(false);
const [aiChatContext, setAIChatContext] = createSignal<AIChatContext>({});
const [contextItems, setContextItems] = createSignal<ContextItem[]>([]);
const [messages, setMessages] = createSignal<Message[]>(loadMessagesFromStorage());
const [aiEnabled, setAiEnabled] = createSignal<boolean | null>(null); // null = not checked yet

// Session management state
const [currentSessionId, setCurrentSessionId] = createSignal<string>(loadOrCreateSessionId());
const [sessionTitle, setSessionTitle] = createSignal<string>('');
const [_sessions, _setSessions] = createSignal<AIChatSessionSummary[]>([]);
const [_syncEnabled, _setSyncEnabled] = createSignal<boolean>(true);
const [_isSyncing, _setIsSyncing] = createSignal<boolean>(false);

// Debounce timer for saving
let saveDebounceTimer: ReturnType<typeof setTimeout> | null = null;
const SAVE_DEBOUNCE_MS = 2000; // Save 2 seconds after last change

// Store reference to AI input for focusing from keyboard shortcuts
let aiInputRef: HTMLTextAreaElement | null = null;

// Sync current session to server (debounced)
// NOTE: Session sync is disabled - OpenCode handles session management internally
const syncToServer = async () => {
  // Disabled: OpenCode manages sessions through its embedded UI
  return;
};

// Debounced sync
const debouncedSync = () => {
  if (saveDebounceTimer) {
    clearTimeout(saveDebounceTimer);
  }
  saveDebounceTimer = setTimeout(syncToServer, SAVE_DEBOUNCE_MS);
};

// Load session from server
// NOTE: Session sync is disabled - OpenCode handles session management internally
const loadSessionFromServer = async (_sessionId: string): Promise<boolean> => {
  // Disabled: OpenCode manages sessions through its embedded UI
  return false;
};

export const aiChatStore = {
  // Check if chat is open (non-reactive getter for simple checks)
  get isOpen() {
    return isAIChatOpen();
  },

  // Reactive accessor - use this in Show/createEffect for proper reactivity
  isOpenSignal: isAIChatOpen,

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

  // Get current session ID
  get sessionId() {
    return currentSessionId();
  },

  // Get session title
  get title() {
    return sessionTitle();
  },

  // Get all sessions (for session picker)
  get sessions() {
    return _sessions();
  },

  // Check if syncing
  get syncing() {
    return _isSyncing();
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
    saveMessagesToStorage(msgs);
    debouncedSync();
  },

  // Set session title
  setTitle(title: string) {
    setSessionTitle(title);
    debouncedSync();
  },

  // Initialize sync - call this on app startup
  async initSync() {
    try {
      // Try to load current session from server
      const sessionId = currentSessionId();
      const loaded = await loadSessionFromServer(sessionId);

      if (!loaded) {
        // Server doesn't have this session, use local storage
        const localMessages = loadMessagesFromStorage();
        if (localMessages.length > 0) {
          // Sync local messages to server
          setMessages(localMessages);
          await syncToServer();
        }
      }

      // Load session list
      await this.refreshSessions();
    } catch (e) {
      logger.error('Failed to initialize chat sync:', e);
    }
  },

  // Refresh session list from server
  // NOTE: Session sync is disabled - OpenCode handles session management internally
  async refreshSessions() {
    // Disabled: OpenCode manages sessions through its embedded UI
    return;
  },

  // Switch to a different session
  async switchSession(sessionId: string) {
    // Save current session first
    await syncToServer();

    // Load new session
    setCurrentSessionId(sessionId);
    try {
      localStorage.setItem(SESSION_ID_KEY, sessionId);
    } catch (e) {
      logger.error('Failed to save session ID:', e);
    }

    const loaded = await loadSessionFromServer(sessionId);
    if (!loaded) {
      // Session doesn't exist, clear messages
      setMessages([]);
      setSessionTitle('');
      saveMessagesToStorage([]);
    }
  },

  // Start a new conversation
  async newConversation() {
    // Save current session first
    await syncToServer();

    // Generate new session ID
    const newId = generateSessionId();
    setCurrentSessionId(newId);
    try {
      localStorage.setItem(SESSION_ID_KEY, newId);
    } catch (e) {
      logger.error('Failed to save session ID:', e);
    }

    // Clear messages
    setMessages([]);
    setSessionTitle('');
    saveMessagesToStorage([]);
    localStorage.removeItem(HISTORY_STORAGE_KEY);

    // Refresh session list
    await this.refreshSessions();
  },

  // Delete a session
  // NOTE: Session management is handled by OpenCode's embedded UI
  async deleteSession(_sessionId: string) {
    // Disabled: OpenCode manages sessions through its embedded UI
    return;
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

  // Clear conversation (start fresh) - now creates a new session
  clearConversation() {
    this.newConversation();
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

  // Force sync now (for manual save)
  async syncNow() {
    if (saveDebounceTimer) {
      clearTimeout(saveDebounceTimer);
      saveDebounceTimer = null;
    }
    await syncToServer();
  },
};
