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
import { investigationStatusLabels, investigationOutcomeLabels, investigationOutcomeColors, type InvestigationStatus } from '@/api/patrol';
import { AIAPI, type RemediationPlan } from '@/api/ai';
import { formatRelativeTime } from '@/utils/format';
import { logger } from '@/utils/logger';

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
  critical: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  warning: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  watch: 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300',
};

// Source badge colors
const sourceColors: Record<string, string> = {
  'threshold': 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-800 dark:bg-orange-900 dark:text-orange-300',
  'ai-patrol': 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  'anomaly': 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  'ai-chat': 'border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-900 dark:text-teal-300',
  'correlation': 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
  'forecast': 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300',
};

// Investigation status badge colors
const investigationStatusColors: Record<InvestigationStatus, string> = {
  pending: 'border-slate-200 bg-slate-50 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400',
  running: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  completed: 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  failed: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
};

// Patrol loop state badge colors (best-effort; state is optional)
const loopStateColors: Record<string, string> = {
  detected: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  investigating: 'border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-800 dark:bg-indigo-900 dark:text-indigo-300',
  remediation_planned: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  remediating: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  remediation_failed: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  timed_out: 'border-slate-300 bg-slate-100 text-slate-700 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300',
  resolved: 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  dismissed: 'border-slate-300 bg-slate-100 text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300',
  snoozed: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  suppressed: 'border-slate-300 bg-slate-100 text-slate-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300',
};

const formatLoopState = (s: string) => s.replace(/_/g, ' ');

const lifecycleLabels: Record<string, string> = {
  detected: 'Detected',
  regressed: 'Regressed',
  acknowledged: 'Acknowledged',
  snoozed: 'Snoozed',
  unsnoozed: 'Unsnoozed',
  dismissed: 'Dismissed',
  undismissed: 'Undismissed',
  suppressed: 'Suppressed',
  resolved: 'Resolved',
  auto_resolved: 'Auto-resolved',
  verification_passed: 'Fix verified',
  investigation_updated: 'Investigation updated',
  investigation_outcome: 'Investigation outcome',
  user_note_updated: 'Note updated',
  loop_state: 'Loop state changed',
  seen_while_suppressed: 'Seen while suppressed',
  loop_transition_violation: 'Invalid transition blocked',
};

const formatLifecycleType = (value: string) => lifecycleLabels[value] || value.replace(/_/g, ' ');

interface FindingsPanelProps {
  resourceId?: string;
  showResolved?: boolean;
  maxItems?: number;
  onFindingClick?: (finding: UnifiedFinding) => void;
  filterOverride?: 'all' | 'active' | 'resolved' | 'approvals' | 'attention';
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
  const [filter, setFilter] = createSignal<'all' | 'active' | 'resolved' | 'approvals' | 'attention'>(props.filterOverride ?? 'active');
  const [sortBy, setSortBy] = createSignal<'severity' | 'time'>('severity');
  const [expandedId, setExpandedId] = createSignal<string | null>(null);
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const [editingNoteId, setEditingNoteId] = createSignal<string | null>(null);
  const [noteText, setNoteText] = createSignal('');
  const [dismissingId, setDismissingId] = createSignal<string | null>(null);
  const [dismissReason, setDismissReason] = createSignal<'not_an_issue' | 'expected_behavior' | 'will_fix_later'>('will_fix_later');
  const [dismissNote, setDismissNote] = createSignal('');

  // Remediation plan artifacts (generated by Patrol and/or investigations).
  const [remediationPlans, setRemediationPlans] = createSignal<RemediationPlan[]>([]);

  const handleDismissPlan = async (plan: RemediationPlan, e: Event) => {
    e.stopPropagation();
    setRemediationPlans(prev => prev.filter(p => p.id !== plan.id));
    notificationStore.success('Remediation plan dismissed');
  };

  // Map of finding_id -> latest remediation plan artifact
  const plansByFindingId = createMemo(() => {
    const map = new Map<string, RemediationPlan>();
    for (const plan of remediationPlans()) {
      const existing = map.get(plan.finding_id);
      if (!existing) {
        map.set(plan.finding_id, plan);
        continue;
      }
      const a = Date.parse(plan.created_at);
      const b = Date.parse(existing.created_at);
      if (!Number.isNaN(a) && !Number.isNaN(b) && a > b) {
        map.set(plan.finding_id, plan);
      }
    }
    return map;
  });

  // Load findings and remediation plans on mount
  createEffect(() => {
    aiIntelligenceStore.loadFindings();
    // Fetch remediation plan artifacts
    AIAPI.getRemediationPlans()
      .then((response: { plans: RemediationPlan[] }) => setRemediationPlans(response.plans))
      .catch((error) => {
        logger.warn('[FindingsPanel] Failed to load remediation plans', error);
      });
  });

  createEffect(() => {
    if (props.filterOverride) {
      setFilter(props.filterOverride);
    }
  });

  // Auto-reset filter when the conditional filter buttons disappear
  createEffect(() => {
    if (filter() === 'attention' && aiIntelligenceStore.needsAttentionCount === 0) {
      setFilter('active');
    }
    if (filter() === 'approvals' && aiIntelligenceStore.pendingApprovalCount === 0) {
      setFilter('active');
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
      findings = findings.filter(f => f.status === 'resolved' || f.status === 'dismissed' || f.status === 'snoozed');
    } else if (filter() === 'attention') {
      const attentionIds = new Set(aiIntelligenceStore.findingsNeedingAttention.map(f => f.id));
      findings = findings.filter(f => attentionIds.has(f.id));
    } else if (filter() === 'approvals') {
      const approvalFindingIds = new Set(aiIntelligenceStore.findingsWithPendingApprovals.map(f => f.id));
      findings = findings.filter(f => approvalFindingIds.has(f.id));
    }

    // Filter by specific finding IDs if provided
    if (props.filterFindingIds && props.filterFindingIds.length > 0) {
      const idSet = new Set(props.filterFindingIds);
      findings = findings.filter(f => idSet.has(f.id));
    }

    // Outcome priority: findings needing attention sort first
    const outcomeOrder: Record<string, number> = {
      fix_verification_failed: 0,
      fix_verification_unknown: 1,
      fix_failed: 0,
      timed_out: 1,
      needs_attention: 1,
      cannot_fix: 1,
      fix_queued: 2,
    };

    // Sort
    findings.sort((a, b) => {
      // Active findings with actionable outcomes sort above others
      const aOutcome = (a.status === 'active' && a.investigationOutcome) ? (outcomeOrder[a.investigationOutcome] ?? 3) : 3;
      const bOutcome = (b.status === 'active' && b.investigationOutcome) ? (outcomeOrder[b.investigationOutcome] ?? 3) : 3;
      if (aOutcome !== bOutcome) return aOutcome - bOutcome;

      if (sortBy() === 'severity') {
        const aPriority = severityOrder[a.severity] ?? 4;
        const bPriority = severityOrder[b.severity] ?? 4;
        if (aPriority !== bPriority) return aPriority - bPriority;

        // Within same severity, sort acknowledged findings below unacknowledged
        const aAcked = a.acknowledgedAt ? 1 : 0;
        const bAcked = b.acknowledgedAt ? 1 : 0;
        if (aAcked !== bAcked) return aAcked - bAcked;
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

  const handleStartDismiss = (finding: UnifiedFinding, reason: 'not_an_issue' | 'expected_behavior' | 'will_fix_later', e: Event) => {
    e.stopPropagation();
    setDismissReason(reason);
    setDismissNote('');
    setDismissingId(finding.id);
    setExpandedId(finding.id);
  };

  const handleConfirmDismiss = async (findingId: string, e: Event) => {
    e.stopPropagation();
    setActionLoading(findingId);
    const ok = await aiIntelligenceStore.dismissFinding(findingId, dismissReason(), dismissNote());
    setActionLoading(null);
    setDismissingId(null);
    if (ok) {
      notificationStore.success('Finding dismissed');
    } else {
      notificationStore.error('Failed to dismiss finding');
    }
  };

  const handleCancelDismiss = (e: Event) => {
    e.stopPropagation();
    setDismissingId(null);
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

  const handleOpenPlanInAssistant = (finding: UnifiedFinding, plan: RemediationPlan, e: Event) => {
    e.stopPropagation();

    let prompt = `Pulse Patrol generated a remediation plan for a finding. Please help me apply it safely.\n\n`;
    prompt += `**Finding:** ${finding.title} on ${finding.resourceName}\n`;
    if (plan.title) prompt += `**Plan:** ${plan.title}\n`;
    if (plan.risk_level) prompt += `**Risk level:** ${plan.risk_level}\n`;
    if (plan.description) prompt += `\n**Plan context:** ${plan.description}\n`;
    prompt += `\n**Steps:**\n`;
    for (const step of plan.steps || []) {
      prompt += `${step.order}. ${step.action}\n`;
      if (step.command) prompt += `   Command: \`${step.command}\`\n`;
      if (step.rollback_command) prompt += `   Rollback: \`${step.rollback_command}\`\n`;
    }
    prompt += `\nIf any step is risky or ambiguous, ask me before proceeding.`;

    aiChatStore.openWithPrompt(prompt, {
      targetType: finding.resourceType,
      targetId: finding.resourceId,
      findingId: finding.id,
    });
  };

  const formatTime = (isoString: string) => formatRelativeTime(isoString, { compact: true });

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
      switch (finding.investigationOutcome) {
        case 'fix_verified':
          return `Fixed by Patrol ${resolvedTime}`;
        case 'fix_executed':
          return `Fix applied by Patrol ${resolvedTime}`;
        case 'resolved':
          return `Resolved by Patrol ${resolvedTime}`;
        case 'fix_failed':
          return `Resolved after fix failed ${resolvedTime}`;
        case 'fix_queued':
          return `Resolved while fix was pending ${resolvedTime}`;
        case 'fix_verification_failed':
          return `Resolved after failed verification ${resolvedTime}`;
        case 'fix_verification_unknown':
          return `Resolved after inconclusive verification ${resolvedTime}`;
        case 'timed_out':
          return `Resolved after investigation timeout ${resolvedTime}`;
        case 'cannot_fix':
          return `Resolved manually ${resolvedTime}`;
        case 'needs_attention':
          return `Resolved after manual review ${resolvedTime}`;
        default:
          return `Issue no longer detected ${resolvedTime}`;
      }
    }

    return `Resolved ${resolvedTime}`;
  };

  // Render a single finding item
  const renderFindingItem = (finding: UnifiedFinding, showSourceBadge: boolean = false) => (
    <div
      id={`finding-${finding.id}`}
      class={`p-3 cursor-pointer transition-colors ${finding.status === 'active'
 ? finding.acknowledgedAt
 ? 'opacity-60 hover:opacity-80 bg-surface-alt'
 : 'hover:bg-surface-hover'
 : 'opacity-60 bg-surface-alt hover:opacity-80'
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
              <span class={`px-1.5 py-0.5 border text-[10px] font-medium rounded ${finding.status === 'resolved'
 ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300'
 : finding.status === 'snoozed'
 ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300'
 : 'border-slate-200 bg-slate-50 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300'
 }`}>
                {finding.status === 'resolved' ? 'Resolved' : finding.status === 'snoozed' ? 'Snoozed' : 'Dismissed'}
              </span>
            </Show>
            {/* Source badge - only show when requested */}
            <Show when={showSourceBadge}>
              <span class={`px-1.5 py-0.5 border text-[10px] font-medium rounded ${sourceColors[finding.source] || sourceColors['ai-patrol']}`}>
                {sourceLabels[finding.source] || finding.source}
              </span>
            </Show>
            {/* Severity badge */}
            <span class={`px-1.5 py-0.5 border text-[10px] font-medium rounded uppercase ${severityColors[finding.severity]}`}>
              {finding.severity}
            </span>
            {/* Alert-triggered badge */}
            <Show when={finding.alertId}>
              <span
                class="px-1.5 py-0.5 border text-[10px] font-medium rounded border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900 text-amber-700 dark:text-amber-300"
                title={finding.alertType ? `Alert: ${finding.alertType}` : `Alert ID: ${finding.alertId}`}
              >
                Alert-triggered
              </span>
            </Show>
            <Show when={finding.acknowledgedAt && finding.status === 'active'}>
              <span class="px-1.5 py-0.5 border text-[10px] font-medium rounded border-border bg-surface-hover text-slate-600 dark:text-slate-300">
                Acknowledged
              </span>
            </Show>
            <Show when={finding.status === 'active' && finding.loopState && !finding.investigationStatus && !finding.investigationOutcome}>
              <span
                class={`px-1.5 py-0.5 border text-[10px] font-medium rounded ${loopStateColors[finding.loopState!] || 'border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-800 text-muted'}`}
                title={`Patrol loop: ${formatLoopState(finding.loopState!)}`}
              >
                {formatLoopState(finding.loopState!)}
              </span>
            </Show>
            <Show when={isOutOfScope(finding)}>
              <span
                class="px-1.5 py-0.5 border text-[10px] font-medium rounded border-amber-300 bg-amber-100 text-amber-900 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-100"
                title="This finding references a resource outside the selected run scope."
              >
                Out of scope
              </span>
            </Show>
            {/* Investigation status badge — only when no outcome badge will show */}
            <Show when={finding.investigationStatus && !(finding.investigationOutcome && finding.investigationStatus !== 'running' && finding.investigationStatus !== 'pending')}>
              <span
                class={`px-1.5 py-0.5 border text-[10px] font-medium rounded flex items-center gap-1 ${investigationStatusColors[finding.investigationStatus!]}`}
                title={`Investigation: ${investigationStatusLabels[finding.investigationStatus!]}`}
              >
                <Show when={finding.investigationStatus === 'running' || finding.investigationStatus === 'pending'}>
                  <span class={`h-2 w-2 rounded-full ${finding.investigationStatus === 'running' ? 'border border-current border-t-transparent animate-spin' : 'bg-current animate-pulse'}`} />
                </Show>
                {investigationStatusLabels[finding.investigationStatus!]}
              </span>
            </Show>
            {/* Investigation outcome badge — replaces status badge when outcome is known */}
            <Show when={finding.investigationOutcome && finding.investigationStatus !== 'running' && finding.investigationStatus !== 'pending'}>
              <span class={`px-1.5 py-0.5 border text-[10px] font-medium rounded ${investigationOutcomeColors[finding.investigationOutcome!] || 'border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-800 text-muted'}`}>
                {investigationOutcomeLabels[finding.investigationOutcome!]}
              </span>
            </Show>
            {/* Title */}
            <span class={`font-medium text-sm truncate ${finding.status === 'active'
 ? 'text-base-content'
 : 'text-muted'
 }`}>
              {finding.title}
            </span>
          </div>
          {/* Resource info */}
          <div class="text-xs text-muted mt-1">
            {finding.resourceName} ({finding.resourceType}) - {formatTime(finding.detectedAt)}
            <Show when={finding.status === 'resolved' && finding.resolvedAt}>
              <span class="ml-2 text-green-600 dark:text-green-400">
                {getResolutionReason(finding)}
              </span>
            </Show>
            <Show when={finding.dismissedReason}>
              <span class="ml-2 text-muted">
                ({finding.dismissedReason?.replace(/_/g, ' ')})
              </span>
            </Show>
            <Show when={finding.status === 'snoozed' && finding.snoozedUntil}>
              <span class="ml-2 text-blue-500 dark:text-blue-400">
                snoozed until {formatTime(finding.snoozedUntil!)}
              </span>
            </Show>
            <Show when={finding.acknowledgedAt && finding.status === 'active'}>
              <span class="ml-2 text-muted">
                acknowledged {formatTime(finding.acknowledgedAt!)}
              </span>
            </Show>
            <Show when={finding.status === 'active' && finding.lastInvestigatedAt}>
              <span class="ml-2 text-muted">
                last investigated {formatTime(finding.lastInvestigatedAt!)}
              </span>
            </Show>
          </div>
        </div>
        {/* Actions */}
        <div class="flex items-center gap-1 shrink-0">
          <Show when={finding.status === 'active'}>
            <Show when={!finding.acknowledgedAt}>
              <button
                type="button"
                onClick={(e) => handleAcknowledge(finding, e)}
                class="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                title="Acknowledge"
                disabled={actionLoading() === finding.id}
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4" />
                </svg>
              </button>
            </Show>
            <button
              type="button"
              onClick={(e) => handleSnooze(finding, 24, e)}
              class="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
              title="Snooze 24h"
              disabled={actionLoading() === finding.id}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              type="button"
              onClick={(e) => handleStartDismiss(finding, 'will_fix_later', e)}
              class="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
              title="Dismiss"
              disabled={actionLoading() === finding.id}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </Show>
          {/* Expand indicator */}
          <svg
            class={`w-4 h-4 text-slate-400 transition-transform ${expandedId() === finding.id ? 'rotate-180' : ''}`}
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
    <div class="mt-3 pt-3 border-t border-border-subtle">
      <Show when={finding.alertId}>
        <div class="text-xs text-amber-700 dark:text-amber-300 mb-2">
          Triggered by alert{finding.alertType ? ` (${finding.alertType})` : ''} • ID {finding.alertId}
        </div>
      </Show>
      <p class="text-sm text-muted">
        {finding.description}
      </p>
      <Show when={finding.recommendation}>
        <p class="text-sm text-base-content mt-2">
          <span class="font-medium">Recommendation:</span> {finding.recommendation}
        </p>
      </Show>
      <Show when={(finding.regressionCount || 0) > 0}>
        <p class="text-xs text-amber-700 dark:text-amber-300 mt-2">
          Regressions: {finding.regressionCount}
          <Show when={finding.lastRegressionAt}>
            {' '}
            (last {formatRelativeTime(finding.lastRegressionAt)})
          </Show>
        </p>
      </Show>

      <Show when={finding.lifecycle && finding.lifecycle.length > 0}>
        <div class="mt-3 p-2 rounded border border-border bg-surface-alt">
          <div class="text-xs font-medium text-base-content mb-2">Lifecycle</div>
          <div class="space-y-1">
            <For each={[...(finding.lifecycle || [])].slice(-6).reverse()}>
              {(event) => (
                <div class="text-xs text-muted flex items-start justify-between gap-2">
                  <span class="truncate">
                    <span class="font-medium text-base-content">{formatLifecycleType(event.type)}</span>
                    <Show when={event.message}>
                      {' '}
                      <span>{event.message}</span>
                    </Show>
                    <Show when={event.from && event.to}>
                      {' '}
                      <span class="text-muted">({event.from} {'->'} {event.to})</span>
                    </Show>
                  </span>
                  <span class="shrink-0">{formatRelativeTime(event.at)}</span>
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      {/* User note display / editor */}
      <Show when={editingNoteId() === finding.id}>
        <div class="mt-3 p-2 rounded border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900" onClick={(e) => e.stopPropagation()}>
          <textarea
            class="w-full text-sm rounded border border-border bg-surface text-base-content px-2 py-1.5 resize-none focus:outline-none focus:ring-1 focus:ring-blue-500"
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
 class="px-3 py-1 text-xs font-medium rounded border border-border hover:bg-surface-hover"
 >
 Cancel
 </button>
 </div>
 </div>
 </Show>
 <Show when={editingNoteId() !== finding.id && finding.userNote}>
 <div class="mt-3 p-2 rounded border border-border bg-surface-alt flex items-start gap-2">
 <svg class="w-4 h-4 text-muted mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
 </svg>
 <p class="text-sm text-muted flex-1">{finding.userNote}</p>
 <button
 type="button"
 onClick={(e) => handleStartEditNote(finding, e)}
 class="p-1 hover: dark:hover:text-slate-300 flex-shrink-0"
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
 class="px-2 py-1 rounded border border-border hover:bg-surface-hover flex items-center gap-1"
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
 class="px-2 py-1 rounded bg-blue-50 text-blue-700 dark:bg-blue-900 dark:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900 flex items-center gap-1 transition-colors"
 >
 <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
 </svg>
 Discuss with Assistant
 </button>
 </div>

 <Show when={finding.status ==='active'}>
        <div class="mt-3 flex flex-wrap gap-2 text-xs">
          <Show when={!finding.acknowledgedAt}>
            <button
              type="button"
              onClick={(e) => handleAcknowledge(finding, e)}
              class="px-2 py-1 rounded border border-border hover:bg-surface-hover"
              disabled={actionLoading() === finding.id}
            >
              Acknowledge
            </button>
          </Show>
          <button
            type="button"
            onClick={(e) => handleSnooze(finding, 1, e)}
            class="px-2 py-1 rounded border border-border hover:bg-surface-hover"
            disabled={actionLoading() === finding.id}
          >
            Snooze 1h
          </button>
          <button
            type="button"
            onClick={(e) => handleSnooze(finding, 24, e)}
            class="px-2 py-1 rounded border border-border hover:bg-surface-hover"
            disabled={actionLoading() === finding.id}
          >
            Snooze 24h
          </button>
          <button
            type="button"
            onClick={(e) => handleSnooze(finding, 168, e)}
            class="px-2 py-1 rounded border border-border hover:bg-surface-hover"
            disabled={actionLoading() === finding.id}
          >
            Snooze 7d
          </button>
          <button
            type="button"
            onClick={(e) => handleStartDismiss(finding, 'not_an_issue', e)}
            class="px-2 py-1 rounded border border-border text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900"
            disabled={actionLoading() === finding.id}
          >
            Dismiss: Not an issue
          </button>
          <button
            type="button"
            onClick={(e) => handleStartDismiss(finding, 'expected_behavior', e)}
            class="px-2 py-1 rounded border border-border hover:bg-surface-hover"
            disabled={actionLoading() === finding.id}
          >
            Dismiss: Expected
          </button>
          <button
            type="button"
            onClick={(e) => handleStartDismiss(finding, 'will_fix_later', e)}
            class="px-2 py-1 rounded border border-border hover:bg-surface-hover"
            disabled={actionLoading() === finding.id}
          >
            Dismiss: Later
          </button>
        </div>
      </Show>
      {/* Inline dismiss confirmation */}
      <Show when={dismissingId() === finding.id}>
        <div class="mt-2 p-2 rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900">
          <div class="flex items-center gap-2 mb-1.5">
            <span class="text-xs font-medium text-red-700 dark:text-red-300">
              Dismiss as: {dismissReason().replace(/_/g, ' ')}
            </span>
          </div>
          <textarea
            class="w-full text-xs px-2 py-1.5 rounded border border-border bg-surface text-base-content resize-none focus:outline-none focus:ring-1 focus:ring-red-400"
            rows={2}
            placeholder="Optional note (for learning context)..."
            value={dismissNote()}
            onInput={(e) => setDismissNote(e.currentTarget.value)}
            onClick={(e) => e.stopPropagation()}
          />
          <div class="flex items-center gap-2 mt-1.5">
            <button
              type="button"
              onClick={(e) => handleConfirmDismiss(finding.id, e)}
              disabled={actionLoading() === finding.id}
              class="px-2.5 py-1 text-xs font-medium text-white bg-red-600 hover:bg-red-700 disabled:bg-red-400 rounded transition-colors"
            >
              Confirm Dismiss
            </button>
            <button
              type="button"
              onClick={handleCancelDismiss}
              class="px-2.5 py-1 text-xs font-medium text-muted hover:bg-surface-hover rounded transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      </Show>
      <Show when={finding.correlatedFindingIds && finding.correlatedFindingIds.length > 0}>
        <div class="mt-2 text-xs text-muted">
          Related findings: {finding.correlatedFindingIds?.length}
        </div>
      </Show>

      {/* Inline Investigation Section (replaces drawer) */}
      <Show when={finding.investigationSessionId}>
        <InvestigationSection
          findingId={finding.id}
          investigationStatus={finding.investigationStatus}
          investigationOutcome={finding.investigationOutcome}
          investigationAttempts={finding.investigationAttempts}
        />
      </Show>

      {/* Inline Approval Section (replaces manual approval JSX) */}
      <Show when={finding.status === 'active' && (
        finding.investigationOutcome === 'fix_queued' ||
        finding.investigationOutcome === 'fix_executed' ||
        finding.investigationOutcome === 'fix_failed' ||
        finding.investigationOutcome === 'fix_verified' ||
        finding.investigationOutcome === 'fix_verification_failed' ||
        finding.investigationOutcome === 'fix_verification_unknown'
      )}>
        <ApprovalSection
          findingId={finding.id}
          investigationOutcome={finding.investigationOutcome}
          findingTitle={finding.title}
          resourceName={finding.resourceName}
          resourceType={finding.resourceType}
          resourceId={finding.resourceId}
        />
      </Show>

      {/* Remediation Plan artifact (generated by Patrol and/or an investigation) */}
      <Show when={finding.status === 'active' && plansByFindingId().get(finding.id)}>
        {(plan) => (
          <div class="mt-3 pt-3 border-t border-border-subtle">
            <div class="flex items-center gap-2 mb-2">
              <svg class="w-4 h-4 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
              </svg>
              <span class="text-sm font-medium text-base-content">Remediation Plan</span>
              <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${plan().risk_level === 'high' ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300' :
 plan().risk_level === 'medium' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300' :
 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
 }`}>
                {plan().risk_level} risk
              </span>
            </div>
            <div class="space-y-2">
              <For each={plan().steps}>
                {(step) => (
                  <div class="flex items-start gap-2 text-sm">
                    <span class="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded-full bg-surface-hover text-xs font-medium text-muted">
                      {step.order}
                    </span>
                    <div class="flex-1 min-w-0">
                      <div class="text-base-content">{step.action}</div>
                      <Show when={step.command}>
                        <div class="mt-1 font-mono text-[11px] whitespace-pre-wrap break-words text-muted bg-surface-alt px-2 py-1 rounded">
                          {step.command}
                        </div>
                      </Show>
                    </div>
                  </div>
                )}
              </For>
            </div>

            <div class="flex items-center gap-2 mt-3 pt-3 border-t border-border-subtle">
              <button
                type="button"
                onClick={(e) => handleOpenPlanInAssistant(finding, plan(), e)}
                class="flex-1 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 8h10M7 12h10M7 16h10" />
                </svg>
                Open In Assistant
              </button>
              <button
                type="button"
                onClick={(e) => handleDismissPlan(plan(), e)}
                class="px-3 py-1.5 hover:bg-surface-hover text-muted text-xs font-medium rounded"
              >
                Dismiss
              </button>
            </div>
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
 ? 'bg-surface-alt text-base-content border-border shadow-sm'
 : 'border-transparent text-muted hover:text-base-content'
 }`}
            >
              Active
            </button>
            <button
              type="button"
              onClick={() => setFilter('all')}
              class={`px-2 py-1 border-y border-x ${filter() === 'all'
 ? 'bg-surface-alt text-base-content border-border shadow-sm'
 : 'border-transparent text-muted hover:text-base-content'
 }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => setFilter('resolved')}
              class={`px-2 py-1 border-y border-r ${filter() === 'resolved'
 ? 'bg-surface-alt text-base-content border-border shadow-sm'
 : 'border-transparent text-muted hover:text-base-content'
 } ${aiIntelligenceStore.needsAttentionCount > 0 || aiIntelligenceStore.pendingApprovalCount > 0 ? '' : 'rounded-r border-r'}`}
            >
              Resolved
            </button>
            <Show when={aiIntelligenceStore.needsAttentionCount > 0}>
              <button
                type="button"
                onClick={() => setFilter('attention')}
                class={`px-2 py-1 border-y border-r ${filter() === 'attention'
 ? 'bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700 shadow-sm'
 : 'border-transparent text-muted hover:text-base-content'
 } ${aiIntelligenceStore.pendingApprovalCount > 0 ? '' : 'rounded-r border-r'}`}
              >
                Needs Attention ({aiIntelligenceStore.needsAttentionCount})
              </button>
            </Show>
            <Show when={aiIntelligenceStore.pendingApprovalCount > 0}>
              <button
                type="button"
                onClick={() => setFilter('approvals')}
                class={`px-2 py-1 rounded-r border-y border-r ${filter() === 'approvals'
 ? 'bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700 shadow-sm'
 : 'border-transparent text-muted hover:text-base-content'
 }`}
              >
                Approvals ({aiIntelligenceStore.pendingApprovalCount})
              </button>
            </Show>
          </div>
          <select
            value={sortBy()}
            onChange={(e) => setSortBy(e.currentTarget.value as 'severity' | 'time')}
            class="text-xs px-2 py-1 rounded border border-border bg-surface"
          >
            <option value="severity">By Severity</option>
            <option value="time">By Time</option>
          </select>
        </div>
      </Show>

      {/* Loading/Error states */}
      <Show when={aiIntelligenceStore.findingsLoading}>
        <div class="p-4 text-sm text-muted flex items-center gap-2">
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
          <div class="bg-blue-50 dark:bg-blue-900 px-4 py-3 border-b border-border">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
                <span class="font-medium text-base-content">Pulse Patrol Findings</span>
                <Show when={patrolFindings().length > 0}>
                  <span class="px-2 py-0.5 text-xs font-medium bg-blue-200 dark:bg-blue-700 text-blue-800 dark:text-blue-200 rounded-full">
                    {patrolFindings().length}
                  </span>
                </Show>
              </div>
              <span class="text-xs text-muted">AI-discovered insights</span>
            </div>
          </div>
          {/* Content */}
          <div class="divide-y divide-gray-100 dark:divide-gray-800">
            <Show when={patrolFindings().length === 0}>
              <div class="p-6 text-sm text-muted text-center">
                <Show when={filter() === 'active'}>
 <div class="flex flex-col items-center gap-3">
 <svg class="w-10 h-10 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
 </svg>
 <div>
 <p class="font-medium text-base-content">No active findings</p>
 <p class="text-xs mt-1">Your infrastructure looks healthy!</p>
 </div>
 <Show when={props.nextPatrolAt || props.lastPatrolAt || props.patrolIntervalMs}>
 <div class="mt-2 pt-3 border-t border-border w-full max-w-xs">
 <div class="flex items-center justify-center gap-4 text-xs">
 <Show when={props.lastPatrolAt}>
 <div class="flex items-center gap-1.5">
 <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
 </svg>
 <span>Last: {formatTime(props.lastPatrolAt!)}</span>
 </div>
 </Show>
 <Show when={props.nextPatrolAt}>
 <div class="flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
 <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
 </svg>
 <span>Next: {formatTime(props.nextPatrolAt!)}</span>
 </div>
 </Show>
 <Show when={!props.nextPatrolAt && !props.lastPatrolAt && props.patrolIntervalMs}>
 <div class="flex items-center gap-1.5 text-muted">
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
 <Show when={filter() ==='attention'}>
                  No findings need attention right now.
                </Show>
                <Show when={filter() === 'approvals'}>
                  No pending approvals.
                </Show>
                <Show when={filter() !== 'active' && filter() !== 'attention' && filter() !== 'approvals'}>
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
