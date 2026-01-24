/**
 * UnifiedFindingsPanel (Phase 7 - Task 7.3.3)
 *
 * Combined alert + AI findings view that shows:
 * - Source indicator (threshold vs AI)
 * - Severity-based sorting
 * - Quick actions (dismiss, snooze, acknowledge)
 * - Correlation links
 */

import { Component, createSignal, createEffect, Show, For, createMemo } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { aiIntelligenceStore, type UnifiedFinding } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { InvestigationDrawer } from './InvestigationDrawer';
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

interface UnifiedFindingsPanelProps {
  resourceId?: string;
  showResolved?: boolean;
  maxItems?: number;
  onFindingClick?: (finding: UnifiedFinding) => void;
  filterOverride?: 'all' | 'active' | 'resolved';
  showControls?: boolean;
}

export const UnifiedFindingsPanel: Component<UnifiedFindingsPanelProps> = (props) => {
  const navigate = useNavigate();
  const [filter, setFilter] = createSignal<'all' | 'active' | 'resolved'>(props.filterOverride ?? 'active');
  const [sortBy, setSortBy] = createSignal<'severity' | 'time'>('severity');
  const [expandedId, setExpandedId] = createSignal<string | null>(null);
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);

  // Investigation drawer state
  const [investigationDrawerOpen, setInvestigationDrawerOpen] = createSignal(false);
  const [selectedFindingForInvestigation, setSelectedFindingForInvestigation] = createSignal<UnifiedFinding | null>(null);

  // Remediation plans state
  const [remediationPlans, setRemediationPlans] = createSignal<RemediationPlan[]>([]);

  const openInvestigationDrawer = (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    setSelectedFindingForInvestigation(finding);
    setInvestigationDrawerOpen(true);
  };

  // Map of finding_id -> remediation plan
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
    }

    // Sort
    findings.sort((a, b) => {
      if (sortBy() === 'severity') {
        // Sort by urgency: critical (0) first, then warning (1), watch (2), info (3)
        const aPriority = severityOrder[a.severity] ?? 4;
        const bPriority = severityOrder[b.severity] ?? 4;
        if (aPriority !== bPriority) return aPriority - bPriority;
      }
      // Secondary sort by time (most recent first)
      return new Date(b.detectedAt).getTime() - new Date(a.detectedAt).getTime();
    });

    // Limit items
    if (props.maxItems && props.maxItems > 0) {
      findings = findings.slice(0, props.maxItems);
    }

    return findings;
  });

  const isThresholdFinding = (finding: UnifiedFinding) =>
    finding.source === 'threshold' || Boolean(finding.isThreshold || finding.alertId);

  const handleAcknowledge = async (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    if (isThresholdFinding(finding)) {
      navigate('/alerts/overview');
      return;
    }
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
    if (isThresholdFinding(finding)) {
      navigate('/alerts/overview');
      return;
    }
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
    if (isThresholdFinding(finding)) {
      navigate('/alerts/overview');
      return;
    }
    setActionLoading(finding.id);
    const ok = await aiIntelligenceStore.snoozeFinding(finding.id, durationHours);
    setActionLoading(null);
    if (ok) {
      notificationStore.success(`Finding snoozed for ${durationHours}h`);
    } else {
      notificationStore.error('Failed to snooze finding');
    }
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

  // Get meaningful resolution reason based on finding type
  const getResolutionReason = (finding: UnifiedFinding): string => {
    const resolvedTime = finding.resolvedAt ? formatTime(finding.resolvedAt) : '';

    // For threshold alerts, provide specific reasons based on alert type
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

    // For AI patrol findings
    if (finding.source === 'ai-patrol') {
      return `Issue no longer detected ${resolvedTime}`;
    }

    // Generic fallback
    return `Resolved ${resolvedTime}`;
  };

  return (
    <Card padding="none" class="overflow-hidden">
      {/* Header */}
      <div class="bg-gradient-to-r from-blue-50 to-blue-100 dark:from-blue-900/20 dark:to-blue-900/30 px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="font-medium text-gray-900 dark:text-gray-100">Unified Findings</span>
            <Show when={filteredFindings().length > 0}>
              <span class="px-2 py-0.5 text-xs font-medium bg-blue-200 dark:bg-blue-700 text-blue-800 dark:text-blue-200 rounded-full">
                {filteredFindings().length}
              </span>
            </Show>
          </div>
          <div class="flex items-center gap-2">
            {/* Filter tabs */}
            <Show when={props.showControls !== false}>
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
                  class={`px-2 py-1 rounded-r border ${filter() === 'resolved'
                    ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                    : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                    }`}
                >
                  Resolved
                </button>
              </div>
            </Show>
            {/* Sort dropdown */}
            <select
              value={sortBy()}
              onChange={(e) => setSortBy(e.currentTarget.value as 'severity' | 'time')}
              class="text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
            >
              <option value="severity">By Severity</option>
              <option value="time">By Time</option>
            </select>
          </div>
        </div>
      </div>

      {/* Content */}
      <div class="divide-y divide-gray-100 dark:divide-gray-800">
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

        <Show when={!aiIntelligenceStore.findingsLoading && filteredFindings().length === 0}>
          <div class="p-4 text-sm text-gray-500 dark:text-gray-400 text-center">
            No findings to display
          </div>
        </Show>

        <For each={filteredFindings()}>
          {(finding) => (
            <div
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
                    {/* Source badge */}
                    <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${sourceColors[finding.source] || sourceColors['ai-patrol']}`}>
                      {sourceLabels[finding.source] || finding.source}
                    </span>
                    {/* Severity badge */}
                    <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded uppercase ${severityColors[finding.severity]}`}>
                      {finding.severity}
                    </span>
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
                  <Show when={finding.status === 'active' && !isThresholdFinding(finding)}>
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
                  {/* Investigation button - show for AI findings with investigation data */}
                  <Show when={!isThresholdFinding(finding) && finding.investigationSessionId}>
                    <button
                      type="button"
                      onClick={(e) => openInvestigationDrawer(finding, e)}
                      class="p-1 text-purple-500 hover:text-purple-700 dark:hover:text-purple-300"
                      title="View investigation"
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
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
                <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    {finding.description}
                  </p>
                  <Show when={finding.recommendation}>
                    <p class="text-sm text-gray-700 dark:text-gray-300 mt-2">
                      <span class="font-medium">Recommendation:</span> {finding.recommendation}
                    </p>
                  </Show>
                  <Show when={finding.status === 'active' && isThresholdFinding(finding)}>
                    <div class="mt-3">
                      <button
                        type="button"
                        onClick={() => navigate('/alerts/overview')}
                        class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                      >
                        Manage in Alerts
                      </button>
                    </div>
                  </Show>
                  <Show when={finding.status === 'active' && !isThresholdFinding(finding)}>
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
                  {/* Remediation Plan - only shown for active findings */}
                  <Show when={finding.status === 'active' && plansByFindingId().get(finding.id)}>
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
                      </div>
                    )}
                  </Show>
                  {/* Investigation link */}
                  <Show when={finding.investigationSessionId && !isThresholdFinding(finding)}>
                    <div class="mt-2">
                      <button
                        type="button"
                        onClick={(e) => openInvestigationDrawer(finding, e)}
                        class="text-xs text-purple-600 dark:text-purple-400 hover:underline flex items-center gap-1"
                      >
                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                        </svg>
                        View investigation details
                        <Show when={finding.investigationStatus === 'running'}>
                          <span class="ml-1 h-2 w-2 border border-purple-400 border-t-transparent rounded-full animate-spin" />
                        </Show>
                      </button>
                    </div>
                  </Show>
                </div>
              </Show>
            </div>
          )}
        </For>
      </div>

      {/* Investigation Drawer */}
      <InvestigationDrawer
        open={investigationDrawerOpen()}
        onClose={() => {
          setInvestigationDrawerOpen(false);
          setSelectedFindingForInvestigation(null);
        }}
        findingId={selectedFindingForInvestigation()?.id}
        findingTitle={selectedFindingForInvestigation()?.title}
        findingSeverity={selectedFindingForInvestigation()?.severity as 'critical' | 'warning' | 'watch' | 'info'}
        resourceName={selectedFindingForInvestigation()?.resourceName}
        resourceType={selectedFindingForInvestigation()?.resourceType}
        onReinvestigate={() => {
          // Reload findings after re-investigation is triggered
          aiIntelligenceStore.loadFindings();
        }}
      />
    </Card>
  );
};

export default UnifiedFindingsPanel;
