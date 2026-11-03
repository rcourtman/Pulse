import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import {
  PATROL_AUTONOMY_POLICY_PRESENTATION,
  getPatrolAutopilotExpiry,
} from '../PatrolIntelligenceHeader';
import {
  getPatrolAlertAnalysisAvailabilityPresentation,
  getPatrolAutonomyAvailabilityPresentation,
  PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON,
} from '../patrolAutonomyAvailability';

const headerSource = readFileSync(
  resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
  'utf-8',
);

describe('PatrolIntelligenceHeader', () => {
  it('resolves the header setup action from the block cause before readiness (#1789)', () => {
    expect(headerSource).toContain(
      'resolvePatrolBlockedActionCause(state.blockedCause(), state.patrolReadiness()?.cause)',
    );
  });

  it('does not present the server zero-time sentinel as an Autopilot expiry', () => {
    expect(getPatrolAutopilotExpiry('0001-01-01T00:00:00Z')).toBeNull();
    expect(getPatrolAutopilotExpiry('not-a-date')).toBeNull();
    expect(getPatrolAutopilotExpiry(undefined)).toBeNull();
    expect(getPatrolAutopilotExpiry('2026-12-31T12:00:00Z')?.toISOString()).toBe(
      '2026-12-31T12:00:00.000Z',
    );
  });

  it('turns provider-blocked manual run controls into setup actions', () => {
    expect(headerSource).toContain('Fix setup');
  });

  it('keeps primary Patrol actions touch-sized on phones without inflating desktop chrome', () => {
    expect(headerSource).toContain(
      'flex min-h-11 items-center justify-center gap-2 rounded-md border border-border',
    );
    expect(headerSource).toContain(
      'flex min-h-11 items-center justify-center gap-2 rounded-md border border-transparent',
    );
  });

  it('makes Patrol mode a simple four-level choice without rendering plan-locked paid modes', () => {
    expect(PATROL_AUTONOMY_POLICY_PRESENTATION).toEqual({
      monitor: {
        label: 'Watch only',
        detail: 'Patrol checks infrastructure and reports issues only. It does not start fixes.',
        compactLabel: 'Watch only',
      },
      approval: {
        label: 'Ask first',
        detail: 'Patrol investigates and prepares fixes, but every change waits for your approval.',
        compactLabel: 'Ask first',
      },
      assisted: {
        label: 'Safe auto-fix',
        detail:
          'Patrol can run low- or medium-risk fixes allowed by policy. Higher-risk work still asks first.',
        compactLabel: 'Safe auto-fix',
      },
      full: {
        label: 'Autopilot',
        detail:
          'Patrol can act automatically within policy and still asks when approval is required.',
        compactLabel: 'Autopilot',
      },
    });
    expect(headerSource).toContain('Patrol mode');
  });

  it('explains when Patrol mode is locked by plan or runtime', () => {
    expect(
      getPatrolAutonomyAvailabilityPresentation({
        autoFixLocked: false,
        planUpgradeDestination: {
          href: '/settings/pulse-intelligence/billing/plan',
          external: false,
        },
      }),
    ).toMatchObject({
      kind: 'available',
      locked: false,
      title: 'Patrol mode available',
      body: 'Choose the mode for this install.',
    });

    expect(
      getPatrolAutonomyAvailabilityPresentation({
        autoFixLocked: true,
        planUpgradeDestination: {
          href: '/settings/pulse-intelligence/billing/plan',
          external: false,
        },
      }),
    ).toMatchObject({
      kind: 'plan_locked',
      locked: true,
      title: 'Watch only',
      body: 'This install watches infrastructure and shows issues.',
      actionLabel: 'Plans & Billing',
      destination: {
        href: '/settings/pulse-intelligence/billing/plan',
        external: false,
      },
    });

    const hiddenUpgradePresentation = getPatrolAutonomyAvailabilityPresentation({
      autoFixLocked: true,
      upgradePromptsHidden: true,
      planUpgradeDestination: {
        href: '/settings/pulse-intelligence/billing/plan',
        external: false,
      },
    });
    expect(hiddenUpgradePresentation).toMatchObject({
      kind: 'plan_locked',
      locked: true,
      title: 'Watch only',
      body: 'This install watches infrastructure and shows issues.',
    });
    expect(hiddenUpgradePresentation).not.toHaveProperty('actionLabel');
    expect(hiddenUpgradePresentation).not.toHaveProperty('destination');

    const hiddenCommercialPresentation = getPatrolAutonomyAvailabilityPresentation({
      autoFixLocked: true,
      commercialSurfacesHidden: true,
      planUpgradeDestination: {
        href: '/settings/pulse-intelligence/billing/plan',
        external: false,
      },
    });
    expect(hiddenCommercialPresentation).toMatchObject({
      kind: 'plan_locked',
      locked: true,
      title: 'Watch only',
      body: 'This install watches infrastructure and shows issues.',
    });
    expect(hiddenCommercialPresentation).not.toHaveProperty('actionLabel');
    expect(hiddenCommercialPresentation).not.toHaveProperty('destination');

    expect(
      getPatrolAutonomyAvailabilityPresentation({
        autoFixLocked: true,
        runtimeCapabilityBlock: {
          key: 'ai_autofix',
          reason: PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON,
          action_url: 'https://pulserelay.pro/download.html',
        },
        runtime: { build: 'community', label: 'Pulse Community runtime' },
        planUpgradeDestination: {
          href: '/settings/pulse-intelligence/billing/plan',
          external: false,
        },
      }),
    ).toMatchObject({
      kind: 'runtime_locked',
      locked: true,
      title: 'Pulse Pro runtime required',
      actionLabel: 'Open Pro downloads',
      destination: { href: 'https://pulserelay.pro/download.html', external: true },
    });
  });

  it('explains the container update risk lock through the same presentation policy', () => {
    const planUpgradeDestination = {
      href: '/settings/pulse-intelligence/billing/plan',
      external: false,
    };
    expect(
      getPatrolAlertAnalysisAvailabilityPresentation({
        alertAnalysisLocked: true,
        planUpgradeDestination,
      }),
    ).toMatchObject({
      kind: 'plan_locked',
      title: 'Higher license plan required',
      actionLabel: 'Plans & Billing',
      destination: planUpgradeDestination,
    });
    expect(
      getPatrolAlertAnalysisAvailabilityPresentation({
        alertAnalysisLocked: true,
        upgradePromptsHidden: true,
        planUpgradeDestination,
      }),
    ).not.toHaveProperty('actionLabel');
    expect(
      getPatrolAlertAnalysisAvailabilityPresentation({
        alertAnalysisLocked: true,
        commercialSurfacesHidden: true,
        planUpgradeDestination,
      }),
    ).toEqual({
      kind: 'plan_locked',
      locked: true,
      title: 'Not available',
      body: 'This install does not include container update risk.',
    });
  });
});
