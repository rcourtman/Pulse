import { afterEach, describe, expect, it } from 'vitest';
import {
  presentationPolicyHidesUpgradePrompts,
  syncSessionPresentationPolicy,
} from '@/stores/sessionPresentationPolicy';

const resetPresentationPolicy = () => {
  syncSessionPresentationPolicy(null);
};

describe('session presentation policy', () => {
  afterEach(() => {
    resetPresentationPolicy();
  });

  it('defaults unresolved self-hosted sessions to hiding upgrade prompts', () => {
    const policy = syncSessionPresentationPolicy(null);

    expect(policy.hideUpgrade).toBe(true);
    expect(presentationPolicyHidesUpgradePrompts()).toBe(true);
  });

  it('honors an explicit hosted policy that enables upgrade prompts', () => {
    const policy = syncSessionPresentationPolicy({
      presentationPolicy: {
        demoMode: false,
        readOnly: false,
        hideCommercial: false,
        hideUpgrade: false,
      },
      sessionCapabilities: {
        demoMode: false,
      },
    });

    expect(policy.hideUpgrade).toBe(false);
    expect(presentationPolicyHidesUpgradePrompts()).toBe(false);
  });

  it('forces upgrade prompts hidden in demo mode', () => {
    const policy = syncSessionPresentationPolicy({
      presentationPolicy: {
        demoMode: true,
        readOnly: false,
        hideCommercial: false,
        hideUpgrade: false,
      },
      sessionCapabilities: {
        demoMode: false,
      },
    });

    expect(policy.hideUpgrade).toBe(true);
    expect(presentationPolicyHidesUpgradePrompts()).toBe(true);
  });
});
