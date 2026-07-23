import { describe, expect, it } from 'vitest';
import { getInfrastructureSourcePickerItemForRouteStep } from '@/utils/infrastructureOnboardingPresentation';

// The 6 uncovered branches + the never-called function reported by coverage
// all live in getInfrastructureSourcePickerItemForRouteStep (source lines
// 480-490). That function has exactly three decision points, each with two
// arms:
//   1. `(step || '').trim()`  -> `||` truthy arm vs falsy arm
//   2. `if (!normalized)`     -> empty (return null) vs non-empty (continue)
//   3. `if (item.routeStep === normalized)` -> match (return) vs miss (loop on)
// This file closes every one of those arms and exercises each of the nine
// route steps the function recognises, plus one it does not.
//
// SOURCE_PICKER_ITEM_ORDER (the only ids the loop can ever return) is, in
// order: pve, pbs, pmg, truenas, vmware, unraid, docker, kubernetes, linux-host.

const RECOGNISED_STEP_EXPECTATIONS = [
  { step: 'pve', id: 'pve', connectionType: 'pve', label: 'Proxmox VE' },
  { step: 'pbs', id: 'pbs', connectionType: 'pbs', label: 'Proxmox Backup Server' },
  { step: 'pmg', id: 'pmg', connectionType: 'pmg', label: 'Proxmox Mail Gateway' },
  { step: 'truenas', id: 'truenas', connectionType: 'truenas', label: 'TrueNAS SCALE' },
  { step: 'vmware', id: 'vmware', connectionType: 'vmware', label: 'VMware vCenter' },
  { step: 'unraid', id: 'unraid', connectionType: 'agent', label: 'Unraid' },
  { step: 'docker', id: 'docker', connectionType: 'agent', label: 'Docker' },
  { step: 'kubernetes', id: 'kubernetes', connectionType: 'agent', label: 'Kubernetes' },
  {
    step: 'linux-host',
    id: 'linux-host',
    connectionType: 'agent',
    label: 'Linux, macOS, Windows host',
  },
] as const;

describe('infrastructureOnboardingPresentation — branch coverage (branchcov0723pm)', () => {
  describe('getInfrastructureSourcePickerItemForRouteStep', () => {
    describe('`(step || "")` falsy arm + `!normalized` true arm -> null', () => {
      // Each of these is falsy, so `(step || "")` collapses to `""`, `.trim()`
      // stays `""`, and `if (!normalized) return null` fires.
      it('returns null for null', () => {
        expect(getInfrastructureSourcePickerItemForRouteStep(null)).toBeNull();
      });

      it('returns null for undefined', () => {
        expect(getInfrastructureSourcePickerItemForRouteStep(undefined)).toBeNull();
      });

      it('returns null for the empty string', () => {
        expect(getInfrastructureSourcePickerItemForRouteStep('')).toBeNull();
      });
    });

    describe('`(step || "")` truthy arm + trim empties it + `!normalized` true arm -> null', () => {
      // A whitespace-only string is a TRUTHY string, so it takes the `step`
      // side of `||` (not the `""` side), then `.trim()` produces `""` and the
      // `!normalized` early return fires. This is the only input shape that
      // exercises the truthy `||` arm while still landing on the null return.
      it('returns null for a whitespace-only string (trim empties it)', () => {
        expect(getInfrastructureSourcePickerItemForRouteStep('     ')).toBeNull();
        expect(getInfrastructureSourcePickerItemForRouteStep('\t\n  ')).toBeNull();
      });
    });

    describe('recognised route steps (`routeStep === normalized` match arm)', () => {
      // Exercises the truthy `||` arm, the `!normalized` false arm (non-empty,
      // so the loop runs), and the TRUE arm of the inner equality for the
      // matching iteration. Each case asserts the concrete id / connectionType
      // / label of the resolved card so we prove WHICH branch matched rather
      // than merely that something came back.
      it.each(RECOGNISED_STEP_EXPECTATIONS)(
        'resolves the "$step" route step to the $label card',
        ({ step, id, connectionType, label }) => {
          const item = getInfrastructureSourcePickerItemForRouteStep(step);
          expect(item).not.toBeNull();
          expect(item!.id).toBe(id);
          expect(item!.routeStep).toBe(step);
          expect(item!.connectionType).toBe(connectionType);
          expect(item!.label).toBe(label);
        },
      );

      it('returns the Proxmox VE card verbatim for the first ordered step (pve)', () => {
        // pve is the FIRST id in SOURCE_PICKER_ITEM_ORDER, so this confirms the
        // loop returns on the very first iteration rather than scanning further.
        const item = getInfrastructureSourcePickerItemForRouteStep('pve');
        expect(item).toMatchObject({
          id: 'pve',
          connectionType: 'pve',
          label: 'Proxmox VE',
          sourceStrategy: 'api-agent',
        });
      });

      it('returns the linux-host card for the LAST ordered step', () => {
        // linux-host is the LAST id in SOURCE_PICKER_ITEM_ORDER, so the loop
        // must scan every earlier id (all missing) before matching — exercising
        // the inner `===` FALSE arm repeatedly before the single TRUE arm.
        const item = getInfrastructureSourcePickerItemForRouteStep('linux-host');
        expect(item).toMatchObject({
          id: 'linux-host',
          connectionType: 'agent',
          label: 'Linux, macOS, Windows host',
        });
      });
    });

    describe('whitespace padding (`.trim()` is applied to the input)', () => {
      // Still the truthy `||` arm + `!normalized` false arm + match TRUE arm,
      // but proves the comparison runs against the TRIMMED value, not the raw
      // input. Without the trim, `  pve  ` would miss and return null.
      it('strips surrounding whitespace before matching a recognised step', () => {
        const item = getInfrastructureSourcePickerItemForRouteStep('  pve\t');
        expect(item).not.toBeNull();
        expect(item!.id).toBe('pve');
        expect(item!.routeStep).toBe('pve');
      });
    });

    describe('unrecognised step (inner `===` FALSE on every iteration -> final null)', () => {
      // A valid non-empty string that no item's routeStep equals: the loop runs
      // to completion, the inner `if` is never true, and control reaches the
      // final `return null`. This is the only path that exercises the FALSE arm
      // of the inner equality without a subsequent TRUE arm rescuing it.
      it('returns null for a completely unknown step', () => {
        expect(getInfrastructureSourcePickerItemForRouteStep('hyper-v')).toBeNull();
      });

      it('returns null for a recognised connection-type that is not itself a route step', () => {
        // 'agent' is a connectionType but is NOT a routeStep id (the host path
        // is 'linux-host'), so it must miss every item and return null.
        expect(getInfrastructureSourcePickerItemForRouteStep('agent')).toBeNull();
      });

      it('is case-sensitive: an uppercased recognised step does not match', () => {
        // The function trims but never lowercases, and every routeStep is
        // lowercase, so 'PVE' !== 'pve' and the lookup misses.
        expect(getInfrastructureSourcePickerItemForRouteStep('PVE')).toBeNull();
        expect(getInfrastructureSourcePickerItemForRouteStep('Docker')).toBeNull();
      });
    });
  });
});
