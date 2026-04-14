import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  buildFindingFilterOptions,
  formatFindingLifecycleType,
  formatFindingLoopState,
  getFindingActiveRuntimeSortOrder,
  getFindingEmptyStateCopy,
  getFindingManualControlsPresentation,
  getFindingPrimaryActionPresentation,
  getFindingSubjectPresentation,
  getFindingTitlePresentation,
  getPatrolFindingClassification,
  getFindingSeverityCompactLabel,
  getFindingSeveritySortOrder,
  getFindingResolutionReason,
  getFindingRecencyPresentation,
  getFindingLoopStateBadgeClasses,
  getFindingSeverityBadgeClasses,
  getFindingSeverityPresentation,
  getFindingStatusBadgeClasses,
  getFindingStatusLabel,
  getFindingSeverityToneClasses,
  getPatrolFindingsBadgePresentation,
  getFindingSourceBadgeClasses,
  getFindingSourceLabel,
  hasFindingInvestigationDetails,
  hasPendingInvestigationFixApproval,
  isPatrolInvestigationFixApproval,
  getInvestigationOutcomeBadgeClasses,
  getInvestigationOutcomeLabel,
  getInvestigationOutcomeSortOrder,
  getInvestigationStatusLabel,
  getInvestigationStatusBadgeClasses,
  doesFindingNeedAttention,
} from '@/utils/aiFindingPresentation';

const findingsPanelSource = readFileSync(
  resolve(__dirname, '..', 'FindingsPanel.tsx'),
  'utf-8',
);
const patrolWorkspaceSource = readFileSync(
  resolve(__dirname, '..', '..', '..', 'features', 'patrol', 'PatrolIntelligenceWorkspace.tsx'),
  'utf-8',
);

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
          title: 'Pulse Patrol: Insufficient API credits',
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
        uppercase: true,
      });
    });

    it('renders runtime-qualified severity badges for patrol runtime findings', () => {
      expect(
        getFindingSeverityPresentation({
          severity: 'warning',
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Insufficient API credits',
        }),
      ).toEqual({
        label: 'Runtime issue',
        badgeClasses:
          'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
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
            title: 'Pulse Patrol: Insufficient API credits',
          },
        ]),
      ).toEqual({
        toneClasses: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
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
        toneClasses: getFindingSeverityToneClasses('warning'),
      });
    });
  });

  describe('findingStatusPresentation', () => {
    it('returns canonical badge classes', () => {
      expect(getFindingStatusBadgeClasses('resolved')).toContain('green');
      expect(getFindingStatusBadgeClasses('snoozed')).toContain('blue');
      expect(getFindingStatusBadgeClasses('dismissed')).toContain('bg-surface-alt');
      expect(getFindingStatusBadgeClasses('unexpected')).toContain('bg-surface-alt');
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
    it('classifies ai-service findings as patrol runtime issues', () => {
      expect(
        getPatrolFindingClassification({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Insufficient API credits',
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
          title: 'Pulse Patrol: Insufficient API credits',
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
          title: 'Pulse Patrol: Insufficient API credits',
        }),
      ).toEqual({
        label: 'Open Patrol provider settings',
        href: '/settings/system-ai',
      });
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
          title: 'Insufficient API credits',
        }),
      ).toEqual({
        label: 'Open Patrol provider settings',
        href: '/settings/system-ai',
      });
    });
  });

  describe('findingTitlePresentation', () => {
    it('strips the product prefix from Patrol runtime finding titles', () => {
      expect(
        getFindingTitlePresentation({
          resourceId: 'ai-service',
          resourceName: 'Pulse Patrol Service',
          title: 'Pulse Patrol: Insufficient API credits',
        }),
      ).toEqual({
        label: 'Insufficient API credits',
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
          title: 'Pulse Patrol: Insufficient API credits',
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
      expect(findingsPanelSource).toContain("href={action().href}");
    });

    it('resolves unified resources and shared route links for expanded finding handoffs', () => {
      expect(findingsPanelSource).toContain("import { useResources } from '@/hooks/useResources';");
      expect(findingsPanelSource).toContain(
        "import { buildResolvedResourceSurfaceLinks } from '@/routing/resourceLinks';",
      );
      expect(findingsPanelSource).toContain('const { get: getResource } = useResources();');
      expect(findingsPanelSource).toContain('buildResolvedResourceSurfaceLinks({');
      expect(findingsPanelSource).toContain('resource: getResource(finding.resourceId)');
      expect(findingsPanelSource).toContain("{link.compactLabel}");
    });

    it('routes generic finding controls through the shared manual-controls helper', () => {
      expect(findingsPanelSource).toContain('getFindingManualControlsPresentation(finding)');
      expect(findingsPanelSource).toContain('manualControls.acknowledge');
      expect(findingsPanelSource).toContain('manualControls.snooze');
      expect(findingsPanelSource).toContain('manualControls.dismiss');
    });

    it('loads remediation artifacts through the shared Patrol store instead of probing the API directly', () => {
      expect(findingsPanelSource).toContain('aiIntelligenceStore.loadRemediationPlans()');
      expect(findingsPanelSource).toContain('aiIntelligenceStore.remediationPlans');
      expect(findingsPanelSource).not.toContain('AIAPI.getRemediationPlans()');
    });

    it('routes same-severity ordering through the shared patrol runtime sort helper', () => {
      expect(findingsPanelSource).toContain('getFindingActiveRuntimeSortOrder(a)');
      expect(findingsPanelSource).toContain('getFindingActiveRuntimeSortOrder(b)');
    });

    it('only shows the sort control when there are multiple Patrol findings to sort', () => {
      expect(findingsPanelSource).toContain('<Show when={patrolFindings().length > 1}>');
      expect(findingsPanelSource).toContain('<option value="severity">By Severity</option>');
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
        "() => props.runSnapshot !== undefined && props.filterFindingIds === undefined",
      );
    });

    it('uses explicit textual separators in finding metadata instead of relying on visual spacing', () => {
      expect(findingsPanelSource).toContain("{' · '}acknowledged");
      expect(findingsPanelSource).toContain("{' · '}last investigated");
      expect(findingsPanelSource).toContain("{' · '}snoozed until");
    });

    it('uses explicit textual separators for patrol tab badges instead of css-only spacing', () => {
      expect(patrolWorkspaceSource).toContain("aria-hidden=\"true\"");
      expect(patrolWorkspaceSource).toContain("{' '}");
      expect(patrolWorkspaceSource).toContain('{state.findingsTabBadgeCount()}');
      expect(patrolWorkspaceSource).toContain('{state.displayRunHistory().length}');
    });

    it('routes the findings tab badge tone through the shared patrol findings badge helper', () => {
      expect(patrolWorkspaceSource).toContain('getPatrolFindingsBadgePresentation');
      expect(patrolWorkspaceSource).toContain('state.findingsTabBadgeFindings()');
      expect(patrolWorkspaceSource).toContain('findingsBadgePresentation().toneClasses');
    });

    it('does not stack a detected loop-state badge on top of acknowledged active findings', () => {
      expect(findingsPanelSource).toContain(
        "!(finding.acknowledgedAt && finding.loopState === 'detected')",
      );
    });

    it('uses canonical finding recency presentation instead of raw detected timestamps for active rows', () => {
      expect(findingsPanelSource).toContain('const recency = getFindingRecencyPresentation(finding);');
      expect(findingsPanelSource).toContain('{subject.label} - {recency.label} ');
      expect(findingsPanelSource).toContain('{formatTime(recency.timestamp)}');
    });

    it('routes row badges through the shared severity presentation helper', () => {
      expect(findingsPanelSource).toContain(
        'const severityPresentation = getFindingSeverityPresentation(finding);',
      );
      expect(findingsPanelSource).toContain('severityPresentation.badgeClasses');
      expect(findingsPanelSource).toContain('severityPresentation.label');
    });

    it('routes visible finding titles through the shared title presentation helper', () => {
      expect(findingsPanelSource).toContain('const title = getFindingTitlePresentation(finding);');
      expect(findingsPanelSource).toContain('{title.label}');
      expect(findingsPanelSource).toContain(
        'findingTitle={getFindingTitlePresentation(finding).label}',
      );
    });

    it('uses the shared finding subject presentation instead of raw patrol service resource tokens', () => {
      expect(findingsPanelSource).toContain('const subject = getFindingSubjectPresentation(finding);');
      expect(findingsPanelSource).toContain('{subject.label} - {recency.label}');
      expect(findingsPanelSource).not.toContain('{finding.resourceName} ({finding.resourceType})');
    });
  });

  describe('sourceColors', () => {
    it('has threshold color', () => {
      expect(getFindingSourceBadgeClasses('threshold')).toContain('orange');
    });

    it('has ai-patrol color', () => {
      expect(getFindingSourceBadgeClasses('ai-patrol')).toContain('blue');
    });

    it('has ai-chat color', () => {
      expect(getFindingSourceBadgeClasses('ai-chat')).toContain('teal');
    });
  });

  describe('investigationStatusColors', () => {
    it('has pending color', () => {
      expect(getInvestigationStatusBadgeClasses('pending')).toContain('bg-surface-alt');
    });

    it('has running color', () => {
      expect(getInvestigationStatusBadgeClasses('running')).toContain('blue');
    });

    it('has completed color', () => {
      expect(getInvestigationStatusBadgeClasses('completed')).toContain('green');
    });

    it('has failed color', () => {
      expect(getInvestigationStatusBadgeClasses('failed')).toContain('red');
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
      expect(getInvestigationOutcomeBadgeClasses('cannot_fix')).toContain('bg-surface-alt');
      expect(getInvestigationOutcomeSortOrder('fix_failed')).toBe(0);
      expect(getInvestigationOutcomeSortOrder('needs_attention')).toBe(1);
      expect(getInvestigationOutcomeSortOrder('fix_queued')).toBe(2);
      expect(getInvestigationOutcomeSortOrder(undefined)).toBe(3);
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
	              expiresAt: '2026-04-30T00:06:00Z',
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
	              expiresAt: '2026-04-30T00:06:00Z',
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

      expect(isPatrolInvestigationFixApproval({ toolId: 'investigation_fix' } as never)).toBe(
        true,
      );
      expect(isPatrolInvestigationFixApproval({ toolId: 'run_command' } as never)).toBe(false);
    });
  });

  describe('loopStateColors', () => {
    it('has detected color', () => {
      expect(getFindingLoopStateBadgeClasses('detected')).toContain('blue');
    });

    it('has resolved color', () => {
      expect(getFindingLoopStateBadgeClasses('resolved')).toContain('green');
    });

    it('has remediation_failed color', () => {
      expect(getFindingLoopStateBadgeClasses('remediation_failed')).toContain('red');
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
  });
});
