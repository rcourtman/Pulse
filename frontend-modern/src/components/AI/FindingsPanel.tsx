/**
 * FindingsPanel
 *
 * Separated view showing:
 * - Pulse Patrol Findings (AI-discovered insights)
 * - Threshold Alerts (user-configured rules)
 * Each section has severity-based sorting and quick actions
 *
 * Investigation and approval details are shown inline via
 * InvestigationSection and ApprovalSection components.
 */

import { Component, createSignal, createEffect, Show, For, createMemo } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { aiIntelligenceStore, type UnifiedFinding } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { InvestigationSection, ApprovalSection } from '@/components/patrol';
import { investigationStatusLabels, investigationOutcomeLabels, type InvestigationStatus } from '@/api/patrol';
import { AIAPI, type RemediationPlan } from '@/api/ai';

// Severity priority for sorting (lower number = higher priority)
const severityOrder: Record<string, number> = {
  critical: 0,
  warning: 1,
  watch: 2,
  info: 3,
};

// Source display names
const sourceLabels: Record<string, string> = {
  'threshold': 'Alert',
  'ai-patrol': 'Pulse Patrol',
  'anomaly': 'Anomaly',
  'ai-chat': 'Pulse Assistant',
  'correlation': 'Correlation',
  'forecast': 'Forecast',
};

// Severity badge colors
const severityColors: Record<string, string> = {
  critical: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  watch: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
};

// Source badge colors
const sourceColors: Record<string, string> = {
  'threshold': 'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300',
  'ai-patrol': 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  'anomaly': 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  'ai-chat': 'bg-teal-100 text-teal-800 dark:bg-teal-900/40 dark:text-teal-300',
  'correlation': 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-300',
  'forecast': 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300',
};

// Investigation status badge colors
const investigationStatusColors: Record<InvestigationStatus, string> = {
  pending: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400',
  running: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  completed: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
  failed: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
  needs_attention: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
};

interface FindingsPanelProps {
  resourceId?: string;
  showResolved?: boolean;
  maxItems?: number;
  onFindingClick?: (finding: UnifiedFinding) => void;
  filterOverride?: 'all' | 'active' | 'resolved' | 'approvals';
  filterFindingIds?: string[];
  showControls?: boolean;
  nextPatrolAt?: string;
  lastPatrolAt?: string;
  patrolIntervalMs?: number;
  scopeResourceIds?: string[];
  scopeResourceTypes?: string[];
  showScopeWarnings?: boolean;
}

export const FindingsPanel: Component<FindingsPanelProps> = (props) => {
  const location = useLocation();
  const [filter, setFilter] = createSignal<'all' | 'active' | 'resolved' | 'approvals'>(props.filterOverride ?? 'active');
  const [sortBy, setSortBy] = createSignal<'severity' | 'time'>('severity');
  const [expandedId, setExpandedId] = createSignal<string | null>(null);
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const [editingNoteId, setEditingNoteId] = createSignal<string | null>(null);
  const [noteText, setNoteText] = createSignal('');

  // Legacy remediation plans (generic plans without real execution)
  const [remediationPlans, setRemediationPlans] = createSignal<RemediationPlan[]>([]);
  const [planActionLoading, setPlanActionLoading] = createSignal<string | null>(null);

  // Remediation plan actions (legacy)
  const handleApprovePlan = async (plan: RemediationPlan, e: Event) => {
    e.stopPropagation();
    setPlanActionLoading(plan.id);
    try {
      const approval = await AIAPI.approveRemediationPlan(plan.id);
      if (!approval.execution?.id) {
        notificationStore.error('Approval succeeded but no execution ID returned');
        return;
      }
      const result = await AIAPI.executeRemediationPlan(approval.execution.id);

      if (result.status === 'success') {
        notificationStore.success('Remediation executed successfully');
      } else if (result.status === 'partial') {
        notificationStore.success(`Remediation partially completed: ${result.steps_completed} steps`);
      } else {
        notificationStore.error(result.error || 'Remediation execution failed');
      }
      const response = await AIAPI.getRemediationPlans();
      setRemediationPlans(response.plans);
      aiIntelligenceStore.loadFindings();
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute remediation');
    } finally {
      setPlanActionLoading(null);
    }
  };

  const handleDismissPlan = async (plan: RemediationPlan, e: Event) => {
    e.stopPropagation();
    setRemediationPlans(prev => prev.filter(p => p.id !== plan.id));
    notificationStore.success('Remediation plan dismissed');
  };

  // Map of finding_id -> remediation plan (legacy plans)
  const plansByFindingId = createMemo(() => {
    const map = new Map<string, RemediationPlan>();
    for (const plan of remediationPlans()) {
      map.set(plan.finding_id, plan);
    }
    return map;
  });

  // Load findings and remediation plans on mount
  createEffect(() => {
    aiIntelligenceStore.loadFindings();
    // Fetch legacy remediation plans
    AIAPI.getRemediationPlans()
      .then((response: { plans: RemediationPlan[] }) => setRemediationPlans(response.plans))
      .catch(() => {}); // Silently ignore errors
  });

  createEffect(() => {
    if (props.filterOverride) {
      setFilter(props.filterOverride);
    }
  });

  // Filter and sort findings
  const filteredFindings = createMemo(() => {
    let findings = [...aiIntelligenceStore.findings];

    // Filter by resource if specified
    if (props.resourceId) {
      findings = findings.filter(f => f.resourceId === props.resourceId);
    }

    // Filter by status
    if (filter() === 'active') {
      findings = findings.filter(f => f.status === 'active');
    } else if (filter() === 'resolved') {
      findings = findings.filter(f => f.status === 'resolved' || f.status === 'dismissed');
    } else if (filter() === 'approvals') {
      const approvalFindingIds = new Set(aiIntelligenceStore.findingsWithPendingApprovals.map(f => f.id));
      findings = findings.filter(f => approvalFindingIds.has(f.id));
    }

    // Filter by specific finding IDs if provided
    if (props.filterFindingIds && props.filterFindingIds.length > 0) {
      const idSet = new Set(props.filterFindingIds);
      findings = findings.filter(f => idSet.has(f.id));
    }

    // Sort
    findings.sort((a, b) => {
      if (sortBy() === 'severity') {
        const aPriority = severityOrder[a.severity] ?? 4;
        const bPriority = severityOrder[b.severity] ?? 4;
        if (aPriority !== bPriority) return aPriority - bPriority;
      }
      return new Date(b.detectedAt).getTime() - new Date(a.detectedAt).getTime();
    });

    // Limit items
    if (props.maxItems && props.maxItems > 0) {
      findings = findings.slice(0, props.maxItems);
    }

    return findings;
  });

  // Filter to only show Patrol findings (exclude threshold alerts)
  const patrolFindings = createMemo(() =>
    filteredFindings().filter(f => f.source !== 'threshold' && !f.isThreshold && !f.alertId)
  );

  const isOutOfScope = (finding: UnifiedFinding): boolean => {
    if (!props.showScopeWarnings) {
      return false;
    }
    const scopeIds = props.scopeResourceIds ?? [];
    const scopeTypes = props.scopeResourceTypes ?? [];
    if (scopeIds.length === 0 && scopeTypes.length === 0) {
      return false;
    }
    const idMatch = scopeIds.length > 0 ? scopeIds.includes(finding.resourceId) : false;
    const typeMatch = scopeTypes.length > 0 ? scopeTypes.includes(finding.resourceType) : false;
    return !(idMatch || typeMatch);
  };

  const scrollToFindingHash = () => {
    const hash = location.hash;
    if (!hash || !hash.startsWith('#finding-')) {
      setLastHashScrolled(null);
      return;
    }
    if (hash === lastHashScrolled()) {
      return;
    }
    const targetId = hash.slice(1);
    const target = document.getElementById(targetId);
    if (!target) {
      return;
    }
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setExpandedId(targetId.replace('finding-', ''));
    setLastHashScrolled(hash);
  };

  createEffect(() => {
    location.hash;
    aiIntelligenceStore.findingsSignal();
    requestAnimationFrame(scrollToFindingHash);
  });

  const handleAcknowledge = async (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    setActionLoading(finding.id);
    const ok = await aiIntelligenceStore.acknowledgeFinding(finding.id);
    setActionLoading(null);
    if (ok) {
      notificationStore.success('Finding acknowledged');
    } else {
      notificationStore.error('Failed to acknowledge finding');
    }
  };

  const handleDismiss = async (finding: UnifiedFinding, reason: 'not_an_issue' | 'expected_behavior' | 'will_fix_later', e: Event) => {
    e.stopPropagation();
    const note = window.prompt('Add an optional note (for learning context):', '') ?? '';
    setActionLoading(finding.id);
    const ok = await aiIntelligenceStore.dismissFinding(finding.id, reason, note);
    setActionLoading(null);
    if (ok) {
      notificationStore.success('Finding dismissed');
    } else {
      notificationStore.error('Failed to dismiss finding');
    }
  };

  const handleSnooze = async (finding: UnifiedFinding, durationHours: number, e: Event) => {
    e.stopPropagation();
    setActionLoading(finding.id);
    const ok = await aiIntelligenceStore.snoozeFinding(finding.id, durationHours);
    setActionLoading(null);
    if (ok) {
      notificationStore.success(`Finding snoozed for ${durationHours}h`);
    } else {
      notificationStore.error('Failed to snooze finding');
    }
  };

  const handleStartEditNote = (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    setEditingNoteId(finding.id);
    setNoteText(finding.userNote || '');
  };

  const handleSaveNote = async (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    setActionLoading(finding.id);
    const ok = await aiIntelligenceStore.setFindingNote(finding.id, noteText());
    setActionLoading(null);
    if (ok) {
      setEditingNoteId(null);
      notificationStore.success('Note saved');
    } else {
      notificationStore.error('Failed to save note');
    }
  };

  const handleCancelNote = (e: Event) => {
    e.stopPropagation();
    setEditingNoteId(null);
  };

  const handleDiscussWithAssistant = (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    aiChatStore.openWithPrompt(
      `I'd like to discuss this Patrol finding: "${finding.title}" on ${finding.resourceName}.\n\n${finding.description}`,
      { targetType: finding.resourceType, targetId: finding.resourceId, findingId: finding.id },
    );
  };

  const formatTime = (isoString: string) => {
    const date = new Date(isoString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
  };

  const formatInterval = (ms: number) => {
    const hours = Math.floor(ms / 3600000);
    const minutes = Math.floor((ms % 3600000) / 60000);
    if (hours >= 24) {
      const days = Math.floor(hours / 24);
      return days === 1 ? '1 day' : `${days} days`;
    }
    if (hours > 0 && minutes > 0) return `${hours}h ${minutes}m`;
    if (hours > 0) return hours === 1 ? '1 hour' : `${hours} hours`;
    return minutes === 1 ? '1 minute' : `${minutes} minutes`;
  };

  // Get meaningful resolution reason based on finding type
  const getResolutionReason = (finding: UnifiedFinding): string => {
    const resolvedTime = finding.resolvedAt ? formatTime(finding.resolvedAt) : '';

    if (finding.isThreshold || finding.source === 'threshold') {
      const alertType = finding.alertType || '';
      switch (alertType) {
        case 'powered-off':
          return `Guest came online ${resolvedTime}`;
        case 'host-offline':
          return `Host came online ${resolvedTime}`;
        case 'cpu':
          return `CPU returned to normal ${resolvedTime}`;
        case 'memory':
          return `Memory returned to normal ${resolvedTime}`;
        case 'disk':
          return `Disk usage returned to normal ${resolvedTime}`;
        case 'network':
          return `Network recovered ${resolvedTime}`;
        default:
          return `Condition cleared ${resolvedTime}`;
      }
    }

    if (finding.source === 'ai-patrol') {
      return `Issue no longer detected ${resolvedTime}`;
    }

    return `Resolved ${resolvedTime}`;
  };

  // Render a single finding item
  const renderFindingItem = (finding: UnifiedFinding, showSourceBadge: boolean = false) => (
    <div
      id={`finding-${finding.id}`}
      class={`p-3 cursor-pointer transition-colors ${
        finding.status === 'active'
          ? 'hover:bg-gray-50 dark:hover:bg-gray-800/50'
          : 'opacity-60 bg-gray-50/50 dark:bg-gray-800/30 hover:opacity-80'
      }`}
      onClick={() => {
        if (expandedId() === finding.id) {
          setExpandedId(null);
        } else {
          setExpandedId(finding.id);
        }
        props.onFindingClick?.(finding);
      }}
    >
      {/* Finding header */}
      <div class="flex items-start justify-between gap-2">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            {/* Status badge for non-active findings */}
            <Show when={finding.status !== 'active'}>
              <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${
                finding.status === 'resolved'
                  ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                  : 'bg-gray-200 text-gray-600 dark:bg-gray-600 dark:text-gray-300'
              }`}>
                {finding.status === 'resolved' ? 'Resolved' : 'Dismissed'}
              </span>
            </Show>
            {/* Source badge - only show when requested */}
            <Show when={showSourceBadge}>
              <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${sourceColors[finding.source] || sourceColors['ai-patrol']}`}>
                {sourceLabels[finding.source] || finding.source}
              </span>
            </Show>
            {/* Severity badge */}
            <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded uppercase ${severityColors[finding.severity]}`}>
              {finding.severity}
            </span>
            {/* Alert-triggered badge */}
            <Show when={finding.alertId}>
              <span
                class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300"
                title={finding.alertType ? `Alert: ${finding.alertType}` : `Alert ID: ${finding.alertId}`}
              >
                Alert-triggered
              </span>
            </Show>
            <Show when={isOutOfScope(finding)}>
              <span
                class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-200 text-amber-900 dark:bg-amber-900/60 dark:text-amber-100"
                title="This finding references a resource outside the selected run scope."
              >
                Out of scope
              </span>
            </Show>
            {/* Investigation status badge */}
            <Show when={finding.investigationStatus}>
              <span
                class={`px-1.5 py-0.5 text-[10px] font-medium rounded flex items-center gap-1 ${investigationStatusColors[finding.investigationStatus!]}`}
                title={`Investigation: ${investigationStatusLabels[finding.investigationStatus!]}`}
              >
                <Show when={finding.investigationStatus === 'running'}>
                  <span class="h-2 w-2 border border-current border-t-transparent rounded-full animate-spin" />
                </Show>
                {investigationStatusLabels[finding.investigationStatus!]}
              </span>
            </Show>
            {/* Investigation outcome badge */}
            <Show when={finding.investigationOutcome && finding.investigationStatus === 'completed'}>
              <span class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">
                {investigationOutcomeLabels[finding.investigationOutcome!]}
              </span>
            </Show>
            {/* Title */}
            <span class={`font-medium text-sm truncate ${
              finding.status === 'active'
                ? 'text-gray-900 dark:text-gray-100'
                : 'text-gray-500 dark:text-gray-400'
            }`}>
              {finding.title}
            </span>
          </div>
          {/* Resource info */}
          <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
            {finding.resourceName} ({finding.resourceType}) - {formatTime(finding.detectedAt)}
            <Show when={finding.status === 'resolved' && finding.resolvedAt}>
              <span class="ml-2 text-green-600 dark:text-green-400">
                {getResolutionReason(finding)}
              </span>
            </Show>
            <Show when={finding.dismissedReason}>
              <span class="ml-2 text-gray-400 dark:text-gray-500">
                ({finding.dismissedReason?.replace(/_/g, ' ')})
              </span>
            </Show>
          </div>
        </div>
        {/* Actions */}
        <div class="flex items-center gap-1 shrink-0">
          <Show when={finding.status === 'active'}>
            <button
              type="button"
              onClick={(e) => handleAcknowledge(finding, e)}
              class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              title="Acknowledge"
              disabled={actionLoading() === finding.id}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4" />
              </svg>
            </button>
            <button
              type="button"
              onClick={(e) => handleSnooze(finding, 24, e)}
              class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              title="Snooze 24h"
              disabled={actionLoading() === finding.id}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              type="button"
              onClick={(e) => handleDismiss(finding, 'will_fix_later', e)}
              class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              title="Dismiss (Will fix later)"
              disabled={actionLoading() === finding.id}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </Show>
          {/* Expand indicator */}
          <svg
            class={`w-4 h-4 text-gray-400 transition-transform ${expandedId() === finding.id ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </div>

      {/* Expanded content */}
      <Show when={expandedId() === finding.id}>
        {renderExpandedContent(finding)}
      </Show>
    </div>
  );

  // Render expanded content for a finding
  const renderExpandedContent = (finding: UnifiedFinding) => (
    <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
      <Show when={finding.alertId}>
        <div class="text-xs text-amber-700 dark:text-amber-300 mb-2">
          Triggered by alert{finding.alertType ? ` (${finding.alertType})` : ''} • ID {finding.alertId}
        </div>
      </Show>
      <p class="text-sm text-gray-600 dark:text-gray-400">
        {finding.description}
      </p>
      <Show when={finding.recommendation}>
        <p class="text-sm text-gray-700 dark:text-gray-300 mt-2">
          <span class="font-medium">Recommendation:</span> {finding.recommendation}
        </p>
      </Show>

      {/* User note display / editor */}
      <Show when={editingNoteId() === finding.id}>
        <div class="mt-3 p-2 rounded border border-blue-200 dark:border-blue-800 bg-blue-50/50 dark:bg-blue-900/20" onClick={(e) => e.stopPropagation()}>
          <textarea
            class="w-full text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 px-2 py-1.5 resize-none focus:outline-none focus:ring-1 focus:ring-blue-500"
            rows={3}
            value={noteText()}
            onInput={(e) => setNoteText(e.currentTarget.value)}
            placeholder="Add context for Patrol (e.g., 'PBS server was decommissioned last week')"
          />
          <div class="flex gap-2 mt-2">
            <button
              type="button"
              onClick={(e) => handleSaveNote(finding, e)}
              class="px-3 py-1 text-xs font-medium rounded bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-50"
              disabled={actionLoading() === finding.id}
            >
              Save
            </button>
            <button
              type="button"
              onClick={handleCancelNote}
              class="px-3 py-1 text-xs font-medium rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
          </div>
        </div>
      </Show>
      <Show when={editingNoteId() !== finding.id && finding.userNote}>
        <div class="mt-3 p-2 rounded border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 flex items-start gap-2">
          <svg class="w-4 h-4 text-gray-400 dark:text-gray-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
          </svg>
          <p class="text-sm text-gray-600 dark:text-gray-400 flex-1">{finding.userNote}</p>
          <button
            type="button"
            onClick={(e) => handleStartEditNote(finding, e)}
            class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 flex-shrink-0"
            title="Edit note"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
            </svg>
          </button>
        </div>
      </Show>

      {/* Add Note / Discuss with Assistant buttons */}
      <div class="mt-3 flex flex-wrap gap-2 text-xs">
        <Show when={editingNoteId() !== finding.id && !finding.userNote}>
          <button
            type="button"
            onClick={(e) => handleStartEditNote(finding, e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center gap-1"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
            </svg>
            Add Note
          </button>
        </Show>
        <button
          type="button"
          onClick={(e) => handleDiscussWithAssistant(finding, e)}
          class="px-2 py-1 rounded border border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300 hover:bg-blue-50 dark:hover:bg-blue-900/30 flex items-center gap-1"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          Discuss with Assistant
        </button>
      </div>

      <Show when={finding.status === 'active'}>
        <div class="mt-3 flex flex-wrap gap-2 text-xs">
          <button
            type="button"
            onClick={(e) => handleAcknowledge(finding, e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            disabled={actionLoading() === finding.id}
          >
            Acknowledge
          </button>
          <button
            type="button"
            onClick={(e) => handleSnooze(finding, 1, e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            disabled={actionLoading() === finding.id}
          >
            Snooze 1h
          </button>
          <button
            type="button"
            onClick={(e) => handleSnooze(finding, 24, e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            disabled={actionLoading() === finding.id}
          >
            Snooze 24h
          </button>
          <button
            type="button"
            onClick={(e) => handleSnooze(finding, 168, e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            disabled={actionLoading() === finding.id}
          >
            Snooze 7d
          </button>
          <button
            type="button"
            onClick={(e) => handleDismiss(finding, 'not_an_issue', e)}
            class="px-2 py-1 rounded border border-red-200 text-red-700 dark:border-red-700 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/30"
            disabled={actionLoading() === finding.id}
          >
            Dismiss: Not an issue
          </button>
          <button
            type="button"
            onClick={(e) => handleDismiss(finding, 'expected_behavior', e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            disabled={actionLoading() === finding.id}
          >
            Dismiss: Expected
          </button>
          <button
            type="button"
            onClick={(e) => handleDismiss(finding, 'will_fix_later', e)}
            class="px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
            disabled={actionLoading() === finding.id}
          >
            Dismiss: Later
          </button>
        </div>
      </Show>
      <Show when={finding.correlatedFindingIds && finding.correlatedFindingIds.length > 0}>
        <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          Related findings: {finding.correlatedFindingIds?.length}
        </div>
      </Show>

      {/* Inline Investigation Section (replaces drawer) */}
      <Show when={finding.investigationSessionId}>
        <InvestigationSection
          findingId={finding.id}
          investigationStatus={finding.investigationStatus}
          investigationOutcome={finding.investigationOutcome}
        />
      </Show>

      {/* Inline Approval Section (replaces manual approval JSX) */}
      <Show when={finding.status === 'active' && (
        finding.investigationOutcome === 'fix_queued' ||
        finding.investigationOutcome === 'fix_executed' ||
        finding.investigationOutcome === 'fix_failed'
      )}>
        <ApprovalSection
          findingId={finding.id}
          investigationOutcome={finding.investigationOutcome}
        />
      </Show>

      {/* Legacy Remediation Plan - only shown if no investigation fix exists */}
      <Show when={finding.status === 'active' && !finding.investigationOutcome && plansByFindingId().get(finding.id)}>
        {(plan) => (
          <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
            <div class="flex items-center gap-2 mb-2">
              <svg class="w-4 h-4 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
              </svg>
              <span class="text-sm font-medium text-gray-900 dark:text-gray-100">Remediation Plan</span>
              <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${
                plan().risk_level === 'high' ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300' :
                plan().risk_level === 'medium' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300' :
                'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
              }`}>
                {plan().risk_level} risk
              </span>
              <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${
                plan().status === 'completed' ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300' :
                plan().status === 'approved' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300' :
                plan().status === 'executing' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300' :
                plan().status === 'failed' ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300' :
                'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
              }`}>
                {plan().status}
              </span>
            </div>
            <div class="space-y-2">
              <For each={plan().steps}>
                {(step) => (
                  <div class="flex items-start gap-2 text-sm">
                    <span class="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded-full bg-gray-100 dark:bg-gray-700 text-xs font-medium text-gray-600 dark:text-gray-400">
                      {step.order}
                    </span>
                    <span class="text-gray-700 dark:text-gray-300">{step.action}</span>
                  </div>
                )}
              </For>
            </div>

            <Show when={plan().status === 'pending'}>
              <div class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
                <button
                  type="button"
                  onClick={(e) => handleApprovePlan(plan(), e)}
                  disabled={planActionLoading() === plan().id}
                  class="flex-1 px-3 py-1.5 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
                >
                  <Show when={planActionLoading() === plan().id}>
                    <span class="h-3 w-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  </Show>
                  <Show when={planActionLoading() !== plan().id}>
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                    </svg>
                  </Show>
                  Approve & Execute
                </button>
                <button
                  type="button"
                  onClick={(e) => handleDismissPlan(plan(), e)}
                  disabled={planActionLoading() === plan().id}
                  class="px-3 py-1.5 bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 disabled:opacity-50 text-gray-600 dark:text-gray-400 text-xs font-medium rounded"
                >
                  Dismiss
                </button>
              </div>
            </Show>

            <Show when={plan().status === 'executing'}>
              <div class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700 text-amber-600 dark:text-amber-400">
                <span class="h-3 w-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
                <span class="text-xs font-medium">Executing remediation...</span>
              </div>
            </Show>

            <Show when={plan().status === 'completed'}>
              <div class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700 text-green-600 dark:text-green-400">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span class="text-xs font-medium">Remediation completed successfully</span>
              </div>
            </Show>

            <Show when={plan().status === 'failed'}>
              <div class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700 text-red-600 dark:text-red-400">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span class="text-xs font-medium">Remediation failed</span>
              </div>
            </Show>
          </div>
        )}
      </Show>
    </div>
  );

  return (
    <div class="space-y-4">
      {/* Controls */}
      <Show when={props.showControls !== false}>
        <div class="flex items-center justify-between">
          <div class="flex text-xs">
            <button
              type="button"
              onClick={() => setFilter('active')}
              class={`px-2 py-1 rounded-l border ${filter() === 'active'
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
            >
              Active
            </button>
            <button
              type="button"
              onClick={() => setFilter('all')}
              class={`px-2 py-1 border-t border-b ${filter() === 'all'
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => setFilter('resolved')}
              class={`px-2 py-1 border ${filter() === 'resolved'
                ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                } ${aiIntelligenceStore.pendingApprovalCount > 0 ? '' : 'rounded-r'}`}
            >
              Resolved
            </button>
            <Show when={aiIntelligenceStore.pendingApprovalCount > 0}>
              <button
                type="button"
                onClick={() => setFilter('approvals')}
                class={`px-2 py-1 rounded-r border ${filter() === 'approvals'
                  ? 'bg-amber-100 dark:bg-amber-900/50 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700'
                  : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                  }`}
              >
                Approvals ({aiIntelligenceStore.pendingApprovalCount})
              </button>
            </Show>
          </div>
          <select
            value={sortBy()}
            onChange={(e) => setSortBy(e.currentTarget.value as 'severity' | 'time')}
            class="text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
          >
            <option value="severity">By Severity</option>
            <option value="time">By Time</option>
          </select>
        </div>
      </Show>

      {/* Loading/Error states */}
      <Show when={aiIntelligenceStore.findingsLoading}>
        <div class="p-4 text-sm text-gray-500 dark:text-gray-400 flex items-center gap-2">
          <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
          Loading findings...
        </div>
      </Show>

      <Show when={aiIntelligenceStore.findingsError && !aiIntelligenceStore.findingsLoading}>
        <div class="p-4 text-sm text-red-600 dark:text-red-400">
          {aiIntelligenceStore.findingsError}
        </div>
      </Show>

      {/* Pulse Patrol Findings Section */}
      <Show when={!aiIntelligenceStore.findingsLoading}>
        <Card padding="none" class="overflow-hidden">
          {/* Header */}
          <div class="bg-gradient-to-r from-purple-50 to-purple-100 dark:from-purple-900/20 dark:to-purple-900/30 px-4 py-3 border-b border-gray-200 dark:border-gray-700">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <svg class="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
                <span class="font-medium text-gray-900 dark:text-gray-100">Pulse Patrol Findings</span>
                <Show when={patrolFindings().length > 0}>
                  <span class="px-2 py-0.5 text-xs font-medium bg-purple-200 dark:bg-purple-700 text-purple-800 dark:text-purple-200 rounded-full">
                    {patrolFindings().length}
                  </span>
                </Show>
              </div>
              <span class="text-xs text-gray-500 dark:text-gray-400">AI-discovered insights</span>
            </div>
          </div>
          {/* Content */}
          <div class="divide-y divide-gray-100 dark:divide-gray-800">
            <Show when={patrolFindings().length === 0}>
              <div class="p-6 text-sm text-gray-500 dark:text-gray-400 text-center">
                <Show when={filter() === 'active'}>
                  <div class="flex flex-col items-center gap-3">
                    <svg class="w-10 h-10 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <div>
                      <p class="font-medium text-gray-700 dark:text-gray-300">No active findings</p>
                      <p class="text-xs mt-1">Your infrastructure looks healthy!</p>
                    </div>
                    <Show when={props.nextPatrolAt || props.lastPatrolAt || props.patrolIntervalMs}>
                      <div class="mt-2 pt-3 border-t border-gray-200 dark:border-gray-700 w-full max-w-xs">
                        <div class="flex items-center justify-center gap-4 text-xs">
                          <Show when={props.lastPatrolAt}>
                            <div class="flex items-center gap-1.5">
                              <svg class="w-3.5 h-3.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                              </svg>
                              <span>Last: {formatTime(props.lastPatrolAt!)}</span>
                            </div>
                          </Show>
                          <Show when={props.nextPatrolAt}>
                            <div class="flex items-center gap-1.5 text-purple-600 dark:text-purple-400">
                              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                              </svg>
                              <span>Next: {formatTime(props.nextPatrolAt!)}</span>
                            </div>
                          </Show>
                          <Show when={!props.nextPatrolAt && !props.lastPatrolAt && props.patrolIntervalMs}>
                            <div class="flex items-center gap-1.5 text-gray-500 dark:text-gray-400">
                              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                              </svg>
                              <span>Runs every {formatInterval(props.patrolIntervalMs!)}</span>
                            </div>
                          </Show>
                        </div>
                      </div>
                    </Show>
                  </div>
                </Show>
                <Show when={filter() !== 'active'}>
                  No Patrol findings to display
                </Show>
              </div>
            </Show>
            <For each={patrolFindings()}>
              {(finding) => renderFindingItem(finding, false)}
            </For>
          </div>
        </Card>
      </Show>
    </div>
  );
};

export default FindingsPanel;
