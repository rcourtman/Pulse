/**
 * FindingsPanel
 *
 * Separated view showing:
 * - Patrol findings
 * - Threshold Alerts (user-configured rules)
 * Each section has severity-based sorting and quick actions
 *
 * Investigation and approval details are shown inline via
 * InvestigationSection and ApprovalSection components.
 */

import { Component, createSignal, createEffect, Show, For, createMemo } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { FormSelect } from '@/components/shared/FormSelect';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { UpgradeButtonLink } from '@/components/shared/UpgradeLink';
import { aiIntelligenceStore, type UnifiedFinding } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import {
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantFindingHandoffFromUnifiedFinding,
  buildPatrolAssistantProposedFixBriefingInput,
  buildPatrolAssistantProposedFixBriefingInputFromApproval,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantModelContext,
  patrolAssistantFindingHandoffRequiresApprovalMode,
  type PatrolAssistantApprovalBriefingInput,
  type PatrolAssistantProposedFixBriefingInput,
} from '@/features/patrol/patrolInvestigationContextModel';
import { InvestigationSection, ApprovalSection } from '@/components/patrol';
import { AIAPI, type RemediationPlan } from '@/api/ai';
import { createSuppressionRuleFromFinding } from '@/api/patrol';
import type { PatrolRunRecord, PatrolRuntimeState } from '@/api/patrol';
import { formatRelativeTime } from '@/utils/format';
import { getFindingAlertIdentifier, hasTriggeringAlert } from '@/utils/findingAlertIdentity';
import { segmentedButtonClass } from '@/utils/segmentedButton';
import { getSemanticTonePresentation } from '@/utils/semanticTonePresentation';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';
import { getPatrolFindingsEmptyState } from '@/utils/patrolEmptyStatePresentation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import InfoIcon from 'lucide-solid/icons/info';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import {
  buildFindingFilterOptions,
  formatFindingForClipboard,
  formatFindingLifecycleType,
  formatOperatorStateDismissCauseLabel,
  getOperatorStateDismissCause,
  formatFindingLoopState,
  getFindingActiveRuntimeSortOrder,
  getFindingSeverityPresentation,
  buildPatrolFindingDisplayGroups,
  getPatrolFindingIssueCountLabel,
  getFindingPatrolWorkflowPresentation,
  getFindingSeveritySortOrder,
  getFindingResolutionReason,
  getFindingLoopStateBadgeTone,
  getFindingStatusBadgeTone,
  getFindingStatusLabel,
  getFindingSourceBadgeTone,
  getFindingSourceLabel,
  getFindingManualControlsPresentation,
  getFindingPrimaryActionPresentation,
  getFindingSubjectPresentation,
  getFindingTitlePresentation,
  getFindingRecencyPresentation,
  hasFindingInvestigationDetails,
  hasFindingInvestigationHandoffPointer,
  getInvestigationConfidenceBadgeTone,
  getInvestigationOutcomeBadgeTone,
  getInvestigationOutcomeLabel,
  getInvestigationStatusLabel,
  getInvestigationOutcomeSortOrder,
  getInvestigationStatusBadgeTone,
} from '@/utils/aiFindingPresentation';
import { copyToClipboard } from '@/utils/clipboard';

const FINDING_ROW_BADGE_PROPS = {
  appearance: 'outline',
  size: 'xs',
  shape: 'rounded',
} as const;

interface FindingsPanelProps {
  resourceId?: string;
  showResolved?: boolean;
  maxItems?: number;
  onFindingClick?: (finding: UnifiedFinding) => void;
  filterOverride?: 'all' | 'active' | 'resolved' | 'approvals' | 'attention' | 'overdue';
  filterFindingIds?: string[];
  showControls?: boolean;
  scopeResourceIds?: string[];
  scopeResourceTypes?: string[];
  showScopeWarnings?: boolean;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
  overallHealth?: IntelligenceHealthScore;
  historicalRegressionCount?: number;
  patrolRuns?: PatrolRunRecord[];
  findingsSource?: 'unified' | 'patrol';
  onAssistantHandoff?: (finding: UnifiedFinding) => void;
  patrolProHandoff?: (
    finding: UnifiedFinding,
  ) => { detail: string; actionLabel?: string; destination?: UpgradeDestination } | undefined;
  runSnapshot?: Pick<
    PatrolRunRecord,
    | 'resources_checked'
    | 'scope_resource_ids'
    | 'effective_scope_resource_ids'
    | 'finding_ids'
    | 'status'
    | 'error_count'
  >;
}

function getPrimaryAssistantFindingAction(_finding: UnifiedFinding): {
  label: string;
  title: string;
} {
  return {
    label: 'Open in Assistant',
    title: 'Open Pulse Assistant with this finding attached',
  };
}

function getFindingSuppressionRuleScope(finding: UnifiedFinding) {
  const resourceId = finding.resourceId.trim();
  const category = finding.category.trim();
  return {
    resourceId,
    resourceName: finding.resourceName.trim() || resourceId || 'this resource',
    category,
    canCreate: Boolean(resourceId && category),
  };
}

export const FindingsPanel: Component<FindingsPanelProps> = (props) => {
  const location = useLocation();
  const [filter, setFilter] = createSignal<
    'all' | 'active' | 'resolved' | 'approvals' | 'attention' | 'overdue'
  >(props.filterOverride ?? 'active');
  const [sortBy, setSortBy] = createSignal<'severity' | 'time'>('severity');
  const [expandedId, setExpandedId] = createSignal<string | null>(null);
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const [editingNoteId, setEditingNoteId] = createSignal<string | null>(null);
  const [noteText, setNoteText] = createSignal('');
  const [dismissingId, setDismissingId] = createSignal<string | null>(null);
  const [dismissReason, setDismissReason] = createSignal<
    'not_an_issue' | 'expected_behavior' | 'will_fix_later'
  >('will_fix_later');
  const [dismissNote, setDismissNote] = createSignal('');

  const [dismissedPlanIds, setDismissedPlanIds] = createSignal<string[]>([]);
  const isPatrolFindingsSource = createMemo(() => props.findingsSource === 'patrol');
  const hasUnknownRunSnapshot = createMemo(
    () => props.runSnapshot !== undefined && props.filterFindingIds === undefined,
  );
  const hasEmptyRunSnapshot = createMemo(
    () =>
      props.runSnapshot !== undefined &&
      Array.isArray(props.filterFindingIds) &&
      props.filterFindingIds.length === 0,
  );
  const canRenderRunSnapshotWithoutSourceFindings = createMemo(
    () => hasUnknownRunSnapshot() || hasEmptyRunSnapshot(),
  );
  const shouldShowFindingLifecycle = (finding: UnifiedFinding) =>
    Boolean(finding.lifecycle && finding.lifecycle.length > 0) &&
    (!isPatrolFindingsSource() ||
      finding.status !== 'active' ||
      props.runSnapshot !== undefined ||
      filter() === 'all' ||
      filter() === 'resolved');
  const shouldShowCollapsedPrimaryAction = (finding: UnifiedFinding) =>
    isPatrolFindingsSource() &&
    finding.status === 'active' &&
    props.runSnapshot === undefined &&
    expandedId() !== finding.id &&
    Boolean(getFindingPrimaryActionPresentation(finding));
  const shouldShowInlineManualControls = (finding: UnifiedFinding) =>
    finding.status === 'active' && !isPatrolFindingsSource();
  const sourceFindings = createMemo(() =>
    isPatrolFindingsSource() ? aiIntelligenceStore.patrolFindings : aiIntelligenceStore.findings,
  );
  const sourceHasFindings = createMemo(() => sourceFindings().length > 0);
  const sourceFindingsLoading = createMemo(() =>
    isPatrolFindingsSource()
      ? aiIntelligenceStore.patrolFindingsLoading
      : aiIntelligenceStore.findingsLoading,
  );
  const shouldShowLoadingState = createMemo(
    () =>
      sourceFindingsLoading() &&
      !sourceHasFindings() &&
      !canRenderRunSnapshotWithoutSourceFindings(),
  );
  const sourceFindingsError = createMemo(() =>
    isPatrolFindingsSource()
      ? aiIntelligenceStore.patrolFindingsError
      : aiIntelligenceStore.findingsError,
  );
  const sourceFindingsNeedingAttention = createMemo(() =>
    isPatrolFindingsSource()
      ? aiIntelligenceStore.patrolFindingsNeedingAttention
      : aiIntelligenceStore.findingsNeedingAttention,
  );
  const sourceFindingsWithPendingApprovals = createMemo(() =>
    isPatrolFindingsSource()
      ? aiIntelligenceStore.patrolFindingsWithPendingApprovals
      : aiIntelligenceStore.findingsWithPendingApprovals,
  );

  const handleDismissPlan = async (plan: RemediationPlan, e: Event) => {
    e.stopPropagation();
    setDismissedPlanIds((prev) => [...prev, plan.id]);
    notificationStore.success('Remediation plan dismissed');
  };

  // Map of finding_id -> latest remediation plan artifact
  const plansByFindingId = createMemo(() => {
    const dismissedPlanIdsSet = new Set(dismissedPlanIds());
    const map = new Map<string, RemediationPlan>();
    for (const plan of aiIntelligenceStore.remediationPlans) {
      if (dismissedPlanIdsSet.has(plan.id)) continue;
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
    if (isPatrolFindingsSource()) {
      aiIntelligenceStore.loadPatrolFindings();
    } else {
      aiIntelligenceStore.loadFindings();
    }
    aiIntelligenceStore.loadRemediationPlans();
  });

  createEffect(() => {
    if (props.filterOverride) {
      setFilter(props.filterOverride);
    }
  });

  // The Patrol findings endpoint defaults to active-only. When the
  // operator switches to a tab that needs history (Resolved, or All —
  // both surfaces are meaningless if they only show what the Active
  // tab already shows), fetch the full
  // active+resolved+dismissed+snoozed set so the audit-trail filter has
  // data to show. The trust strip credits "N auto-resolved" without
  // this load and those tabs are otherwise empty.
  createEffect(() => {
    const f = filter();
    if ((f === 'resolved' || f === 'all') && isPatrolFindingsSource()) {
      void aiIntelligenceStore.loadPatrolFindings({ includeResolved: true });
    }
  });

  // Filter and sort findings
  const filteredFindings = createMemo(() => {
    if (hasUnknownRunSnapshot()) {
      return [];
    }

    let findings = [...sourceFindings()];

    // Filter by resource if specified
    if (props.resourceId) {
      findings = findings.filter((f) => f.resourceId === props.resourceId);
    }

    // Filter by status
    if (filter() === 'active') {
      findings = findings.filter((f) => f.status === 'active');
    } else if (filter() === 'resolved') {
      findings = findings.filter(
        (f) => f.status === 'resolved' || f.status === 'dismissed' || f.status === 'snoozed',
      );
    } else if (filter() === 'attention') {
      const attentionIds = new Set(sourceFindingsNeedingAttention().map((f) => f.id));
      findings = findings.filter((f) => attentionIds.has(f.id));
    } else if (filter() === 'approvals') {
      const approvalFindingIds = new Set(sourceFindingsWithPendingApprovals().map((f) => f.id));
      findings = findings.filter((f) => approvalFindingIds.has(f.id));
    } else if (filter() === 'overdue') {
      // Overdue surfaces will_fix_later commitments whose RemindAt
      // deadline has already passed. The Go store's proactive sweep
      // (SweepWillFixLaterReminders) wakes these on a 1h timer, but
      // this filter lets the operator see them on demand — including
      // any that the sweep has not yet promoted on this cycle.
      const nowMs = Date.now();
      findings = findings.filter((f) => {
        if (f.dismissedReason !== 'will_fix_later') {
          return false;
        }
        if (!f.remindAt) {
          return false;
        }
        const due = Date.parse(f.remindAt);
        return Number.isFinite(due) && due <= nowMs;
      });
    }

    // Filter by specific finding IDs if provided
    if (props.filterFindingIds !== undefined) {
      const idSet = new Set(props.filterFindingIds);
      findings = findings.filter((f) => idSet.has(f.id));
    }

    // Sort
    findings.sort((a, b) => {
      // Active findings with actionable outcomes sort above others
      const aOutcome =
        a.status === 'active' && a.investigationOutcome
          ? getInvestigationOutcomeSortOrder(a.investigationOutcome)
          : 3;
      const bOutcome =
        b.status === 'active' && b.investigationOutcome
          ? getInvestigationOutcomeSortOrder(b.investigationOutcome)
          : 3;
      if (aOutcome !== bOutcome) return aOutcome - bOutcome;

      if (sortBy() === 'severity') {
        const aPriority = getFindingSeveritySortOrder(a.severity);
        const bPriority = getFindingSeveritySortOrder(b.severity);
        if (aPriority !== bPriority) return aPriority - bPriority;

        const aRuntimePriority = getFindingActiveRuntimeSortOrder(a);
        const bRuntimePriority = getFindingActiveRuntimeSortOrder(b);
        if (aRuntimePriority !== bRuntimePriority) return aRuntimePriority - bRuntimePriority;

        // Within same severity, sort acknowledged findings below unacknowledged
        const aAcked = a.acknowledgedAt ? 1 : 0;
        const bAcked = b.acknowledgedAt ? 1 : 0;
        if (aAcked !== bAcked) return aAcked - bAcked;
      }

      const aRecency = getFindingRecencyPresentation(a);
      const bRecency = getFindingRecencyPresentation(b);
      return new Date(bRecency.timestamp).getTime() - new Date(aRecency.timestamp).getTime();
    });

    // Limit items
    if (props.maxItems && props.maxItems > 0) {
      findings = findings.slice(0, props.maxItems);
    }

    return findings;
  });

  // Filter to only show Patrol findings (exclude threshold alerts)
  const allPatrolFindings = createMemo(() => {
    if (isPatrolFindingsSource()) {
      return sourceFindings();
    }
    return sourceFindings().filter(
      (f) => f.source !== 'threshold' && !f.isThreshold && !hasTriggeringAlert(f),
    );
  });
  const runSnapshotScopedPatrolFindings = createMemo(() => {
    if (hasUnknownRunSnapshot()) {
      return [];
    }
    if (props.filterFindingIds === undefined) {
      return allPatrolFindings();
    }
    const snapshotFindingIds = new Set(props.filterFindingIds);
    return allPatrolFindings().filter((finding) => snapshotFindingIds.has(finding.id));
  });
  const useRunSnapshotScopedControls = createMemo(() => props.runSnapshot !== undefined);
  const scopedNeedsAttentionCount = createMemo(() => {
    const attentionIds = new Set(sourceFindingsNeedingAttention().map((f) => f.id));
    return runSnapshotScopedPatrolFindings().filter((finding) => attentionIds.has(finding.id))
      .length;
  });
  const scopedPendingApprovalCount = createMemo(() => {
    const approvalIds = new Set(sourceFindingsWithPendingApprovals().map((finding) => finding.id));
    return runSnapshotScopedPatrolFindings().filter((finding) => approvalIds.has(finding.id))
      .length;
  });
  const filterCounts = createMemo(() => ({
    needsAttentionCount: useRunSnapshotScopedControls()
      ? scopedNeedsAttentionCount()
      : sourceFindingsNeedingAttention().length,
    pendingApprovalCount: useRunSnapshotScopedControls()
      ? scopedPendingApprovalCount()
      : sourceFindingsWithPendingApprovals().length,
  }));
  // Count of will_fix_later commitments whose RemindAt deadline has
  // already passed. Drives the Overdue commitments chip below.
  const overdueCount = createMemo(() => {
    const nowMs = Date.now();
    return sourceFindings().filter((f) => {
      if (f.dismissedReason !== 'will_fix_later') return false;
      if (!f.remindAt) return false;
      const due = Date.parse(f.remindAt);
      return Number.isFinite(due) && due <= nowMs;
    }).length;
  });
  const patrolFindings = createMemo(() => {
    if (isPatrolFindingsSource()) {
      return filteredFindings();
    }
    return filteredFindings().filter(
      (f) => f.source !== 'threshold' && !f.isThreshold && !hasTriggeringAlert(f),
    );
  });
  const shouldGroupPatrolFindingsByResource = createMemo(
    () =>
      isPatrolFindingsSource() &&
      filter() === 'active' &&
      props.runSnapshot === undefined &&
      props.resourceId === undefined,
  );
  const patrolFindingDisplayGroups = createMemo(() => {
    const findings = patrolFindings();
    if (!shouldGroupPatrolFindingsByResource()) {
      return findings.map((finding) => ({
        id: `finding:${finding.id}`,
        resourceKey: `finding:${finding.id}`,
        resourceLabel: getFindingSubjectPresentation(finding).label,
        primaryFinding: finding,
        relatedFindings: [],
        findings: [finding],
      }));
    }

    return buildPatrolFindingDisplayGroups(findings);
  });
  const filterOptions = createMemo(() => buildFindingFilterOptions(filterCounts()));
  const emptyStateCopy = createMemo(() => {
    // 'overdue' is a FindingsPanel-local extension to the shared
    // FindingsFilter union. The empty state for that case is rendered
    // inline below (the <Show when={filter() === 'overdue'}> branch),
    // so the value passed here is intentionally a no-op fallback that
    // keeps the helper inside its FindingsFilter contract.
    const currentFilter = filter();
    const emptyFilter = currentFilter === 'overdue' ? 'all' : currentFilter;
    return getPatrolFindingsEmptyState({
      filter: emptyFilter,
      overallHealth: props.overallHealth,
      runtimeState: props.runtimeState,
      blockedReason: props.blockedReason,
      historicalRegressionCount: props.historicalRegressionCount,
      runs: props.patrolRuns,
      runSnapshot: props.runSnapshot,
    });
  });
  const emptyStateTone = createMemo(() => getSemanticTonePresentation(emptyStateCopy().tone));
  const shouldShowRichEmptyState = createMemo(
    () => filter() === 'active' || props.runSnapshot !== undefined,
  );
  // Auto-reset filter when the conditional filter buttons disappear
  createEffect(() => {
    if (filter() === 'attention' && filterCounts().needsAttentionCount === 0) {
      setFilter('active');
    }
    if (filter() === 'approvals' && filterCounts().pendingApprovalCount === 0) {
      setFilter('active');
    }
    if (filter() === 'overdue' && overdueCount() === 0) {
      setFilter('active');
    }
  });
  const showFilterControls = createMemo(
    () =>
      props.showControls !== false &&
      (useRunSnapshotScopedControls()
        ? runSnapshotScopedPatrolFindings().length > 0 ||
          filterCounts().needsAttentionCount > 0 ||
          filterCounts().pendingApprovalCount > 0 ||
          overdueCount() > 0
        : allPatrolFindings().length > 0 ||
          filterCounts().needsAttentionCount > 0 ||
          filterCounts().pendingApprovalCount > 0 ||
          overdueCount() > 0),
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
    if (isPatrolFindingsSource()) {
      aiIntelligenceStore.patrolFindingsSignal();
    } else {
      aiIntelligenceStore.findingsSignal();
    }
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

  // Operator-driven manual resolve. Use case: the operator fixed the
  // underlying issue out-of-band and wants to close the loop without
  // waiting for Pulse's auto-detection to clear it. The server records
  // auto=false so analytics can distinguish operator vs Pulse resolution;
  // re-detection still flows through the regression path.
  const handleResolve = async (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    setActionLoading(finding.id);
    const ok = await aiIntelligenceStore.resolveFinding(finding.id);
    setActionLoading(null);
    if (ok) {
      notificationStore.success('Finding marked resolved');
    } else {
      notificationStore.error('Failed to mark finding resolved');
    }
  };

  const handleStartDismiss = (
    finding: UnifiedFinding,
    reason: 'not_an_issue' | 'expected_behavior' | 'will_fix_later',
    e: Event,
  ) => {
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

  const loadLatestInvestigationProposedFixBriefing = async (
    finding: UnifiedFinding,
    pendingApprovalBriefing: PatrolAssistantApprovalBriefingInput | undefined,
  ): Promise<PatrolAssistantProposedFixBriefingInput | undefined> => {
    if (finding.investigationRecord?.proposed_fix) {
      return undefined;
    }
    const hasInvestigationPointer =
      hasFindingInvestigationHandoffPointer(finding) || Boolean(pendingApprovalBriefing?.id);
    if (!hasInvestigationPointer) {
      return undefined;
    }
    if (
      !patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationOutcome: finding.investigationOutcome,
        remediationId: finding.remediationPlanId,
        pendingApproval: pendingApprovalBriefing,
        investigationRecord: finding.investigationRecord,
      })
    ) {
      return undefined;
    }

    try {
      const investigation = await AIAPI.getInvestigation(finding.id);
      return buildPatrolAssistantProposedFixBriefingInput(investigation?.proposed_fix);
    } catch {
      return undefined;
    }
  };

  const openFindingInAssistant = async (finding: UnifiedFinding) => {
    await aiIntelligenceStore.loadPendingApprovals();
    const pendingApproval = aiIntelligenceStore.patrolPendingApprovals.find(
      (approval) => approval.toolId === 'investigation_fix' && approval.targetId === finding.id,
    );
    const pendingApprovalBriefing = buildPatrolAssistantApprovalBriefingInput(pendingApproval);
    const latestInvestigationProposedFix = await loadLatestInvestigationProposedFixBriefing(
      finding,
      pendingApprovalBriefing,
    );
    const proposedFix =
      latestInvestigationProposedFix ||
      buildPatrolAssistantProposedFixBriefingInputFromApproval(pendingApproval);
    const handoff = buildPatrolAssistantFindingHandoffFromUnifiedFinding(finding, {
      pendingApproval: pendingApprovalBriefing,
      proposedFix,
    });
    aiChatStore.open(handoff.context);
    props.onAssistantHandoff?.(finding);
  };

  // Create rule from this is the promotion path: take the operator's
  // implicit pattern (silencing the same {resource, category} pair
  // repeatedly) and turn it into a durable suppression rule the backend
  // remembers. After creation, future findings matching the rule
  // auto-dismiss inside FindingsStore.isSuppressedInternal rather than
  // re-surfacing every Patrol run. The button confirms the scope and
  // requires a reason so the rule has audit context.
  const [creatingRuleForId, setCreatingRuleForId] = createSignal<string | null>(null);
  const [createRuleDescription, setCreateRuleDescription] = createSignal('');

  const handleStartCreateRule = (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    const scope = getFindingSuppressionRuleScope(finding);
    if (!scope.canCreate) {
      notificationStore.error(
        'This finding is missing the resource or category needed for a scoped rule',
      );
      return;
    }
    setCreatingRuleForId(finding.id);
    setCreateRuleDescription(finding.userNote || '');
    setExpandedId(finding.id);
  };

  const handleCancelCreateRule = (e: Event) => {
    e.stopPropagation();
    setCreatingRuleForId(null);
  };

  const handleConfirmCreateRule = async (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    const description = createRuleDescription().trim();
    if (!description) {
      notificationStore.error('A reason for the rule is required');
      return;
    }
    const scope = getFindingSuppressionRuleScope(finding);
    if (!scope.canCreate) {
      notificationStore.error(
        'This finding is missing the resource or category needed for a scoped rule',
      );
      return;
    }
    setActionLoading(finding.id);
    try {
      await createSuppressionRuleFromFinding({
        resourceId: scope.resourceId,
        resourceName: scope.resourceName,
        category: scope.category,
        description,
      });
      notificationStore.success(
        `Rule created: future ${scope.category} findings on ${scope.resourceName} will auto-dismiss`,
      );
      setCreatingRuleForId(null);
      // Refresh so the operator sees the finding update (typically the
      // rule takes effect on next Patrol cycle, but the local view
      // should reflect the audit-trail-of-record).
      void aiIntelligenceStore.loadDashboardData();
    } catch (err) {
      console.error('Failed to create suppression rule:', err);
      notificationStore.error('Failed to create suppression rule');
    } finally {
      setActionLoading(null);
    }
  };

  // Copy a Markdown summary of the finding to the clipboard so the operator
  // can paste it into a chat, ticket, or incident channel. The shape mirrors
  // the seven-question schema (title + impact + recommendation + trust
  // signals) so a teammate seeing the finding cold has the operator-facing
  // context they need without opening Pulse. Investigation evidence and
  // rollback plans are deferred to Discuss with Assistant — they're
  // conversation context, not "share this finding" context.
  const handleCopyFindingSummary = async (finding: UnifiedFinding, e: Event) => {
    e.stopPropagation();
    const text = formatFindingForClipboard(finding);
    const ok = await copyToClipboard(text);
    if (ok) {
      notificationStore.success('Finding summary copied');
    } else {
      notificationStore.error('Could not copy finding summary');
    }
  };

  const handleOpenPlanInAssistant = (finding: UnifiedFinding, plan: RemediationPlan, e: Event) => {
    e.stopPropagation();
    const subject = getFindingSubjectPresentation(finding).label;
    const title = getFindingTitlePresentation(finding).label;
    const briefing = buildPatrolRemediationPlanAssistantBriefing({ title, subject, plan });
    const handoffContext = buildPatrolRemediationPlanAssistantModelContext({
      title,
      subject,
      plan,
    });

    aiChatStore.open({
      targetType: finding.resourceType,
      targetId: finding.resourceId,
      findingId: finding.id,
      handoffContext,
      briefing,
      autonomousMode: false,
    });
  };

  const formatTime = (isoString: string) => formatRelativeTime(isoString, { compact: true });

  // Mirror the backend's DefaultWillFixLaterRemindAfter (7 days) so the dismiss
  // confirmation panel can preview the remind-at date before the operator
  // confirms. Kept as a helper instead of a constant so the date stays current
  // each time the panel re-renders.
  const formatWillFixLaterRemindDate = (): string => {
    const remindAt = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
    try {
      return remindAt.toLocaleDateString(undefined, {
        weekday: 'short',
        month: 'short',
        day: 'numeric',
      });
    } catch {
      return remindAt.toISOString().slice(0, 10);
    }
  };

  // Get meaningful resolution reason based on finding type
  const getResolutionReason = (finding: UnifiedFinding): string => {
    const resolvedTime = finding.resolvedAt ? formatTime(finding.resolvedAt) : '';
    return getFindingResolutionReason(finding, resolvedTime);
  };

  // Render a single finding item
  const renderFindingItem = (
    finding: UnifiedFinding,
    showSourceBadge: boolean = false,
    options: { hideSubject?: boolean; relatedFindings?: readonly UnifiedFinding[] } = {},
  ) => {
    const recency = getFindingRecencyPresentation(finding);
    const subject = getFindingSubjectPresentation(finding);
    const title = getFindingTitlePresentation(finding);
    const manualControls = getFindingManualControlsPresentation(finding);
    const severityPresentation = getFindingSeverityPresentation(finding);
    const rowPrimaryAction = getFindingPrimaryActionPresentation(finding);
    const patrolWorkflow = () =>
      getFindingPatrolWorkflowPresentation(finding, aiIntelligenceStore.patrolPendingApprovals);
    const collapsedApprovalAction = () => {
      const workflow = patrolWorkflow();
      return workflow?.stage === 'approval' ? workflow : undefined;
    };
    const shouldShowLoopStateBadge = () =>
      !isPatrolFindingsSource() &&
      finding.status === 'active' &&
      finding.source !== 'ai-patrol' &&
      Boolean(finding.loopState) &&
      finding.loopState !== 'detected' &&
      !finding.investigationStatus &&
      !finding.investigationOutcome;
    const shouldShowInvestigationStatusBadge = () =>
      !isPatrolFindingsSource() &&
      Boolean(finding.investigationStatus) &&
      !(
        finding.investigationOutcome &&
        finding.investigationStatus !== 'running' &&
        finding.investigationStatus !== 'pending'
      );
    const shouldShowInvestigationOutcomeBadge = () =>
      !isPatrolFindingsSource() &&
      Boolean(finding.investigationOutcome) &&
      finding.investigationStatus !== 'running' &&
      finding.investigationStatus !== 'pending';
    const shouldShowInvestigationConfidenceBadge = () =>
      !isPatrolFindingsSource() && Boolean(finding.investigationRecord?.confidence);

    const toggleExpanded = () => {
      if (expandedId() === finding.id) {
        setExpandedId(null);
      } else {
        setExpandedId(finding.id);
      }
      props.onFindingClick?.(finding);
    };

    return (
      <div
        id={`finding-${finding.id}`}
        class={`p-3 transition-colors ${
          finding.status === 'active'
            ? finding.acknowledgedAt
              ? 'opacity-60 hover:opacity-80 bg-surface-alt'
              : 'hover:bg-surface-hover'
            : 'opacity-60 bg-surface-alt hover:opacity-80'
        }`}
      >
        {/* Finding header */}
        <div class="flex items-start justify-between gap-2">
          <div
            role="button"
            tabIndex={0}
            aria-label={`${expandedId() === finding.id ? 'Close issue details' : 'Open issue details'} for ${title.label}`}
            aria-expanded={expandedId() === finding.id}
            aria-controls={`finding-${finding.id}-details`}
            onClick={toggleExpanded}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                toggleExpanded();
              }
            }}
            class="min-w-0 flex-1 cursor-pointer rounded text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
          >
            <div class="flex items-center gap-2 flex-wrap">
              {/* Status badge for non-active findings */}
              <Show when={finding.status !== 'active'}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone={getFindingStatusBadgeTone(finding.status)}
                >
                  {getFindingStatusLabel(finding.status)}
                </MetadataBadge>
              </Show>
              {/* Source badge - only show when requested */}
              <Show when={showSourceBadge}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone={getFindingSourceBadgeTone(finding.source)}
                >
                  {getFindingSourceLabel(finding.source)}
                </MetadataBadge>
              </Show>
              {/* Severity badge */}
              <MetadataBadge
                {...FINDING_ROW_BADGE_PROPS}
                tone={severityPresentation.badgeTone}
                uppercase={severityPresentation.uppercase}
              >
                {severityPresentation.label}
              </MetadataBadge>
              {/* Alert-triggered badge */}
              <Show when={hasTriggeringAlert(finding)}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone="warning"
                  title={
                    finding.alertType
                      ? `Alert: ${finding.alertType}`
                      : `Alert Identifier: ${getFindingAlertIdentifier(finding)}`
                  }
                >
                  Alert-triggered
                </MetadataBadge>
              </Show>
              <Show when={finding.acknowledgedAt && finding.status === 'active'}>
                <MetadataBadge {...FINDING_ROW_BADGE_PROPS} tone="muted">
                  Acknowledged
                </MetadataBadge>
              </Show>
              <Show when={shouldShowLoopStateBadge()}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone={getFindingLoopStateBadgeTone(finding.loopState!)}
                  title={`Patrol status: ${formatFindingLoopState(finding.loopState!)}`}
                >
                  {formatFindingLoopState(finding.loopState!)}
                </MetadataBadge>
              </Show>
              <Show when={isOutOfScope(finding)}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone="warning"
                  title="This finding references a resource outside the selected run scope."
                >
                  Out of scope
                </MetadataBadge>
              </Show>
              {/* Non-Patrol investigation status badge — only when no outcome badge will show. */}
              <Show when={shouldShowInvestigationStatusBadge()}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone={getInvestigationStatusBadgeTone(finding.investigationStatus!)}
                  title={`Investigation: ${getInvestigationStatusLabel(finding.investigationStatus!)}`}
                >
                  <Show
                    when={
                      finding.investigationStatus === 'running' ||
                      finding.investigationStatus === 'pending'
                    }
                  >
                    <Show
                      when={finding.investigationStatus === 'running'}
                      fallback={<span class="h-2 w-2 rounded-full bg-current animate-pulse" />}
                    >
                      <LoadingSpinner size="xs" />
                    </Show>
                  </Show>
                  {getInvestigationStatusLabel(finding.investigationStatus!)}
                </MetadataBadge>
              </Show>
              {/* Non-Patrol investigation outcome badge — replaces status badge when known. */}
              <Show when={shouldShowInvestigationOutcomeBadge()}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone={getInvestigationOutcomeBadgeTone(finding.investigationOutcome!)}
                >
                  {getInvestigationOutcomeLabel(finding.investigationOutcome!)}
                </MetadataBadge>
              </Show>
              {/* Non-Patrol confidence badge. Patrol keeps investigation detail
                  in the expanded issue so the default row stays simple. */}
              <Show when={shouldShowInvestigationConfidenceBadge()}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone={getInvestigationConfidenceBadgeTone(
                    finding.investigationRecord!.confidence!,
                  )}
                  title={`Investigation confidence: ${finding.investigationRecord!.confidence!}`}
                >
                  {finding.investigationRecord!.confidence!} confidence
                </MetadataBadge>
              </Show>
              {/* Regression pill — Pulse's "Learn" signal on the collapsed row.
                  A finding that has regressed before is not a one-off; it
                  needs to be triaged differently from a fresh detection.
                  Sits next to the confidence badge so trust + recurrence
                  can be scanned together without expanding the card. */}
              <Show when={(finding.regressionCount || 0) > 0}>
                <MetadataBadge
                  {...FINDING_ROW_BADGE_PROPS}
                  tone="warning"
                  title={`This finding has regressed ${finding.regressionCount} time${finding.regressionCount === 1 ? '' : 's'} after being resolved before.`}
                >
                  regressed {finding.regressionCount}×
                </MetadataBadge>
              </Show>
              {/* Title */}
              <span
                class={`font-medium text-sm truncate ${
                  finding.status === 'active' ? 'text-base-content' : 'text-muted'
                }`}
              >
                {title.label}
              </span>
            </div>
            {/* Resource info */}
            <div class="text-xs text-muted mt-1">
              <Show
                when={options.hideSubject}
                fallback={
                  <>
                    {subject.label} - {recency.label} {formatTime(recency.timestamp)}
                  </>
                }
              >
                {recency.label} {formatTime(recency.timestamp)}
              </Show>
              <Show when={finding.status === 'resolved' && finding.resolvedAt}>
                <span class="ml-2 text-green-600 dark:text-green-400">
                  {' · '}
                  {getResolutionReason(finding)}
                </span>
              </Show>
              <Show when={finding.dismissedReason}>
                <span class="ml-2 text-muted">
                  {' · '}({formatIdentifierLabel(finding.dismissedReason)})
                </span>
              </Show>
              {/* Distinguish Pulse-auto-suppressed dismissals from operator
                  decisions — both serialize as DismissedReason=
                  'expected_behavior' on the wire, but the auto-dismiss
                  carries `operator_state_cause` lifecycle metadata. The
                  badge attributes the suppression so the operator sees
                  "Pulse stayed quiet because of my maintenance window"
                  vs "I decided this was expected." */}
              <Show
                when={(() => {
                  const cause = getOperatorStateDismissCause(finding);
                  return cause && formatOperatorStateDismissCauseLabel(cause);
                })()}
              >
                <span
                  class="ml-2 text-blue-600 dark:text-blue-400"
                  title="Auto-suppressed by Pulse based on operator-set state for this resource."
                >
                  {' · '}auto:{' '}
                  {formatOperatorStateDismissCauseLabel(getOperatorStateDismissCause(finding))}
                </span>
              </Show>
              <Show when={finding.dismissedReason === 'will_fix_later' && finding.remindAt}>
                <span
                  class="ml-2 text-amber-600 dark:text-amber-400"
                  title="Pulse will surface this finding again on this date if it is still tripping."
                >
                  {' · '}Reminding {formatTime(finding.remindAt!)}
                </span>
              </Show>
              <Show when={finding.status === 'snoozed' && finding.snoozedUntil}>
                <span class="ml-2 text-blue-500 dark:text-blue-400">
                  {' · '}snoozed until {formatTime(finding.snoozedUntil!)}
                </span>
              </Show>
              <Show when={finding.acknowledgedAt && finding.status === 'active'}>
                <span class="ml-2 text-muted">
                  {' · '}acknowledged {formatTime(finding.acknowledgedAt!)}
                </span>
              </Show>
              <Show when={finding.status === 'active' && finding.lastInvestigatedAt}>
                <span class="ml-2 text-muted">
                  {' · '}last investigated {formatTime(finding.lastInvestigatedAt!)}
                </span>
              </Show>
            </div>
          </div>
          {/* Actions */}
          <div class="flex items-center gap-1 shrink-0">
            <Show when={expandedId() !== finding.id ? collapsedApprovalAction() : undefined}>
              {(action) => (
                <button
                  type="button"
                  onClick={toggleExpanded}
                  class="rounded bg-amber-600 px-2 py-1 text-xs font-semibold text-white transition-colors hover:bg-amber-700"
                  title={action().detail}
                >
                  {action().label}
                </button>
              )}
            </Show>
            <Show when={shouldShowCollapsedPrimaryAction(finding) && rowPrimaryAction}>
              {(action) => (
                <a
                  href={action().href}
                  onClick={(e) => e.stopPropagation()}
                  class="inline-flex items-center rounded bg-blue-600 px-2 py-1 text-xs font-semibold text-white transition-colors hover:bg-blue-700"
                >
                  {action().label}
                </a>
              )}
            </Show>
            <Show when={shouldShowInlineManualControls(finding)}>
              <Show when={manualControls.acknowledge && !finding.acknowledgedAt}>
                <button
                  type="button"
                  onClick={(e) => handleAcknowledge(finding, e)}
                  class="p-1 text-slate-400 hover:text-muted"
                  title="Acknowledge"
                  disabled={actionLoading() === finding.id}
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4"
                    />
                  </svg>
                </button>
              </Show>
              <Show when={manualControls.snooze}>
                <button
                  type="button"
                  onClick={(e) => handleSnooze(finding, 24, e)}
                  class="p-1 text-slate-400 hover:text-muted"
                  title="Snooze 24h"
                  disabled={actionLoading() === finding.id}
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </button>
              </Show>
              <Show when={manualControls.dismiss}>
                <button
                  type="button"
                  onClick={(e) => handleStartDismiss(finding, 'will_fix_later', e)}
                  class="p-1 text-slate-400 hover:text-muted"
                  title="Dismiss"
                  disabled={actionLoading() === finding.id}
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </Show>
            </Show>
            <button
              type="button"
              aria-label={`${expandedId() === finding.id ? 'Hide' : 'View'} details for ${title.label}`}
              aria-expanded={expandedId() === finding.id}
              aria-controls={`finding-${finding.id}-details`}
              onClick={toggleExpanded}
              class="inline-flex items-center gap-1 rounded border border-border bg-surface px-2 py-1 text-xs font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
              title={expandedId() === finding.id ? 'Hide details' : 'View details'}
            >
              <span>{expandedId() === finding.id ? 'Hide' : 'Details'}</span>
              <ChevronDownIcon
                class={`h-3.5 w-3.5 transition-transform ${expandedId() === finding.id ? 'rotate-180' : ''}`}
              />
            </button>
          </div>
        </div>

        {/* Expanded content */}
        <Show when={expandedId() === finding.id}>{renderExpandedContent(finding, options)}</Show>
      </div>
    );
  };

  // Render expanded content for a finding
  const renderExpandedContent = (
    finding: UnifiedFinding,
    options: { relatedFindings?: readonly UnifiedFinding[] } = {},
  ) => {
    const primaryAction = getFindingPrimaryActionPresentation(finding);
    const manualControls = getFindingManualControlsPresentation(finding);
    const relatedFindings = options.relatedFindings ?? [];
    const shouldShowAssistantPrimaryAction = !isPatrolFindingsSource();
    const shouldShowAssistantManageAction = isPatrolFindingsSource() && !primaryAction;
    const proHandoff = !primaryAction ? props.patrolProHandoff?.(finding) : undefined;
    const shouldShowExpandedManageMenu =
      !primaryAction ||
      manualControls.acknowledge ||
      manualControls.snooze ||
      manualControls.dismiss;

    return (
      <div id={`finding-${finding.id}-details`} class="mt-3 pt-3 border-t border-border-subtle">
        <Show when={hasTriggeringAlert(finding)}>
          <div class="text-xs text-amber-700 dark:text-amber-300 mb-2">
            Triggered by alert{finding.alertType ? ` (${finding.alertType})` : ''} • Identifier{' '}
            {getFindingAlertIdentifier(finding)}
          </div>
        </Show>
        <p class="text-sm text-muted">{finding.description}</p>
        <Show when={finding.impact}>
          <p class="text-sm text-base-content mt-2">
            <span class="font-medium">Impact:</span> {finding.impact}
          </p>
        </Show>
        <Show when={finding.previousResolvedFixSummary}>
          <p class="text-sm text-base-content mt-2 px-2 py-1 rounded border border-emerald-200 bg-emerald-50/40 dark:border-emerald-800 dark:bg-emerald-950/30">
            <span class="font-medium text-emerald-800 dark:text-emerald-300">
              Last time this resolved:
            </span>{' '}
            {finding.previousResolvedFixSummary}
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

        <Show when={relatedFindings.length > 0}>
          <div class="mt-3 rounded border border-border bg-surface-alt p-2">
            <div class="text-xs font-medium text-base-content">Related issues</div>
            <div class="mt-2 space-y-1">
              <For each={relatedFindings}>
                {(relatedFinding) => {
                  const relatedSeverity = getFindingSeverityPresentation(relatedFinding);
                  const relatedTitle = getFindingTitlePresentation(relatedFinding);
                  const relatedRecency = getFindingRecencyPresentation(relatedFinding);
                  return (
                    <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
                      <MetadataBadge
                        {...FINDING_ROW_BADGE_PROPS}
                        tone={relatedSeverity.badgeTone}
                        uppercase={relatedSeverity.uppercase}
                      >
                        {relatedSeverity.label}
                      </MetadataBadge>
                      <span class="font-medium text-base-content">{relatedTitle.label}</span>
                      <span>
                        {relatedRecency.label} {formatTime(relatedRecency.timestamp)}
                      </span>
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>

        <Show when={shouldShowFindingLifecycle(finding)}>
          <div class="mt-3 p-2 rounded border border-border bg-surface-alt">
            <div class="text-xs font-medium text-base-content mb-2">Lifecycle</div>
            <div class="space-y-1">
              <For each={[...(finding.lifecycle || [])].slice(-6).reverse()}>
                {(event) => {
                  const typeLabel = formatFindingLifecycleType(event.type);
                  // Some historical events have a message that just restates
                  // the type label ("Detected" / "Detected by Pulse Patrol").
                  // Drop the message in that case so the row reads cleanly.
                  const showMessage = () => {
                    const msg = event.message?.trim();
                    if (!msg) return false;
                    return !msg.toLowerCase().startsWith(typeLabel.toLowerCase());
                  };
                  // A from->to span where from === to is a no-op transition
                  // (a heartbeat that pre-dates the lifecycle dedupe fix).
                  // Hide it; only render real transitions.
                  const showTransition = () =>
                    Boolean(event.from) && Boolean(event.to) && event.from !== event.to;
                  return (
                    <div class="text-xs text-muted flex items-start justify-between gap-2">
                      <span class="truncate">
                        <span class="font-medium text-base-content">{typeLabel}</span>
                        <Show when={showMessage()}>
                          {' '}
                          <span>{event.message}</span>
                        </Show>
                        <Show when={showTransition()}>
                          {' '}
                          <span class="text-muted">
                            ({event.from} {'->'} {event.to})
                          </span>
                        </Show>
                      </span>
                      <span class="shrink-0">{formatRelativeTime(event.at)}</span>
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>

        {/* User note display / editor */}
        <Show when={editingNoteId() === finding.id}>
          <div
            class="mt-3 p-2 rounded border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900"
            onClick={(e) => e.stopPropagation()}
          >
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
            <svg
              class="w-4 h-4 text-muted mt-0.5 flex-shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z"
              />
            </svg>
            <p class="text-sm text-muted flex-1">{finding.userNote}</p>
            <button
              type="button"
              onClick={(e) => handleStartEditNote(finding, e)}
              class="p-1 hover:text-base-content flex-shrink-0"
              title="Edit note"
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
                />
              </svg>
            </button>
          </div>
        </Show>

        <div
          class="mt-3 flex flex-wrap items-start gap-2 text-xs"
          onClick={(e) => e.stopPropagation()}
        >
          <Show when={primaryAction}>
            {(action) => (
              <a
                href={action().href}
                onClick={(e) => e.stopPropagation()}
                class="inline-flex items-center gap-1 rounded bg-blue-600 px-3 py-1.5 font-semibold text-white transition-colors hover:bg-blue-700"
              >
                {action().label}
              </a>
            )}
          </Show>
          <Show when={!primaryAction && shouldShowAssistantPrimaryAction}>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                void openFindingInAssistant(finding);
              }}
              class="inline-flex items-center gap-1 rounded bg-blue-600 px-3 py-1.5 font-semibold text-white transition-colors hover:bg-blue-700"
              title={getPrimaryAssistantFindingAction(finding).title}
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
              {getPrimaryAssistantFindingAction(finding).label}
            </button>
          </Show>
          <Show when={proHandoff}>
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-muted">{proHandoff!.detail}</span>
              <Show when={proHandoff!.destination} keyed>
                {(destination) => (
                  <UpgradeButtonLink
                    destination={destination}
                    size="sm"
                    mobileFullWidth={false}
                  >
                    {proHandoff!.actionLabel}
                  </UpgradeButtonLink>
                )}
              </Show>
            </div>
          </Show>

          <Show when={shouldShowExpandedManageMenu}>
            <details onClick={(e) => e.stopPropagation()}>
              <summary class="list-none cursor-pointer rounded border border-border bg-surface px-3 py-1.5 font-medium text-base-content hover:bg-surface-hover">
                Manage
              </summary>
              <div class="mt-1 flex min-w-48 flex-col gap-1 rounded border border-border bg-surface p-1 shadow-sm">
                <Show when={shouldShowAssistantManageAction}>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      void openFindingInAssistant(finding);
                    }}
                    class="rounded px-2 py-1 text-left hover:bg-surface-hover"
                    title={getPrimaryAssistantFindingAction(finding).title}
                  >
                    {getPrimaryAssistantFindingAction(finding).label}
                  </button>
                </Show>
                <Show when={editingNoteId() !== finding.id && !finding.userNote}>
                  <button
                    type="button"
                    onClick={(e) => handleStartEditNote(finding, e)}
                    class="rounded px-2 py-1 text-left hover:bg-surface-hover"
                  >
                    Add Note
                  </button>
                </Show>
                <button
                  type="button"
                  onClick={(e) => handleCopyFindingSummary(finding, e)}
                  class="rounded px-2 py-1 text-left hover:bg-surface-hover"
                >
                  Copy summary
                </button>
                <Show when={finding.status === 'active'}>
                  <Show when={manualControls.acknowledge && !finding.acknowledgedAt}>
                    <button
                      type="button"
                      onClick={(e) => handleAcknowledge(finding, e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={actionLoading() === finding.id}
                    >
                      Acknowledge
                    </button>
                  </Show>
                  <Show when={manualControls.acknowledge}>
                    <button
                      type="button"
                      onClick={(e) => handleResolve(finding, e)}
                      class="rounded px-2 py-1 text-left text-emerald-700 hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-300 dark:hover:bg-emerald-900"
                      disabled={actionLoading() === finding.id}
                    >
                      Mark resolved
                    </button>
                  </Show>
                  <Show when={manualControls.snooze}>
                    <button
                      type="button"
                      onClick={(e) => handleSnooze(finding, 1, e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={actionLoading() === finding.id}
                    >
                      Snooze 1h
                    </button>
                    <button
                      type="button"
                      onClick={(e) => handleSnooze(finding, 24, e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={actionLoading() === finding.id}
                    >
                      Snooze 24h
                    </button>
                    <button
                      type="button"
                      onClick={(e) => handleSnooze(finding, 168, e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={actionLoading() === finding.id}
                    >
                      Snooze 7d
                    </button>
                  </Show>
                  <Show when={manualControls.dismiss}>
                    <button
                      type="button"
                      onClick={(e) => handleStartDismiss(finding, 'not_an_issue', e)}
                      class="rounded px-2 py-1 text-left text-red-600 hover:bg-red-50 disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900"
                      disabled={actionLoading() === finding.id}
                    >
                      Dismiss: Not an issue
                    </button>
                    <button
                      type="button"
                      onClick={(e) => handleStartDismiss(finding, 'expected_behavior', e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={actionLoading() === finding.id}
                    >
                      Remember as expected
                    </button>
                    <button
                      type="button"
                      onClick={(e) => handleStartDismiss(finding, 'will_fix_later', e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={actionLoading() === finding.id}
                    >
                      Dismiss: Later
                    </button>
                    <button
                      type="button"
                      onClick={(e) => handleStartCreateRule(finding, e)}
                      class="rounded px-2 py-1 text-left hover:bg-surface-hover disabled:opacity-50"
                      disabled={
                        actionLoading() === finding.id ||
                        !getFindingSuppressionRuleScope(finding).canCreate
                      }
                      title={
                        getFindingSuppressionRuleScope(finding).canCreate
                          ? 'Promote this dismissal into a permanent rule for this resource and category'
                          : 'This finding is missing the resource or category needed for a scoped rule'
                      }
                    >
                      Create rule from this
                    </button>
                  </Show>
                </Show>
              </div>
            </details>
          </Show>
        </div>
        {/* Inline create-rule confirmation. Confirms scope (resource +
            category) and requires a reason so the persisted rule has
            audit context. Mirrors the dismiss-confirmation panel
            visually but uses neutral surface styling — this isn't a
            dismissal, it's a permanent commitment. */}
        <Show when={creatingRuleForId() === finding.id}>
          <div class="mt-2 p-2 rounded border border-border bg-surface-alt">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="text-xs font-medium text-base-content">
                Create suppression rule for{' '}
                <span class="font-semibold">
                  {getFindingSuppressionRuleScope(finding).resourceName}
                </span>{' '}
                ({getFindingSuppressionRuleScope(finding).category})
              </span>
            </div>
            <p class="text-[11px] text-muted mb-1.5">
              Future findings matching this resource and category will be auto-dismissed by Patrol
              without surfacing as new findings. You can list or remove rules later from the
              suppressions management surface.
            </p>
            <textarea
              class="w-full text-xs px-2 py-1.5 rounded border border-border bg-surface text-base-content resize-none focus:outline-none focus:ring-1 focus:ring-blue-400"
              rows={2}
              value={createRuleDescription()}
              onInput={(e) => setCreateRuleDescription(e.currentTarget.value)}
              placeholder="Why this rule? (required — e.g. 'delly backups are intentionally off-site, ignore failures')"
              onClick={(e) => e.stopPropagation()}
            />
            <div class="flex gap-2 mt-1.5">
              <button
                type="button"
                onClick={(e) => handleConfirmCreateRule(finding, e)}
                class="px-3 py-1 text-xs font-medium rounded bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-50"
                disabled={actionLoading() === finding.id || !createRuleDescription().trim()}
              >
                Create rule
              </button>
              <button
                type="button"
                onClick={handleCancelCreateRule}
                class="px-3 py-1 text-xs font-medium rounded border border-border hover:bg-surface-hover"
              >
                Cancel
              </button>
            </div>
          </div>
        </Show>
        {/* Inline dismiss confirmation. The header verb tracks the
            intent: "Remembering as" for expected_behavior (future-looking,
            "Pulse should know this is expected"), "Dismiss as" for the
            other reasons (past-looking, "make this go away"). */}
        <Show when={dismissingId() === finding.id}>
          <div class="mt-2 p-2 rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="text-xs font-medium text-red-700 dark:text-red-300">
                {dismissReason() === 'expected_behavior'
                  ? 'Remembering as expected'
                  : `Dismiss as: ${formatIdentifierLabel(dismissReason())}`}
              </span>
            </div>
            <Show when={dismissReason() === 'will_fix_later'}>
              <p class="text-[11px] text-amber-700 dark:text-amber-300 mb-1.5">
                Pulse will stay quiet on this for 7 days, then surface it again on{' '}
                <span class="font-semibold">{formatWillFixLaterRemindDate()}</span> if it is still
                happening.
              </p>
            </Show>
            <Show when={dismissReason() === 'expected_behavior'}>
              <p class="text-[11px] text-muted mb-1.5">
                Pulse will keep this finding visible as acknowledged and remember that this state is
                expected on this resource. It won't re-notify you for it, but severity escalation
                will still wake it.
              </p>
            </Show>
            <Show when={dismissReason() === 'not_an_issue'}>
              <p class="text-[11px] text-muted mb-1.5">
                Pulse will permanently suppress this and similar findings on this resource. Use
                "Expected" or "Later" if the detection itself is correct.
              </p>
            </Show>
            {/* Recurrence hint: if a finding has regressed multiple times, the
                operator may be silently dismissing something that keeps coming
                back. Surface that fact for not_an_issue and expected_behavior
                (the two "stay quiet forever" paths) so they can reconsider
                'will_fix_later' as a reminder-bearing alternative. */}
            <Show
              when={
                (finding.regressionCount || 0) > 1 &&
                (dismissReason() === 'not_an_issue' || dismissReason() === 'expected_behavior')
              }
            >
              <p class="text-[11px] text-amber-700 dark:text-amber-300 mb-1.5 italic">
                Heads up: this finding has regressed {finding.regressionCount} times before. If you
                intend to fix it eventually, "Later" sets a 7-day reminder instead of going silent
                permanently.
              </p>
            </Show>
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
        <Show when={hasFindingInvestigationDetails(finding)}>
          <InvestigationSection
            findingId={finding.id}
            investigationStatus={finding.investigationStatus}
            investigationOutcome={finding.investigationOutcome}
            investigationAttempts={finding.investigationAttempts}
            investigationRecord={finding.investigationRecord}
          />
        </Show>

        {/* Inline Approval Section (replaces manual approval JSX) */}
        <Show
          when={
            finding.status === 'active' &&
            (finding.investigationOutcome === 'fix_queued' ||
              finding.investigationOutcome === 'fix_executed' ||
              finding.investigationOutcome === 'fix_failed' ||
              finding.investigationOutcome === 'fix_rejected' ||
              finding.investigationOutcome === 'fix_verified' ||
              finding.investigationOutcome === 'fix_verification_failed' ||
              finding.investigationOutcome === 'fix_verification_unknown')
          }
        >
          <ApprovalSection
            findingId={finding.id}
            investigationOutcome={finding.investigationOutcome}
            findingTitle={getFindingTitlePresentation(finding).label}
            resourceName={finding.resourceName}
            resourceType={finding.resourceType}
            resourceId={finding.resourceId}
          />
        </Show>

        {/* Existing model-owned action artifact: compact Assistant review entry. */}
        <Show when={finding.status === 'active' && plansByFindingId().get(finding.id)}>
          {(plan) => {
            return (
              <div class="mt-3 pt-3 border-t border-border-subtle">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div class="flex min-w-0 items-center gap-2">
                    <AlertCircleIcon class="h-4 w-4 flex-shrink-0 text-muted" />
                    <span class="text-sm font-medium text-base-content">Assistant context</span>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={(e) => handleOpenPlanInAssistant(finding, plan(), e)}
                      class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
                    >
                      Ask Assistant
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
              </div>
            );
          }}
        </Show>
      </div>
    );
  };

  return (
    <div class="space-y-4">
      {/* Controls */}
      <Show when={showFilterControls()}>
        <div class="flex items-center justify-between">
          <div class="flex text-xs">
            <For each={filterOptions()}>
              {(option, index) => {
                const isFirst = () => index() === 0;
                const isLast = () => index() === filterOptions().length - 1 && overdueCount() === 0;
                return (
                  <button
                    type="button"
                    onClick={() => setFilter(option.value)}
                    class={`px-2 py-1 border ${
                      isFirst() ? 'rounded-l border-r-0' : isLast() ? 'rounded-r' : 'border-r-0'
                    } ${segmentedButtonClass(filter() === option.value, false, option.tone ?? 'default')}`}
                  >
                    {option.label}
                    <Show when={typeof option.count === 'number'}>{` (${option.count})`}</Show>
                  </button>
                );
              }}
            </For>
            <Show when={overdueCount() > 0}>
              <button
                type="button"
                data-testid="findings-panel-filter-overdue"
                onClick={() => setFilter('overdue')}
                class={`px-2 py-1 border rounded-r ${segmentedButtonClass(filter() === 'overdue', false, 'warning')}`}
                title="Will-fix-later commitments past their remind deadline"
              >
                Overdue commitments ({overdueCount()})
              </button>
            </Show>
          </div>
          <Show when={patrolFindingDisplayGroups().length > 1}>
            <FormSelect
              label="Sort findings"
              labelClass="sr-only"
              fieldBaseClass="contents"
              value={sortBy()}
              onChange={(e) => setSortBy(e.currentTarget.value as 'severity' | 'time')}
              selectBaseClass="text-xs px-2 py-1 rounded border border-border bg-surface"
            >
              <option value="severity">By Severity</option>
              <option value="time">By Time</option>
            </FormSelect>
          </Show>
        </div>
      </Show>

      {/* Loading/Error states */}
      <Show when={shouldShowLoadingState()}>
        <div class="p-4 text-sm text-muted flex items-center gap-2">
          <LoadingSpinner size="md" />
          Loading findings...
        </div>
      </Show>

      <Show when={sourceFindingsError() && !sourceFindingsLoading()}>
        <div class="p-4 text-sm text-red-600 dark:text-red-400">{sourceFindingsError()}</div>
      </Show>

      <Show when={!shouldShowLoadingState()}>
        <Card padding="none" class="overflow-hidden">
          {/* Content */}
          <div class="divide-y divide-border-subtle">
            <Show when={patrolFindingDisplayGroups().length === 0}>
              <div class="p-6 text-sm text-muted text-center">
                <Show when={shouldShowRichEmptyState()}>
                  <div class="flex flex-col items-center gap-3">
                    <Show
                      when={emptyStateCopy().tone === 'success'}
                      fallback={
                        <Show
                          when={emptyStateCopy().tone === 'error'}
                          fallback={
                            <Show
                              when={emptyStateCopy().tone === 'warning'}
                              fallback={
                                <InfoIcon class={`w-10 h-10 ${emptyStateTone().iconClass}`} />
                              }
                            >
                              <AlertTriangleIcon
                                class={`w-10 h-10 ${emptyStateTone().iconClass}`}
                              />
                            </Show>
                          }
                        >
                          <AlertTriangleIcon class={`w-10 h-10 ${emptyStateTone().iconClass}`} />
                        </Show>
                      }
                    >
                      <CheckCircleIcon class={`w-10 h-10 ${emptyStateTone().iconClass}`} />
                    </Show>
                    <div>
                      <p class="font-medium text-base-content">{emptyStateCopy().title}</p>
                      <Show when={emptyStateCopy().body}>
                        <p class="text-xs mt-1">{emptyStateCopy().body}</p>
                      </Show>
                    </div>
                  </div>
                </Show>
                <Show when={filter() === 'attention'}>{emptyStateCopy().title}</Show>
                <Show when={filter() === 'approvals'}>{emptyStateCopy().title}</Show>
                <Show when={filter() === 'overdue'}>No overdue will-fix-later commitments.</Show>
                <Show
                  when={
                    filter() !== 'active' &&
                    filter() !== 'attention' &&
                    filter() !== 'approvals' &&
                    filter() !== 'overdue' &&
                    props.runSnapshot === undefined
                  }
                >
                  {emptyStateCopy().title}
                </Show>
              </div>
            </Show>
            <For each={patrolFindingDisplayGroups()}>
              {(group) => (
                <Show
                  when={group.findings.length > 1}
                  fallback={renderFindingItem(group.primaryFinding, false)}
                >
                  <section
                    class="divide-y divide-border-subtle"
                    aria-label={`${group.resourceLabel}: ${getPatrolFindingIssueCountLabel(group.findings.length)}`}
                  >
                    <div class="flex flex-wrap items-center justify-between gap-2 bg-surface-alt/60 px-3 py-2 text-xs">
                      <span class="font-semibold text-base-content">{group.resourceLabel}</span>
                      <span class="text-muted">
                        {getPatrolFindingIssueCountLabel(group.findings.length)}
                      </span>
                    </div>
                    {renderFindingItem(group.primaryFinding, false, {
                      hideSubject: true,
                      relatedFindings: group.relatedFindings,
                    })}
                  </section>
                </Show>
              )}
            </For>
          </div>
        </Card>
      </Show>
    </div>
  );
};

export default FindingsPanel;
