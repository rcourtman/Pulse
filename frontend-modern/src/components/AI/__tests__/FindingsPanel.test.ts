import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  buildPatrolFindingDisplayGroups,
  buildFindingFilterOptions,
  formatFindingForClipboard,
  formatFindingLifecycleType,
  formatOperatorStateDismissCauseLabel,
  getOperatorStateDismissCause,
  formatFindingLoopState,
  getFindingActiveRuntimeSortOrder,
  getFindingEmptyStateCopy,
  getFindingManualControlsPresentation,
  getFindingPrimaryActionPresentation,
  getFindingSubjectPresentation,
  getFindingTitlePresentation,
  getFindingPatrolWorkflowPresentation,
  getPatrolFindingIssueCountLabel,
  getPatrolFindingClassification,
  getFindingSeverityCompactLabel,
  getFindingSeveritySortOrder,
  getFindingResolutionReason,
  getFindingRecencyPresentation,
  getFindingLoopStateBadgeClasses,
  getFindingLoopStateBadgeTone,
  getFindingSeverityBadgeClasses,
  getFindingSeverityPresentation,
  getFindingStatusBadgeClasses,
  getFindingStatusBadgeTone,
  getFindingStatusLabel,
  getFindingSeverityToneClasses,
  getPatrolFindingsBadgePresentation,
  getFindingSourceBadgeClasses,
  getFindingSourceBadgeTone,
  getFindingSourceLabel,
  hasFindingInvestigationDetails,
  hasPendingInvestigationFixApproval,
  isPatrolInvestigationFixApproval,
  getInvestigationOutcomeBadgeClasses,
  getInvestigationOutcomeBadgeTone,
  getInvestigationOutcomeLabel,
  getInvestigationOutcomeSortOrder,
  getInvestigationStatusLabel,
  getInvestigationStatusBadgeClasses,
  getInvestigationStatusBadgeTone,
  getInvestigationConfidenceBadgeTone,
  doesFindingNeedAttention,
} from '@/utils/aiFindingPresentation';
import { PATROL_PROVIDER_SETTINGS_ACTION } from '@/utils/patrolRuntimeActions';

const findingsPanelSource = readFileSync(resolve(__dirname, '..', 'FindingsPanel.tsx'), 'utf-8');
const patrolWorkspaceSource = readFileSync(
  resolve(__dirname, '..', '..', '..', 'features', 'patrol', 'PatrolIntelligenceWorkspace.tsx'),
  'utf-8',
);
const patrolInvestigationContextModelSource = readFileSync(
  resolve(__dirname, '..', '..', '..', 'features', 'patrol', 'patrolInvestigationContextModel.ts'),
  'utf-8',
);
const aiFindingPresentationSource = readFileSync(
  resolve(__dirname, '..', '..', '..', 'utils', 'aiFindingPresentation.ts'),
  'utf-8',
);

describe('FindingsPanel assistant handoff', () => {
  it('routes finding loading indicators through the shared LoadingSpinner primitive', () => {
    expect(findingsPanelSource).toContain("from '@/components/shared/LoadingSpinner'");
    expect(findingsPanelSource).toContain('<LoadingSpinner size="xs" />');
    expect(findingsPanelSource).toContain('<LoadingSpinner size="md" />');
  });

  it('renders the plan-locked at-need Pulse Pro handoff in the finding primary-action area', () => {
    expect(findingsPanelSource).toContain('patrolProHandoff?:');
    expect(findingsPanelSource).toContain('{proHandoff!.detail}');
    expect(findingsPanelSource).toContain('UpgradeButtonLink');
    expect(patrolWorkspaceSource).toContain('getPatrolProInvestigationHandoff');
    expect(patrolWorkspaceSource).toContain('patrolProHandoff=');
  });

  it('routes Patrol investigation records into the Assistant briefing context', () => {
    expect(findingsPanelSource).toContain('buildPatrolAssistantFindingHandoffFromUnifiedFinding');
    expect(findingsPanelSource).toContain('buildPatrolAssistantApprovalBriefingInput');
    expect(findingsPanelSource).toContain(
      'buildPatrolAssistantProposedFixBriefingInputFromApproval',
    );
    expect(findingsPanelSource).toContain('aiChatStore.open(handoff.context)');
    expect(findingsPanelSource).not.toContain('autoSendInitialPrompt');
    expect(findingsPanelSource).not.toContain('openWithPrompt');
    expect(patrolInvestigationContextModelSource).toContain(
      'investigationOutcome: finding.investigationOutcome',
    );
    expect(patrolInvestigationContextModelSource).toContain(
      'investigationStatus: finding.investigationStatus',
    );
    expect(patrolInvestigationContextModelSource).toContain(
      'remediationId: finding.remediationPlanId',
    );
    expect(patrolInvestigationContextModelSource).toContain('resourceId: finding.resourceId');
    expect(patrolInvestigationContextModelSource).toContain('detectedAt: finding.detectedAt');
    expect(patrolInvestigationContextModelSource).toContain('lastSeenAt: finding.lastSeenAt');
    expect(patrolInvestigationContextModelSource).toContain(
      'investigationRecord: finding.investigationRecord',
    );
    expect(findingsPanelSource).toContain('pendingApproval: pendingApprovalBriefing');
    expect(findingsPanelSource).toContain('proposedFix,');
    expect(findingsPanelSource).not.toContain('nextStepAction,');
    expect(findingsPanelSource).toContain('AIAPI.getInvestigation(finding.id)');
    expect(findingsPanelSource).toContain('await aiIntelligenceStore.loadPendingApprovals()');
  });

  it('routes remediation plan handoffs through the command-free Patrol handoff model', () => {
    expect(findingsPanelSource).not.toContain('buildPatrolRemediationPlanAssistantPrompt');
    expect(findingsPanelSource).toContain('buildPatrolRemediationPlanAssistantModelContext');
    expect(findingsPanelSource).toContain('buildPatrolRemediationPlanAssistantBriefing');
    expect(findingsPanelSource).toContain('handoffContext,');
    expect(findingsPanelSource).toContain('autonomousMode: false');
    expect(findingsPanelSource).not.toContain('Command: `');
    expect(findingsPanelSource).not.toContain('Rollback: `');
  });

  it('renders previousResolvedFixSummary as operational memory in the expanded card', () => {
    // The previous-fix summary is captured at regression time and surfaced
    // in chat context for Assistant. The expanded card must also show it
    // directly so operators see "what worked last time" without opening
    // Assistant. The styling is intentionally distinct (emerald accent) so
    // it reads as a positive memory cue rather than another alert.
    expect(findingsPanelSource).toContain('finding.previousResolvedFixSummary');
    expect(findingsPanelSource).toContain('Last time this resolved');
  });

  it('does not expose intent-specific Assistant routing from finding actions', () => {
    expect(findingsPanelSource).toContain('Open in Assistant');
    expect(findingsPanelSource).not.toContain('handleExplainFinding');
    expect(findingsPanelSource).not.toContain('handleInvestigateFinding');
    expect(findingsPanelSource).not.toContain('handleWhyFinding');
    expect(findingsPanelSource).not.toContain('handleVerifyFixFinding');
    expect(findingsPanelSource).not.toContain("openFindingInAssistant(finding, '");
  });

  it('surfaces investigation_record.confidence as a badge in the collapsed row', () => {
    // The seven-question schema's confidence answer should be visible to
    // operators without expanding the card. The badge sits next to the
    // investigation outcome badge so trust can be scanned in the row.
    expect(findingsPanelSource).toContain('finding.investigationRecord?.confidence');
    expect(findingsPanelSource).toContain('getInvestigationConfidenceBadgeTone');
    expect(findingsPanelSource).toContain('confidence');
  });

  it('keeps collapsed finding disclosure separate from inline manual controls', () => {
    // The collapsed finding row contains acknowledge/snooze/dismiss controls.
    // Those controls must remain siblings of the disclosure area, while the
    // row body itself opens the same details panel for normal scanning.
    expect(findingsPanelSource).toContain('role="button"');
    expect(findingsPanelSource).toContain('tabIndex={0}');
    expect(findingsPanelSource).toContain('class="min-w-0 flex-1 cursor-pointer rounded text-left');
    expect(findingsPanelSource).toContain('aria-expanded={expandedId() === finding.id}');
    expect(findingsPanelSource).toContain(
      "aria-label={`${expandedId() === finding.id ? 'Close issue details' : 'Open issue details'} for ${title.label}`}",
    );
    expect(findingsPanelSource).toContain(
      "aria-label={`${expandedId() === finding.id ? 'Hide' : 'View'} details for ${title.label}`}",
    );
    expect(findingsPanelSource).toContain("e.key === 'Enter' || e.key === ' '");
    expect(findingsPanelSource).toContain('onClick={toggleExpanded}');
  });

  it('previews the will_fix_later remind-at deadline before the operator confirms dismiss', () => {
    // The backend now treats will_fix_later as a real operational commitment
    // (Finding.RemindAt, default 7 days), not a silent shut-up. The dismiss
    // confirmation panel must communicate that to the operator BEFORE they
    // confirm — otherwise the new behavior is invisible until the reminder
    // fires a week later. See ai-runtime / patrol-intelligence subsystem
    // contracts for the three-way dismissal semantics.
    expect(findingsPanelSource).toContain('formatWillFixLaterRemindDate');
    expect(findingsPanelSource).toContain("dismissReason() === 'will_fix_later'");
    expect(findingsPanelSource).toContain('Pulse will stay quiet on this for 7 days');
    // The other two reasons must also have explanatory copy so all three
    // dismissal paths feel deliberate, not undifferentiated.
    expect(findingsPanelSource).toContain("dismissReason() === 'expected_behavior'");
    expect(findingsPanelSource).toContain("dismissReason() === 'not_an_issue'");
    expect(findingsPanelSource).toContain('Pulse will keep this finding visible as acknowledged');
    expect(findingsPanelSource).toContain('Pulse will permanently suppress');
  });

  it('exposes a Copy summary action wired to formatFindingForClipboard and the shared clipboard helper', () => {
    // Operators frequently need to delegate a finding by pasting a summary
    // into Slack, email, or a ticket. Pin the wiring so the Copy summary
    // button routes through the canonical formatter (same shape as the
    // helper test below) and the shared copyToClipboard helper, with
    // success/error notifications routed through notificationStore.
    expect(findingsPanelSource).toContain('handleCopyFindingSummary');
    expect(findingsPanelSource).toContain('formatFindingForClipboard(finding)');
    expect(findingsPanelSource).toContain('copyToClipboard(text)');
    expect(findingsPanelSource).toContain('Finding summary copied');
    expect(findingsPanelSource).toContain('Could not copy finding summary');
    expect(findingsPanelSource).toMatch(/>\s*Copy summary\s*</);
  });

  it('keeps expanded finding actions behind a canonical primary action and compact Manage menu', () => {
    expect(findingsPanelSource).toContain('getFindingPrimaryActionPresentation(finding)');
    expect(findingsPanelSource).toContain('getPrimaryAssistantFindingAction');
    expect(findingsPanelSource).toContain('const shouldShowAssistantPrimaryAction =');
    expect(findingsPanelSource).toContain('const shouldShowAssistantManageAction =');
    expect(findingsPanelSource).toContain(
      '<Show when={!primaryAction && shouldShowAssistantPrimaryAction}>',
    );
    expect(findingsPanelSource).toContain('<Show when={shouldShowAssistantManageAction}>');
    expect(findingsPanelSource).not.toContain(
      'fallback={\n              <button\n                type="button"\n                onClick={(e) => {\n                  e.stopPropagation();\n                  void openFindingInAssistant(finding);',
    );
    expect(findingsPanelSource).toContain('Manage');
    expect(findingsPanelSource).toContain('min-w-48');
    expect(findingsPanelSource).not.toContain('min-w-40');
  });

  it('keeps primary-action Patrol runtime findings out of secondary action clutter', () => {
    // Runtime/setup findings already have one canonical recovery path:
    // provider settings. The expanded card must not add the Assistant
    // secondary button or generic Manage menu beside that primary action.
    expect(findingsPanelSource).toContain('const shouldShowExpandedManageMenu =');
    expect(findingsPanelSource).toContain('!primaryAction ||');
    expect(findingsPanelSource).toContain('<Show when={shouldShowExpandedManageMenu}>');
    expect(findingsPanelSource).toContain('when={primaryAction}');
    expect(findingsPanelSource).not.toContain(
      'getPrimaryAssistantFindingAction(finding).label}\n                </button>\n              </>',
    );
  });

  it('surfaces primary Patrol issue actions before expansion', () => {
    expect(findingsPanelSource).toContain('shouldShowCollapsedPrimaryAction');
    expect(findingsPanelSource).toContain('isPatrolFindingsSource()');
    expect(findingsPanelSource).toContain("finding.status === 'active'");
    expect(findingsPanelSource).toContain('props.runSnapshot === undefined');
    expect(findingsPanelSource).toContain('expandedId() !== finding.id');
    expect(findingsPanelSource).toContain(
      'const rowPrimaryAction = getFindingPrimaryActionPresentation(finding)',
    );
    expect(findingsPanelSource).toContain(
      'shouldShowCollapsedPrimaryAction(finding) && rowPrimaryAction',
    );
  });

  it('exposes a manual Mark resolved action that goes through the patrol resolve store action', () => {
    // The /api/ai/patrol/resolve endpoint exists server-side but had no
    // operator path on the canonical Patrol surface. An operator fixing
    // an issue out-of-band must be able to close the loop without waiting
    // for auto-resolution. The button is gated to active findings (the
    // server rejects double-resolves with 404) and routes through
    // aiIntelligenceStore.resolveFinding so refresh/error UX stays
    // uniform with the existing acknowledge/snooze/dismiss patterns.
    expect(findingsPanelSource).toContain('handleResolve');
    expect(findingsPanelSource).toContain('aiIntelligenceStore.resolveFinding(finding.id)');
    expect(findingsPanelSource).toContain('Mark resolved');
    expect(findingsPanelSource).toContain("finding.status === 'active'");
    expect(findingsPanelSource).toContain('Show when={manualControls.acknowledge}');
    expect(findingsPanelSource).toContain('Finding marked resolved');
  });

  it('fails closed before creating a per-finding suppression rule without concrete scope', () => {
    // Empty resource/category fields are backend wildcards. The per-finding
    // shortcut must not turn missing metadata into a broad suppression rule.
    expect(findingsPanelSource).toContain('getFindingSuppressionRuleScope');
    expect(findingsPanelSource).toContain('canCreate: Boolean(resourceId && category)');
    expect(findingsPanelSource).toContain('!scope.canCreate');
    expect(findingsPanelSource).toContain('resourceId: scope.resourceId');
    expect(findingsPanelSource).toContain('category: scope.category');
    expect(findingsPanelSource).toContain(
      'missing the resource or category needed for a scoped rule',
    );
  });

  it('surfaces regressionCount as a pill on the collapsed finding row', () => {
    // regressionCount is the strongest "this is not a one-off" signal Pulse
    // can give an operator scanning a list. The pill must appear in the
    // collapsed row (alongside the confidence badge) so triage decisions
    // can be made without expanding each card. The pill must only appear
    // when regressionCount > 0 — fresh detections must stay clean.
    expect(findingsPanelSource).toContain('(finding.regressionCount || 0) > 0');
    expect(findingsPanelSource).toContain('regressed {finding.regressionCount}×');
    expect(findingsPanelSource).toContain('text-amber-700 dark:text-amber-300');
    // The regression pill must sit next to the confidence badge in source order
    // so the row reads as one trust-related cluster.
    const confidenceIndex = findingsPanelSource.indexOf('confidence');
    const regressionIndex = findingsPanelSource.indexOf('regressed {finding.regressionCount}×');
    expect(confidenceIndex).toBeGreaterThan(0);
    expect(regressionIndex).toBeGreaterThan(0);
    expect(regressionIndex).toBeGreaterThan(confidenceIndex);
  });

  it('surfaces only actionable Patrol workflow states from the shared presentation model', () => {
    // Collapsed Patrol rows should stay issue-focused. Workflow state can
    // drive a direct approval action, but it must not add process badges such
    // as "detected", "investigating", or "verify outcome" to the scan view.
    expect(findingsPanelSource).toContain('getFindingPatrolWorkflowPresentation');
    expect(findingsPanelSource).toContain(
      'getFindingPatrolWorkflowPresentation(finding, aiIntelligenceStore.patrolPendingApprovals)',
    );
    expect(findingsPanelSource).toContain("workflow?.stage === 'approval'");
    expect(findingsPanelSource).not.toContain("workflow.stage === 'investigating'");
    expect(findingsPanelSource).not.toContain("workflow.stage === 'verification'");
    expect(findingsPanelSource).not.toContain("workflow.stage === 'attention'");
    expect(findingsPanelSource).not.toContain("workflow.stage === 'recorded'");
    expect(findingsPanelSource).not.toContain("workflow.stage === 'paused'");
  });

  it('turns approval-required Patrol findings into a direct collapsed action', () => {
    expect(findingsPanelSource).toContain('const collapsedApprovalAction = () => {');
    expect(findingsPanelSource).toContain("workflow?.stage === 'approval'");
    expect(findingsPanelSource).toContain(
      '<Show when={expandedId() !== finding.id ? collapsedApprovalAction() : undefined}>',
    );
    expect(findingsPanelSource).toContain('title={action().detail}');
    expect(findingsPanelSource).toContain('{action().label}');
  });

  it('warns the operator before dismissing a recurrent finding as not_an_issue or expected_behavior', () => {
    // not_an_issue and expected_behavior both stay quiet forever after dismiss.
    // If the finding has already regressed before, the operator may be
    // silently dismissing a recurring issue. The dismiss confirmation panel
    // must surface that recurrence as a non-blocking hint and nudge them
    // toward the reminder-bearing 'will_fix_later' path. The hint must NOT
    // appear for will_fix_later itself (already commitment-tracking) or for
    // findings with no prior regression.
    expect(findingsPanelSource).toContain('(finding.regressionCount || 0) > 1');
    expect(findingsPanelSource).toContain("dismissReason() === 'not_an_issue'");
    expect(findingsPanelSource).toContain("dismissReason() === 'expected_behavior'");
    expect(findingsPanelSource).toContain('this finding has regressed');
    expect(findingsPanelSource).toContain('"Later" sets a');
  });

  it('badges dismissed-as-will_fix_later rows with their pending remind-at deadline', () => {
    // Once a finding is dismissed as will_fix_later, the row must surface the
    // pending reminder so the operator knows the commitment exists; otherwise
    // the only place the deadline lives is the lifecycle log. The amber tone
    // signals "pending operator action" rather than a generic muted note.
    expect(findingsPanelSource).toContain(
      "finding.dismissedReason === 'will_fix_later' && finding.remindAt",
    );
    expect(findingsPanelSource).toContain('Reminding {formatTime(finding.remindAt!)}');
    expect(findingsPanelSource).toContain('text-amber-600 dark:text-amber-400');
  });

  it('keeps remediation artifacts as compact Assistant context only', () => {
    // Patrol findings should not grow a frontend-authored proposal surface.
    // Any existing artifact is a pointer for Assistant review, not a visible
    // fix plan, capacity proposal, or approval bypass.
    expect(findingsPanelSource).toContain('Assistant context');
    expect(findingsPanelSource).toContain('Ask Assistant');
    expect(findingsPanelSource).toContain('handleOpenPlanInAssistant');
    expect(findingsPanelSource).toContain('handleDismissPlan');
    expect(findingsPanelSource).not.toContain('capacity_forecast');
    expect(findingsPanelSource).not.toContain('Capacity-forecast proposal');
    expect(findingsPanelSource).not.toContain('Approve proposal');
    expect(findingsPanelSource).not.toContain('handleApproveProposedPlan');
    expect(findingsPanelSource).not.toContain('AIAPI.approveRemediationPlan(plan.id)');
    expect(findingsPanelSource).not.toContain('Remediation Plan');
    expect(findingsPanelSource).not.toContain('<For each={plan().steps}>');
  });

  it('renders the operator-facing Impact line without a Pulse-authored recommendation line', () => {
    // The expanded finding card must surface Finding.Impact directly so
    // detection-time consequence-if-ignored copy reaches the operator on the
    // findings list, not just inside the durable investigation record.
    expect(findingsPanelSource).toContain('Show when={finding.impact}');
    expect(findingsPanelSource).toContain('>Impact:</span> {finding.impact}');
    expect(findingsPanelSource).not.toContain('>Recommendation:</span>');
  });

  it('keeps active Patrol lifecycle history out of the default issue expansion', () => {
    // Current issues should answer "what needs doing" first. Raw lifecycle
    // history is still available when the operator chooses all/resolved history
    // or a run record, but it should not be dumped into the default active
    // Patrol issue expansion.
    expect(findingsPanelSource).toContain('shouldShowFindingLifecycle');
    expect(findingsPanelSource).toContain('!isPatrolFindingsSource()');
    expect(findingsPanelSource).toContain("finding.status !== 'active'");
    expect(findingsPanelSource).toContain('props.runSnapshot !== undefined');
    expect(findingsPanelSource).toContain("filter() === 'all'");
    expect(findingsPanelSource).toContain("filter() === 'resolved'");
    expect(findingsPanelSource).toContain('Show when={shouldShowFindingLifecycle(finding)}');
  });
});

describe('aiFindingPresentation', () => {
  describe('severity presentation', () => {
    it('has correct sort order for critical', () => {
      expect(getFindingSeveritySortOrder('critical')).toBe(0);
    });

    it('has correct sort order for warning', () => {
      expect(getFindingSeveritySortOrder('warning')).toBe(1);
    });

    it('has correct sort order for watch', () => {
      expect(getFindingSeveritySortOrder('watch')).toBe(2);
    });

    it('has correct sort order for info', () => {
      expect(getFindingSeveritySortOrder('info')).toBe(3);
    });

    it('prioritizes active patrol runtime findings within the same severity tier', () => {
      expect(
        getFindingActiveRuntimeSortOrder({
          status: 'active',
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Provider billing or quota issue',
        }),
      ).toBe(0);
      expect(
        getFindingActiveRuntimeSortOrder({
          status: 'active',
          resourceId: 'vm-101',
          resourceName: 'db-01',
          title: 'Disk nearly full',
        }),
      ).toBe(1);
    });

    it('returns compact severity labels', () => {
      expect(getFindingSeverityCompactLabel('critical')).toBe('CRIT');
      expect(getFindingSeverityCompactLabel('warning')).toBe('WARN');
      expect(getFindingSeverityCompactLabel('watch')).toBe('WATCH');
      expect(getFindingSeverityCompactLabel('info')).toBe('INFO');
    });
  });

  describe('sourceLabels', () => {
    it('has correct label for threshold', () => {
      expect(getFindingSourceLabel('threshold')).toBe('Alert');
    });

    it('has correct label for ai-patrol', () => {
      expect(getFindingSourceLabel('ai-patrol')).toBe('Pulse Patrol');
    });

    it('has correct label for anomaly', () => {
      expect(getFindingSourceLabel('anomaly')).toBe('Anomaly');
    });

    it('has correct label for ai-chat', () => {
      expect(getFindingSourceLabel('ai-chat')).toBe('Pulse Assistant');
    });

    it('has correct label for correlation', () => {
      expect(getFindingSourceLabel('correlation')).toBe('Correlation');
    });

    it('has correct label for forecast', () => {
      expect(getFindingSourceLabel('forecast')).toBe('Forecast');
    });
  });

  describe('severityColors', () => {
    it('contains critical color classes', () => {
      expect(getFindingSeverityBadgeClasses('critical')).toContain('red-200');
      expect(getFindingSeverityBadgeClasses('critical')).toContain('red-700');
    });

    it('contains warning color classes', () => {
      expect(getFindingSeverityBadgeClasses('warning')).toContain('amber-200');
      expect(getFindingSeverityBadgeClasses('warning')).toContain('amber-700');
    });

    it('contains info color classes', () => {
      expect(getFindingSeverityBadgeClasses('info')).toContain('blue-200');
      expect(getFindingSeverityBadgeClasses('info')).toContain('blue-700');
    });

    it('contains watch color classes', () => {
      expect(getFindingSeverityBadgeClasses('watch')).toContain('bg-surface-alt');
    });

    it('contains compact tone classes for critical severity', () => {
      expect(getFindingSeverityToneClasses('critical')).toContain('bg-red-100');
    });

    it('keeps ordinary infrastructure severity badges generic', () => {
      expect(
        getFindingSeverityPresentation({
          severity: 'warning',
          resourceId: 'vm-101',
          resourceName: 'db-01',
          title: 'Disk nearly full',
        }),
      ).toEqual({
        label: 'warning',
        badgeClasses: getFindingSeverityBadgeClasses('warning'),
        badgeTone: 'warning',
        uppercase: true,
      });
    });

    it('renders runtime-qualified severity badges for patrol runtime findings', () => {
      expect(
        getFindingSeverityPresentation({
          severity: 'warning',
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Provider billing or quota issue',
        }),
      ).toEqual({
        label: 'Runtime issue',
        badgeClasses:
          'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
        badgeTone: 'sky',
        uppercase: false,
      });
    });

    it('uses patrol runtime badge tones when only runtime findings are active', () => {
      expect(
        getPatrolFindingsBadgePresentation([
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Provider billing or quota issue',
          },
        ]),
      ).toEqual({
        tone: 'info',
      });
    });

    it('keeps warning tones for infrastructure warning findings', () => {
      expect(
        getPatrolFindingsBadgePresentation([
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'vm-101',
            resourceName: 'db-01',
            title: 'Disk nearly full',
          },
        ]),
      ).toEqual({
        tone: 'warning',
      });
    });
  });

  describe('findingStatusPresentation', () => {
    it('returns canonical badge classes', () => {
      expect(getFindingStatusBadgeClasses('resolved')).toContain('green');
      expect(getFindingStatusBadgeTone('resolved')).toBe('success');
      expect(getFindingStatusBadgeClasses('snoozed')).toContain('blue');
      expect(getFindingStatusBadgeTone('snoozed')).toBe('info');
      expect(getFindingStatusBadgeClasses('dismissed')).toContain('bg-surface-alt');
      expect(getFindingStatusBadgeTone('dismissed')).toBe('muted');
      expect(getFindingStatusBadgeClasses('unexpected')).toContain('bg-surface-alt');
      expect(getFindingStatusBadgeTone('unexpected')).toBe('muted');
    });

    it('returns canonical labels', () => {
      expect(getFindingStatusLabel('resolved')).toBe('Resolved');
      expect(getFindingStatusLabel('snoozed')).toBe('Snoozed');
      expect(getFindingStatusLabel('dismissed')).toBe('Dismissed');
      expect(getFindingStatusLabel('unexpected')).toBe('Dismissed');
    });
  });

  describe('findingRecencyPresentation', () => {
    it('uses last seen recency for active findings', () => {
      expect(
        getFindingRecencyPresentation({
          status: 'active',
          detectedAt: '2026-03-01T00:00:00Z',
          lastSeenAt: '2026-03-25T12:00:00Z',
        }),
      ).toEqual({
        label: 'last seen',
        timestamp: '2026-03-25T12:00:00Z',
      });
    });

    it('falls back to detected recency for inactive findings', () => {
      expect(
        getFindingRecencyPresentation({
          status: 'resolved',
          detectedAt: '2026-03-01T00:00:00Z',
          lastSeenAt: '2026-03-25T12:00:00Z',
        }),
      ).toEqual({
        label: 'detected',
        timestamp: '2026-03-01T00:00:00Z',
      });
    });
  });

  describe('patrolFindingClassification', () => {
    it('describes grouped Patrol findings as issues instead of internal signals', () => {
      expect(getPatrolFindingIssueCountLabel(0)).toBe('0 issues');
      expect(getPatrolFindingIssueCountLabel(1)).toBe('1 issue');
      expect(getPatrolFindingIssueCountLabel(2)).toBe('2 issues');
      expect(getPatrolFindingIssueCountLabel(2.9)).toBe('2 issues');
      expect(aiFindingPresentationSource).toContain('getPatrolFindingIssueCountLabel');
      expect(aiFindingPresentationSource).not.toContain('getPatrolFindingSignalCountLabel');
    });

    it('classifies ai-service findings as patrol runtime issues', () => {
      expect(
        getPatrolFindingClassification({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Provider billing or quota issue',
        }),
      ).toEqual({
        kind: 'runtime',
        label: 'Patrol runtime',
        badgeClasses:
          'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
      });
    });

    it('keeps ordinary findings classified as infrastructure', () => {
      expect(
        getPatrolFindingClassification({
          resourceId: 'vm-101',
          resourceName: 'db-01',
          title: 'Disk nearly full',
        }),
      ).toEqual({
        kind: 'infrastructure',
        label: 'Infrastructure',
        badgeClasses: 'border-border bg-surface-alt text-muted',
      });
    });
  });

  describe('findingSubjectPresentation', () => {
    it('renders patrol-owned service findings as patrol runtime', () => {
      expect(
        getFindingSubjectPresentation({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          resourceType: 'service',
          title: 'Pulse Patrol: Provider billing or quota issue',
        }),
      ).toEqual({
        label: 'Patrol runtime',
      });
    });

    it('normalizes ordinary resource type labels', () => {
      expect(
        getFindingSubjectPresentation({
          resourceId: 'ct-101',
          resourceName: 'app-ct',
          resourceType: 'system-container',
          title: 'Disk nearly full',
        }),
      ).toEqual({
        label: 'app-ct (system-container)',
      });
    });
  });

  describe('findingPrimaryActionPresentation', () => {
    it('offers Patrol provider settings as the primary action for Patrol runtime findings', () => {
      expect(
        getFindingPrimaryActionPresentation({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Provider billing or quota issue',
        }),
      ).toEqual(PATROL_PROVIDER_SETTINGS_ACTION);
    });

    it('does not expose Patrol provider settings as the primary action for infrastructure findings', () => {
      expect(
        getFindingPrimaryActionPresentation({
          resourceId: 'vm-101',
          resourceName: 'db-01',
          title: 'Disk nearly full',
        }),
      ).toBeUndefined();
    });

    it('keeps Patrol runtime actions keyed to the shared runtime identity even without product-prefixed titles', () => {
      expect(
        getFindingPrimaryActionPresentation({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Provider billing or quota issue',
        }),
      ).toEqual(PATROL_PROVIDER_SETTINGS_ACTION);
    });
  });

  describe('findingTitlePresentation', () => {
    it('normalizes legacy Patrol runtime credit titles', () => {
      expect(
        getFindingTitlePresentation({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Insufficient API credits',
        }),
      ).toEqual({
        label: 'Provider billing or quota issue',
      });
    });

    it('keeps infrastructure finding titles unchanged', () => {
      expect(
        getFindingTitlePresentation({
          resourceId: 'vm-101',
          resourceName: 'db-01',
          title: 'Disk nearly full',
        }),
      ).toEqual({
        label: 'Disk nearly full',
      });
    });
  });

  describe('findingManualControlsPresentation', () => {
    it('disables generic feedback controls for patrol runtime findings', () => {
      expect(
        getFindingManualControlsPresentation({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Provider billing or quota issue',
        }),
      ).toEqual({
        acknowledge: false,
        snooze: false,
        dismiss: false,
      });
    });

    it('keeps generic feedback controls for infrastructure findings', () => {
      expect(
        getFindingManualControlsPresentation({
          resourceId: 'vm-101',
          resourceName: 'db-01',
          title: 'Disk nearly full',
        }),
      ).toEqual({
        acknowledge: true,
        snooze: true,
        dismiss: true,
      });
    });
  });

  describe('filterPresentation', () => {
    it('builds canonical filter options', () => {
      expect(
        buildFindingFilterOptions({
          needsAttentionCount: 2,
          pendingApprovalCount: 1,
        }),
      ).toEqual([
        { value: 'active', label: 'Active' },
        { value: 'all', label: 'All' },
        { value: 'resolved', label: 'Resolved' },
        { value: 'attention', label: 'Needs Attention', tone: 'warning', count: 2 },
        { value: 'approvals', label: 'Approvals', tone: 'warning', count: 1 },
      ]);
    });

    it('returns canonical empty-state copy', () => {
      expect(getFindingEmptyStateCopy('active')).toEqual({
        title: 'No active findings',
        body: 'Your infrastructure looks healthy!',
      });
      expect(getFindingEmptyStateCopy('attention')).toEqual({
        title: 'No findings need attention right now.',
      });
      expect(getFindingEmptyStateCopy('approvals')).toEqual({
        title: 'No pending approvals.',
      });
      expect(getFindingEmptyStateCopy('resolved')).toEqual({
        title: 'No Patrol findings to display',
      });
    });
  });

  describe('patrol empty-state presentation', () => {
    it('does not duplicate patrol timing metadata in the findings empty state', () => {
      expect(findingsPanelSource).not.toContain('CountdownTimer');
      expect(findingsPanelSource).not.toContain('lastPatrolLabel');
      expect(findingsPanelSource).not.toContain('Runs every');
      expect(findingsPanelSource).not.toContain('Next: ');
    });

    it('keeps the findings card header functional instead of repeating product marketing copy', () => {
      expect(findingsPanelSource).not.toContain(
        '<span class="font-medium text-base-content">Patrol findings</span>',
      );
      expect(findingsPanelSource).not.toContain('Pulse Patrol Findings');
      expect(findingsPanelSource).not.toContain('AI-discovered insights');
    });

    it('exposes the canonical primary action for Patrol runtime findings inside the expanded row', () => {
      expect(findingsPanelSource).toContain('getFindingPrimaryActionPresentation');
      expect(findingsPanelSource).toContain('{action().label}');
      expect(findingsPanelSource).toContain('href={action().href}');
      expect(findingsPanelSource).toContain('shouldShowAssistantManageAction');
    });

    it('does not cross-jump expanded findings to broad Infrastructure or aggregate workspace routes', () => {
      // The platform-first migration retired buildResolvedResourceSurfaceLinks
      // and the broad surface-link chips; Patrol findings now stay in place
      // rather than offering external surface jumps.
      expect(findingsPanelSource).not.toContain("from '@/hooks/useResources'");
      expect(findingsPanelSource).not.toContain('buildResolvedResourceSurfaceLinks');
      expect(findingsPanelSource).not.toContain('useResources()');
      expect(findingsPanelSource).not.toContain('{link.compactLabel}');
    });

    it('routes generic finding controls through the shared manual-controls helper', () => {
      expect(findingsPanelSource).toContain('getFindingManualControlsPresentation(finding)');
      expect(findingsPanelSource).toContain('manualControls.acknowledge');
      expect(findingsPanelSource).toContain('manualControls.snooze');
      expect(findingsPanelSource).toContain('manualControls.dismiss');
    });

    it('keeps inline manual controls out of the default Patrol queue rows', () => {
      expect(findingsPanelSource).toContain('const shouldShowInlineManualControls =');
      expect(findingsPanelSource).toContain(
        "finding.status === 'active' && !isPatrolFindingsSource()",
      );
      expect(findingsPanelSource).toContain(
        '<Show when={shouldShowInlineManualControls(finding)}>',
      );
    });

    it('loads remediation artifacts through the shared Patrol store instead of probing the API directly', () => {
      expect(findingsPanelSource).toContain('aiIntelligenceStore.loadRemediationPlans()');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.remediationPlans');
      expect(findingsPanelSource).not.toContain('AIAPI.getRemediationPlans()');
    });

    it('can source Patrol findings from the direct Patrol store boundary', () => {
      expect(findingsPanelSource).toContain("findingsSource?: 'unified' | 'patrol'");
      expect(findingsPanelSource).toContain('aiIntelligenceStore.loadPatrolFindings()');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.patrolFindings');
      expect(findingsPanelSource).toContain('const sourceHasFindings = createMemo(');
      expect(findingsPanelSource).toContain('const shouldShowLoadingState = createMemo(');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.patrolFindingsLoading');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.patrolFindingsError');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.patrolFindingsSignal()');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.patrolFindingsNeedingAttention');
      expect(findingsPanelSource).toContain(
        'aiIntelligenceStore.patrolFindingsWithPendingApprovals',
      );
      expect(patrolWorkspaceSource).toContain('findingsSource="patrol"');
    });

    it('passes Patrol history regression evidence into the findings empty-state helper', () => {
      expect(findingsPanelSource).toContain('historicalRegressionCount?: number');
      expect(findingsPanelSource).toContain(
        'historicalRegressionCount: props.historicalRegressionCount',
      );
      expect(patrolWorkspaceSource).toContain(
        'historicalRegressionCount={state.historicalRegressionCount()}',
      );
    });

    it('uses an informational icon for non-warning Patrol empty-state context', () => {
      expect(findingsPanelSource).toContain("import InfoIcon from 'lucide-solid/icons/info';");
      expect(findingsPanelSource).toContain("when={emptyStateCopy().tone === 'warning'}");
      expect(findingsPanelSource).toContain(
        '<InfoIcon class={`w-10 h-10 ${emptyStateTone().iconClass}`} />',
      );
    });

    it('routes same-severity ordering through the shared patrol runtime sort helper', () => {
      expect(findingsPanelSource).toContain('getFindingActiveRuntimeSortOrder(a)');
      expect(findingsPanelSource).toContain('getFindingActiveRuntimeSortOrder(b)');
    });

    it('only shows the sort control when there are multiple Patrol findings to sort', () => {
      expect(findingsPanelSource).toContain('patrolFindingDisplayGroups().length > 1');
      expect(findingsPanelSource).toContain('FormSelect');
      expect(findingsPanelSource).toContain('label="Sort findings"');
      expect(findingsPanelSource).toContain('<option value="severity">By Severity</option>');
      expect(findingsPanelSource).not.toContain('<select');
    });

    it('hides the filter bar when there are no Patrol findings or special buckets to navigate', () => {
      expect(findingsPanelSource).toContain('const showFilterControls = createMemo(');
      expect(findingsPanelSource).toContain('const hasUnknownRunSnapshot = createMemo(');
      expect(findingsPanelSource).toContain('const useRunSnapshotScopedControls = createMemo(');
      expect(findingsPanelSource).toContain('const runSnapshotScopedPatrolFindings = createMemo(');
      expect(findingsPanelSource).toContain('const filterCounts = createMemo(() => ({');
      expect(findingsPanelSource).toContain('filterCounts().needsAttentionCount > 0');
      expect(findingsPanelSource).toContain('filterCounts().pendingApprovalCount > 0');
      expect(findingsPanelSource).toContain('allPatrolFindings().length > 0');
      expect(findingsPanelSource).toContain('<Show when={showFilterControls()}>');
    });

    it('auto-resets conditional filters from the same scoped count model used by the filter bar', () => {
      expect(findingsPanelSource).toContain(
        "if (filter() === 'attention' && filterCounts().needsAttentionCount === 0)",
      );
      expect(findingsPanelSource).toContain(
        "if (filter() === 'approvals' && filterCounts().pendingApprovalCount === 0)",
      );
      expect(findingsPanelSource).toContain('buildFindingFilterOptions(filterCounts())');
    });

    it('fails closed on legacy run snapshots instead of showing global findings in a selected run view', () => {
      expect(findingsPanelSource).toContain('if (hasUnknownRunSnapshot()) {');
      expect(findingsPanelSource).toContain('return [];');
      expect(findingsPanelSource).toContain(
        '() => props.runSnapshot !== undefined && props.filterFindingIds === undefined',
      );
    });

    it('does not let global Patrol loading mask a selected run empty snapshot', () => {
      expect(findingsPanelSource).toContain('const hasEmptyRunSnapshot = createMemo(');
      expect(findingsPanelSource).toContain(
        'const canRenderRunSnapshotWithoutSourceFindings = createMemo(',
      );
      expect(findingsPanelSource).toContain('!canRenderRunSnapshotWithoutSourceFindings()');
      expect(findingsPanelSource).toContain('const shouldShowRichEmptyState = createMemo(');
      expect(findingsPanelSource).toContain("filter() === 'active' || props.runSnapshot");
    });

    it('uses explicit textual separators in finding metadata instead of relying on visual spacing', () => {
      expect(findingsPanelSource).toContain("{' · '}acknowledged");
      expect(findingsPanelSource).toContain("{' · '}last investigated");
      expect(findingsPanelSource).toContain("{' · '}snoozed until");
    });

    it('uses shared metadata badges for the current Patrol queue resource count only', () => {
      expect(patrolWorkspaceSource).toContain('aria-hidden="true"');
      expect(patrolWorkspaceSource).toContain('<MetadataBadge');
      expect(patrolWorkspaceSource).toContain('getPatrolQueueBadgeLabel');
      expect(patrolWorkspaceSource).toContain('queueAffectedResourceCount');
      expect(patrolWorkspaceSource).toContain('{queueBadgeLabel()}');
      expect(patrolWorkspaceSource).not.toContain('{runHistoryCount()}');
    });

    it('routes the findings tab badge tone through the shared patrol findings badge helper', () => {
      expect(patrolWorkspaceSource).toContain('getPatrolFindingsBadgePresentation');
      expect(patrolWorkspaceSource).toContain('state.findingsTabBadgeFindings()');
      expect(patrolWorkspaceSource).toContain('MetadataBadge');
      expect(patrolWorkspaceSource).toContain('findingsBadgePresentation().tone');
      expect(patrolWorkspaceSource).not.toContain('findingsBadgePresentation().toneClasses');
    });

    it('does not stack a default detected loop-state badge on active findings', () => {
      expect(findingsPanelSource).toContain('const shouldShowLoopStateBadge = () =>');
      expect(findingsPanelSource).toContain('!isPatrolFindingsSource()');
      expect(findingsPanelSource).toContain("finding.status === 'active'");
      expect(findingsPanelSource).toContain("finding.source !== 'ai-patrol'");
      expect(findingsPanelSource).toContain("finding.loopState !== 'detected'");
      expect(findingsPanelSource).toContain('when={shouldShowLoopStateBadge()}');
      expect(findingsPanelSource).toContain('Patrol status:');
      expect(findingsPanelSource).not.toContain('Patrol loop:');
    });

    it('keeps raw Patrol process badges out of the collapsed Patrol issue row', () => {
      expect(findingsPanelSource).toContain('const shouldShowInvestigationStatusBadge = () =>');
      expect(findingsPanelSource).toContain('const shouldShowInvestigationOutcomeBadge = () =>');
      expect(findingsPanelSource).toContain('const shouldShowInvestigationConfidenceBadge = () =>');
      expect(findingsPanelSource).toContain('!isPatrolFindingsSource() &&');
      expect(findingsPanelSource).not.toContain('const collapsedPatrolStatusBadge = () => {');
      expect(findingsPanelSource).not.toContain(
        "workflow?.stage === 'review' ? undefined : workflow",
      );
      expect(findingsPanelSource).not.toContain('<Show when={collapsedPatrolStatusBadge()}>');
    });

    it('groups active Patrol queue findings by resource without changing run records', () => {
      expect(findingsPanelSource).toContain('buildPatrolFindingDisplayGroups');
      expect(findingsPanelSource).toContain('getPatrolFindingIssueCountLabel');
      expect(findingsPanelSource).toContain('const shouldGroupPatrolFindingsByResource');
      expect(findingsPanelSource).toContain("filter() === 'active'");
      expect(findingsPanelSource).toContain('props.runSnapshot === undefined');
      expect(findingsPanelSource).toContain(
        'aria-label={`${group.resourceLabel}: ${getPatrolFindingIssueCountLabel(',
      );
      expect(findingsPanelSource).toContain(
        '{getPatrolFindingIssueCountLabel(group.findings.length)}',
      );
      expect(findingsPanelSource).toContain('renderFindingItem(group.primaryFinding, false, {');
      expect(findingsPanelSource).toContain('relatedFindings: group.relatedFindings');
      expect(findingsPanelSource).not.toContain('<For each={group.findings}>');
      expect(findingsPanelSource).toContain('hideSubject: true');
    });

    it('uses canonical finding recency presentation instead of raw detected timestamps for active rows', () => {
      expect(findingsPanelSource).toContain(
        'const recency = getFindingRecencyPresentation(finding);',
      );
      expect(findingsPanelSource).toContain('{subject.label} - {recency.label} ');
      expect(findingsPanelSource).toContain('{formatTime(recency.timestamp)}');
    });

    it('routes row badges through shared metadata badge primitives', () => {
      expect(findingsPanelSource).toContain(
        'const severityPresentation = getFindingSeverityPresentation(finding);',
      );
      expect(findingsPanelSource).toContain('MetadataBadge');
      expect(findingsPanelSource).toContain('FINDING_ROW_BADGE_PROPS');
      expect(findingsPanelSource).toContain('severityPresentation.badgeTone');
      expect(findingsPanelSource).toContain('severityPresentation.label');
      expect(findingsPanelSource).not.toContain(
        'px-1.5 py-0.5 border text-[10px] font-medium rounded',
      );
    });

    it('routes visible finding titles through the shared title presentation helper', () => {
      expect(findingsPanelSource).toContain('const title = getFindingTitlePresentation(finding);');
      expect(findingsPanelSource).toContain('{title.label}');
      expect(findingsPanelSource).toContain(
        'findingTitle={getFindingTitlePresentation(finding).label}',
      );
    });

    it('uses the shared finding subject presentation instead of raw patrol service resource tokens', () => {
      expect(findingsPanelSource).toContain(
        'const subject = getFindingSubjectPresentation(finding);',
      );
      expect(findingsPanelSource).toContain('{subject.label} - {recency.label}');
      expect(findingsPanelSource).not.toContain('{finding.resourceName} ({finding.resourceType})');
    });
  });

  describe('sourceColors', () => {
    it('has threshold color', () => {
      expect(getFindingSourceBadgeClasses('threshold')).toContain('orange');
      expect(getFindingSourceBadgeTone('threshold')).toBe('orange');
    });

    it('has ai-patrol color', () => {
      expect(getFindingSourceBadgeClasses('ai-patrol')).toContain('blue');
      expect(getFindingSourceBadgeTone('ai-patrol')).toBe('info');
    });

    it('has ai-chat color', () => {
      expect(getFindingSourceBadgeClasses('ai-chat')).toContain('teal');
      expect(getFindingSourceBadgeTone('ai-chat')).toBe('teal');
    });
  });

  describe('investigationStatusColors', () => {
    it('has pending color', () => {
      expect(getInvestigationStatusBadgeClasses('pending')).toContain('bg-surface-alt');
      expect(getInvestigationStatusBadgeTone('pending')).toBe('muted');
    });

    it('has running color', () => {
      expect(getInvestigationStatusBadgeClasses('running')).toContain('blue');
      expect(getInvestigationStatusBadgeTone('running')).toBe('info');
    });

    it('has completed color', () => {
      expect(getInvestigationStatusBadgeClasses('completed')).toContain('green');
      expect(getInvestigationStatusBadgeTone('completed')).toBe('success');
    });

    it('has failed color', () => {
      expect(getInvestigationStatusBadgeClasses('failed')).toContain('red');
      expect(getInvestigationStatusBadgeTone('failed')).toBe('danger');
    });

    it('returns canonical status labels', () => {
      expect(getInvestigationStatusLabel('pending')).toBe('Pending');
      expect(getInvestigationStatusLabel('running')).toBe('Running');
      expect(getInvestigationStatusLabel('completed')).toBe('Completed');
      expect(getInvestigationStatusLabel('failed')).toBe('Failed');
      expect(getInvestigationStatusLabel('needs_attention')).toBe('Needs Attention');
    });
  });

  describe('investigationOutcomePresentation', () => {
    it('returns canonical outcome labels and badge classes', () => {
      expect(getInvestigationOutcomeLabel('fix_verified')).toBe('Fix verified');
      expect(getInvestigationOutcomeBadgeClasses('fix_failed')).toContain('red');
      expect(getInvestigationOutcomeBadgeTone('fix_failed')).toBe('danger');
      expect(getInvestigationOutcomeLabel('fix_rejected')).toBe('Fix rejected');
      expect(getInvestigationOutcomeBadgeTone('fix_rejected')).toBe('warning');
      expect(getInvestigationOutcomeBadgeClasses('cannot_fix')).toContain('bg-surface-alt');
      expect(getInvestigationOutcomeBadgeTone('cannot_fix')).toBe('muted');
      expect(getInvestigationOutcomeSortOrder('fix_failed')).toBe(0);
      expect(getInvestigationOutcomeSortOrder('needs_attention')).toBe(1);
      expect(getInvestigationOutcomeSortOrder('fix_queued')).toBe(2);
      expect(getInvestigationOutcomeSortOrder(undefined)).toBe(3);
      expect(getInvestigationConfidenceBadgeTone('high')).toBe('success');
      expect(getInvestigationConfidenceBadgeTone('medium')).toBe('neutral');
      expect(getInvestigationConfidenceBadgeTone('low')).toBe('warning');
    });

    it('treats any investigation metadata as enough to render investigation details', () => {
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '',
          investigationStatus: 'failed',
          investigationOutcome: undefined,
          investigationAttempts: 0,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '',
          investigationStatus: undefined,
          investigationOutcome: 'fix_queued',
          investigationAttempts: 0,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '',
          investigationStatus: undefined,
          investigationOutcome: undefined,
          investigationAttempts: 2,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: 'session-1',
          investigationStatus: undefined,
          investigationOutcome: undefined,
          investigationAttempts: 0,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '   ',
          investigationStatus: undefined,
          investigationOutcome: undefined,
          investigationAttempts: 0,
        } as never),
      ).toBe(false);
    });

    it('promotes queued fixes without a pending approval into the needs-attention bucket', () => {
      expect(
        hasPendingInvestigationFixApproval('finding-1', [
          {
            status: 'pending',
            toolId: 'investigation_fix',
            targetId: 'finding-1',
            expiresAt: '2026-12-30T00:06:00Z',
          },
        ] as never),
      ).toBe(true);

      expect(
        doesFindingNeedAttention(
          {
            id: 'finding-1',
            status: 'active',
            investigationOutcome: 'fix_queued',
          } as never,
          [],
        ),
      ).toBe(true);

      expect(
        doesFindingNeedAttention(
          {
            id: 'finding-1',
            status: 'active',
            investigationOutcome: 'fix_queued',
          } as never,
          [
            {
              status: 'pending',
              toolId: 'investigation_fix',
              targetId: 'finding-1',
              expiresAt: '2026-12-30T00:06:00Z',
            },
          ] as never,
        ),
      ).toBe(false);

      expect(
        hasPendingInvestigationFixApproval(
          'finding-1',
          [
            {
              status: 'pending',
              toolId: 'investigation_fix',
              targetId: 'finding-1',
              expiresAt: '2026-03-01T00:06:00Z',
            },
          ] as never,
          Date.parse('2026-03-01T00:06:01Z'),
        ),
      ).toBe(false);

      expect(isPatrolInvestigationFixApproval({ toolId: 'investigation_fix' } as never)).toBe(true);
      expect(isPatrolInvestigationFixApproval({ toolId: 'run_command' } as never)).toBe(false);
    });
  });

  describe('loopStateColors', () => {
    it('has detected color', () => {
      expect(getFindingLoopStateBadgeClasses('detected')).toContain('blue');
      expect(getFindingLoopStateBadgeTone('detected')).toBe('info');
    });

    it('has resolved color', () => {
      expect(getFindingLoopStateBadgeClasses('resolved')).toContain('green');
      expect(getFindingLoopStateBadgeTone('resolved')).toBe('success');
    });

    it('has remediation_failed color', () => {
      expect(getFindingLoopStateBadgeClasses('remediation_failed')).toContain('red');
      expect(getFindingLoopStateBadgeTone('remediation_failed')).toBe('danger');
    });
  });

  describe('formatLoopState', () => {
    it('replaces underscores with spaces', () => {
      expect(formatFindingLoopState('in_progress')).toBe('in progress');
    });

    it('handles single word', () => {
      expect(formatFindingLoopState('detected')).toBe('detected');
    });

    it('handles multiple underscores', () => {
      expect(formatFindingLoopState('remediation_planned')).toBe('remediation planned');
    });
  });

  describe('Patrol workflow presentation', () => {
    const baseFinding = {
      id: 'finding-1',
      source: 'ai-patrol',
      status: 'active',
      resourceId: 'vm-101',
      resourceName: 'database-01',
      title: 'Disk pressure',
    } as const;

    it('does not invent a workflow badge for plain active Patrol findings', () => {
      expect(getFindingPatrolWorkflowPresentation(baseFinding)).toBeUndefined();
    });

    it('keeps Patrol runtime setup copy neutral to watch-only and paid control modes', () => {
      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol service',
          title: 'Pulse Patrol: Provider connection issue',
        }),
      ).toMatchObject({
        stage: 'attention',
        label: 'Fix Patrol setup',
        detail:
          'Patrol needs runtime or provider setup before it can check infrastructure reliably.',
        tone: 'info',
      });
    });

    it('promotes live governed approvals above generic review context', () => {
      expect(
        getFindingPatrolWorkflowPresentation(
          {
            ...baseFinding,
            investigationOutcome: 'fix_queued',
          },
          [
            {
              status: 'pending',
              toolId: 'investigation_fix',
              targetId: 'finding-1',
              expiresAt: '2026-06-20T18:15:00Z',
            },
          ],
          Date.parse('2026-06-20T18:00:00Z'),
        ),
      ).toMatchObject({
        stage: 'approval',
        label: 'Approve or reject',
        tone: 'warning',
      });
    });

    it('asks the operator to recover a queued fix when no live approval exists', () => {
      expect(
        getFindingPatrolWorkflowPresentation(
          {
            ...baseFinding,
            investigationOutcome: 'fix_queued',
          },
          [
            {
              status: 'pending',
              toolId: 'investigation_fix',
              targetId: 'finding-1',
              expiresAt: '2026-06-20T17:59:00Z',
            },
          ],
          Date.parse('2026-06-20T18:00:00Z'),
        ),
      ).toMatchObject({
        stage: 'approval',
        label: 'Review fix',
        tone: 'warning',
      });
    });

    it('separates rejected, executed, and verified outcomes into distinct operator next steps', () => {
      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          investigationOutcome: 'fix_rejected',
        }),
      ).toMatchObject({
        stage: 'attention',
        label: 'Decide follow-up',
      });

      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          investigationOutcome: 'fix_executed',
        }),
      ).toMatchObject({
        stage: 'verification',
        label: 'Verify outcome',
      });

      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          investigationOutcome: 'fix_verified',
        }),
      ).toMatchObject({
        stage: 'recorded',
        label: 'Outcome recorded',
      });
    });

    it('uses concrete labels for failed or input-needed Patrol outcomes', () => {
      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          investigationOutcome: 'fix_failed',
        }),
      ).toMatchObject({
        stage: 'attention',
        label: 'Fix failed',
      });

      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          investigationOutcome: 'needs_attention',
        }),
      ).toMatchObject({
        stage: 'attention',
        label: 'Needs input',
      });
    });

    it('does not attach the Patrol operations loop to threshold alerts', () => {
      expect(
        getFindingPatrolWorkflowPresentation({
          ...baseFinding,
          source: 'threshold',
        }),
      ).toBeUndefined();
    });
  });

  describe('Patrol finding display groups', () => {
    it('groups findings with the same resource identity while preserving sorted order', () => {
      const groups = buildPatrolFindingDisplayGroups([
        {
          id: 'critical',
          resourceId: 'storage:tower-array',
          resourceName: 'Tower Array',
          resourceType: 'storage',
          title: 'No parity protection',
        },
        {
          id: 'warning',
          resourceId: 'storage:tower-array',
          resourceName: 'Tower Array',
          resourceType: 'storage',
          title: 'High usage',
        },
        {
          id: 'provider',
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol service',
          resourceType: 'service',
          title: 'Provider connection issue',
        },
      ]);

      expect(groups).toHaveLength(2);
      expect(groups[0]).toMatchObject({
        resourceKey: 'subject:tower array (storage)',
        resourceLabel: 'Tower Array (storage)',
      });
      expect(groups[0].primaryFinding.id).toBe('critical');
      expect(groups[0].findings.map((finding) => finding.id)).toEqual(['critical', 'warning']);
      expect(groups[0].relatedFindings.map((finding) => finding.id)).toEqual(['warning']);
      expect(groups[1].findings.map((finding) => finding.id)).toEqual(['provider']);
      expect(groups[1].relatedFindings).toEqual([]);
    });

    it('groups display-identical resources even when backend finding ids differ', () => {
      const groups = buildPatrolFindingDisplayGroups([
        {
          id: 'array-risk',
          resourceId: 'unraid:array:tower',
          resourceName: 'Tower Array',
          resourceType: 'storage',
          title: 'No parity protection',
        },
        {
          id: 'pool-usage',
          resourceId: 'unraid:pool:tower-array',
          resourceName: 'Tower Array',
          resourceType: 'storage',
          title: 'High usage',
        },
      ]);

      expect(groups).toHaveLength(1);
      expect(groups[0].findings.map((finding) => finding.id)).toEqual(['array-risk', 'pool-usage']);
    });

    it('keeps findings without resource identity separate instead of inventing duplicates', () => {
      const groups = buildPatrolFindingDisplayGroups([
        { id: 'a', resourceId: '', resourceName: '', resourceType: '', title: 'First' },
        { id: 'b', resourceId: '', resourceName: '', resourceType: '', title: 'Second' },
      ]);

      expect(groups).toHaveLength(2);
      expect(groups.map((group) => group.resourceKey)).toEqual(['finding:a', 'finding:b']);
    });
  });

  describe('lifecycleLabels', () => {
    it('has detected label', () => {
      expect(formatFindingLifecycleType('detected')).toBe('Detected');
    });

    it('has resolved label', () => {
      expect(formatFindingLifecycleType('resolved')).toBe('Resolved');
    });

    it('has snoozed label', () => {
      expect(formatFindingLifecycleType('snoozed')).toBe('Snoozed');
    });

    it('has dismissed label', () => {
      expect(formatFindingLifecycleType('dismissed')).toBe('Dismissed');
    });

    it('labels a key-collision content replacement as a re-detection with different details', () => {
      // Backend emits content_replaced when a same-key re-detection's text
      // is substantially different from the existing finding (a key
      // collision); the timeline label must explain the shift in operator
      // language rather than the raw identifier fallback.
      expect(formatFindingLifecycleType('content_replaced')).toBe(
        'Re-detected with different details',
      );
    });
  });

  describe('formatLifecycleType', () => {
    it('returns known label for known value', () => {
      expect(formatFindingLifecycleType('detected')).toBe('Detected');
    });

    it('replaces underscores for unknown value', () => {
      expect(formatFindingLifecycleType('some_unknown_event')).toBe('some unknown event');
    });

    it('handles auto_resolved', () => {
      expect(formatFindingLifecycleType('auto_resolved')).toBe('Auto-resolved');
    });

    it('handles verification_passed', () => {
      expect(formatFindingLifecycleType('verification_passed')).toBe('Fix verified');
    });
  });

  describe('resolutionReasonPresentation', () => {
    it('returns canonical threshold resolution reasons', () => {
      expect(
        getFindingResolutionReason(
          {
            isThreshold: true,
            source: 'threshold',
            alertType: 'cpu',
            investigationOutcome: undefined,
          } as never,
          '2m ago',
        ),
      ).toBe('CPU returned to normal 2m ago');
    });

    it('formats findings as a Markdown summary mirroring the seven-question schema for paste-into-chat sharing', () => {
      // The Copy summary action must produce paste-ready Markdown that
      // mirrors render order: severity + title + resource header, then
      // description, impact, recommendation, plus trust signals
      // (confidence, regression count). Anything beyond that — evidence,
      // rollback plans, internal IDs — is deferred to the Discuss flow
      // because that's conversation context, not "share this finding"
      // context. Pin the exact shape so the operator-facing output
      // doesn't drift between releases.
      const formatted = formatFindingForClipboard({
        severity: 'warning',
        title: 'Disk pressure',
        resourceName: 'db-01',
        resourceType: 'vm',
        description: 'Datastore is at 92% capacity.',
        impact: 'Backups will be skipped if free space drops below 5%.',
        recommendation: 'Free 200GB before the next backup window.',
        investigationRecord: { confidence: 'medium' } as never,
        regressionCount: 2,
      } as never);
      expect(formatted).toContain('**[WARNING] Disk pressure**');
      expect(formatted).toContain('Resource: db-01 (vm)');
      expect(formatted).toContain('Description: Datastore is at 92% capacity.');
      expect(formatted).toContain('Impact: Backups will be skipped if free space drops below 5%.');
      expect(formatted).not.toContain('Recommendation: Free 200GB before the next backup window.');
      expect(formatted).toContain('Confidence: medium');
      expect(formatted).toContain('Regressed: 2×');
    });

    it('omits empty fields cleanly when the finding has not been investigated yet', () => {
      // A threshold-only finding (no InvestigationRecord, no Impact) must
      // still produce useful Markdown — the formatter must not include
      // empty "Impact:" or "Confidence:" lines that read as alarm signals.
      const formatted = formatFindingForClipboard({
        severity: 'critical',
        title: 'Host offline',
        resourceName: 'pve-01',
        resourceType: 'node',
        description: 'Host is not responding.',
      } as never);
      expect(formatted).toContain('**[CRITICAL] Host offline**');
      expect(formatted).toContain('Resource: pve-01 (node)');
      expect(formatted).toContain('Description: Host is not responding.');
      expect(formatted).not.toContain('Impact:');
      expect(formatted).not.toContain('Recommendation:');
      expect(formatted).not.toContain('Confidence:');
      expect(formatted).not.toContain('Regressed:');
    });

    it('returns canonical patrol resolution reasons', () => {
      expect(
        getFindingResolutionReason(
          {
            isThreshold: false,
            source: 'ai-patrol',
            alertType: undefined,
            investigationOutcome: 'fix_verified',
          } as never,
          '1h ago',
        ),
      ).toBe('Fixed by Patrol 1h ago');
    });

    it('attributes operator-driven Mark resolved closures to "Resolved by you"', () => {
      // Slice 22 added the manual Mark resolved button which sets
      // auto_resolved=false. The resolution-reason copy must distinguish
      // operator-driven closure from Pulse's auto-detection so the timeline
      // reads honestly. This branch must take priority over the
      // category-specific "Condition cleared" / "CPU returned to normal"
      // copy (those describe Pulse's auto-detection).
      expect(
        getFindingResolutionReason(
          {
            isThreshold: true,
            source: 'threshold',
            alertType: 'cpu',
            investigationOutcome: undefined,
            autoResolved: false,
          } as never,
          '5m ago',
        ),
      ).toBe('Resolved by you 5m ago');

      expect(
        getFindingResolutionReason(
          {
            isThreshold: false,
            source: 'ai-patrol',
            alertType: undefined,
            investigationOutcome: 'cannot_fix',
            autoResolved: false,
          } as never,
          '20m ago',
        ),
      ).toBe('Resolved by you 20m ago');
    });

    it('keeps Patrol fix-applied copy when Pulse closed the loop autonomously', () => {
      // Patrol's own fix outcomes (fix_verified, fix_executed, resolved) are
      // more specific than "auto-resolved" — they describe Pulse's actual
      // remediation. The autoResolved branch must NOT override those.
      expect(
        getFindingResolutionReason(
          {
            isThreshold: false,
            source: 'ai-patrol',
            alertType: undefined,
            investigationOutcome: 'fix_verified',
            autoResolved: true,
          } as never,
          '2h ago',
        ),
      ).toBe('Fixed by Patrol 2h ago');
    });
  });

  describe('operator-state dismiss cause attribution', () => {
    it('reports the most recent operator_state_cause from the lifecycle', () => {
      // Lifecycle scan must be newest-first so a later auto-dismiss
      // overrides an older manual one. Mirror the Go-side helper's
      // contract — the rendering branch depends on it for the badge.
      expect(
        getOperatorStateDismissCause({
          lifecycle: [
            {
              at: '2026-04-01T00:00:00Z',
              type: 'dismissed',
              metadata: { reason: 'expected_behavior' },
            },
            { at: '2026-04-02T00:00:00Z', type: 'undismissed' },
            {
              at: '2026-04-03T00:00:00Z',
              type: 'dismissed',
              metadata: { operator_state_cause: 'maintenance_window' },
            },
          ],
        } as never),
      ).toBe('maintenance_window');
    });

    it('returns empty when the most recent dismissed event is a manual operator dismissal', () => {
      // A manual dismiss that supersedes an earlier auto-dismiss must
      // NOT report the stale auto-dismiss cause — that would falsely
      // badge the finding as auto-suppressed when the operator
      // overrode it.
      expect(
        getOperatorStateDismissCause({
          lifecycle: [
            {
              at: '2026-04-01T00:00:00Z',
              type: 'dismissed',
              metadata: { operator_state_cause: 'maintenance_window' },
            },
            { at: '2026-04-02T00:00:00Z', type: 'undismissed' },
            {
              at: '2026-04-03T00:00:00Z',
              type: 'dismissed',
              metadata: { reason: 'expected_behavior' },
            },
          ],
        } as never),
      ).toBe('');
    });

    it('returns empty for findings without lifecycle entries', () => {
      expect(getOperatorStateDismissCause({ lifecycle: undefined } as never)).toBe('');
      expect(getOperatorStateDismissCause({ lifecycle: [] } as never)).toBe('');
    });

    it('formats canonical operator-state causes as human labels', () => {
      expect(formatOperatorStateDismissCauseLabel('maintenance_window')).toBe('maintenance');
      expect(formatOperatorStateDismissCauseLabel('intentionally_offline')).toBe(
        'intentionally offline',
      );
      // Unknown causes return empty so render code can gate cleanly
      // and not show a bogus "auto: <unknown>" badge.
      expect(formatOperatorStateDismissCauseLabel('something_new')).toBe('');
      expect(formatOperatorStateDismissCauseLabel('')).toBe('');
    });
  });

  describe('FindingsPanel operator-state dismiss badge wiring', () => {
    it('renders an "auto: <cause>" badge for operator-state-driven dismissals', () => {
      // Both manual and auto-dismissed findings show DismissedReason=
      // expected_behavior; the auto-dismiss is distinguished by the
      // operator_state_cause lifecycle metadata. Pin the wiring so the
      // FindingsPanel renders the parallel badge through the canonical
      // helper, not by re-implementing the lifecycle scan inline.
      expect(findingsPanelSource).toContain('getOperatorStateDismissCause(finding)');
      expect(findingsPanelSource).toContain('formatOperatorStateDismissCauseLabel(');
      expect(findingsPanelSource).toContain('auto:');
      expect(findingsPanelSource).toContain(
        'formatOperatorStateDismissCauseLabel(getOperatorStateDismissCause(finding))',
      );
      // The badge sits next to the existing dismissed-reason badge in
      // source order so the two pieces of information read together
      // (the reason and the attribution).
      const dismissedReasonIndex = findingsPanelSource.indexOf(
        '({formatIdentifierLabel(finding.dismissedReason)})',
      );
      const autoBadgeIndex = findingsPanelSource.indexOf('auto:');
      expect(dismissedReasonIndex).toBeGreaterThan(0);
      expect(autoBadgeIndex).toBeGreaterThan(0);
      expect(autoBadgeIndex).toBeGreaterThan(dismissedReasonIndex);
    });
  });
});
