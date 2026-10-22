import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import { buildPatrolInvestigationRecordPresentation } from '../patrolInvestigationContextModel';

describe('patrolInvestigationContextModel', () => {
  it('builds operator-facing Patrol record presentation without exposing raw commands', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'record-1',
      finding_id: 'finding-1',
      subject: { resource_id: 'vm-100', resource_name: 'web', resource_type: 'vm' },
      trigger: {
        title: 'High CPU usage',
        detected_at: '2026-05-06T12:00:00Z',
      },
      status: 'completed',
      outcome: 'fix_queued',
      confidence: 'high',
      conclusion: 'Backup job saturated CPU.',
      recommended_action: 'Approve a controlled restart after the backup completes.',
      evidence: [
        { kind: 'metrics', summary: 'CPU stayed above 95% for 10 minutes' },
        { kind: 'logs', summary: 'Backup process held IO wait.' },
      ],
      proposed_fix: {
        id: 'fix-1',
        description: 'Restart the workload service',
        commands: ['systemctl restart workload.service'],
        risk_level: 'medium',
        destructive: false,
        target_host: 'pve-1',
        rationale: 'The process is wedged after backup IO pressure.',
      },
      verification: ['CPU returned below 50%'],
      rollback: [],
      tools_used: ['metrics.history', 'ssh.exec'],
      started_at: '2026-05-06T12:00:00Z',
    });

    expect(presentation).toMatchObject({
      hasRecord: true,
      statusLabel: 'Completed',
      outcomeLabel: 'Fix Queued',
      confidenceLabel: 'High confidence',
      conclusion: 'Backup job saturated CPU.',
      evidenceSummaries: ['CPU stayed above 95% for 10 minutes', 'Backup process held IO wait.'],
      verificationSummaries: ['CPU returned below 50%'],
      toolsUsed: ['Metrics history', 'SSH exec'],
      proposedFix: {
        description: 'Restart the workload service',
        riskLabel: 'Medium',
        targetHost: 'pve-1',
        commandSummary: '1 command recorded for approval context',
      },
    });
    expect(JSON.stringify(presentation)).not.toContain('systemctl restart workload.service');
    expect(presentation.rollbackSummaries).toEqual([]);
    expect(presentation.impact).toBe('');
  });

  it('keeps previous-fix operational memory at the finding shell, not in record presentation', () => {
    // Operational memory (previousResolvedFixSummary) lives on the finding
    // shell and is rendered by FindingsPanel as a distinct row. The
    // investigation-record presentation must not absorb it into impact,
    // verification, or rollback — those represent the CURRENT investigation,
    // not history. This test pins the boundary so future refactors do not
    // collapse the per-record schema with the per-finding memory shell.
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'rec-prev-fix-isolation',
      finding_id: 'f-prev-fix-isolation',
      subject: { resource_id: 'vm-9' },
      trigger: { detected_at: '2026-05-08T12:00:00Z' },
      status: 'completed',
      evidence: [],
      verification: [],
      rollback: [],
      tools_used: [],
      started_at: '2026-05-08T12:00:00Z',
    });
    // The presentation shape exposes confidence, conclusion, impact,
    // recommendedAction, etc., but no previousResolvedFixSummary field —
    // that lives on UnifiedFinding/Finding shells, not InvestigationRecord.
    const opaque = presentation as unknown as Record<string, unknown>;
    expect(opaque.previousResolvedFixSummary).toBeUndefined();
    expect(opaque.previous_resolved_fix_summary).toBeUndefined();
  });

  it('does not let the patrol context model synthesize impact from trust counters', () => {
    // Trust counters (FindingsTrustSummary on the patrol-status response) are
    // an operator-page concern, not a per-finding context concern. The
    // investigation-context model must not derive impact, recommendation, or
    // any other per-finding text from trust counts; the source authoring rule
    // (see ai-runtime contract) forbids synthesis from severity, category,
    // OR aggregate counts. This test pins that boundary.
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'rec-trust-isolation',
      finding_id: 'f-trust-isolation',
      subject: { resource_id: 'vm-9' },
      trigger: { detected_at: '2026-05-08T12:00:00Z' },
      status: 'completed',
      // Intentionally empty impact/rollback so the test reflects what the
      // model produces when only trust signals are available externally.
      evidence: [],
      verification: [],
      rollback: [],
      tools_used: [],
      started_at: '2026-05-08T12:00:00Z',
    });
    expect(presentation.impact).toBeFalsy();
    expect(presentation.rollbackSummaries).toEqual([]);
  });

  it('surfaces investigation impact and rollback when the backend record carries them', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'record-2',
      finding_id: 'finding-2',
      subject: { resource_id: 'vm-101' },
      trigger: { title: 'Backup job failing', detected_at: '2026-05-06T12:00:00Z' },
      status: 'completed',
      outcome: 'fix_queued',
      confidence: 'medium',
      conclusion: 'Datastore quota exhausted.',
      impact: 'Nightly backups will be skipped; recovery window grows by one day per skip.',
      recommended_action: 'Free 200GB on the datastore before the next backup window.',
      evidence: [{ kind: 'metrics', summary: 'datastore 99% full' }],
      verification: ['Backup job exits 0 on next run'],
      rollback: ['Restore prior retention policy', 'Re-pin previous datastore mount'],
      tools_used: [],
      started_at: '2026-05-06T12:00:00Z',
    });

    expect(presentation.impact).toBe(
      'Nightly backups will be skipped; recovery window grows by one day per skip.',
    );
    expect(presentation.rollbackSummaries).toEqual([
      'Restore prior retention policy',
      'Re-pin previous datastore mount',
    ]);
  });
});

describe('Patrol page header IA framing', () => {
  it('surfaces presenter-owned coverage wording on the recency line in the page header', () => {
    // Third wedge of the Patrol page IA reframe. The recency line tells the
    // operator when Pulse last ran; the coverage signal tells them what it
    // covered. Pin the wiring so the recency render reads
    // resourcesCheckedLabel from getPatrolRecencyPresentation and gates on a
    // truthy <Show> so zero-coverage runs do not render a coverage phrase,
    // and failed or scoped runs do not get hardcoded "verified" wording.
    const headerSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
      'utf-8',
    );
    expect(headerSource).toContain('recency().resourcesCheckedLabel');
    expect(headerSource).toContain('Show when={recency().resourcesCheckedLabel}');
    expect(headerSource).not.toContain('verified {recency().resourcesChecked}');
  });

  it('keeps trust counters out of default Patrol chrome', () => {
    // Trust counters remain backend/state evidence. The default Patrol surface
    // should not reintroduce a second current-work/history strip to explain them.
    const surfaceSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceSurface.tsx'),
      'utf-8',
    );
    const headerSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
      'utf-8',
    );
    const workspaceSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceWorkspace.tsx'),
      'utf-8',
    );
    expect(surfaceSource).not.toContain('PatrolIntelligenceSummary');
    expect(surfaceSource).not.toContain('compactAssessmentSummary');
    expect(surfaceSource).not.toContain('state.patrolStatus()?.trust?.regressed_at_least_once');
    expect(headerSource).not.toContain('aria-label="Patrol trust summary header"');
    expect(workspaceSource).not.toContain('aria-label="Patrol trust summary"');
  });

  it('names the operator job on the canonical Patrol surface when control is available', async () => {
    // The Patrol page header is the most visible piece of operator-facing
    // copy on the canonical Patrol surface. The IA framing must keep the
    // product boundary clear without turning the page header into a
    // mini spec for the full operations loop.
    const { getPatrolPageHeaderMeta, PATROL_PAGE_DESCRIPTION, PATROL_PAGE_TITLE_TOOLTIP } =
      await import('@/utils/patrolPagePresentation');
    expect(PATROL_PAGE_DESCRIPTION).toBe(
      'See what needs a decision, choose the next step, and keep a verified record.',
    );
    expect(PATROL_PAGE_DESCRIPTION).toContain('verified record');
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toMatchObject({
      title: 'Patrol',
      description: PATROL_PAGE_DESCRIPTION,
      titleTooltip: PATROL_PAGE_DESCRIPTION,
    });
    expect(getPatrolPageHeaderMeta({ autonomyLocked: true })).toMatchObject({
      title: 'Patrol',
      description: PATROL_PAGE_DESCRIPTION,
      titleTooltip: PATROL_PAGE_DESCRIPTION,
    });
  });
});
