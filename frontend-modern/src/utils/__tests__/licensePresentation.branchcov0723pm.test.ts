/**
 * Branch-coverage tests for licensePresentation.ts (0723pm batch).
 *
 * Targets the remaining uncovered licence-state fallback arms:
 * - absent / malformed expiry dates (formatUnixSeconds value <= 0 arm)
 * - zero, negative, and fractional patrol-loop counts (normalizeStatusCount)
 * - alias-subtraction underflow in getFirstPartyPatrolControlCount
 * - resolved-without-value-state verification in hasVerifiedPatrolOperatorOutcome
 * - unknown tier fallbacks in getSelfHostedUnlockedFeatures /
 *   getSelfHostedIncludedExtras / getSelfHostedActivePlanSummary
 * - empty-feature skip and 8-item cap in getSelfHostedActivationHighlights
 *
 * Does NOT modify the source module or any existing test file.
 */
import { describe, expect, it } from 'vitest';

import {
  getBillingAdminTrialStatus,
  getSelfHostedActivationSuccessPresentation,
  getSelfHostedCurrentPlanPresentation,
  PATROL_CONTROL_STARTER_URL,
} from '@/utils/licensePresentation';

/* ------------------------------------------------------------------ *
 * formatUnixSeconds — absent / negative expiry dates
 * (exercised through getBillingAdminTrialStatus)
 *
 * The private helper has three arms:
 *   if (!value || value <= 0) return 'N/A';        // (A) falsy, (B) <= 0
 *   if (Number.isNaN(date.getTime())) return …;    // (C)
 *   return date.toLocaleString();                  // (D)
 *
 * Existing tests cover arm (A) via trial_started_at: 0 and arm (C) via
 * Infinity.  A *negative* value is truthy so it skips (A) and lands on
 * arm (B) — a distinct branch.
 * ------------------------------------------------------------------ */

describe('formatUnixSeconds via getBillingAdminTrialStatus - negative timestamp arms', () => {
  it('returns N/A for a negative trial_ends_at on a trial subscription (value <= 0, not falsy)', () => {
    const result = getBillingAdminTrialStatus({
      subscription_state: 'trial',
      trial_ends_at: -1,
    } as never);
    expect(result).toBe('Trial (ends N/A)');
  });

  it('returns N/A for both timestamps when they are negative on a non-trial subscription', () => {
    const result = getBillingAdminTrialStatus({
      subscription_state: 'active',
      trial_started_at: -100,
      trial_ends_at: -200,
    } as never);
    expect(result).toBe('Trial (started N/A, ends N/A)');
  });
});

/* ------------------------------------------------------------------ *
 * normalizeStatusCount — zero, negative, and fractional patrol counts
 * (exercised through getSelfHostedCurrentPlanPresentation)
 * ------------------------------------------------------------------ */

describe('patrol-control counts - fractional completed count truncated to 0', () => {
  it('truncates 0.7 to 0 so the decision stage is skipped and the stage is continue', () => {
    // If 0.7 were not truncated, completedCount would be > 0 and the
    // governed-decision-only outcome branch would fire (stage 'decision').
    // Math.trunc(0.7) === 0 keeps completedCount at 0, so the loop has
    // starters but no completed/resolved work → stage 'continue'.
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Patrol Applies Safe Fixes and Verifies the Result',
      ],
      patrolOperatorStatus: {
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlCompletedOperationsLoopCount: 0.7,
        patrolControlResolvedOperationsLoopCount: 0,
        externalAgentReady: false,
      },
    });
    expect(result.patrolControlAction).toEqual({
      actionLabel: 'Open Patrol',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });
  });
});

describe('patrol-control counts - negative counts clamped to 0', () => {
  it('clamps all negative counts to 0 so no patrol work is detected (stage set)', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Patrol Applies Safe Fixes and Verifies the Result',
      ],
      patrolOperatorStatus: {
        patrolControlOperationsLoopStarterCount: -3,
        patrolControlCompletedOperationsLoopCount: -1,
        patrolControlResolvedOperationsLoopCount: -2,
        externalAgentReady: false,
      },
    });
    // All counts clamped to 0 → getPatrolControlValueState returns undefined
    // (the counts-all-zero ternary arm) → stage 'set'.
    expect(result.patrolControlAction).toEqual({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });
  });
});

describe('getFirstPartyPatrolControlCount - alias subtraction underflow', () => {
  it('clamps the net count to 0 when pro-activation aliases exceed the patrol-control count', () => {
    // starter: 2 patrol-control − 5 pro-activation aliases = −3 → clamped to 0.
    // With every count at 0, no patrol work is detected → stage 'set'.
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Patrol Applies Safe Fixes and Verifies the Result',
      ],
      patrolOperatorStatus: {
        patrolControlOperationsLoopStarterCount: 2,
        proActivationOperationsLoopStarterCount: 5,
        externalAgentReady: false,
      },
    });
    expect(result.patrolControlAction).toEqual({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });
  });
});

describe('hasVerifiedPatrolOperatorOutcome - resolved count without explicit value state', () => {
  it('treats a resolved loop with no valueProofState as verified, yielding the set stage', () => {
    // starterCount = 1 → the counts > 0 ternary in getPatrolControlValueState
    // is entered, but both patrolControlValueState and patrolAutonomyValueState
    // are undefined, so valueProofState = undefined.
    //
    // The uncovered arm is:  (!valueProofState && resolvedCount > 0) → true.
    // Without this arm, hasVerified would be false and the stage would be
    // 'continue' (Open Patrol).  Because the arm fires, stage is 'set'.
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Patrol Applies Safe Fixes and Verifies the Result',
      ],
      patrolOperatorStatus: {
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlResolvedOperationsLoopCount: 1,
        externalAgentReady: false,
      },
    });
    expect(result.patrolControlAction).toEqual({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });
  });
});

/* ------------------------------------------------------------------ *
 * Unknown-tier fallbacks in current-plan presentation
 *
 * Exercises internal branches that the existing tests do not reach
 * because they always supply a recognised tier:
 *   - getSelfHostedUnlockedFeatures  → else: return displayableCapabilities
 *   - getSelfHostedIncludedExtras    → planDefinition?.includedExtras ?? []
 *   - getSelfHostedActivePlanSummary → if (!planDefinition) return null
 *   - getSelfHostedPlanLabel         → titleCase fallback
 * ------------------------------------------------------------------ */

describe('getSelfHostedCurrentPlanPresentation - unknown active tier fallbacks', () => {
  it('sources unlocked features from displayable capabilities and returns empty extras', () => {
    const displayable = ['Custom Capability A', 'Custom Capability B'];
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'team_seats',
        subscription_state: 'active',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: displayable,
    });
    // planLabel via titleCase fallback
    expect(result.title).toBe('Current plan: Team Seats');
    // getSelfHostedActivePlanSummary returns null (no planDefinition) → || fallback
    expect(result.body).toBe(
      'Team Seats is active on this instance. These capabilities are available right now.',
    );
    // getSelfHostedUnlockedFeatures else branch: returns displayableCapabilities
    expect(result.unlockedFeatures).toEqual(displayable);
    // getSelfHostedIncludedExtras: planDefinition null → ?? [] fallback
    expect(result.includedExtras).toEqual([]);
    expect(result.includedExtrasLabel).toBeUndefined();
    // Unknown tier ≠ pro → no patrol action
    expect(result.patrolControlAction).toBeUndefined();
  });
});

describe('getSelfHostedCurrentPlanPresentation - unknown trial tier fallbacks', () => {
  it('shows the trial-capabilities-active body when displayable capabilities are present', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'team_seats',
        subscription_state: 'trial',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: ['Custom Capability A'],
    });
    // planDefinition null → unlockedFeatures comes from displayableCapabilities
    // (the else branch).  unlockedFeatures.length > 0 → "capabilities are active"
    // body arm (previously only exercised via Relay, which uses the plan-
    // definition highlights path, not the displayableCapabilities path).
    expect(result.title).toBe('Current plan: Team Seats Trial');
    expect(result.body).toBe(
      'Team Seats trial capabilities are active on this instance right now.',
    );
    expect(result.unlockedFeatures).toEqual(['Custom Capability A']);
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedActivationHighlights — empty-feature skip and 8-item cap
 * (exercised through getSelfHostedActivationSuccessPresentation with an
 *  unknown tier so that prioritized is empty and unlockedFeatures ==
 *  displayableCapabilities, giving full control over the feature list)
 * ------------------------------------------------------------------ */

describe('getSelfHostedActivationHighlights - skips falsy feature entries', () => {
  it('drops empty strings from the highlights list while preserving non-empty entries', () => {
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: 'team_seats',
        subscription_state: 'active',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: ['Real Feature', '', 'Another Feature'],
      source: 'purchase',
    });
    expect(result).not.toBeNull();
    // The '' entry is falsy so `!feature` is true and it is skipped.
    expect(result?.highlights).toEqual(['Real Feature', 'Another Feature']);
  });
});

describe('getSelfHostedActivationHighlights - caps at 8 entries', () => {
  it('stops collecting highlights once 8 unique entries have been gathered', () => {
    const caps = Array.from({ length: 10 }, (_, i) => `Capability ${i + 1}`);
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: 'team_seats',
        subscription_state: 'active',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: caps,
      source: 'purchase',
    });
    expect(result).not.toBeNull();
    expect(result?.highlights).toHaveLength(8);
    expect(result?.highlights).toEqual(caps.slice(0, 8));
    expect(result?.highlights).not.toContain('Capability 9');
    expect(result?.highlights).not.toContain('Capability 10');
  });
});
