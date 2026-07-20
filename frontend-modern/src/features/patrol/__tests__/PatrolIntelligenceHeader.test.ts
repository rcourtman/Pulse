import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import {
  PATROL_AUTONOMY_POLICY_PRESENTATION,
  getPatrolConfigurationFailureInlineDetails,
} from '../PatrolIntelligenceHeader';
import {
  getPatrolAutonomyAvailabilityPresentation,
  PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON,
} from '../patrolAutonomyAvailability';

const headerSource = readFileSync(
  resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
  'utf-8',
);

describe('PatrolIntelligenceHeader', () => {
  it('keeps Patrol mode readiness context visible inline', () => {
    expect(
      getPatrolConfigurationFailureInlineDetails({
        message: 'Patrol mode could not be saved.',
        code: 'patrol_readiness_not_ready',
        readiness: {
          status: 'not_ready',
          cause: 'model_unsupported_tools',
          summary:
            'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
          provider: 'openrouter',
          model: 'openrouter:deepseek/deepseek-r1',
        },
      }),
    ).toEqual([
      'patrol_readiness_not_ready · model_unsupported_tools',
      'Setup: The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      'Provider: openrouter',
      'Model: openrouter:deepseek/deepseek-r1',
    ]);
  });

  it('falls back to the blocked cause when readiness cause is absent', () => {
    expect(
      getPatrolConfigurationFailureInlineDetails({
        message: 'Patrol mode could not be saved.',
        code: 'patrol_autonomy_pro_required',
        blockedCause: 'license_required',
      }),
    ).toEqual(['patrol_autonomy_pro_required · license_required']);
  });

  it('keeps trigger runtime status in the header chrome after setup', () => {
    expect(headerSource).toContain('getPatrolTriggerStatusSummary');
    expect(headerSource).toContain('state.patrolStatus()?.trigger_status');
    expect(headerSource).toContain('!state.shouldShowPatrolSetupOnly() && triggerStatusSummary()');
    expect(headerSource).not.toContain('Automation:');
    expect(headerSource).not.toContain('Trigger status:');
  });

  it('keeps Patrol mode visible while setup blocks running and configuration', () => {
    const setupOnlyGateCount = headerSource.match(/!state\.shouldShowPatrolSetupOnly\(\)/g)?.length;

    expect(setupOnlyGateCount).toBeGreaterThanOrEqual(3);
    expect(headerSource).toContain('<Show when={!state.shouldShowPatrolSetupOnly()}>');
    expect(headerSource).toContain('id={PATROL_CONTROL_ANCHOR}');
    expect(headerSource).not.toContain(
      '<Show when={!state.shouldShowPatrolSetupOnly()}>\n        <div\n          id={PATROL_CONTROL_ANCHOR}',
    );
    expect(headerSource).toContain('Open Patrol settings');
    expect(headerSource).toContain("settingsTabPath('system-ai-patrol')");
    expect(headerSource).not.toContain('Patrol schedule and model settings');
    expect(headerSource).toContain('Run Patrol');
  });

  it('turns provider-blocked manual run controls into setup actions', () => {
    expect(headerSource).toContain('runBlockedByProviderSetup');
    expect(headerSource).toContain("state.patrolReadiness()?.status === 'not_ready'");
    expect(headerSource).toContain('getPatrolSetupAction');
    expect(headerSource).toContain('providerSetupAction().href');
    expect(headerSource).toContain('Fix setup');
    expect(headerSource).toContain('getPatrolSetupAction(state.patrolReadiness()?.cause)');
    expect(headerSource).toContain('runButtonDisabled');
    expect(headerSource).not.toContain('!state.canTriggerPatrol() ||');
  });

  it('keeps manual page sync out of the primary Patrol header actions', () => {
    expect(headerSource).not.toContain('handleRefreshPatrol');
    expect(headerSource).not.toContain('Sync page data');
    expect(headerSource).not.toContain('RefreshCwIcon');
    expect(headerSource).not.toContain('animate-spin');
    expect(headerSource).not.toContain('Refresh Patrol');
  });

  it('makes Patrol mode a simple four-level choice without rendering plan-locked paid modes', () => {
    expect(PATROL_AUTONOMY_POLICY_PRESENTATION).toEqual({
      monitor: {
        label: 'Watch only',
        detail: 'Patrol checks infrastructure and reports issues only; it does not start fixes.',
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
          'Patrol can run low- or medium-risk fixes allowed by policy; higher-risk work still asks first.',
        compactLabel: 'Safe auto-fix',
      },
      full: {
        label: 'Autopilot',
        detail:
          'Patrol can act automatically within policy and still asks when approval is required.',
        compactLabel: 'Autopilot',
      },
    });
    expect(headerSource).toContain("['monitor', 'approval', 'assisted', 'full']");
    expect(headerSource).toContain("state.autoFixLocked() ? 'monitor' : state.autonomyLevel()");
    expect(headerSource).toContain('value={effectiveAutonomyLevel()}');
    expect(headerSource).toContain('canChooseAutonomyLevel');
    expect(headerSource).toContain('shouldShowAutonomyOptions');
    expect(headerSource).toContain('shouldShowAutonomyActionColumn');
    expect(headerSource).toContain("autonomyAvailability().kind === 'runtime_locked'");
    expect(headerSource).toContain('<Show when={shouldShowAutonomyOptions()}>');
    expect(headerSource).toContain('<Show when={shouldShowAutonomyActionColumn()}>');
    expect(headerSource).toContain(
      "const lockedPaidMode = state.autoFixLocked() && level !== 'monitor'",
    );
    expect(headerSource).toContain('disabled: lockedPaidMode');
    expect(headerSource).toContain('compactLabel: presentation.compactLabel');
    expect(headerSource).not.toContain('showPlanLockedModeAction');
    expect(headerSource).toContain('Patrol mode');
    expect(headerSource).not.toContain('getPatrolAutonomyContractPresentation');
    expect(headerSource).not.toContain('selectedAutonomyContract');
    expect(headerSource).not.toContain('showControlDetails');
    expect(headerSource).not.toContain("'Limits'");
    expect(headerSource).not.toContain('Hide limits');
    expect(headerSource).not.toContain('Hard limits');
    expect(headerSource).not.toContain('requires Pulse Pro');
    expect(headerSource).not.toContain('const isProLocked = () =>');
    expect(headerSource).not.toContain('visualLabel: proLocked');
    expect(headerSource).not.toContain('Current mode:');
    expect(headerSource).not.toContain('How much can Patrol do?');
    expect(headerSource).not.toContain("level === 'full' ? 'assisted'");
    expect(headerSource).not.toContain('Autonomous critical remediation');
    expect(headerSource).not.toContain('Ask before changes');
    expect(headerSource).not.toContain('Auto-fix safe issues');
    expect(headerSource).not.toContain('Policy autopilot');
  });

  it('keeps Patrol mode inline and routes configuration to settings', () => {
    const autonomyControlCallCount =
      headerSource.match(/renderAutonomyPolicyControl\(\{/g)?.length ?? 0;
    const configurationAutonomyIndex = headerSource.indexOf("ariaLabel: 'Patrol mode'");

    expect(autonomyControlCallCount).toBe(1);
    expect(configurationAutonomyIndex).toBeGreaterThan(-1);
    expect(headerSource).toContain('Open Patrol settings');
    expect(headerSource).toContain("settingsTabPath('system-ai-patrol')");
    expect(headerSource).not.toContain('Schedule & model');
    expect(headerSource).not.toContain('Patrol schedule and model settings');
    expect(headerSource).not.toContain('label="Provider model"');
    expect(headerSource).not.toContain('label="Run Every"');
    expect(headerSource).not.toContain('FormSelect');
    expect(headerSource).not.toContain('showAdvancedSettings');
    expect(headerSource).not.toContain('Advanced settings');
    expect(headerSource).not.toContain('Advanced Patrol settings');
    expect(headerSource).not.toContain('Done');
    expect(headerSource).not.toContain('Set Patrol mode');
    expect(headerSource).not.toContain('Save Patrol mode');
    expect(headerSource).not.toContain('Patrol policy');
    expect(headerSource).not.toContain("variant: 'prominent'");
    expect(headerSource).not.toContain('Patrol Configuration');
    expect(headerSource).not.toContain('Apply Configuration');
  });

  it('keeps the Patrol model catalog out of the operator page', () => {
    expect(headerSource).not.toContain('showModelCatalog');
    expect(headerSource).not.toContain('Provider model');
    expect(headerSource).not.toContain('Change model');
    expect(headerSource).not.toContain('Hide model list');
    expect(headerSource).not.toContain('Choose available model');
    expect(headerSource).not.toContain('formatPatrolModelOptionLabel(model)');
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
    expect(headerSource).toContain('showAutonomyPlanBillingAction');
    expect(headerSource).toContain("autonomyAvailability().kind === 'plan_locked'");
    expect(headerSource).toContain('CreditCardIcon');
    expect(headerSource).toContain('{autonomyAvailability().actionLabel}');

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

  it('keeps trust counters out of the page header chrome', () => {
    // Needs Attention and deliberate history review own trust
    // state. The header should stay focused on title, recency, and controls.
    expect(headerSource).not.toContain('aria-label="Patrol trust summary header"');
    expect(headerSource).not.toContain('state.patrolStatus()?.trust');
    expect(headerSource).not.toContain('trust.currently_active');
    expect(headerSource).not.toContain('trust.regressed_at_least_once');
    expect(headerSource).not.toContain('trust.fix_verified');
  });
});
