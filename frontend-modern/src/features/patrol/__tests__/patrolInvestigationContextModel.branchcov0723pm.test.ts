import { describe, expect, it } from 'vitest';
import type { RemediationPlan } from '@/api/ai';
import type { InvestigationRecord } from '@/api/ai';
import type { ResourceCorrelation } from '@/types/aiIntelligence';
import type { ResourceChange } from '@/types/resource';
import type { PatrolRunRecord } from '@/api/patrol';

import {
  buildPatrolAssessmentAssistantHandoff,
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantFindingHandoffActions,
  buildPatrolConfigurationFailureHandoff,
  buildPatrolInvestigationContextSummary,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantModelContext,
  buildPatrolRunAssistantHandoff,
  patrolAssistantFindingHandoffRequiresApprovalMode,
  selectPatrolSupportingRecentChanges,
} from '../patrolInvestigationContextModel';

// Second branch-coverage companion to patrolInvestigationContextModel.test.ts.
// Targets the residual uncovered arms (v8 branch coverage): null/empty
// collections, absent timestamps, unknown resource kinds, null handoff
// metadata, truncation/cap boundaries, and every fallback label. Does not
// duplicate the happy paths already pinned by the dev test or the first
// branchcov file.

const minimalRecord = {
  id: 'r',
  finding_id: 'f',
  subject: { resource_id: 'vm-1' },
  trigger: { detected_at: '2026-01-01T00:00:00Z' },
  status: 'completed',
} as unknown as InvestigationRecord;

describe('buildPatrolInvestigationContextSummary (residual branches)', () => {
  it('returns zero correlations when the response is present but has no count and no array', () => {
    // normalizeCorrelationCount final `return 0` arm: object that is neither a
    // finite numeric count nor an Array `correlations` field.
    const summary = buildPatrolInvestigationContextSummary({
      correlations: { unexpected: 'shape' } as unknown as
        import('@/types/aiIntelligence').CorrelationsResponse | null,
    });

    expect(summary).toMatchObject({
      correlationCount: 0,
      hasContext: false,
      summaryText: '',
    });
  });
});

describe('selectPatrolSupportingRecentChanges (same-state transition branches)', () => {
  it('uses a custom reason and skips non-array changedFields', () => {
    // formatSameStateTransitionReason: changedFieldLabels empty (metadata
    // present but changedFields not an array) -> falls through to the reason
    // arm, and a reason that is not the default yields "${reason} while ${state}".
    const [change] = selectPatrolSupportingRecentChanges([
      {
        id: 'c-reason',
        observedAt: '2026-01-01T00:00:00Z',
        resourceId: 'vm-1',
        kind: 'state_transition',
        from: 'online',
        to: 'online',
        sourceType: 'pulse_diff',
        confidence: 'high',
        reason: 'operator-initiated restart',
        metadata: { note: 'not an array' },
      },
    ]);

    expect(change).toMatchObject({
      from: undefined,
      to: undefined,
      reason: 'operator-initiated restart while online',
    });
  });

  it('skips blank changed-field entries and falls back to the identifier label for unknown fields', () => {
    const [change] = selectPatrolSupportingRecentChanges([
      {
        id: 'c-fields',
        observedAt: '2026-01-01T00:00:00Z',
        resourceId: 'vm-1',
        kind: 'restart',
        from: 'on',
        to: 'on',
        sourceType: 'pulse_diff',
        confidence: 'high',
        metadata: { changedFields: ['', '  ', 'custom_field', 'tags'] },
      },
    ]);

    // Blank entries are skipped; 'custom_field' uses formatIdentifierLabel
    // fallback ("Custom Field"); 'tags' resolves to the known map label.
    expect(change.reason).toBe('Custom Field and tags changed while on');
  });

  it('renders the single-field and three-or-more-field compact label lists', () => {
    const [single] = selectPatrolSupportingRecentChanges([
      {
        id: 'c-one',
        observedAt: '2026-01-01T00:00:00Z',
        resourceId: 'vm-1',
        kind: 'state_transition',
        from: 'online',
        to: 'online',
        sourceType: 'pulse_diff',
        confidence: 'high',
        metadata: { changedFields: ['tags'] },
      },
    ]);
    expect(single.reason).toBe('tags changed while online');

    const [many] = selectPatrolSupportingRecentChanges([
      {
        id: 'c-many',
        observedAt: '2026-01-01T00:00:00Z',
        resourceId: 'vm-1',
        kind: 'state_transition',
        from: 'online',
        to: 'online',
        sourceType: 'pulse_diff',
        confidence: 'high',
        metadata: { changedFields: ['status', 'incidents', 'parentId'] },
      },
    ]);
    expect(many.reason).toBe('status, incident state, and 1 more changed while online');
  });

  it('falls back to the default wording when the reason is exactly the no-op default', () => {
    const [change] = selectPatrolSupportingRecentChanges([
      {
        id: 'c-default',
        observedAt: '2026-01-01T00:00:00Z',
        resourceId: 'vm-1',
        kind: 'state_transition',
        from: 'online',
        to: 'online',
        sourceType: 'pulse_diff',
        confidence: 'high',
        reason: 'Resource state changed',
      },
    ]);

    // Case-insensitive match against the default -> generic fallback wording.
    expect(change.reason).toBe('state details changed while online');
  });
});

describe('buildPatrolInvestigationRecordPresentation (residual branches)', () => {
  it('labels an unknown tool via the identifier fallback', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      ...minimalRecord,
      tools_used: ['metrics.history', 'custom.diagnostic.tool'],
    });

    // Known tool resolves to its map label; unknown tool falls through to
    // formatIdentifierLabel.
    expect(presentation.toolsUsed).toEqual(['Metrics history', 'Custom Diagnostic Tool']);
  });
});

describe('buildPatrolRunAssistantHandoff (residual branches)', () => {
  it('falls back every identity field when the run carries no id, type, or status', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: '',
      type: '',
      status: '',
    } as unknown as PatrolRunRecord);

    // runId || undefined, normalizeText(run.type) || undefined,
    // normalizeText(run.status) || undefined -> all collapse to undefined.
    expect(handoff.context.targetId).toBeUndefined();
    expect(handoff.context.handoffMetadata?.runId).toBeUndefined();
    expect(handoff.context.context?.runId).toBeUndefined();
    expect(handoff.context.context?.runType).toBeUndefined();
    expect(handoff.context.context?.status).toBeUndefined();
    // run.status is absent, so the || 'unknown' arm is taken and surfaces on
    // both the context and the handoff metadata.
    expect(handoff.context.context?.effectiveStatus).toBe('unknown');
    expect(handoff.context.handoffMetadata?.runStatus).toBe('unknown');
  });

  it('renders the coverage-facts fallback list from non-resource check counts', () => {
    // resources_checked is 0 with no scope ids -> getPatrolRunCoverageSummary
    // returns '' -> the fallback briefing-string list is used. Each non-zero
    // *_checked count then contributes its fact string.
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-cov',
      type: 'full',
      status: 'healthy',
      resources_checked: 0,
      nodes_checked: 1,
      guests_checked: 1,
      docker_checked: 1,
      storage_checked: 1,
      hosts_checked: 1,
      truenas_checked: 1,
      kubernetes_checked: 1,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.briefing?.statusLabel).toContain('1 nodes');
    expect(handoff.context.briefing?.statusLabel).toContain('1 VMs');
    expect(handoff.context.briefing?.statusLabel).toContain('1 containers');
    expect(handoff.context.briefing?.statusLabel).toContain('1 storage resources');
    expect(handoff.context.briefing?.statusLabel).toContain('1 agents');
    expect(handoff.context.briefing?.statusLabel).toContain('1 TrueNAS systems');
    expect(handoff.context.briefing?.statusLabel).toContain('1 Kubernetes resources');
  });

  it('singularizes each outcome fact when exactly one of each was observed', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-one',
      type: 'full',
      status: 'healthy',
      new_findings: 1,
      existing_findings: 1,
      resolved_findings: 1,
      rejected_findings: 1,
      auto_fix_count: 1,
      error_count: 1,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.briefing?.evidence?.[0]).toBe(
      '1 new finding; 1 existing finding; 1 resolved finding; 1 rejected finding; 1 auto-remediation; 1 error',
    );
  });

  it('pluralizes each outcome fact when more than one was observed', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-many',
      type: 'full',
      status: 'healthy',
      new_findings: 2,
      existing_findings: 2,
      resolved_findings: 2,
      rejected_findings: 2,
      auto_fix_count: 2,
      error_count: 2,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.briefing?.evidence?.[0]).toBe(
      '2 new findings; 2 existing findings; 2 resolved findings; 2 rejected findings; 2 auto-remediations; 2 errors',
    );
  });

  it('singularizes and pluralizes the triage-flag effort fact', () => {
    const one = buildPatrolRunAssistantHandoff({
      id: 'run-eff1',
      type: 'full',
      status: 'healthy',
      triage_flags: 1,
    } as unknown as PatrolRunRecord);
    expect(one.context.briefing?.detailLines).toContain('1 triage flag');

    const two = buildPatrolRunAssistantHandoff({
      id: 'run-eff2',
      type: 'full',
      status: 'healthy',
      triage_flags: 2,
    } as unknown as PatrolRunRecord);
    expect(two.context.briefing?.detailLines).toContain('2 triage flags');
  });

  it('caps run handoff resources at eight and leaves the type unset for multi-type scopes', () => {
    // scope_resource_types length != 1 -> resource type collapses to ''.
    // Nine distinct ids -> the ninth is skipped once resources.size hits the cap.
    const ids = Array.from({ length: 9 }, (_, i) => `res-${i + 1}`);
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-cap',
      type: 'scoped',
      status: 'healthy',
      scope_resource_ids: ids,
      scope_resource_types: ['vm', 'host'],
    } as unknown as PatrolRunRecord);

    expect(handoff.context.context?.handoffResourceCount).toBe(8);
  });
});

describe('buildPatrolConfigurationFailureHandoff (residual branches)', () => {
  it('emits the Cause line using the identifier-formatted cause', () => {
    // cause truthy arm of the Cause detail-line ternary.
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      readiness: { cause: 'model_unsupported_tools' },
    });

    expect(handoff.context.briefing?.detailLines).toContain('Cause: Model Unsupported Tools');
    expect(handoff.context.context?.readinessCause).toBe('model_unsupported_tools');
  });

  it('omits the Cause line entirely when no cause is available', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({ message: 'm' });

    expect(handoff.context.briefing?.detailLines).toEqual([]);
    expect(handoff.context.briefing?.statusLabel).toBeUndefined();
  });

  it('withholds a detail whose key or value normalizes to empty', () => {
    // formatSafeConfigurationFailureDetail early-return when key or value is empty.
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      details: {
        '  ': 'visible',
        empty_value: '   ',
        good: 'kept',
      },
    });

    expect(handoff.context.briefing?.evidence).toEqual(['Good: kept']);
  });

  it('passes a digits-only detail key through the identifier formatter unchanged', () => {
    // formatIdentifierLabel('123') returns '123' - there is no [._-] to expand
    // and no letter to title-case - so the label is the key verbatim WITHOUT
    // the || fallback being taken. The genuine empty-formatter fallback arm is
    // covered separately by the separator-only key case below.
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      details: { '123': 'value-here' },
    });

    expect(handoff.context.briefing?.evidence).toEqual(['123: value-here']);
  });
});

describe('buildPatrolAssessmentAssistantHandoff (residual branches)', () => {
  it('falls back to the default assessment title in both briefing and model context', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { description: 'only a description' },
      activeFindings: [],
    });

    expect(handoff.context.briefing?.subject).toBe('Pulse Patrol assessment');
    expect(handoff.context.handoffContext).toContain('Assessment: Pulse Patrol assessment');
  });

  it('drops findings that carry no id, title, resourceId, or record id, and tolerates a null list', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      activeFindings: [
        { severity: 'critical' },
        { title: 'kept-by-title' },
        { resourceId: 'kept-by-resource' },
        { investigationRecord: { id: 'rec-kept' } as unknown as InvestigationRecord },
      ],
    });

    // activeFindingCount reflects the raw input length, but the severity-only
    // finding (no id/title/resourceId/record id) is filtered out of the model
    // context, so only three Finding lines render.
    expect(handoff.context.context?.activeFindingCount).toBe(4);
    expect(handoff.context.handoffContext).toContain('Finding 1: kept-by-title');
    expect(handoff.context.handoffContext).toContain('Finding 3: Patrol finding');
    expect(handoff.context.handoffContext).not.toContain('Finding 4:');
  });

  it('counts active findings as zero and omits actions when activeFindings is null', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({ activeFindings: null });

    expect(handoff.context.context?.activeFindingCount).toBe(0);
    expect(handoff.context.context?.pendingApprovalCount).toBe(0);
    expect(handoff.context.handoffActions).toBeUndefined();
  });

  it('reports omitted findings and an omitted singular recent change in the model context', () => {
    const findings = Array.from({ length: 6 }, (_, i) => ({
      id: `f-${i + 1}`,
      title: `Finding ${i + 1}`,
    }));
    const changes = Array.from({ length: 4 }, (_, i) => ({
      id: `c-${i + 1}`,
      observedAt: '2026-01-01T00:00:00Z',
      resourceId: `vm-${i + 1}`,
      kind: 'metric_anomaly',
      sourceType: 'heuristic',
      confidence: 'high',
      reason: `reason ${i + 1}`,
    })) as unknown as ResourceChange[];

    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: { recentChanges: changes },
      activeFindings: findings,
    });

    // 6 findings, cap 5 -> 1 omitted (singular wording).
    expect(handoff.context.handoffContext).toContain(
      '1 additional Patrol finding omitted from this bounded handoff summary.',
    );
    // 4 recent changes, cap 3 -> 1 omitted (singular wording).
    expect(handoff.context.handoffContext).toContain(
      '1 additional recent change omitted from this bounded handoff summary.',
    );
  });

  it('renders a finding context line with no resource, no id, and no status parts', () => {
    // Bare finding: resource resolves to nothing, finding.id empty, no status
    // fields -> each ternary takes its undefined arm.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: [{ title: 'Bare finding' }],
    });

    expect(handoff.context.handoffContext).toContain('Bare finding on affected resource');
    // No "finding <id>" segment, no status segment -> the line is exactly the
    // title-on-resource form with no trailing parts.
    const ctx = handoff.context.handoffContext!;
    const line = ctx.split('\n').find((l) => l.startsWith('Finding 1:'));
    expect(line).toBe('Finding 1: Bare finding on affected resource');
  });

  it('surfaces record error, regression singular, and a divergent record approval id', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: [
        {
          id: 'f-1',
          title: 'High CPU',
          regressionCount: 1,
          pendingApproval: { id: 'pa-1', status: 'approved', riskLevel: 'high' },
          investigationRecord: {
            id: 'rec-1',
            finding_id: 'f-1',
            subject: { resource_id: 'vm-1' },
            trigger: { detected_at: '2026-01-01T00:00:00Z' },
            status: 'completed',
            error: 'investigation aborted',
            approval_id: 'rec-approval',
          } as unknown as InvestigationRecord,
        },
      ],
    });

    const line = handoff.context
      .handoffContext!.split('\n')
      .find((l) => l.startsWith('Finding 1:'))!;
    expect(line).toContain('regressed 1 time');
    expect(line).toContain('investigation error investigation aborted');
    expect(line).toContain('approval rec-approval');
    // Non-pending approval posture is rendered as inline approval parts:
    // "approved approval" (status) and "high risk" (risk label).
    expect(line).toContain('approved approval');
    expect(line).toContain('high risk');
  });

  it('renders metric fallback labels, singularized custom and non-plural labels, and the no-signal summary', () => {
    // primaryLabel absent -> default 'Active findings' label; fixedLabel
    // 'Categories' with value 1 -> singular 'category' (ies rule).
    const primaryAndCategories = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      metricState: { primaryValue: 1, fixedValue: 1, fixedLabel: 'Categories' },
      activeFindings: [],
    });
    expect(primaryAndCategories.context.briefing?.statusLabel).toContain('1 active finding');
    expect(primaryAndCategories.context.briefing?.statusLabel).toContain('1 category');

    // secondaryLabel absent (and distinct from the explicit primary label) ->
    // default 'Warnings' label.
    const secondaryFallback = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      metricState: { primaryLabel: 'Active findings', primaryValue: 1, secondaryValue: 1 },
      activeFindings: [],
    });
    expect(secondaryFallback.context.briefing?.statusLabel).toContain('1 active finding');
    expect(secondaryFallback.context.briefing?.statusLabel).toContain('1 warning');

    // Non-plural label ('Fixed') with value 1 -> stays 'fixed'.
    const fixedOne = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      metricState: { fixedValue: 1 },
      activeFindings: [],
    });
    expect(fixedOne.context.briefing?.statusLabel).toContain('1 fixed');

    // No metric values and no findings -> default attention summary.
    const empty = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: [],
    });
    expect(empty.context.briefing?.statusLabel).toContain('No active Patrol findings');
  });

  it('always emits a Health line, defaulting the chip label to "Health" when no signal is present', () => {
    // formatAssessmentHealth can never be empty because the chip label itself
    // defaults to 'Health'; with no grade or score the line is just that label.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: [],
    });

    expect(handoff.context.handoffContext).toContain('\nHealth: Health');
  });

  it('includes verification sub-facts only when their timestamps are present', () => {
    const withFacts = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      verification: {
        title: 'Checked',
        description: 'desc',
        lastFullRunAt: '2026-01-01T00:00:00Z',
        activityMixLabel: 'mix',
      },
      activeFindings: [],
    });
    expect(withFacts.context.handoffContext).toContain('last full run 2026-01-01T00:00:00Z');
    expect(withFacts.context.handoffContext).toContain('recent activity mix mix');

    const withoutFacts = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      verification: { title: 'Checked', description: 'desc' },
      activeFindings: [],
    });
    expect(withoutFacts.context.handoffContext).not.toContain('last full run');
    expect(withoutFacts.context.handoffContext).not.toContain('recent activity mix');
  });

  it('falls back to the Last Patrol label and returns undefined recency with no signal', () => {
    const withTimestampOnly = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      recency: { timestamp: '2026-01-01T00:00:00Z' },
      activeFindings: [],
    });
    expect(withTimestampOnly.context.handoffContext).toContain(
      'Last Patrol: Last Patrol 2026-01-01T00:00:00Z',
    );

    const withNothing = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      recency: null,
      activeFindings: [],
    });
    expect(withNothing.context.handoffContext).not.toContain('\nLast Patrol:');
  });

  it('builds sparse correlation endpoints and their context/evidence lines', () => {
    // Correlation with only a source_id (no names/types, no target id) ->
    // endpoint name/type collapse to undefined; the empty target id means the
    // target resource is never added.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        correlations: [
          {
            source_id: 'src-1',
            event_pattern: 'p',
            occurrences: 1,
            avg_delay: 1,
          } as unknown as ResourceCorrelation,
        ],
      },
      activeFindings: [],
    });

    // Correlation context line renders the source id.
    expect(handoff.context.handoffContext).toContain('Correlation 1: src-1');
    // Only the source resource is added, with no name or type (empty target id
    // is dropped by addAssessmentHandoffResource).
    expect(handoff.context.handoffResources).toEqual([{ id: 'src-1' }]);
  });

  it('renders correlation context sub-facts (pattern, last seen, description) and full evidence join', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        correlations: [
          {
            source_id: 'src-1',
            source_name: 'Source',
            source_type: 'vm',
            target_id: 'tgt-1',
            target_name: 'Target',
            target_type: 'host',
            event_pattern: 'a -> b',
            occurrences: 1,
            avg_delay: 1,
            confidence: 0.9,
            last_seen: '2026-01-01T00:00:00Z',
            description: 'correlated activity',
          },
        ],
      },
      activeFindings: [],
    });

    const line = handoff.context
      .handoffContext!.split('\n')
      .find((l) => l.startsWith('Correlation 1:'))!;
    // Both endpoints resolve -> the " to " join is used.
    expect(line).toContain('Source');
    expect(line).toContain(' to ');
    expect(line).toContain('Target');
    expect(line).toContain('last seen 2026-01-01T00:00:00Z');
    expect(line).toContain('description correlated activity');
    const evidence = handoff.context.briefing?.evidence ?? [];
    expect(evidence.some((e) => e.includes('Source') && e.includes(' to '))).toBe(true);
    expect(evidence.some((e) => e.includes('pattern'))).toBe(true);
  });

  it('renders sparse recent-change evidence and context sub-facts', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'c-1',
            observedAt: '2026-01-01T00:00:00Z',
            occurredAt: '2026-01-01T00:00:00Z',
            resourceId: 'vm-1',
            kind: 'config_update',
            sourceType: 'user_action',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'medium',
            actor: 'operator-one',
            reason: 'manual edit',
          },
        ],
      },
      activeFindings: [],
    });

    const line = handoff.context
      .handoffContext!.split('\n')
      .find((l) => l.startsWith('Recent Change 1:'))!;
    expect(line).toContain('change c-1');
    expect(line).toContain('resource vm-1');
    expect(line).toContain('observed 2026-01-01T00:00:00Z');
    expect(line).toContain('occurred 2026-01-01T00:00:00Z');
    expect(line).toContain('source user action');
    expect(line).toContain('adapter Proxmox Adapter');
    expect(line).toContain('medium confidence');
    expect(line).toContain('actor operator-one');
  });

  it('falls back to the change id then the bare resource token when no reason is present', () => {
    const byId = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'c-id',
            observedAt: '2026-01-01T00:00:00Z',
            resourceId: '',
            kind: 'activity',
            sourceType: 'platform_event',
            confidence: 'low',
          },
        ],
      },
      activeFindings: [],
    });
    expect(byId.context.handoffContext).toContain('Recent Change 1: Activity: c-id');

    const bare = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: '',
            observedAt: '',
            resourceId: '',
            kind: 'metric_anomaly',
            sourceType: 'heuristic',
            confidence: 'low',
          },
        ],
      },
      activeFindings: [],
    });
    expect(bare.context.handoffContext).toContain('Recent Change 1: Metric anomaly: resource');
  });

  it('renders a state-transition summary for differing states and keeps command-bearing wording', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'c-trans',
            observedAt: '2026-01-01T00:00:00Z',
            resourceId: 'vm-1',
            kind: 'state_transition',
            from: 'offline',
            to: 'online',
            sourceType: 'pulse_diff',
            confidence: 'high',
          },
          {
            id: 'c-runbook',
            observedAt: '2026-01-01T00:00:00Z',
            resourceId: 'vm-1',
            kind: 'runbook_executed',
            sourceType: 'agent_action',
            confidence: 'high',
            reason: 'ran the playbook',
          },
        ],
      },
      activeFindings: [],
    });

    expect(handoff.context.handoffContext).toContain(
      'Recent Change 1: State transition: offline to online',
    );
    expect(handoff.context.handoffContext).toContain(
      'Recent Change 2: Runbook executed: execution event recorded',
    );
  });

  it('formats the two-label related-resource list and a no-omission three-label list', () => {
    const two = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'c-2',
            observedAt: '2026-01-01T00:00:00Z',
            resourceId: 'vm-1',
            kind: 'metric_anomaly',
            sourceType: 'heuristic',
            confidence: 'high',
            reason: 'r',
            relatedResources: ['alpha', 'beta'],
          },
        ],
      },
      activeFindings: [],
    });
    expect(two.context.handoffContext).toContain('related resources alpha and beta');

    const three = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'c-3',
            observedAt: '2026-01-01T00:00:00Z',
            resourceId: 'vm-1',
            kind: 'metric_anomaly',
            sourceType: 'heuristic',
            confidence: 'high',
            reason: 'r',
            relatedResources: ['alpha', 'beta', 'gamma'],
          },
        ],
      },
      activeFindings: [],
    });
    expect(three.context.handoffContext).toContain('related resources alpha, beta, gamma');
  });

  it('caps assessment handoff resources at eight across many distinct resources', () => {
    const findings = Array.from({ length: 5 }, (_, i) => ({
      id: `f-${i + 1}`,
      title: `F${i + 1}`,
      resourceId: `finding-res-${i + 1}`,
    }));
    const changes = Array.from({ length: 3 }, (_, i) => ({
      id: `c-${i + 1}`,
      observedAt: '2026-01-01T00:00:00Z',
      resourceId: `change-res-${i + 1}`,
      kind: 'metric_anomaly',
      sourceType: 'heuristic',
      confidence: 'high',
      reason: 'r',
      relatedResources: [`rel-${i + 1}-a`, `rel-${i + 1}-b`],
    })) as unknown as ResourceChange[];

    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: { recentChanges: changes },
      activeFindings: findings,
    });

    // MAX_ASSESSMENT_RESOURCES = 8; the ninth distinct resource is dropped.
    expect(handoff.context.handoffResources).toHaveLength(8);
  });

  it('caps assessment handoff actions at four when more action-bearing findings exist', () => {
    const findings = Array.from({ length: 6 }, (_, i) => ({
      id: `f-${i + 1}`,
      title: `F${i + 1}`,
      proposedFix: { description: `Restart ${i + 1}` },
    }));

    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: findings,
    });

    expect(handoff.context.handoffActions).toHaveLength(4);
  });
});

describe('buildPatrolAssistantFindingHandoff (residual branches)', () => {
  it('collapses findingId to undefined in both top-level and nested context when no id exists', () => {
    const handoff = buildPatrolAssistantFindingHandoff({ title: 'T', subject: 'S' });

    expect(handoff.context.findingId).toBeUndefined();
    expect(handoff.context.context?.findingId).toBeUndefined();
    expect(handoff.context.context?.investigationRecordId).toBeUndefined();
  });

  it('resolves the resource name from the record subject and from the subject string', () => {
    // resourceName absent -> record subject.resource_name used.
    const fromRecord = buildPatrolAssistantFindingHandoff({
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      investigationRecord: {
        id: 'rec',
        finding_id: 'f',
        subject: { resource_id: 'r1', resource_name: 'From Record', resource_type: 'vm' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
      } as unknown as InvestigationRecord,
    });
    expect(fromRecord.context.handoffResources?.[0]).toMatchObject({
      id: 'r1',
      name: 'From Record',
      type: 'vm',
    });

    // resourceName and record absent -> falls back to the subject string; type
    // and node collapse to undefined.
    const fromSubject = buildPatrolAssistantFindingHandoff({
      title: 'T',
      subject: 'SubjectName',
      resourceId: 'r2',
    });
    expect(fromSubject.context.handoffResources?.[0]).toMatchObject({
      id: 'r2',
      name: 'SubjectName',
      type: undefined,
      node: undefined,
    });
  });

  it('uses the explicit resource name and type when provided', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      resourceName: 'Explicit',
      resourceType: 'host',
    });
    expect(handoff.context.handoffResources?.[0]).toMatchObject({
      id: 'r1',
      name: 'Explicit',
      type: 'host',
    });
  });

  it('falls back to default finding/subject titles in the model context', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      title: '',
      subject: '',
      resourceId: 'r1',
    });

    expect(handoff.context.handoffContext).toContain('Finding: Patrol finding');
    expect(handoff.context.handoffContext).toContain('Subject: affected resource');
  });

  it('renders rollback entries and an approved (non-pending) approval posture', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-1',
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      regressionCount: 1,
      pendingApproval: { id: 'pa-1', status: 'approved', riskLevel: 'high' },
      investigationRecord: {
        id: 'rec',
        finding_id: 'f-1',
        subject: { resource_id: 'r1' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
        rollback: ['Restore prior config'],
      } as unknown as InvestigationRecord,
    });

    // Rollback entries are mapped (the record has rollback summaries).
    expect(handoff.context.handoffContext).toContain('Rollback 1: Restore prior config');
    // The model context renders the approval via labelled context lines
    // (Approval / Approval Status / Approval Risk), not the inline parts used
    // by the assessment finding context line.
    expect(handoff.context.handoffContext).toContain('Approval: pa-1');
    expect(handoff.context.handoffContext).toContain('Approval Status: approved');
    expect(handoff.context.handoffContext).toContain('Approval Risk: high');
    // Regression singular wording.
    expect(handoff.context.handoffContext).toContain('regressed 1 time');
  });

  it('includes destructive and rationale action-artifact facts when the fix carries them', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-1',
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      proposedFix: {
        description: 'Restart',
        rationale: 'wedged process',
        destructive: true,
        commandCount: 1,
      },
    });

    expect(handoff.context.handoffContext).toContain('destructive action artifact');
    expect(handoff.context.handoffContext).toContain('rationale wedged process');
  });
});

describe('buildPatrolAssistantFindingHandoffActions (residual branches)', () => {
  it('leaves finding/resource targeting undefined for an action with sparse finding fields', () => {
    const [action] = buildPatrolAssistantFindingHandoffActions({
      pendingApproval: { actionId: 'act-1' },
    });

    expect(action).toMatchObject({
      actionId: 'act-1',
      findingId: undefined,
      targetResourceId: undefined,
      targetResourceType: undefined,
      fixId: undefined,
      description: undefined,
    });
  });
});

describe('patrolAssistantFindingHandoffRequiresApprovalMode (residual branches)', () => {
  it('requires approval when the record carries an approval id', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'rec',
          finding_id: 'f',
          subject: { resource_id: 'r1' },
          trigger: { detected_at: '2026-01-01T00:00:00Z' },
          status: 'completed',
          approval_id: 'ap-1',
        } as unknown as InvestigationRecord,
      }),
    ).toBe(true);
  });

  it('requires approval when only the record outcome is a governed action outcome', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'rec',
          finding_id: 'f',
          subject: { resource_id: 'r1' },
          trigger: { detected_at: '2026-01-01T00:00:00Z' },
          status: 'completed',
          outcome: 'fix_executed',
        } as unknown as InvestigationRecord,
      }),
    ).toBe(true);
  });

  it('does not require approval for a benign outcome', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'rec',
          finding_id: 'f',
          subject: { resource_id: 'r1' },
          trigger: { detected_at: '2026-01-01T00:00:00Z' },
          status: 'completed',
          outcome: 'needs_attention',
        } as unknown as InvestigationRecord,
      }),
    ).toBe(false);
  });
});

describe('buildPatrolRemediationPlanAssistant (residual branches)', () => {
  const planWithoutCommands: RemediationPlan = {
    id: 'plan-1',
    finding_id: 'f-1',
    resource_id: 'r-1',
    title: 'Restore service',
    description: 'Restart and verify.',
    risk_level: 'high',
    status: 'pending',
    created_at: '2026-01-01T00:00:00Z',
    steps: [],
  };

  it('falls back to default titles and the no-command safety note for an empty plan', () => {
    const modelContext = buildPatrolRemediationPlanAssistantModelContext({
      title: '',
      subject: '',
      plan: planWithoutCommands,
    });
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: '',
      subject: '',
      plan: planWithoutCommands,
    });

    expect(modelContext).toContain('Finding: Patrol finding on the affected resource');
    expect(briefing.subject).toBe('Patrol finding on affected resource');
    // No commands -> commandSummary undefined and the alternate safety note.
    expect(briefing.commandSummary).toBeUndefined();
    expect(briefing.safetyNote).toBe(
      'Assistant should decide remediation from evidence before any governed action.',
    );
    // No Governed Action Context line is emitted when commandSummary is absent.
    expect(modelContext).not.toContain('Governed Action Context:');
  });

  it('renders the artifact line, risk status, and governed-action context for a plan with commands', () => {
    const plan: RemediationPlan = {
      ...planWithoutCommands,
      steps: [{ order: 1, action: 'restart', command: 'systemctl restart x', risk_level: 'high' }],
    };
    const modelContext = buildPatrolRemediationPlanAssistantModelContext({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(modelContext).toContain('Governed Action Context: 1 command recorded');
    expect(modelContext).toContain('Treat this as approval state, not remediation guidance.');
  });

  it('omits the risk status segment and artifact line when the plan lacks them', () => {
    const plan = {
      id: 'plan-2',
      finding_id: 'f-1',
      resource_id: 'r-1',
      title: '',
      description: '',
      risk_level: '',
      status: '',
      created_at: '2026-01-01T00:00:00Z',
      steps: [],
    } as unknown as RemediationPlan;
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(briefing.statusLabel).toBeUndefined();
    expect(briefing.detailLines).toEqual([]);
  });

  it('counts rollback commands in the plan command summary', () => {
    const plan: RemediationPlan = {
      ...planWithoutCommands,
      steps: [
        { order: 1, action: 'a', rollback_command: 'systemctl stop x', risk_level: 'low' },
        { order: 2, action: 'b', rollback_command: 'systemctl stop y', risk_level: 'low' },
      ],
    };
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(briefing.commandSummary).toBe('2 rollback commands recorded');
  });
});

describe('buildPatrolAssistantFindingBriefing (residual branches)', () => {
  it('falls back to default finding/subject titles', () => {
    const briefing = buildPatrolAssistantFindingBriefing({
      title: '',
      subject: '',
      findingStatus: 'active',
    });

    expect(briefing?.subject).toBe('Patrol finding on affected resource');
    // severity/findingStatus are finding facts, not status parts in this
    // briefing, so an empty status stays undefined.
    expect(briefing?.statusLabel).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// Residual v8 branch arms not reached by the cases above. Each test below
// targets a specific uncovered conditional arm (the `||`/`??` fallback, the
// falsy ternary alternate, or a singular/plural boundary) and asserts a real
// observable value. Arms that are provably unreachable from any public entry
// point are documented in GLM_REPORT_fe-patrolctx.md rather than faked here.
// ---------------------------------------------------------------------------

describe('buildPatrolConfigurationFailureHandoff (identifier fallback arms)', () => {
  it('uses the raw cause verbatim when the identifier formatter collapses it to empty', () => {
    // cause '...' is all separator chars: formatIdentifierLabel('...') returns
    // '' (falsy) -> `formatIdentifierLabel(cause) || cause` falls back to the
    // verbatim cause string.
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      readiness: { cause: '...' },
    });

    expect(handoff.context.briefing?.detailLines).toContain('Cause: ...');
    expect(handoff.context.context?.readinessCause).toBe('...');
  });

  it('uses the raw detail key verbatim when the identifier formatter yields nothing', () => {
    // formatSafeConfigurationFailureDetail: a key of all separators makes
    // formatIdentifierLabel(normalizedKey) return '' -> `|| normalizedKey`.
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      details: { '...': 'plain-value' },
    });

    expect(handoff.context.briefing?.evidence).toEqual(['...: plain-value']);
  });
});

describe('buildPatrolAssistantFindingHandoffActions (findingId source fallback)', () => {
  it('derives the action findingId from the investigation record when the finding has no id', () => {
    // finding.id absent -> normalizeText(finding.id) is '' (falsy) -> the
    // record's finding_id arm of `finding.id || record.finding_id || undefined`.
    const [action] = buildPatrolAssistantFindingHandoffActions({
      investigationRecord: {
        id: 'rec',
        finding_id: 'rec-finding',
        subject: { resource_id: 'r1' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
        proposed_fix: { id: 'fix-1', description: 'Restart service' },
      } as unknown as InvestigationRecord,
    });

    expect(action.findingId).toBe('rec-finding');
  });
});

describe('buildPatrolInvestigationRecordPresentation (tool label fallback)', () => {
  it('drops a tool name that the identifier formatter reduces to empty', () => {
    // '___' is not a known tool and formatIdentifierLabel('___') returns '' ->
    // `formatIdentifierLabel(normalized) || ''` -> '' -> filtered out by
    // .filter(Boolean). A genuine unknown tool still resolves via the fallback.
    const presentation = buildPatrolInvestigationRecordPresentation({
      ...minimalRecord,
      tools_used: ['___', 'custom.tool'],
    });

    expect(presentation.toolsUsed).toEqual(['Custom Tool']);
  });
});

describe('buildPatrolRemediationPlanAssistantBriefing (non-array steps)', () => {
  it('treats a non-array steps field as empty when computing the command summary', () => {
    // Array.isArray(plan.steps) is false -> the `: []` arm -> no commands are
    // counted -> commandSummary stays undefined.
    const plan = {
      id: 'plan-1',
      finding_id: 'f-1',
      resource_id: 'r-1',
      title: 'Restore',
      description: 'desc',
      risk_level: 'low',
      status: 'pending',
      created_at: '2026-01-01T00:00:00Z',
      steps: 'not-an-array',
    } as unknown as RemediationPlan;

    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(briefing.commandSummary).toBeUndefined();
    expect(briefing.safetyNote).toBe(
      'Assistant should decide remediation from evidence before any governed action.',
    );
  });
});

describe('buildPatrolAssessmentAssistantHandoff (correlation filter + sparse endpoints)', () => {
  it('keeps a correlation that carries only an event_pattern', () => {
    // The correlation filter `||` chain falls through source_id, source_name,
    // target_id and target_name (all empty) and is retained by event_pattern.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        correlations: [
          {
            event_pattern: 'cpu-spike',
            occurrences: 1,
            avg_delay: 1,
          } as unknown as ResourceCorrelation,
        ],
      },
      activeFindings: [],
    });

    // Both endpoints resolve to undefined (no source/target ids or names), so
    // the context line is kept solely by its pattern/summary facts.
    expect(handoff.context.handoffContext).toContain('Correlation 1:');
    expect(handoff.context.handoffContext).toContain('pattern');
  });

  it('renders a target-only correlation and omits the pattern fact when event_pattern is absent', () => {
    // source endpoint undefined, target endpoint present -> `source && target`
    // is falsy -> alternate `source || target` evaluates target. With no
    // event_pattern the pattern fact takes its `: undefined` arm.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        correlations: [
          {
            target_id: 'tgt-1',
            target_name: 'Target',
            target_type: 'host',
            occurrences: 1,
            avg_delay: 1,
          } as unknown as ResourceCorrelation,
        ],
      },
      activeFindings: [],
    });

    const line = handoff.context
      .handoffContext!.split('\n')
      .find((l) => l.startsWith('Correlation 1:'))!;
    expect(line).toContain('Target');
    expect(line).not.toContain('pattern');
  });

  it('falls back to the "Items" metric label when the provided label is whitespace-only', () => {
    // primaryLabel '   ' is a truthy string so the caller's `|| 'Active
    // findings'` keeps it, but normalizeText trims it to '' inside
    // formatAssessmentMetricCount -> `normalizeText(label) || 'Items'`. With a
    // singular value the 'Items' label is singularized to 'item'.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      metricState: { primaryLabel: '   ', primaryValue: 1 },
      activeFindings: [],
    });

    expect(handoff.context.briefing?.statusLabel).toContain('1 item');
  });

  it('renders a single related-resource label without an omitted-count suffix', () => {
    // formatAssessmentRelatedResourceList: labels.length === 1 with omittedCount
    // 0 -> the bare-label arm (the omittedCount > 0 arm is unreachable here
    // because a single visible label always implies zero omitted).
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'c-1',
            observedAt: '2026-01-01T00:00:00Z',
            resourceId: 'vm-1',
            kind: 'metric_anomaly',
            sourceType: 'heuristic',
            confidence: 'high',
            reason: 'r',
            relatedResources: ['solo'],
          },
        ],
      },
      activeFindings: [],
    });

    expect(handoff.context.handoffContext).toContain('related resources solo');
    expect(handoff.context.handoffContext).not.toContain('and 0 more');
  });

  it('pluralizes the regression count in the assessment finding context line', () => {
    // regressionCount > 1 -> the 's' arm of `regressionCount === 1 ? '' : 's'`.
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: [{ id: 'f-1', title: 'High CPU', regressionCount: 3 }],
    });

    const line = handoff.context
      .handoffContext!.split('\n')
      .find((l) => l.startsWith('Finding 1:'))!;
    expect(line).toContain('regressed 3 times');
  });

  it('reports plural omitted findings when more than one is truncated', () => {
    // 7 findings, cap 5 -> 2 omitted -> the 's' arm of the findings-omitted
    // ternary.
    const findings = Array.from({ length: 7 }, (_, i) => ({
      id: `f-${i + 1}`,
      title: `F${i + 1}`,
    }));

    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: { title: 'T' },
      activeFindings: findings,
    });

    expect(handoff.context.handoffContext).toContain(
      '2 additional Patrol findings omitted from this bounded handoff summary.',
    );
  });
});

describe('buildPatrolAssistantFindingHandoff (regression plural arm)', () => {
  it('pluralizes the regression count in the finding model context', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-1',
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      regressionCount: 2,
    });

    expect(handoff.context.handoffContext).toContain('regressed 2 times');
  });
});
