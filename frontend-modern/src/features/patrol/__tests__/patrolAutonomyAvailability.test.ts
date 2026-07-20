import { describe, expect, it } from 'vitest';

import type { LicenseRuntimeCapabilityBlock, LicenseRuntimeIdentity } from '@/api/license';

import {
  PATROL_AUTONOMY_FEATURE_KEY,
  PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON,
  getPatrolAutonomyAvailabilityPresentation,
} from '../patrolAutonomyAvailability';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

const planDestination: UpgradeDestination = {
  href: '/plans',
  external: false,
  hardNavigation: false,
  newTab: false,
  preserveOpener: false,
};

const externalDestination = (href: string): UpgradeDestination => ({
  href,
  external: true,
  hardNavigation: true,
  newTab: true,
  preserveOpener: false,
});

const internalDestination = (href: string): UpgradeDestination => ({
  href,
  external: false,
  hardNavigation: false,
  newTab: false,
  preserveOpener: false,
});

const runtimeBlock = (
  overrides: Partial<LicenseRuntimeCapabilityBlock> = {},
): LicenseRuntimeCapabilityBlock => ({
  key: PATROL_AUTONOMY_FEATURE_KEY,
  reason: PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON,
  ...overrides,
});

const runtime = (overrides: Partial<LicenseRuntimeIdentity> = {}): LicenseRuntimeIdentity => ({
  build: 'community-1',
  label: 'Community runtime',
  ...overrides,
});

describe('patrolAutonomyAvailability', () => {
  describe('exported constants', () => {
    it('exposes the ai_autofix feature key', () => {
      expect(PATROL_AUTONOMY_FEATURE_KEY).toBe('ai_autofix');
    });

    it('exposes the paid_runtime_required reason', () => {
      expect(PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON).toBe('paid_runtime_required');
    });
  });

  describe('getPatrolAutonomyAvailabilityPresentation', () => {
    it('is available when autoFix is not locked, ignoring every other field', () => {
      const result = getPatrolAutonomyAvailabilityPresentation({
        autoFixLocked: false,
        runtimeCapabilityBlock: runtimeBlock(),
        runtime: runtime(),
        commercialSurfacesHidden: true,
        planUpgradeDestination: planDestination,
      });

      expect(result).toEqual({
        kind: 'available',
        locked: false,
        title: 'Patrol mode available',
        body: 'Choose the mode for this install.',
      });
    });

    it('is available when autoFixLocked is falsy', () => {
      const result = getPatrolAutonomyAvailabilityPresentation({
        autoFixLocked: undefined as unknown as boolean,
        planUpgradeDestination: planDestination,
      });
      expect(result.kind).toBe('available');
      expect(result.locked).toBe(false);
    });

    it.each([
      [
        'runtime_locked beats commercialSurfacesHidden',
        {
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock(),
          runtime: runtime(),
          commercialSurfacesHidden: true,
          upgradePromptsHidden: true,
          planUpgradeDestination: planDestination,
        },
        'runtime_locked',
      ],
      [
        'plan_locked when commercialSurfacesHidden and no runtime block',
        {
          autoFixLocked: true,
          commercialSurfacesHidden: true,
          planUpgradeDestination: planDestination,
        },
        'plan_locked',
      ],
      [
        'plan_locked by default when locked but no block and surfaces visible',
        {
          autoFixLocked: true,
          planUpgradeDestination: planDestination,
        },
        'plan_locked',
      ],
    ])('resolves kind precedence: %s', (_label, input, expectedKind) => {
      expect(getPatrolAutonomyAvailabilityPresentation(input).kind).toBe(expectedKind);
    });

    describe('runtime_locked presentation', () => {
      it('builds the runtime-locked surface with the runtime label in the body', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock({ action_url: 'https://pro.example/downloads' }),
          runtime: runtime({ label: 'Community runtime' }),
          planUpgradeDestination: planDestination,
        });

        expect(result).toEqual({
          kind: 'runtime_locked',
          locked: true,
          title: 'Pulse Pro runtime required',
          body: 'This install is running Community runtime. Install the Pulse Pro runtime to use Patrol modes.',
          actionLabel: 'Open Pro downloads',
          destination: externalDestination('https://pro.example/downloads'),
        });
      });

      it('uses the runtime download_url when the block has no action_url', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock(),
          runtime: runtime({ download_url: '/downloads/pro' }),
          planUpgradeDestination: planDestination,
        });

        expect(result.actionLabel).toBe('Open Pro downloads');
        expect(result.destination).toEqual(internalDestination('/downloads/pro'));
      });

      it('prefers the block action_url over the runtime download_url', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock({ action_url: 'https://block.example/dl' }),
          runtime: runtime({ download_url: 'https://runtime.example/dl' }),
          planUpgradeDestination: planDestination,
        });

        expect(result.destination).toEqual(externalDestination('https://block.example/dl'));
      });

      it('trims whitespace on the action_url before resolving', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock({ action_url: '  https://pro.example/downloads  ' }),
          runtime: runtime(),
          planUpgradeDestination: planDestination,
        });

        expect(result.destination).toEqual(externalDestination('https://pro.example/downloads'));
      });

      it('falls back to the plan upgrade destination when no runtime download source exists', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock(),
          runtime: runtime(),
          planUpgradeDestination: planDestination,
        });

        expect(result.destination).toBe(planDestination);
      });

      it('falls back to the plan destination when action_url and download_url are blank', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock({ action_url: '   ' }),
          runtime: runtime({ download_url: '   ' }),
          planUpgradeDestination: planDestination,
        });

        expect(result.destination).toBe(planDestination);
      });

      it('omits the action label and destination when upgrade prompts are hidden', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          upgradePromptsHidden: true,
          runtimeCapabilityBlock: runtimeBlock({ action_url: 'https://pro.example/downloads' }),
          runtime: runtime(),
          planUpgradeDestination: planDestination,
        });

        expect(result).toEqual({
          kind: 'runtime_locked',
          locked: true,
          title: 'Pulse Pro runtime required',
          body: 'This install is running Community runtime. Install the Pulse Pro runtime to use Patrol modes.',
        });
        expect(result).not.toHaveProperty('actionLabel');
        expect(result).not.toHaveProperty('destination');
      });

      it('falls back to the "this runtime" label when the runtime label is blank', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          upgradePromptsHidden: true,
          runtimeCapabilityBlock: runtimeBlock(),
          runtime: runtime({ label: '   ' }),
          planUpgradeDestination: planDestination,
        });

        expect(result.body).toBe(
          'This install is running this runtime. Install the Pulse Pro runtime to use Patrol modes.',
        );
      });

      it('falls back to the "this runtime" label when runtime is undefined', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          upgradePromptsHidden: true,
          runtimeCapabilityBlock: runtimeBlock(),
          planUpgradeDestination: planDestination,
        });

        expect(result.body).toBe(
          'This install is running this runtime. Install the Pulse Pro runtime to use Patrol modes.',
        );
      });

      it('trims the runtime label before interpolating it', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          upgradePromptsHidden: true,
          runtimeCapabilityBlock: runtimeBlock(),
          runtime: runtime({ label: '  Pulse Pro  ' }),
          planUpgradeDestination: planDestination,
        });

        expect(result.body).toBe(
          'This install is running Pulse Pro. Install the Pulse Pro runtime to use Patrol modes.',
        );
      });

      it('does not treat a non-matching capability reason as runtime locked', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          runtimeCapabilityBlock: runtimeBlock({ reason: 'some_other_reason' }),
          runtime: runtime(),
          planUpgradeDestination: planDestination,
        });

        expect(result.kind).toBe('plan_locked');
        expect(result.actionLabel).toBe('Plans & Billing');
      });
    });

    describe('plan_locked presentation', () => {
      it('shows the Plans & Billing action when surfaces are visible', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          planUpgradeDestination: planDestination,
        });

        expect(result).toEqual({
          kind: 'plan_locked',
          locked: true,
          title: 'Watch only',
          body: 'This install watches infrastructure and shows issues.',
          actionLabel: 'Plans & Billing',
          destination: planDestination,
        });
      });

      it('omits the action when upgrade prompts are hidden', () => {
        const result = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          upgradePromptsHidden: true,
          planUpgradeDestination: planDestination,
        });

        expect(result).toEqual({
          kind: 'plan_locked',
          locked: true,
          title: 'Watch only',
          body: 'This install watches infrastructure and shows issues.',
        });
        expect(result).not.toHaveProperty('actionLabel');
        expect(result).not.toHaveProperty('destination');
      });

      it('omits the action when commercial surfaces are hidden, regardless of upgrade prompt visibility', () => {
        const withPromptsVisible = getPatrolAutonomyAvailabilityPresentation({
          autoFixLocked: true,
          commercialSurfacesHidden: true,
          planUpgradeDestination: planDestination,
        });

        expect(withPromptsVisible).toEqual({
          kind: 'plan_locked',
          locked: true,
          title: 'Watch only',
          body: 'This install watches infrastructure and shows issues.',
        });
        expect(withPromptsVisible).not.toHaveProperty('actionLabel');
        expect(withPromptsVisible).not.toHaveProperty('destination');
      });
    });
  });
});
