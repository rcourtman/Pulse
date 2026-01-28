/**
 * InvestigationDrawer - Slide-out drawer showing investigation details
 *
 * Displays:
 * - Finding summary header
 * - Investigation status badge
 * - Full chat thread (read-only)
 * - Proposed fix with approve/skip buttons (if pending)
 * - Re-investigate button (if failed or needs attention)
 */

import { Component, createSignal, createEffect, Show, For } from 'solid-js';
import { Portal } from 'solid-js/web';
import {
  type Investigation,
  type ChatMessage,
  type FindingSeverity,
  getInvestigation,
  getInvestigationMessages,
  reinvestigateFinding,
  approveFix,
  denyFix,
  investigationStatusLabels,
  investigationOutcomeLabels,
  severityColors,
  formatTimestamp,
} from '@/api/patrol';
import { notificationStore } from '@/stores/notifications';

interface InvestigationDrawerProps {
  open: boolean;
  onClose: () => void;
  findingId?: string;
  findingTitle?: string;
  findingSeverity?: FindingSeverity;
  resourceName?: string;
  resourceType?: string;
  onReinvestigate?: () => void;
  onFixApproved?: () => void;
}

// Status badge colors
const statusColors: Record<string, string> = {
  pending: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
  running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  completed: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  needs_attention: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
};

// Outcome badge colors
const outcomeColors: Record<string, string> = {
  resolved: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  fix_queued: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  fix_executed: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  fix_failed: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  needs_attention: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  cannot_fix: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
};

export const InvestigationDrawer: Component<InvestigationDrawerProps> = (props) => {
  const [investigation, setInvestigation] = createSignal<Investigation | null>(null);
  const [messages, setMessages] = createSignal<ChatMessage[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [reinvestigating, setReinvestigating] = createSignal(false);
  const [approvingFix, setApprovingFix] = createSignal(false);
  const [denyingFix, setDenyingFix] = createSignal(false);
  const toolsAvailable = () => investigation()?.tools_available ?? [];
  const toolsUsed = () => investigation()?.tools_used ?? [];
  const evidenceIDs = () => investigation()?.evidence_ids ?? [];

  const handleCopyEvidence = async () => {
    if (!evidenceIDs().length) return;
    const payload = evidenceIDs().join('\n');
    try {
      await navigator.clipboard.writeText(payload);
      notificationStore.success('Evidence IDs copied');
    } catch (_err) {
      notificationStore.error('Failed to copy evidence IDs');
    }
  };

  // Load investigation data when drawer opens
  createEffect(async () => {
    if (!props.open || !props.findingId) {
      setInvestigation(null);
      setMessages([]);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const [inv, msgs] = await Promise.all([
        getInvestigation(props.findingId),
        getInvestigationMessages(props.findingId),
      ]);
      setInvestigation(inv);
      setMessages(msgs.messages || []);
    } catch (err) {
      setError((err as Error).message || 'Failed to load investigation');
    } finally {
      setLoading(false);
    }
  });

  const handleReinvestigate = async () => {
    if (!props.findingId) return;

    setReinvestigating(true);
    try {
      await reinvestigateFinding(props.findingId);
      notificationStore.success('Re-investigation started');
      // Reload investigation data
      const inv = await getInvestigation(props.findingId);
      setInvestigation(inv);
      // Notify parent to reload findings
      props.onReinvestigate?.();
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to start re-investigation');
    } finally {
      setReinvestigating(false);
    }
  };

  const handleApproveFix = async () => {
    const inv = investigation();
    if (!inv?.approval_id) return;

    setApprovingFix(true);
    try {
      const result = await approveFix(inv.approval_id);
      if (result.success) {
        notificationStore.success(result.message || 'Fix executed successfully');
      } else {
        // Fix was executed but failed (non-zero exit code)
        notificationStore.error(result.message || 'Fix execution failed');
      }
      // Reload investigation data
      if (props.findingId) {
        const updatedInv = await getInvestigation(props.findingId);
        setInvestigation(updatedInv);
      }
      // Notify parent to reload findings
      props.onFixApproved?.();
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute fix');
    } finally {
      setApprovingFix(false);
    }
  };

  const handleDenyFix = async () => {
    const inv = investigation();
    if (!inv?.approval_id) return;

    setDenyingFix(true);
    try {
      await denyFix(inv.approval_id);
      notificationStore.success('Fix skipped');
      // Reload investigation data
      if (props.findingId) {
        const updatedInv = await getInvestigation(props.findingId);
        setInvestigation(updatedInv);
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to skip fix');
    } finally {
      setDenyingFix(false);
    }
  };

  const hasPendingApproval = () => {
    const inv = investigation();
    return inv?.outcome === 'fix_queued' && inv?.approval_id;
  };

  const canReinvestigate = () => {
    const inv = investigation();
    if (!props.findingId) return false;
    if (inv?.status === 'running') return false;
    // Can reinvestigate if failed, needs attention, or completed with cannot_fix outcome
    const status = inv?.status;
    const outcome = inv?.outcome;
    return status === 'failed' || status === 'needs_attention' || outcome === 'cannot_fix';
  };

  return (
    <Portal>
      {/* Backdrop */}
      <Show when={props.open}>
        <div
          class="fixed inset-0 bg-black/50 z-40 transition-opacity"
          onClick={props.onClose}
        />
      </Show>

      {/* Drawer */}
      <div
        class={`fixed inset-y-0 right-0 w-full max-w-xl bg-white dark:bg-gray-900 shadow-xl z-50 transform transition-transform duration-300 ${
          props.open ? 'translate-x-0' : 'translate-x-full'
        }`}
      >
        {/* Header */}
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-blue-50 to-blue-100 dark:from-blue-900/20 dark:to-blue-900/30">
          <div class="flex items-center gap-2">
            <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
            </svg>
            <span class="font-medium text-gray-900 dark:text-gray-100">Investigation</span>
          </div>
          <button
            type="button"
            onClick={props.onClose}
            class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div class="h-full overflow-y-auto pb-20">
          <Show when={loading()}>
            <div class="p-4 flex items-center justify-center">
              <span class="h-5 w-5 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
              <span class="ml-2 text-gray-500">Loading...</span>
            </div>
          </Show>

          <Show when={error()}>
            <div class="p-4 text-red-600 dark:text-red-400">{error()}</div>
          </Show>

          <Show when={!loading() && !error() && props.findingId}>
            {/* Finding Summary */}
            <div class="p-4 border-b border-gray-200 dark:border-gray-700">
              <div class="flex items-start gap-2">
                {/* Severity indicator */}
                <div
                  class="w-1 h-16 rounded-full"
                  style={{
                    'background-color': props.findingSeverity ? (severityColors[props.findingSeverity]?.text || '#9ca3af') : '#9ca3af',
                  }}
                />
                <div class="flex-1">
                  <div class="flex items-center gap-2 flex-wrap">
                    <Show when={props.findingSeverity}>
                      <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded uppercase ${
                        props.findingSeverity === 'critical' ? 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300' :
                        props.findingSeverity === 'warning' ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300' :
                        'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                      }`}>
                        {props.findingSeverity}
                      </span>
                    </Show>
                    <Show when={investigation()?.status}>
                      <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${
                        statusColors[investigation()!.status] || statusColors.pending
                      }`}>
                        {investigationStatusLabels[investigation()!.status] || investigation()!.status}
                      </span>
                    </Show>
                    <Show when={investigation()?.outcome}>
                      <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${
                        outcomeColors[investigation()!.outcome!] || outcomeColors.needs_attention
                      }`}>
                        {investigationOutcomeLabels[investigation()!.outcome!] || investigation()!.outcome}
                      </span>
                    </Show>
                  </div>
                  <h3 class="font-medium text-gray-900 dark:text-gray-100 mt-1">
                    {props.findingTitle || 'Investigation Details'}
                  </h3>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {props.resourceName} ({props.resourceType})
                  </p>
                </div>
              </div>
            </div>

            {/* Investigation Stats */}
            <Show when={investigation()}>
              <div class="p-4 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
                <div class="grid grid-cols-3 gap-4 text-center">
                  <div>
                    <div class="text-2xl font-bold text-gray-900 dark:text-gray-100">
                      {investigation()!.turn_count}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">Turns Used</div>
                  </div>
                  <div>
                    <div class="text-2xl font-bold text-gray-900 dark:text-gray-100">
                      1
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">Attempt</div>
                  </div>
                  <div>
                    <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                      {investigation()!.started_at ? formatTimestamp(investigation()!.started_at) : '-'}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">Started</div>
                  </div>
                </div>
              </div>
            </Show>

            {/* Summary */}
            <Show when={investigation()?.summary}>
              <div class="p-4 border-b border-gray-200 dark:border-gray-700">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Summary</h4>
                <div class="text-sm text-gray-600 dark:text-gray-400 whitespace-pre-wrap bg-gray-50 dark:bg-gray-800/50 rounded p-3">
                  {investigation()!.summary}
                </div>
              </div>
            </Show>

            {/* Tools & Evidence */}
            <Show when={toolsUsed().length || toolsAvailable().length || evidenceIDs().length}>
              <div class="p-4 border-b border-gray-200 dark:border-gray-700">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Tools & Evidence</h4>

                <Show when={toolsUsed().length}>
                  <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">Tools used</div>
                  <div class="flex flex-wrap gap-1 mb-3">
                    <For each={toolsUsed()}>
                      {(tool) => (
                        <span class="px-1.5 py-0.5 rounded bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 text-[10px] font-medium">
                          {tool}
                        </span>
                      )}
                    </For>
                  </div>
                </Show>

                <Show when={toolsAvailable().length}>
                  <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">Tools available</div>
                  <div class="flex flex-wrap gap-1 mb-3 max-h-20 overflow-y-auto">
                    <For each={toolsAvailable()}>
                      {(tool) => (
                        <span class="px-1.5 py-0.5 rounded bg-gray-50 text-gray-600 dark:bg-gray-800/70 dark:text-gray-300 text-[10px]">
                          {tool}
                        </span>
                      )}
                    </For>
                  </div>
                </Show>

                <Show when={evidenceIDs().length}>
                  <div class="flex items-center justify-between mb-1">
                    <div class="text-xs text-gray-500 dark:text-gray-400">Evidence IDs</div>
                    <button
                      type="button"
                      onClick={handleCopyEvidence}
                      class="text-[10px] px-2 py-0.5 rounded border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100 hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
                    >
                      Copy
                    </button>
                  </div>
                  <div class="flex flex-wrap gap-1">
                    <For each={evidenceIDs()}>
                      {(id) => (
                        <span class="px-1.5 py-0.5 rounded bg-gray-900 text-green-300 text-[10px] font-mono">
                          {id}
                        </span>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            </Show>

            {/* Proposed Fix */}
            <Show when={investigation()?.proposed_fix}>
              <div class="p-4 border-b border-gray-200 dark:border-gray-700">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Proposed Fix</h4>
                <div class="bg-gray-50 dark:bg-gray-800/50 rounded p-3">
                  <p class="text-sm text-gray-600 dark:text-gray-400 mb-2">
                    {investigation()!.proposed_fix!.description}
                  </p>
                  <Show when={investigation()!.proposed_fix!.commands?.length}>
                    <div class="font-mono text-xs bg-gray-900 text-green-400 p-2 rounded mt-2 overflow-x-auto">
                      <For each={investigation()!.proposed_fix!.commands}>
                        {(cmd) => <div>$ {cmd}</div>}
                      </For>
                    </div>
                  </Show>
                  <div class="flex items-center gap-2 mt-2 text-xs">
                    <Show when={investigation()!.proposed_fix!.risk_level}>
                      <span class={`px-1.5 py-0.5 rounded ${
                        investigation()!.proposed_fix!.risk_level === 'critical' ? 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300' :
                        investigation()!.proposed_fix!.risk_level === 'high' ? 'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300' :
                        investigation()!.proposed_fix!.risk_level === 'medium' ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300' :
                        'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300'
                      }`}>
                        {investigation()!.proposed_fix!.risk_level} risk
                      </span>
                    </Show>
                    <Show when={investigation()!.proposed_fix!.destructive}>
                      <span class="px-1.5 py-0.5 rounded bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300">
                        Destructive
                      </span>
                    </Show>
                  </div>

                  {/* Approve/Skip buttons when fix is pending approval */}
                  <Show when={hasPendingApproval()}>
                    <div class="flex items-center gap-2 mt-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                      <button
                        type="button"
                        onClick={handleApproveFix}
                        disabled={approvingFix() || denyingFix()}
                        class="flex-1 px-3 py-2 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white text-sm font-medium rounded-lg flex items-center justify-center gap-2"
                      >
                        <Show when={approvingFix()}>
                          <span class="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                        </Show>
                        <Show when={!approvingFix()}>
                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                          </svg>
                        </Show>
                        Approve & Execute
                      </button>
                      <button
                        type="button"
                        onClick={handleDenyFix}
                        disabled={approvingFix() || denyingFix()}
                        class="px-3 py-2 bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300 text-sm font-medium rounded-lg flex items-center justify-center gap-2"
                      >
                        <Show when={denyingFix()}>
                          <span class="h-4 w-4 border-2 border-gray-500 border-t-transparent rounded-full animate-spin" />
                        </Show>
                        Skip
                      </button>
                    </div>
                  </Show>

                  {/* Show if fix was already executed successfully */}
                  <Show when={investigation()?.outcome === 'resolved' || investigation()?.outcome === 'fix_executed'}>
                    <div class="mt-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-2 text-green-600 dark:text-green-400">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span class="text-sm font-medium">Fix executed successfully</span>
                      </div>
                    </div>
                  </Show>

                  {/* Show if fix execution failed */}
                  <Show when={investigation()?.outcome === 'fix_failed'}>
                    <div class="mt-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-2 text-red-600 dark:text-red-400">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span class="text-sm font-medium">Fix execution failed</span>
                      </div>
                    </div>
                  </Show>
                </div>
              </div>
            </Show>

            {/* Chat Messages */}
            <Show when={messages().length > 0}>
              <div class="p-4">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">Investigation Thread</h4>
                <div class="space-y-3">
                  <For each={messages()}>
                    {(msg) => (
                      <div class={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                        <div class={`max-w-[85%] rounded-lg px-3 py-2 ${
                          msg.role === 'user'
                            ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-900 dark:text-blue-100'
                            : msg.role === 'system'
                            ? 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 text-xs'
                            : 'bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-200'
                        }`}>
                          <div class="text-sm whitespace-pre-wrap">{msg.content}</div>
                          <div class="text-[10px] text-gray-500 dark:text-gray-500 mt-1">
                            {formatTimestamp(msg.timestamp)}
                          </div>
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              </div>
            </Show>

            {/* No Investigation */}
            <Show when={!investigation()}>
              <div class="p-4 text-center text-gray-500 dark:text-gray-400">
                <svg class="w-12 h-12 mx-auto mb-2 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <p class="text-sm">No investigation data available.</p>
                <p class="text-xs mt-1">Enable patrol autonomy to automatically investigate findings.</p>
              </div>
            </Show>
          </Show>
        </div>

        {/* Footer Actions */}
        <Show when={props.findingId && canReinvestigate()}>
          <div class="absolute bottom-0 left-0 right-0 p-4 bg-white dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700">
            <button
              type="button"
              onClick={handleReinvestigate}
              disabled={reinvestigating()}
              class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white rounded-lg font-medium flex items-center justify-center gap-2"
            >
              <Show when={reinvestigating()}>
                <span class="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
              </Show>
              <Show when={!reinvestigating()}>
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
              </Show>
              Re-investigate
            </button>
          </div>
        </Show>
      </div>
    </Portal>
  );
};

export default InvestigationDrawer;
