import { Component, Show, createSignal, onMount, onCleanup, For, createMemo, createEffect } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { AIAPI } from '@/api/ai';
import { AIChatAPI, type ChatSession } from '@/api/aiChat';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { logger } from '@/utils/logger';
import { useResources } from '@/hooks/useResources';
import type { Resource } from '@/types/resource';
import { useChat } from './hooks/useChat';
import { ChatMessages } from './ChatMessages';
import { ModelSelector } from './ModelSelector';
import { MentionAutocomplete, type MentionResource } from './MentionAutocomplete';
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
  const [sessionDropdownPosition, setSessionDropdownPosition] = createSignal({ top: 0, right: 0 });
  let sessionButtonRef: HTMLButtonElement | undefined;
  const [models, setModels] = createSignal<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);
  const [modelsError, setModelsError] = createSignal('');
  const [defaultModel, setDefaultModel] = createSignal('');
  const [chatOverrideModel, setChatOverrideModel] = createSignal('');
  const [controlLevel, setControlLevel] = createSignal<'read_only' | 'controlled' | 'autonomous'>('read_only');
  const [showControlMenu, setShowControlMenu] = createSignal(false);
  const [controlSaving, setControlSaving] = createSignal(false);
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal<boolean | null>(null); // null = loading
  const [discoveryHintDismissed, setDiscoveryHintDismissed] = createSignal(false);
  const [autonomousBannerDismissed, setAutonomousBannerDismissed] = createSignal(false);
  const { byType } = useResources();
  const isCluster = createMemo(() => byType('node').length > 1);

  // @ mention autocomplete state
  const [mentionActive, setMentionActive] = createSignal(false);
  const [mentionQuery, setMentionQuery] = createSignal('');
  const [mentionStartIndex, setMentionStartIndex] = createSignal(0);
  const [mentionResources, setMentionResources] = createSignal<MentionResource[]>([]);
  const [accumulatedMentions, setAccumulatedMentions] = createSignal<MentionResource[]>([]);
  let textareaRef: HTMLTextAreaElement | undefined;

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

  const normalizeControlLevel = (value?: string): 'read_only' | 'controlled' | 'autonomous' => {
    if (value === 'controlled' || value === 'autonomous' || value === 'read_only') {
      return value;
    }
    if (value === 'suggest') {
      return 'controlled';
    }
    return 'read_only';
  };

  const labelForControlLevel = (level: 'read_only' | 'controlled' | 'autonomous') => {
    switch (level) {
      case 'autonomous':
        return 'Autonomous';
      case 'controlled':
        return 'Approval';
      case 'read_only':
      default:
        return 'Read-only';
    }
  };

  const controlLabel = createMemo(() => labelForControlLevel(controlLevel()));

  const controlTone = createMemo(() => {
    switch (controlLevel()) {
      case 'autonomous':
        return 'border-red-200 text-red-700 bg-red-50 dark:border-red-800 dark:text-red-200 dark:bg-red-900/20';
      case 'controlled':
        return 'border-amber-200 text-amber-700 bg-amber-50 dark:border-amber-800 dark:text-amber-200 dark:bg-amber-900/20';
      default:
        return 'border-slate-200 text-slate-600 bg-white dark:border-slate-700 dark:text-slate-200 dark:bg-slate-800';
    }
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
      const resolvedControl = normalizeControlLevel(settings.control_level || (settings.autonomous_mode ? 'autonomous' : undefined));
      setDefaultModel(fallback);
      setChatOverrideModel(chatOverride);
      setControlLevel(resolvedControl);
      setDiscoveryEnabled(settings.discovery_enabled ?? false);
    } catch (error) {
      logger.error('[AIChat] Failed to load AI settings:', error);
    }
  };

  const updateControlLevel = async (nextLevel: 'read_only' | 'controlled' | 'autonomous') => {
    if (controlSaving() || nextLevel === controlLevel()) {
      setShowControlMenu(false);
      return;
    }
    setControlSaving(true);
    const previous = controlLevel();
    try {
      const updated = await AIAPI.updateSettings({ control_level: nextLevel });
      const resolved = normalizeControlLevel(updated.control_level || nextLevel);
      setControlLevel(resolved);
      if (resolved === 'autonomous') setAutonomousBannerDismissed(false);
      aiChatStore.notifySettingsChanged();
      notificationStore.success(`Control mode set to ${labelForControlLevel(resolved)}`, 2000);
    } catch (error) {
      logger.error('[AIChat] Failed to update control level:', error);
      setControlLevel(previous);
      const message = error instanceof Error ? error.message : 'Failed to update control mode.';
      notificationStore.error(message);
    } finally {
      setControlSaving(false);
      setShowControlMenu(false);
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

  // Refresh models when AI settings change (e.g., new API key added)
  createEffect(() => {
    const version = aiChatStore.settingsVersionSignal();
    // Skip the initial run (version 0)
    if (version > 0) {
      loadModels();
      loadSettings();
    }
  });

  // Pre-fill input when opened with an initialPrompt (e.g., "Discuss with Assistant" from findings)
  createEffect(() => {
    const ctx = aiChatStore.context;
    if (ctx.initialPrompt && isOpen()) {
      setInput(ctx.initialPrompt);
      // Clear so it doesn't re-fire on subsequent opens
      aiChatStore.clearInitialPrompt?.();
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
        // AI not running - silently return, don't show warning on every page load
        // Users who intentionally disabled AI don't need a notification about it
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
        setShowControlMenu(false);
      }
      // Close mention autocomplete when clicking outside
      if (!target.closest('[data-mention-autocomplete]') && !target.closest('textarea')) {
        setMentionActive(false);
      }
    };
    document.addEventListener('click', handleClickOutside);
    onCleanup(() => document.removeEventListener('click', handleClickOutside));
  });

  const normalizeMentionKeyPart = (value?: string | null) => (value || '').trim().toLowerCase();

  const mentionStatusRank = (status?: string) => {
    switch (normalizeMentionKeyPart(status || '')) {
      case 'running':
      case 'online':
        return 3;
      case 'stopped':
      case 'offline':
      case 'exited':
        return 2;
      case 'unknown':
        return 1;
      default:
        return 0;
    }
  };

  const dedupeMentionResources = (resources: MentionResource[]) => {
    // Only dedupe host mentions: VMs/containers/docker containers can legitimately share names across nodes/hosts.
    const byKey = new Map<string, { resource: MentionResource; index: number }>();
    const out: MentionResource[] = [];

    for (const resource of resources) {
      if (resource.type !== 'host') {
        out.push(resource);
        continue;
      }

      const key = `${normalizeMentionKeyPart(resource.name)}:${resource.type}`;
      const existing = byKey.get(key);
      if (!existing) {
        const index = out.length;
        byKey.set(key, { resource, index });
        out.push(resource);
        continue;
      }

      if (mentionStatusRank(resource.status) > mentionStatusRank(existing.resource.status)) {
        existing.resource = resource;
        out[existing.index] = resource;
      }
    }

    return out;
  };

  // Build resources for @ mention autocomplete from unified selectors
  createEffect(() => {
    const readPlatformData = (resource: Resource): Record<string, unknown> | undefined => {
      return resource.platformData ? (unwrap(resource.platformData) as Record<string, unknown>) : undefined;
    };

    const parseLegacyVmid = (resource: Resource, platformData: Record<string, unknown> | undefined): number => {
      const vmidRaw = platformData?.vmid;
      if (typeof vmidRaw === 'number' && Number.isFinite(vmidRaw)) return vmidRaw;
      if (typeof vmidRaw === 'string') {
        const parsed = parseInt(vmidRaw, 10);
        if (Number.isFinite(parsed)) return parsed;
      }
      const idTail = resource.id.split('-').pop() ?? '0';
      const parsed = parseInt(idTail, 10);
      return Number.isFinite(parsed) ? parsed : 0;
    };

    const nodes = byType('node');
    const vms = byType('vm');
    const containers = [...byType('container'), ...byType('oci-container')];
    const dockerHosts = byType('docker-host');
    const dockerContainers = byType('docker-container');
    const hosts = byType('host');
    const resources: MentionResource[] = [];

    // Add VMs
    for (const vm of vms) {
      const platformData = readPlatformData(vm);
      const nodeRaw = platformData?.node;
      const node = typeof nodeRaw === 'string' ? nodeRaw : '';
      const vmid = parseLegacyVmid(vm, platformData);
      resources.push({
        id: `vm:${node}:${vmid}`,
        name: vm.name,
        type: 'vm',
        status: vm.status === 'running' ? 'running' : 'stopped',
        node,
      });
    }

    // Add LXC containers (includes OCI containers; preserve legacy mention ID format)
    for (const container of containers) {
      const platformData = readPlatformData(container);
      const nodeRaw = platformData?.node;
      const node = typeof nodeRaw === 'string' ? nodeRaw : '';
      const vmid = parseLegacyVmid(container, platformData);
      resources.push({
        id: `lxc:${node}:${vmid}`,
        name: container.name,
        type: 'container',
        status: container.status === 'running' ? 'running' : 'stopped',
        node,
      });
    }

    // Add Docker hosts
    const dockerContainersByHostId = new Map<string, Resource[]>();
    for (const dockerContainer of dockerContainers) {
      if (!dockerContainer.parentId) continue;
      const existing = dockerContainersByHostId.get(dockerContainer.parentId);
      if (existing) {
        existing.push(dockerContainer);
      } else {
        dockerContainersByHostId.set(dockerContainer.parentId, [dockerContainer]);
      }
    }

    for (const host of dockerHosts) {
      const displayName = host.displayName || host.identity?.hostname || host.name || host.id;
      const hostnameOrId = host.identity?.hostname || host.name || host.id;
      const hostStatus = host.status === 'online' || host.status === 'running' ? 'online' : (host.status || 'online');
      resources.push({
        id: `host:${host.id}`,
        name: displayName,
        type: 'host',
        status: hostStatus,
      });

      // Add Docker containers
      for (const container of dockerContainersByHostId.get(host.id) || []) {
        const originalContainerId = container.id.includes('/')
          ? (container.id.split('/').pop() || container.id)
          : container.id;
        resources.push({
          id: `docker:${host.id}:${originalContainerId}`,
          name: container.name,
          type: 'docker',
          status: container.status === 'running' ? 'running' : 'exited',
          node: hostnameOrId,
        });
      }
    }

    // Add nodes
    for (const node of nodes) {
      resources.push({
        id: `node:${node.platformId || ''}:${node.name}`,
        name: node.name,
        type: 'node',
        status: node.status,
      });
    }

    // Add standalone host agents
    for (const host of hosts) {
      const name = host.displayName || host.identity?.hostname || host.name;
      const hostStatus = host.status === 'online' || host.status === 'running' ? 'online' : host.status;
      resources.push({
        id: `host:${host.id}`,
        name,
        type: 'host',
        status: hostStatus,
      });
    }

    setMentionResources(dedupeMentionResources(resources));
  });

  // Handle submit
  const handleSubmit = () => {
    if (chat.isLoading()) return;
    const prompt = input().trim();
    if (!prompt) return;
    const mentions = accumulatedMentions();
    // Pass findingId from context on the first message, clear after success
    const ctx = aiChatStore.context;
    const findingId = ctx.findingId;
    chat.sendMessage(prompt, mentions.length > 0 ? mentions : undefined, findingId)
      .then((ok) => {
        if (ok && findingId) {
          aiChatStore.clearFindingId?.();
        }
      });
    setInput('');
    setAccumulatedMentions([]);
    setMentionActive(false);
  };

  // Handle input change with @ mention detection
  const handleInputChange = (e: InputEvent & { currentTarget: HTMLTextAreaElement }) => {
    const value = e.currentTarget.value;
    setInput(value);

    const cursorPos = e.currentTarget.selectionStart || 0;
    const textBeforeCursor = value.slice(0, cursorPos);

    // Find the last @ before cursor
    const lastAtIndex = textBeforeCursor.lastIndexOf('@');

    if (lastAtIndex !== -1) {
      // Check if @ is at start or preceded by whitespace
      const charBefore = lastAtIndex > 0 ? textBeforeCursor[lastAtIndex - 1] : ' ';
      if (charBefore === ' ' || charBefore === '\n' || lastAtIndex === 0) {
        const query = textBeforeCursor.slice(lastAtIndex + 1);
        // Only activate if query doesn't contain spaces (still typing the mention)
        if (!query.includes(' ')) {
          setMentionActive(true);
          setMentionQuery(query);
          setMentionStartIndex(lastAtIndex);
          return;
        }
      }
    }

    setMentionActive(false);
  };

  // Handle mention selection
  const handleMentionSelect = (resource: MentionResource) => {
    const currentInput = input();
    const startIndex = mentionStartIndex();
    const cursorPos = textareaRef?.selectionStart || currentInput.length;

    // Replace @query with the resource name
    const before = currentInput.slice(0, startIndex);
    const after = currentInput.slice(cursorPos);
    const newValue = `${before}@${resource.name} ${after}`;

    setInput(newValue);
    setMentionActive(false);

    // Accumulate the structured mention data so we can send it with the prompt
    setAccumulatedMentions(prev => {
      // Deduplicate by id
      if (prev.some(m => m.id === resource.id)) return prev;
      return [...prev, resource];
    });

    // Focus textarea and set cursor position after the inserted name
    setTimeout(() => {
      if (textareaRef) {
        textareaRef.focus();
        const newCursorPos = startIndex + resource.name.length + 2; // +2 for @ and space
        textareaRef.setSelectionRange(newCursorPos, newCursorPos);
      }
    }, 0);
  };

  // Handle key down - submit when not loading, but let autocomplete handle keys when active
  const handleKeyDown = (e: KeyboardEvent) => {
    // Let mention autocomplete handle navigation keys
    if (mentionActive()) {
      if (['ArrowDown', 'ArrowUp', 'Enter', 'Tab', 'Escape'].includes(e.key)) {
        // These are handled by MentionAutocomplete component
        return;
      }
    }

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
      class={`relative flex-shrink-0 h-full bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-700 flex flex-col transition-all duration-300 ${isOpen() ? 'w-full sm:w-[480px] overflow-visible' : 'w-0 border-l-0 overflow-hidden'
        }`}
    >
      <Show when={isOpen()}>
        {/* Floating Close Handle (Desktop only) */}
        <button
          onClick={props.onClose}
          class="hidden sm:flex absolute left-0 top-1/2 -translate-x-full -translate-y-1/2 items-center justify-center w-8 py-3 rounded-l-xl bg-white dark:bg-slate-800 text-blue-600 dark:text-blue-400 shadow-sm border border-r-0 border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors z-50 cursor-pointer"
          title="Collapse Pulse Assistant"
        >
          <svg class="h-5 w-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z"
            />
          </svg>
        </button>
        {/* Header - wraps on mobile */}
        <div class="flex flex-wrap items-center justify-between gap-2 px-4 py-3 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800">
          <div class="flex items-center gap-3">
            <div class="p-2 border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 rounded-md shadow-sm">
              <svg class="w-5 h-5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z" />
              </svg>
            </div>
            <div>
              <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Pulse Assistant</h2>
              <p class="text-[11px] text-slate-500 dark:text-slate-400">
                Infrastructure intelligence
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1.5">
            {/* New chat */}
            <button
              onClick={handleNewConversation}
              class="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] text-slate-600 dark:text-slate-300 hover:text-slate-800 dark:hover:text-slate-100 rounded-md border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600 bg-white dark:bg-slate-800 transition-colors"
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

            {/* Control mode toggle */}
            <div class="relative" data-dropdown>
              <button
                onClick={() => setShowControlMenu(!showControlMenu())}
                class={`flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium rounded-md border transition-colors ${controlTone()} ${controlSaving() ? 'opacity-70 cursor-wait' : 'hover:opacity-90'}`}
                title="Control mode"
                disabled={controlSaving()}
              >
                <span class={`h-1.5 w-1.5 rounded-full ${controlLevel() === 'autonomous' ? 'bg-red-500' : controlLevel() === 'controlled' ? 'bg-amber-500' : 'bg-slate-400'}`} />
                <span>{controlLabel()}</span>
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              <Show when={showControlMenu()}>
                <div class="absolute right-0 mt-2 w-60 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 shadow-sm z-50 overflow-hidden">
                  <div class="px-3 py-2 text-[11px] text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
                    Control mode for this chat
                  </div>
                  <button
                    class={`w-full text-left px-3 py-2.5 text-xs hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors ${controlLevel() === 'read_only' ? 'bg-slate-50 dark:bg-slate-800' : ''}`}
                    onClick={() => updateControlLevel('read_only')}
                  >
                    <div class="font-medium text-slate-800 dark:text-slate-200">Read-only</div>
                    <div class="text-[11px] text-slate-500 dark:text-slate-400">No commands or control actions</div>
                  </button>
                  <button
                    class={`w-full text-left px-3 py-2.5 text-xs hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors ${controlLevel() === 'controlled' ? 'bg-amber-50 dark:bg-amber-900/20' : ''}`}
                    onClick={() => updateControlLevel('controlled')}
                  >
                    <div class="font-medium text-slate-800 dark:text-slate-200">Approval</div>
                    <div class="text-[11px] text-slate-500 dark:text-slate-400">Ask before running commands</div>
                  </button>
                  <button
                    class={`w-full text-left px-3 py-2.5 text-xs hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors ${controlLevel() === 'autonomous' ? 'bg-red-50 dark:bg-red-900/20' : ''}`}
                    onClick={() => updateControlLevel('autonomous')}
                  >
                    <div class="font-medium text-slate-800 dark:text-slate-200">Autonomous</div>
                    <div class="text-[11px] text-slate-500 dark:text-slate-400">Executes without approval (Pro)</div>
                  </button>
                </div>
              </Show>
            </div>

            {/* Session picker */}
            <div class="relative" data-dropdown>
              <button
                ref={sessionButtonRef}
                onClick={() => {
                  const next = !showSessions();
                  if (next && sessionButtonRef) {
                    const rect = sessionButtonRef.getBoundingClientRect();
                    setSessionDropdownPosition({
                      top: rect.bottom + 4,
                      right: window.innerWidth - rect.right,
                    });
                  }
                  setShowSessions(next);
                }}
                class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-md hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
                title="Chat sessions"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              </button>

              <Show when={showSessions()}>
                <div
                  class="fixed w-72 max-h-96 bg-white dark:bg-slate-800 rounded-md shadow-sm border border-slate-200 dark:border-slate-700 z-[9999] overflow-hidden"
                  style={{ top: `${sessionDropdownPosition().top}px`, right: `${sessionDropdownPosition().right}px` }}
                >
                  <button
                    onClick={handleNewConversation}
                    class="w-full px-3 py-2.5 text-left text-sm flex items-center gap-2 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 border-b border-slate-200 dark:border-slate-700"
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
                            class={`group relative px-3 py-2.5 flex items-start gap-2 hover:bg-slate-50 dark:hover:bg-slate-700/50 cursor-pointer ${chat.sessionId() === session.id ? 'bg-blue-50 dark:bg-blue-900/20' : ''}`}
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

            {/* Close button (Always visible as fallback) */}
            <button
              onClick={(e) => {
                e.stopPropagation();
                props.onClose();
              }}
              class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-md hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
              title="Close panel"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <Show when={controlLevel() === 'autonomous' && !autonomousBannerDismissed()}>
          <div class="px-4 py-2 border-b border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 flex items-center justify-between gap-3 text-[11px] text-red-700 dark:text-red-200">
            <div class="flex items-center gap-2">
              <span class="inline-flex h-2 w-2 rounded-full bg-red-500" />
              <span class="font-medium">Autonomous mode</span>
              <span class="text-red-600 dark:text-red-300">Commands execute without approval.</span>
            </div>
            <div class="flex items-center gap-2">
              <button
                onClick={() => updateControlLevel('controlled')}
                class="px-2 py-1 rounded-md border border-red-200 dark:border-red-800 bg-white dark:bg-red-900/30 text-[10px] font-medium text-red-700 dark:text-red-200 hover:bg-white dark:hover:bg-red-900/40"
              >
                Switch to Approval
              </button>
              <button
                onClick={() => setAutonomousBannerDismissed(true)}
                class="p-1 rounded-md text-red-400 hover:text-red-600 dark:hover:text-red-200 hover:bg-red-100 dark:hover:bg-red-900/40 transition-colors"
                title="Dismiss"
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>
        </Show>

        {/* Discovery hint - show when discovery is disabled */}
        <Show when={discoveryEnabled() === false && !discoveryHintDismissed()}>
          <div class="px-4 py-2 border-b border-cyan-200 dark:border-cyan-800 bg-cyan-50 dark:bg-cyan-900/20 flex items-center justify-between gap-3 text-[11px] text-cyan-700 dark:text-cyan-200">
            <div class="flex items-center gap-2">
              <svg class="w-4 h-4 text-cyan-500 dark:text-cyan-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span>
                <span class="font-medium">Discovery is off.</span>
                {' '}Enable it in Settings for more accurate answers about your infrastructure.
              </span>
            </div>
            <button
              onClick={() => setDiscoveryHintDismissed(true)}
              class="p-1 rounded hover:bg-cyan-100 dark:hover:bg-cyan-800/50 text-cyan-500 dark:text-cyan-400"
              title="Dismiss"
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </Show>

        {/* Messages */}
        <ChatMessages
          messages={chat.messages()}
          onApprove={handleApprove}
          onSkip={handleSkip}
          onAnswerQuestion={handleAnswerQuestion}
          onSkipQuestion={handleSkipQuestion}
          recentSessions={sessions()
            .filter(s => s.id !== chat.sessionId() && s.message_count > 0)
            .slice(0, 3)}
          onLoadSession={handleLoadSession}
          emptyState={{
            title: 'Pulse Assistant ready',
            suggestions: isCluster() ? [
              'Analyze overall cluster health',
              'Check node load balancing',
              'Find and fix failed services',
            ] : [
              'Analyze system health',
              'Check storage usage',
              'Scan for security vulnerabilities',
            ],
            onSuggestionClick: (s) => setInput(s),
          }}
        />

        {/* Status indicator bar */}
        <Show when={currentStatus()}>
          <div class="px-4 py-2 bg-slate-50 dark:bg-slate-800 border-t border-slate-200 dark:border-slate-700 flex items-center gap-2.5 text-xs">
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
                <svg class="w-3.5 h-3.5 text-blue-500 dark:text-blue-400 animate-spin" fill="none" viewBox="0 0 24 24">
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
          <form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="flex gap-2 relative">
            <div class="flex-1 relative">
              <textarea
                ref={textareaRef}
                value={input()}
                onInput={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder="Ask about your infrastructure..."
                rows={2}
                class="w-full px-4 py-3 text-sm rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
              />
              <div data-mention-autocomplete>
                <MentionAutocomplete
                  query={mentionQuery()}
                  resources={mentionResources()}
                  position={{ top: 60, left: 0 }}
                  onSelect={handleMentionSelect}
                  onClose={() => setMentionActive(false)}
                  visible={mentionActive()}
                />
              </div>
            </div>
            <div class="flex gap-1.5 self-stretch">
              <Show
                when={!chat.isLoading()}
                fallback={
                  <button
                    type="button"
                    onClick={chat.stop}
                    class="px-4 flex items-center justify-center border border-slate-200 dark:border-slate-700 bg-white hover:bg-slate-50 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-md transition-colors shadow-sm"
                    title="Stop"
                  >
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                      <rect x="6" y="6" width="12" height="12" rx="1" />
                    </svg>
                  </button>
                }
              >
                <button
                  type="submit"
                  disabled={!input().trim()}
                  class="px-4 flex items-center justify-center bg-blue-600 hover:bg-blue-700 text-white rounded-md disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-sm"
                  title="Send"
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                  </svg>
                </button>
              </Show>
            </div>
          </form>
          <div class="flex items-center justify-center gap-3 mt-2 text-[10px] text-slate-400 dark:text-slate-500">
            <span><kbd class="font-sans px-1 rounded bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">Enter</kbd> to send</span>
            <span class="text-slate-300 dark:text-slate-600">&middot;</span>
            <span><span class="font-semibold text-slate-500 dark:text-slate-400">@</span> to mention resources</span>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default AIChat;
