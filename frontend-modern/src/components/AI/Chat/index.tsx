import { Component, Show, createSignal, onMount, onCleanup, For, createMemo, createEffect } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { OpenCodeAPI, type ChatSession } from '@/api/opencode';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { logger } from '@/utils/logger';
import { useChat } from './hooks/useChat';
import { ChatMessages } from './ChatMessages';
import { PROVIDER_DISPLAY_NAMES, getProviderFromModelId, groupModelsByProvider } from '../aiChatUtils';
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
  const [showModelSelector, setShowModelSelector] = createSignal(false);
  const [models, setModels] = createSignal<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);
  const [modelsError, setModelsError] = createSignal('');
  const [modelQuery, setModelQuery] = createSignal('');
  const [defaultModel, setDefaultModel] = createSignal('');
  const [chatOverrideModel, setChatOverrideModel] = createSignal('');
  const [showSessionActions, setShowSessionActions] = createSignal(false);
  const [sessionActionLoading, setSessionActionLoading] = createSignal<string | null>(null);

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

  const selectedModelLabel = createMemo(() => {
    const selected = chat.model().trim();
    if (!selected) {
      const fallback = defaultModelLabel();
      return fallback ? `Default (${fallback})` : 'Default';
    }
    const match = models().find((model) => model.id === selected);
    if (match) return match.name || match.id.split(':').pop() || match.id;
    return selected;
  });

  const filteredModels = createMemo(() => {
    const query = modelQuery().trim().toLowerCase();
    if (!query) return models();
    return models().filter((model) => {
      const provider = getProviderFromModelId(model.id);
      const providerName = PROVIDER_DISPLAY_NAMES[provider] || provider;
      const modelName = model.name || '';
      return (
        model.id.toLowerCase().includes(query) ||
        modelName.toLowerCase().includes(query) ||
        (model.description || '').toLowerCase().includes(query) ||
        provider.toLowerCase().includes(query) ||
        providerName.toLowerCase().includes(query)
      );
    });
  });

  const customModelCandidate = createMemo(() => modelQuery().trim());
  const showCustomModelOption = createMemo(() => {
    const candidate = customModelCandidate();
    if (!candidate) return false;
    return !models().some((model) => model.id === candidate);
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
    setShowModelSelector(false);
    setModelQuery('');
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
    if (chat.model()) {
      chat.setModel('');
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
      const status = await OpenCodeAPI.getStatus();
      if (!status.running) {
        notificationStore.warning('AI is not running');
        return;
      }
      const sessionList = await OpenCodeAPI.listSessions();
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
        setShowModelSelector(false);
        setShowSessions(false);
        setShowSessionActions(false);
      }
    };
    document.addEventListener('click', handleClickOutside);
    onCleanup(() => document.removeEventListener('click', handleClickOutside));
  });

  // Handle submit
  const handleSubmit = () => {
    const prompt = input().trim();
    if (!prompt) return;
    chat.sendMessage(prompt);
    setInput('');
  };

  // Handle key down - allow sending even while loading (will abort and send)
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
      await OpenCodeAPI.deleteSession(sessionId);
      setSessions(prev => prev.filter(s => s.id !== sessionId));
      updateStoredModel(sessionId, '');
      if (chat.sessionId() === sessionId) {
        chat.clearMessages();
      }
    } catch (error) {
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
      const result = await OpenCodeAPI.approveCommand(approval.approvalId);

      // Remove from pending approvals
      chat.updateApproval(messageId, approval.toolId, { removed: true });

      // Add tool result if command was executed
      if (result.approved) {
        const typedResult = result as {
          result?: { stdout?: string; stderr?: string; exit_code?: number };
          error?: string;
          message?: string;
          executed?: boolean;
        };
        const execResult = typedResult.result;

        let output: string;
        let success: boolean;

        if (execResult) {
          output = `Exit code: ${execResult.exit_code}\n${execResult.stdout || ''}${execResult.stderr ? '\nStderr: ' + execResult.stderr : ''}`;
          success = execResult.exit_code === 0;
        } else if (typedResult.error) {
          output = `Execution failed: ${typedResult.error}`;
          success = false;
        } else if (typedResult.message) {
          output = typedResult.message;
          success = false;
        } else {
          output = 'Command approved but no execution result available.';
          success = false;
        }

        chat.addToolResult(messageId, {
          name: approval.toolName,
          input: approval.command,
          output: output.trim(),
          success,
        });

        // Continue the conversation - short message, output is already in the tool result
        const continuationMessage = success
          ? 'Command executed. Please analyze the result and continue.'
          : 'Command completed with issues. Please analyze and advise.';

        // Wait for any ongoing stream to complete before sending continuation
        if (chat.isLoading()) {
          logger.debug('[AIChat] Waiting for chat to become idle before continuation');
          const isIdle = await chat.waitForIdle(30000);
          if (!isIdle) {
            logger.warn('[AIChat] Timeout waiting for chat to become idle');
            notificationStore.warning('Chat is busy. Sending continuation anyway...');
          }
        }

        logger.debug('[AIChat] Sending continuation message', { sessionId: chat.sessionId() });
        await chat.sendMessage(continuationMessage);
      }
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
      await OpenCodeAPI.denyCommand(approval.approvalId, 'User skipped');
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

  const toggleModelSelector = () => {
    const next = !showModelSelector();
    setShowModelSelector(next);
    if (next) {
      setShowSessions(false);
      setModelQuery('');
      if (models().length === 0 && !modelsLoading()) {
        loadModels();
      }
    }
  };

  const handleModelInputKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const candidate = customModelCandidate();
    if (candidate) {
      selectModel(candidate);
    }
  };

  // Session action handlers
  const handleSummarize = async () => {
    const sessionId = chat.sessionId();
    if (!sessionId) return;

    setSessionActionLoading('summarize');
    setShowSessionActions(false);
    try {
      await OpenCodeAPI.summarizeSession(sessionId);
      notificationStore.success('Session summarized to save context');
    } catch (error) {
      notificationStore.error('Failed to summarize session');
    } finally {
      setSessionActionLoading(null);
    }
  };

  const handleGetDiff = async () => {
    const sessionId = chat.sessionId();
    if (!sessionId) return;

    setSessionActionLoading('diff');
    setShowSessionActions(false);
    try {
      const diff = await OpenCodeAPI.getSessionDiff(sessionId);
      const files = diff.files || [];
      if (files.length === 0) {
        notificationStore.info('No file changes in this session');
      } else {
        notificationStore.success(`${files.length} file(s) changed in this session`);
        // Could open a modal here to show detailed diff
      }
    } catch (error) {
      notificationStore.error('Failed to get session diff');
    } finally {
      setSessionActionLoading(null);
    }
  };

  const handleRevert = async () => {
    const sessionId = chat.sessionId();
    if (!sessionId) return;

    setSessionActionLoading('revert');
    setShowSessionActions(false);
    try {
      await OpenCodeAPI.revertSession(sessionId);
      notificationStore.success('Session changes reverted');
    } catch (error) {
      notificationStore.error('Failed to revert session');
    } finally {
      setSessionActionLoading(null);
    }
  };

  return (
    <div
      class={`flex-shrink-0 h-full bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-700 flex flex-col transition-all duration-300 overflow-hidden ${isOpen() ? 'w-[480px]' : 'w-0 border-l-0'
        }`}
    >
      <Show when={isOpen()}>
        {/* Header */}
        <div class="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-gradient-to-br from-purple-500 to-violet-500 rounded-xl shadow-lg shadow-purple-500/20">
              <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5" />
              </svg>
            </div>
            <div>
              <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">AI Assistant</h2>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">
                Powered by OpenCode
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1.5">
            {/* Model selector */}
            <div class="relative" data-dropdown>
              <button
                onClick={toggleModelSelector}
                class="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] text-slate-600 dark:text-slate-300 hover:text-slate-800 dark:hover:text-slate-100 rounded-lg border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600 bg-white dark:bg-slate-800 transition-colors"
                title="Select model for this chat"
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
                <span class="max-w-[120px] truncate font-medium">{selectedModelLabel()}</span>
                <Show when={modelsLoading()}>
                  <svg class="w-3 h-3 text-slate-400 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                </Show>
                <svg class="w-3 h-3 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              <Show when={showModelSelector()}>
                <div class="absolute right-0 top-full mt-1 w-80 max-h-96 overflow-hidden bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 z-50">
                  <div class="flex items-center gap-2 px-3 py-2 border-b border-slate-200 dark:border-slate-700">
                    <input
                      type="text"
                      value={modelQuery()}
                      onInput={(e) => setModelQuery(e.currentTarget.value)}
                      onKeyDown={handleModelInputKeyDown}
                      placeholder="Search or enter model ID"
                      class="flex-1 text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-200 focus:outline-none focus:ring-2 focus:ring-purple-400/50"
                    />
                    <button
                      type="button"
                      onClick={() => loadModels(true)}
                      disabled={modelsLoading()}
                      class="p-1.5 rounded-md text-slate-500 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 disabled:opacity-50"
                      title="Refresh models"
                    >
                      <svg class={`w-3.5 h-3.5 ${modelsLoading() ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v6h6M20 20v-6h-6M5.32 9A7.5 7.5 0 0119 12.5M18.68 15A7.5 7.5 0 015 11.5" />
                      </svg>
                    </button>
                  </div>

                  <Show when={modelsError()}>
                    <div class="px-3 py-2 text-[11px] text-red-500 border-b border-slate-200 dark:border-slate-700">
                      {modelsError()}
                    </div>
                  </Show>

                  <div class="max-h-72 overflow-y-auto py-1">
                    <button
                      onClick={() => selectModel('')}
                      class={`w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700 ${!chat.model() ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                    >
                      <div class="font-medium text-slate-900 dark:text-slate-100">Default</div>
                      <div class="text-[11px] text-slate-500 dark:text-slate-400">
                        {defaultModelLabel() ? `Use configured default model (${defaultModelLabel()})` : 'Use configured default model'}
                      </div>
                    </button>

                    <Show when={chatOverrideModel()}>
                      <button
                        onClick={() => selectModel(chatOverrideModel())}
                        class={`w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700 ${chat.model() === chatOverrideModel() ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                      >
                        <div class="font-medium text-slate-900 dark:text-slate-100">Chat override</div>
                        <div class="text-[11px] text-slate-500 dark:text-slate-400">
                          {chatOverrideLabel() || chatOverrideModel()}
                        </div>
                      </button>
                    </Show>

                    <Show when={showCustomModelOption()}>
                      <button
                        onClick={() => selectModel(customModelCandidate())}
                        class="w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700"
                      >
                        <div class="font-medium text-slate-900 dark:text-slate-100">
                          Use "{customModelCandidate()}"
                        </div>
                        <div class="text-[11px] text-slate-500 dark:text-slate-400">Custom model ID</div>
                      </button>
                    </Show>

                    <Show when={!modelsLoading() && filteredModels().length === 0}>
                      <div class="px-3 py-4 text-center text-[11px] text-slate-500 dark:text-slate-400">
                        No matching models.
                      </div>
                    </Show>

                    <For each={Array.from(groupModelsByProvider(filteredModels()).entries())}>
                      {([provider, providerModels]) => (
                        <>
                          <div class="px-3 py-1.5 text-[11px] font-semibold text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-700/50 sticky top-0">
                            {PROVIDER_DISPLAY_NAMES[provider] || provider}
                          </div>
                          <For each={providerModels}>
                            {(model) => (
                              <button
                                onClick={() => selectModel(model.id)}
                                class={`w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700 ${chat.model() === model.id ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                              >
                                <div class="font-medium text-slate-900 dark:text-slate-100">
                                  {model.name || model.id.split(':').pop() || model.id}
                                </div>
                                <Show when={model.description}>
                                  <div class="text-[11px] text-slate-500 dark:text-slate-400 line-clamp-2">
                                    {model.description}
                                  </div>
                                </Show>
                                <Show when={model.name && model.name !== model.id}>
                                  <div class="text-[10px] text-slate-400 dark:text-slate-500">
                                    {model.id}
                                  </div>
                                </Show>
                              </button>
                            )}
                          </For>
                        </>
                      )}
                    </For>
                  </div>
                </div>
              </Show>
            </div>

            {/* Session picker */}
            <div class="relative" data-dropdown>
              <button
                onClick={() => {
                  setShowModelSelector(false);
                  setShowSessions(!showSessions());
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

            {/* Session Actions Menu */}
            <div class="relative" data-dropdown>
              <button
                onClick={() => {
                  if (!chat.sessionId()) {
                    notificationStore.info('Send a message first to start a session');
                    return;
                  }
                  setShowModelSelector(false);
                  setShowSessions(false);
                  setShowSessionActions(!showSessionActions());
                }}
                disabled={sessionActionLoading() !== null}
                class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                title="Session actions (summarize, diff, revert)"
              >
                <Show when={sessionActionLoading()} fallback={
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                  </svg>
                }>
                  <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                </Show>
              </button>

              <Show when={showSessionActions()}>
                <div class="absolute right-0 top-full mt-1 w-48 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 z-50 overflow-hidden">
                  <button
                    onClick={handleSummarize}
                    class="w-full px-3 py-2 text-left text-sm flex items-center gap-2 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700"
                  >
                    <svg class="w-4 h-4 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16m-7 6h7" />
                    </svg>
                    Summarize context
                  </button>
                  <button
                    onClick={handleGetDiff}
                    class="w-full px-3 py-2 text-left text-sm flex items-center gap-2 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700"
                  >
                    <svg class="w-4 h-4 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                    </svg>
                    View file changes
                  </button>
                  <button
                    onClick={handleRevert}
                    class="w-full px-3 py-2 text-left text-sm flex items-center gap-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 border-t border-slate-200 dark:border-slate-700"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
                    </svg>
                    Revert changes
                  </button>
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
              {/* Send button - always visible, sends new message (aborts current if streaming) */}
              <button
                type="submit"
                disabled={!input().trim()}
                class="px-4 py-3 bg-gradient-to-r from-purple-600 to-violet-600 hover:from-purple-700 hover:to-violet-700 text-white rounded-xl disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg shadow-purple-500/20 hover:shadow-xl hover:shadow-purple-500/30"
                title={chat.isLoading() ? "Send (will interrupt current response)" : "Send"}
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                </svg>
              </button>
              {/* Stop button - only visible while streaming */}
              <Show when={chat.isLoading()}>
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
              </Show>
            </div>
          </form>
          <p class="text-[10px] text-slate-400 dark:text-slate-500 mt-2 text-center">
            Press Enter to send · Shift+Enter for new line
          </p>
        </div>
      </Show>
    </div>
  );
};

export default AIChat;
