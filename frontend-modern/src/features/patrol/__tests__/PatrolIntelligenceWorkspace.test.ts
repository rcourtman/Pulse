import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import {
  PATROL_WORKSPACE_HISTORY_DESCRIPTION,
  PATROL_WORKSPACE_QUEUE_TITLE,
  PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION,
  PATROL_WORKSPACE_RUN_RECORD_TITLE,
  PATROL_WORKSPACE_SETUP_DESCRIPTION,
  PATROL_WORKSPACE_SETUP_TITLE,
} from '../patrolControlPresentation';

const workspaceSource = readFileSync(
  resolve(__dirname, '..', 'PatrolIntelligenceWorkspace.tsx'),
  'utf-8',
);

describe('PatrolIntelligenceWorkspace trust strip', () => {
  it('does not render a standalone Trust strip above the workspace tabs', () => {
    // Needs Attention and deliberate history review own operator-facing trust state.
    // The workspace should not add another standalone counter strip.
    expect(workspaceSource).not.toContain('state.patrolStatus()?.trust');
    expect(workspaceSource).not.toContain('aria-label="Patrol trust summary"');
    expect(workspaceSource).not.toContain('trust.fix_verified');
    expect(workspaceSource).not.toContain('trust.auto_resolved');
    expect(workspaceSource).not.toContain('trust.dismissed_as_noise');
  });

  it('does not invent trust signals not on FindingsTrustSummary', () => {
    // No mention of arbitrary keys like "patrol_score" or "health_grade".
    expect(workspaceSource).not.toMatch(/trust\.patrol_score/);
    expect(workspaceSource).not.toMatch(/trust\.health_grade/);
  });

  it('frames findings and run records as Patrol operation records', () => {
    expect(workspaceSource).toContain('state.shouldShowPatrolSetupOnly()');
    expect(workspaceSource).not.toContain('PATROL_SUPPORTING_CONTEXT');
    expect(workspaceSource).not.toContain('ResourceCorrelationSummary');
    expect(workspaceSource).not.toContain('ResourcePolicySummary');
    expect(workspaceSource).not.toContain('ResourceChangeSummary');
    expect(workspaceSource).not.toContain('showInvestigationContext');
    expect(workspaceSource).not.toContain('shouldSurfaceInvestigationContext');
    expect(workspaceSource).toContain('PATROL_WORKSPACE_SETUP_TITLE');
    expect(workspaceSource).toContain('PATROL_WORKSPACE_SETUP_DESCRIPTION');
    expect(workspaceSource).toContain('buildPatrolFindingDisplayGroups');
    expect(workspaceSource).toContain('getPatrolQueueBadgeLabel');
    expect(workspaceSource).toContain('queueIssueCount');
    expect(workspaceSource).toContain('queueAffectedResourceCount');
    expect(workspaceSource).toContain('affectedResourceCount: queueAffectedResourceCount()');
    expect(workspaceSource).toContain('findingCount: queueIssueCount()');
    expect(workspaceSource).toContain('getPatrolQueueWorkspaceDescription');
    expect(workspaceSource).toContain('getPatrolWorkspaceWorkGroups');
    expect(workspaceSource).toContain('Patrol work groups');
    expect(workspaceSource).toContain("group.id !== 'stale-protection'");
    expect(workspaceSource).toContain('queueIssueCount() > 0');
    expect(workspaceSource).not.toContain('getPatrolWorkspaceProtectionPosture');
    expect(workspaceSource).not.toContain('Patrol protection posture');
    expect(workspaceSource).not.toContain('protectionPostureSummaries');
    expect(workspaceSource).toContain('autonomyLevel: state.autonomyLevel()');
    expect(workspaceSource).toContain('autonomyLocked: state.autoFixLocked()');
    expect(workspaceSource).toContain('Boolean(queueBadgeLabel())');
    expect(workspaceSource).toContain('{queueBadgeLabel()}');
    const removedAllModeCopy =
      'Open work Patrol can ' +
      'watch, investigate, ask you to approve, or record under your Patrol mode.';
    expect(workspaceSource).not.toContain(removedAllModeCopy);
    expect(workspaceSource).toContain('Patrol cannot run yet');
    expect(workspaceSource).toContain('Once ready');
    expect(workspaceSource).toContain('getPatrolSetupAction');
    expect(workspaceSource).toContain('getPatrolSetupHint');
    expect(workspaceSource).toContain('state.patrolRunHistory.value()?.length');
    expect(workspaceSource).not.toContain('showControls={!state.selectedRun() && !isSetupOnly()}');
    expect(PATROL_WORKSPACE_SETUP_TITLE).toBe('Patrol needs setup');
    expect(PATROL_WORKSPACE_SETUP_DESCRIPTION).toBe(
      'Patrol cannot check infrastructure until its selected model passes the Patrol tool check.',
    );
    expect(PATROL_WORKSPACE_QUEUE_TITLE).toBe('Open work');
    expect(workspaceSource).not.toContain('Current Patrol findings');
    expect(workspaceSource).toContain('Show open work');
    expect(workspaceSource).not.toContain('Show open issues');
    expect(workspaceSource).toContain('!state.selectedRun()');
    expect(PATROL_WORKSPACE_RUN_RECORD_TITLE).toBe('Check details');
    expect(PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION).toBe('What Patrol found during this run.');
    expect(PATROL_WORKSPACE_HISTORY_DESCRIPTION).toBe('Past checks and what Patrol recorded.');
  });
});
