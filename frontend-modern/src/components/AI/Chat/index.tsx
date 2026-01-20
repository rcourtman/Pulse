import { Component, Show, createSignal, onMount, onCleanup, For, createMemo, createEffect } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { AIChatAPI, type ChatSession } from '@/api/aiChat';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { logger } from '@/utils/logger';
import { useChat } from './hooks/useChat';
import { ChatMessages } from './ChatMessages';
import { ModelSelector } from './ModelSelector';
import type { PendingApproval, PendingQuestion, ModelInfo } from './types';

const MODEL_LEGACY_STORAGE_KEY = 'pulse:ai_chat_model';
const MODEL_SESSION_STORAGE_KEY = 'pulse:ai_chat_models_by_session';
const DEFAULT_SESSION_KEY = '__default__';

interface AIChatProps {
  onClose: () => void;
}

/**
 * AIChat - Main chat panel component.
 * 
 * Provides a terminal-like chat experience with clear status indicators,
 * session management, and streaming response display.
 */
export const AIChat: Component<AIChatProps> = (props) => {
  // UI state - use store's isOpenSignal for reactivity
  const isOpen = aiChatStore.isOpenSignal;
  const [input, setInput] = createSignal('');
  const [sessions, setSessions] = createSignal<ChatSession[]>([]);
  const [showSessions, setShowSessions] = createSignal(false);
  const [models, setModels] = createSignal<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);
  const [modelsError, setModelsError] = createSignal('');
  const [defaultModel, setDefaultModel] = createSignal('');
  const [chatOverrideModel, setChatOverrideModel] = createSignal('');

  const loadModelSelections = (): Record<string, string> => {
    try {
      const raw = localStorage.getItem(MODEL_SESSION_STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) : {};
      const selections = typeof parsed === 'object' && parsed ? parsed as Record<string, string> : {};
      const legacy = localStorage.getItem(MODEL_LEGACY_STORAGE_KEY);
      if (legacy && !selections[DEFAULT_SESSION_KEY]) {
        selections[DEFAULT_SESSION_KEY] = legacy;
      }
      if (legacy) {
        localStorage.removeItem(MODEL_LEGACY_STORAGE_KEY);
      }
      return selections;
    } catch (error) {
      logger.warn('[AIChat] Failed to read stored models:', error);
      return {};
    }
  };

  const persistModelSelections = (selections: Record<string, string>) => {
    try {
      localStorage.setItem(MODEL_SESSION_STORAGE_KEY, JSON.stringify(selections));
    } catch (error) {
      logger.warn('[AIChat] Failed to persist model selections:', error);
    }
  };

  const initialModelSelections = loadModelSelections();
  const [modelSelections, setModelSelections] = createSignal<Record<string, string>>(initialModelSelections);

  const getStoredModel = (sessionId: string) => {
    const key = sessionId || DEFAULT_SESSION_KEY;
    return modelSelections()[key] || '';
  };

  const updateStoredModel = (sessionId: string, modelId: string) => {
    const key = sessionId || DEFAULT_SESSION_KEY;
    setModelSelections((prev) => {
      const next = { ...prev };
      if (modelId) {
        next[key] = modelId;
      } else {
        delete next[key];
      }
      persistModelSelections(next);
      return next;
    });
  };

  // Chat hook
  const chat = useChat({ model: initialModelSelections[DEFAULT_SESSION_KEY] || '' });

  const defaultModelLabel = createMemo(() => {
    const fallback = defaultModel().trim();
    if (!fallback) return '';
    const match = models().find((model) => model.id === fallback);
    return match ? (match.name || match.id.split(':').pop() || match.id) : fallback;
  });

  const chatOverrideLabel = createMemo(() => {
    const override = chatOverrideModel().trim();
    if (!override) return '';
    const match = models().find((model) => model.id === override);
    return match ? (match.name || match.id.split(':').pop() || match.id) : override;
  });


  const loadModels = async (notify = false) => {
    if (notify) {
      notificationStore.info('Refreshing models...', 2000);
    }
    setModelsLoading(true);
    setModelsError('');
    try {
      const result = await AIAPI.getModels();
      const nextModels = result.models || [];
      setModels(nextModels);
      if (result.error) {
        setModelsError(result.error);
        if (notify) {
          notificationStore.warning(result.error, 6000);
        }
      } else if (notify) {
        notificationStore.success(`Models refreshed (${nextModels.length})`, 2000);
      }
    } catch (error) {
      logger.error('[AIChat] Failed to load models:', error);
      setModels([]);
      const message = error instanceof Error ? error.message : 'Failed to load models.';
      setModelsError(message);
      notificationStore.error(message);
    } finally {
      setModelsLoading(false);
    }
  };

  const loadSettings = async () => {
    try {
      const settings = await AIAPI.getSettings();
      const chatOverride = (settings.chat_model || '').trim();
      const fallback = chatOverride || (settings.model || '').trim();
      setDefaultModel(fallback);
      setChatOverrideModel(chatOverride);
    } catch (error) {
      logger.error('[AIChat] Failed to load AI settings:', error);
    }
  };

  const selectModel = (modelId: string) => {
    chat.setModel(modelId);
    updateStoredModel(chat.sessionId(), modelId);
  };

  createEffect(() => {
    const sessionId = chat.sessionId();
    const storedModel = getStoredModel(sessionId);
    if (storedModel) {
      if (chat.model() !== storedModel) {
        chat.setModel(storedModel);
      }
      return;
    }
    // If there's no stored model for this session but we have a current selection,
    // preserve it (and migrate it to this session)
    const currentModel = chat.model();
    if (currentModel && sessionId) {
      updateStoredModel(sessionId, currentModel);
    }
  });

  // Compute current status for display
  const currentStatus = createMemo(() => {
    if (!chat.isLoading()) return null;

    const messages = chat.messages();
    const lastMessage = messages[messages.length - 1];

    if (!lastMessage || lastMessage.role !== 'assistant') {
      return { type: 'thinking', text: 'Thinking...' };
    }

    if (lastMessage.pendingTools && lastMessage.pendingTools.length > 0) {
      const tool = lastMessage.pendingTools[0];
      const toolName = tool.name.replace(/^pulse_/, '').replace(/_/g, ' ');
      return { type: 'tool', text: `Running ${toolName}...` };
    }

    if (lastMessage.isStreaming) {
      return { type: 'generating', text: 'Generating response...' };
    }

    return { type: 'thinking', text: 'Thinking...' };
  });

  // Load sessions on mount
  onMount(async () => {
    try {
      const status = await AIChatAPI.getStatus();
      if (!status.running) {
        notificationStore.warning('AI is not running');
        return;
      }
      const sessionList = await AIChatAPI.listSessions();
      setSessions(sessionList);
      await loadSettings();
      await loadModels();
    } catch (error) {
      logger.error('[AIChat] Failed to initialize:', error);
    }
  });

  // Click outside handler to close all dropdowns
  onMount(() => {
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      // Only close if click is outside dropdown containers
      if (!target.closest('[data-dropdown]')) {
        setShowSessions(false);
      }
    };
    document.addEventListener('click', handleClickOutside);
    onCleanup(() => document.removeEventListener('click', handleClickOutside));
  });

  // Handle submit
  const handleSubmit = () => {
    if (chat.isLoading()) return;
    const prompt = input().trim();
    if (!prompt) return;
    chat.sendMessage(prompt);
    setInput('');
  };

  // Handle key down - submit when not loading
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  // New conversation
  const handleNewConversation = async () => {
    await chat.newSession();
    setShowSessions(false);
  };

  // Load session
  const handleLoadSession = async (sessionId: string) => {
    await chat.loadSession(sessionId);
    setShowSessions(false);
  };

  // Delete session
  const handleDeleteSession = async (sessionId: string, e: Event) => {
    e.stopPropagation();
    if (!confirm('Delete this conversation?')) return;
    try {
      await AIChatAPI.deleteSession(sessionId);
      setSessions(prev => prev.filter(s => s.id !== sessionId));
      updateStoredModel(sessionId, '');
      if (chat.sessionId() === sessionId) {
        chat.clearMessages();
      }
    } catch (_error) {
      notificationStore.error('Failed to delete session');
    }
  };

  // Approval handlers
  const handleApprove = async (messageId: string, approval: PendingApproval) => {
    if (!approval.approvalId) {
      notificationStore.error('No approval ID available');
      return;
    }

    // Mark as executing
    chat.updateApproval(messageId, approval.toolId, { isExecuting: true });

    try {
      // Call the approve endpoint - this marks it as approved in the backend
      // The agentic loop will detect this and execute the command
      // Execution results will come via tool_end event in the stream
      await AIChatAPI.approveCommand(approval.approvalId);

      // Remove from pending approvals - the tool_end event will show the result
      chat.updateApproval(messageId, approval.toolId, { removed: true });

      logger.debug('[AIChat] Command approved, waiting for agentic loop to execute', {
        approvalId: approval.approvalId,
        toolName: approval.toolName,
      });

      // Note: We don't manually add tool results or send continuation messages here.
      // The agentic loop will:
      // 1. Detect the approval
      // 2. Re-execute the tool with the approval_id
      // 3. Send a tool_end event with the result
      // 4. Continue the conversation automatically

    } catch (error) {
      logger.error('[AIChat] Approval failed:', error);
      notificationStore.error('Failed to approve command');
      chat.updateApproval(messageId, approval.toolId, { isExecuting: false });
    }
  };

  const handleSkip = async (messageId: string, toolId: string) => {
    // Find the approval to get the approvalId
    const msg = chat.messages().find((m) => m.id === messageId);
    const approval = msg?.pendingApprovals?.find((a) => a.toolId === toolId);

    if (!approval?.approvalId) {
      // Just remove from UI if no approval ID
      chat.updateApproval(messageId, toolId, { removed: true });
      return;
    }

    try {
      await AIChatAPI.denyCommand(approval.approvalId, 'User skipped');
      chat.updateApproval(messageId, toolId, { removed: true });
    } catch (error) {
      logger.error('[AIChat] Skip/deny failed:', error);
      // Still remove from UI even if API fails
      chat.updateApproval(messageId, toolId, { removed: true });
    }
  };

  // Question handlers
  const handleAnswerQuestion = async (messageId: string, question: PendingQuestion, answers: Array<{ id: string; value: string }>) => {
    await chat.answerQuestion(messageId, question.questionId, answers);
  };

  const handleSkipQuestion = (messageId: string, questionId: string) => {
    // Just remove from UI - skipping a question
    chat.updateQuestion(messageId, questionId, { removed: true });
  };


  return (
    <div
      class={`flex-shrink-0 h-full bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-700 flex flex-col transition-all duration-300 overflow-hidden ${isOpen() ? 'w-full sm:w-[480px]' : 'w-0 border-l-0'
        }`}
    >
      <Show when={isOpen()}>
        {/* Header - wraps on mobile */}
        <div class="flex flex-wrap items-center justify-between gap-2 px-4 py-3 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-gradient-to-br from-purple-500 to-violet-500 rounded-xl shadow-lg shadow-purple-500/20">
              <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5" />
              </svg>
            </div>
            <div>
              <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">AI Assistant</h2>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">
                Powered by Pulse
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1.5">
            {/* New chat */}
            <button
              onClick={handleNewConversation}
              class="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] text-slate-600 dark:text-slate-300 hover:text-slate-800 dark:hover:text-slate-100 rounded-lg border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600 bg-white dark:bg-slate-800 transition-colors"
              title="New chat"
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
              </svg>
              <span class="font-medium">New</span>
            </button>

            {/* Model selector */}
            <ModelSelector
              models={models()}
              selectedModel={chat.model()}
              defaultModelLabel={defaultModelLabel()}
              chatOverrideModel={chatOverrideModel()}
              chatOverrideLabel={chatOverrideLabel()}
              isLoading={modelsLoading()}
              error={modelsError()}
              onModelSelect={selectModel}
              onRefresh={() => loadModels(true)}
            />

            {/* Session picker */}
            <div class="relative" data-dropdown>
              <button
                onClick={() => {
                  const next = !showSessions();
                  setShowSessions(next);
                }}
                class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
                title="Chat sessions"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              </button>

              <Show when={showSessions()}>
                <div class="absolute right-0 top-full mt-1 w-72 max-h-96 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 z-50 overflow-hidden">
                  <button
                    onClick={handleNewConversation}
                    class="w-full px-3 py-2.5 text-left text-sm flex items-center gap-2 text-purple-600 dark:text-purple-400 hover:bg-purple-50 dark:hover:bg-purple-900/20 border-b border-slate-200 dark:border-slate-700"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                    </svg>
                    <span class="font-medium">New conversation</span>
                  </button>

                  <div class="max-h-64 overflow-y-auto">
                    <Show when={sessions().length > 0} fallback={
                      <div class="px-3 py-6 text-center text-xs text-slate-500 dark:text-slate-400">
                        No previous conversations
                      </div>
                    }>
                      <For each={sessions()}>
                        {(session) => (
                          <div
                            class={`group relative px-3 py-2.5 flex items-start gap-2 hover:bg-slate-50 dark:hover:bg-slate-700/50 cursor-pointer ${chat.sessionId() === session.id ? 'bg-purple-50 dark:bg-purple-900/20' : ''}`}
                            onClick={() => handleLoadSession(session.id)}
                          >
                            <div class="flex-1 min-w-0">
                              <div class="text-sm font-medium truncate text-slate-900 dark:text-slate-100">
                                {session.title || 'Untitled'}
                              </div>
                              <div class="text-xs text-slate-500 dark:text-slate-400">
                                {session.message_count} messages
                              </div>
                            </div>
                            <button
                              type="button"
                              class="flex-shrink-0 p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-red-100 dark:hover:bg-red-900/30 text-slate-400 hover:text-red-500 transition-opacity"
                              onClick={(e) => handleDeleteSession(session.id, e)}
                            >
                              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          </div>
                        )}
                      </For>
                    </Show>
                  </div>
                </div>
              </Show>
            </div>

            {/* Close button */}
            <button
              onClick={(e) => {
                e.stopPropagation();
                props.onClose();
              }}
              class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
              title="Close panel"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 5l7 7-7 7M6 5l7 7-7 7" />
              </svg>
            </button>
          </div>
        </div>

        {/* Messages */}
        <ChatMessages
          messages={chat.messages()}
          onApprove={handleApprove}
          onSkip={handleSkip}
          onAnswerQuestion={handleAnswerQuestion}
          onSkipQuestion={handleSkipQuestion}
          emptyState={{
            title: 'Start a conversation',
            subtitle: 'Ask about your infrastructure, diagnose issues, or get help.',
            suggestions: [
              'What VMs are running?',
              'Show me resource usage',
              'Any issues to investigate?',
            ],
            onSuggestionClick: (s) => setInput(s),
          }}
        />

        {/* Status indicator bar */}
        <Show when={currentStatus()}>
          <div class="px-4 py-2 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-200 dark:border-slate-700 flex items-center gap-2.5 text-xs">
            {/* Status icon based on type */}
            <Show when={currentStatus()?.type === 'thinking'}>
              <div class="flex items-center justify-center w-4 h-4">
                <svg class="w-3.5 h-3.5 text-blue-500 dark:text-blue-400 animate-pulse" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
              </div>
            </Show>
            <Show when={currentStatus()?.type === 'tool'}>
              <div class="flex items-center justify-center w-4 h-4">
                <svg class="w-3.5 h-3.5 text-purple-500 dark:text-purple-400 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
              </div>
            </Show>
            <Show when={currentStatus()?.type === 'generating'}>
              <div class="flex items-center justify-center w-4 h-4">
                <svg class="w-3.5 h-3.5 text-emerald-500 dark:text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                </svg>
              </div>
            </Show>

            <span class="text-slate-600 dark:text-slate-400 font-medium">
              {currentStatus()?.text}
            </span>

            {/* Subtle animated dots */}
            <div class="flex gap-0.5 ml-1">
              <span class="w-1 h-1 rounded-full bg-slate-400 dark:bg-slate-500 animate-bounce" style="animation-delay: 0ms; animation-duration: 1s" />
              <span class="w-1 h-1 rounded-full bg-slate-400 dark:bg-slate-500 animate-bounce" style="animation-delay: 150ms; animation-duration: 1s" />
              <span class="w-1 h-1 rounded-full bg-slate-400 dark:bg-slate-500 animate-bounce" style="animation-delay: 300ms; animation-duration: 1s" />
            </div>
          </div>
        </Show>

        {/* Input */}
        <div class="border-t border-slate-200 dark:border-slate-700 p-4 bg-white dark:bg-slate-900">
          <form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="flex gap-2">
            <textarea
              value={input()}
              onInput={(e) => setInput(e.currentTarget.value)}
              onKeyDown={handleKeyDown}
              placeholder="Ask about your infrastructure..."
              rows={2}
              class="flex-1 px-4 py-3 text-sm rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500 focus:border-transparent resize-none"
            />
            <div class="flex gap-1.5 self-end">
              <Show
                when={!chat.isLoading()}
                fallback={
                  <button
                    type="button"
                    onClick={chat.stop}
                    class="px-4 py-3 bg-red-500 hover:bg-red-600 text-white rounded-xl transition-colors shadow-lg shadow-red-500/20"
                    title="Stop"
                  >
                    <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                      <rect x="6" y="6" width="12" height="12" rx="1" />
                    </svg>
                  </button>
                }
              >
                <button
                  type="submit"
                  disabled={!input().trim()}
                  class="px-4 py-3 bg-gradient-to-r from-purple-600 to-violet-600 hover:from-purple-700 hover:to-violet-700 text-white rounded-xl disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg shadow-purple-500/20 hover:shadow-xl hover:shadow-purple-500/30"
                  title="Send"
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                  </svg>
                </button>
              </Show>
            </div>
          </form>
          <p class="text-[10px] text-slate-400 dark:text-slate-500 mt-2 text-center">
            {chat.isLoading()
              ? 'Generating... click Stop to interrupt'
              : 'Press Enter to send · Shift+Enter for new line'}
          </p>
        </div>
      </Show>
    </div>
  );
};

export default AIChat;
