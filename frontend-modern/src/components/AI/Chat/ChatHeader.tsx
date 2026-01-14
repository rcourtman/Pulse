import { Component, Show, For, createSignal } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';
import {
  PROVIDER_DISPLAY_NAMES,
  groupModelsByProvider,
} from '../aiChatUtils';
import type { ModelInfo } from './types';

interface ChatHeaderProps {
  title: string;
  subtitle?: string;
  onClose: () => void;
  // Model selection
  models: ModelInfo[];
  selectedModel: string;
  onModelChange: (model: string) => void;
  // Autonomous mode
  autonomousMode: boolean;
  onToggleAutonomous: () => void;
  isTogglingAutonomous: boolean;
}

export const ChatHeader: Component<ChatHeaderProps> = (props) => {
  const [showModelSelector, setShowModelSelector] = createSignal(false);
  const [showSessionPicker, setShowSessionPicker] = createSignal(false);

  return (
    <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-purple-50 to-violet-50 dark:from-purple-900/20 dark:to-violet-900/20">
      {/* Left side - title */}
      <div class="flex items-center gap-3">
        <div class="p-2 bg-gradient-to-br from-purple-500 to-violet-500 rounded-xl shadow-lg">
          <svg
            class="w-5 h-5 text-white"
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
            {props.title}
          </h2>
          <Show when={props.subtitle}>
            <p class="text-xs text-gray-500 dark:text-gray-400">{props.subtitle}</p>
          </Show>
        </div>
      </div>

      {/* Right side - controls */}
      <div class="flex items-center gap-2">
        {/* Model selector */}
        <Show when={props.models.length > 0}>
          <div class="relative">
            <button
              onClick={() => setShowModelSelector(!showModelSelector())}
              class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-gray-100 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 bg-white dark:bg-gray-800 transition-colors"
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              <span class="max-w-[80px] truncate font-medium">
                {props.selectedModel || 'Default'}
              </span>
              <svg class="w-3 h-3 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </button>

            <Show when={showModelSelector()}>
              <div class="absolute right-0 top-full mt-1 w-64 max-h-80 overflow-y-auto bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 z-50 py-1">
                <button
                  onClick={() => { props.onModelChange(''); setShowModelSelector(false); }}
                  class={`w-full px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-700 ${!props.selectedModel ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                >
                  <div class="font-medium text-gray-900 dark:text-gray-100">Default</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">Use configured default model</div>
                </button>
                <For each={Array.from(groupModelsByProvider(props.models).entries())}>
                  {([provider, models]) => (
                    <>
                      <div class="px-3 py-1.5 text-xs font-semibold text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/50 sticky top-0">
                        {PROVIDER_DISPLAY_NAMES[provider] || provider}
                      </div>
                      <For each={models}>
                        {(model) => (
                          <button
                            onClick={() => { props.onModelChange(model.id); setShowModelSelector(false); }}
                            class={`w-full px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-700 ${props.selectedModel === model.id ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
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

        {/* Autonomous mode toggle */}
        <button
          onClick={props.onToggleAutonomous}
          disabled={props.isTogglingAutonomous}
          class={`p-2 rounded-lg transition-all ${
            props.autonomousMode
              ? 'text-amber-600 dark:text-amber-400 bg-amber-100 dark:bg-amber-900/40 hover:bg-amber-200 dark:hover:bg-amber-900/60 shadow-sm'
              : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
          } ${props.isTogglingAutonomous ? 'opacity-50 cursor-wait' : ''}`}
          title={props.autonomousMode ? 'Autonomous Mode: ON (commands run without approval)' : 'Autonomous Mode: OFF (commands need approval)'}
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </button>

        {/* Session management */}
        <div class="relative">
          <button
            onClick={() => {
              setShowSessionPicker(!showSessionPicker());
              if (!showSessionPicker()) {
                aiChatStore.refreshSessions();
              }
            }}
            class="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            title="Chat sessions"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </button>

          <Show when={showSessionPicker()}>
            <div class="absolute right-0 top-full mt-1 w-72 max-h-96 bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 z-50 overflow-hidden">
              {/* New conversation */}
              <button
                onClick={() => {
                  aiChatStore.newConversation();
                  setShowSessionPicker(false);
                }}
                class="w-full px-3 py-2.5 text-left text-sm flex items-center gap-2 text-purple-600 dark:text-purple-400 hover:bg-purple-50 dark:hover:bg-purple-900/20 border-b border-gray-200 dark:border-gray-700"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
                <span class="font-medium">New conversation</span>
              </button>

              {/* Session list */}
              <div class="max-h-64 overflow-y-auto">
                <Show when={aiChatStore.sessions.length > 0} fallback={
                  <div class="px-3 py-6 text-center text-xs text-gray-500 dark:text-gray-400">
                    No previous conversations
                  </div>
                }>
                  <For each={aiChatStore.sessions}>
                    {(session) => {
                      const isCurrentSession = () => session.id === aiChatStore.sessionId;
                      const sessionDate = new Date(session.updated_at);
                      const isToday = sessionDate.toDateString() === new Date().toDateString();
                      const dateStr = isToday
                        ? sessionDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
                        : sessionDate.toLocaleDateString([], { month: 'short', day: 'numeric' });

                      return (
                        <div
                          class={`group relative px-3 py-2.5 flex items-start gap-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 cursor-pointer ${isCurrentSession() ? 'bg-purple-50 dark:bg-purple-900/20' : ''}`}
                          onClick={() => {
                            if (!isCurrentSession()) {
                              aiChatStore.switchSession(session.id);
                            }
                            setShowSessionPicker(false);
                          }}
                        >
                          <div class="flex-1 min-w-0">
                            <div class="flex items-center gap-2">
                              <span class={`text-sm font-medium truncate ${isCurrentSession() ? 'text-purple-700 dark:text-purple-300' : 'text-gray-900 dark:text-gray-100'}`}>
                                {session.title || 'Untitled conversation'}
                              </span>
                              <Show when={isCurrentSession()}>
                                <span class="flex-shrink-0 w-1.5 h-1.5 rounded-full bg-purple-500" />
                              </Show>
                            </div>
                            <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                              <span>{session.message_count} messages</span>
                              <span>·</span>
                              <span>{dateStr}</span>
                            </div>
                          </div>
                          {/* Delete button */}
                          <button
                            type="button"
                            class="flex-shrink-0 p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-red-100 dark:hover:bg-red-900/30 text-gray-400 hover:text-red-500 transition-opacity"
                            onClick={(e) => {
                              e.stopPropagation();
                              if (confirm('Delete this conversation?')) {
                                aiChatStore.deleteSession(session.id);
                              }
                            }}
                            title="Delete conversation"
                          >
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                            </svg>
                          </button>
                        </div>
                      );
                    }}
                  </For>
                </Show>
              </div>

              {/* Sync status */}
              <div class="px-3 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
                <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                  <span class="flex items-center gap-1.5">
                    <Show when={aiChatStore.syncing} fallback={
                      <svg class="w-3 h-3 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                      </svg>
                    }>
                      <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                      </svg>
                    </Show>
                    {aiChatStore.syncing ? 'Syncing...' : 'Synced'}
                  </span>
                  <button
                    type="button"
                    onClick={() => setShowSessionPicker(false)}
                    class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                  >
                    Close
                  </button>
                </div>
              </div>
            </div>
          </Show>
        </div>

        {/* Close button */}
        <button
          onClick={props.onClose}
          class="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          title="Collapse panel"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 5l7 7-7 7M6 5l7 7-7 7" />
          </svg>
        </button>
      </div>
    </div>
  );
};
