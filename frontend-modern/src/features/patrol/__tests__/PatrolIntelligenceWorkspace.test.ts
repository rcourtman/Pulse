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
  it('connects a provider in place for config-level causes and keeps setup through polling', () => {
    expect(workspaceSource).toContain(
      'when={setupPanelActive() || isPatrolConnectCause(setupCause())}',
    );
    expect(workspaceSource).toContain('<PatrolSetupPanel');
    expect(workspaceSource).toContain(
      "const PatrolSetupPanel = lazy(() => import('./PatrolSetupPanel'));",
    );
    expect(workspaceSource).toContain('readinessProvider={state.patrolReadiness()?.provider}');
    expect(workspaceSource).toContain('(state.shouldShowPatrolSetupOnly() || setupPanelActive())');
    // The first run waits for readiness to be re-read, or the run gate stays shut.
    expect(workspaceSource).toContain(
      'void state.handleRefreshPatrol().then(() => state.handleRunPatrol());',
    );
    expect(workspaceSource).toContain(
      "setupPanelActive() ? 'Set up Patrol' : 'Patrol cannot run yet'",
    );
  });

  it('frames findings and run records as Patrol operation records', () => {
    expect(workspaceSource).toContain('Patrol work groups');
    expect(workspaceSource).toContain('Patrol cannot run yet');
    expect(workspaceSource).toContain('Once ready');
    expect(workspaceSource).toContain('Show open work');
    expect(PATROL_WORKSPACE_SETUP_TITLE).toBe('Patrol needs setup');
    expect(PATROL_WORKSPACE_SETUP_DESCRIPTION).toBe(
      'Patrol cannot check infrastructure until its selected model passes the Patrol tool check.',
    );
    expect(PATROL_WORKSPACE_QUEUE_TITLE).toBe('Open work');
    expect(PATROL_WORKSPACE_RUN_RECORD_TITLE).toBe('Check details');
    expect(PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION).toBe('What Patrol found during this run.');
    expect(PATROL_WORKSPACE_HISTORY_DESCRIPTION).toBe('Past checks and what Patrol recorded.');
  });
});
