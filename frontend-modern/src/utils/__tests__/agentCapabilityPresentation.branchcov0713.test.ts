import { describe, expect, it } from 'vitest';

import {
  getAgentCapabilityBadgeClass,
  getAgentCapabilityLabel,
} from '@/utils/agentCapabilityPresentation';
import type { AgentCapability } from '@/utils/agentCapabilityPresentation';

// Full badge class strings are mirrored from the source module so assertions
// describe the documented Tailwind badge composition rather than echoing the
// function under test. These are the building blocks the module returns.
const PROXMOX_BADGE = 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-300';
const PBS_BADGE = 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300';
const PMG_BADGE = 'bg-rose-100 text-rose-800 dark:bg-rose-900 dark:text-rose-300';
const TRUENAS_BADGE = 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-300';
const AVAILABILITY_BADGE = 'bg-sky-100 text-sky-800 dark:bg-sky-900 dark:text-sky-300';
const KUBERNETES_BADGE =
  'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-300';
const DEFAULT_BADGE = 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300';

describe('agentCapabilityPresentation branch coverage', () => {
  describe('getAgentCapabilityLabel', () => {
    // The sibling test already covers agent, docker, kubernetes, proxmox,
    // truenas and availability. These two cases close the remaining switch
    // arms that were previously uncovered.
    it('returns the canonical label for the pbs capability', () => {
      expect(getAgentCapabilityLabel('pbs')).toBe('PBS');
    });

    it('returns the canonical label for the pmg capability', () => {
      expect(getAgentCapabilityLabel('pmg')).toBe('PMG');
    });

    it('returns a distinct label for every member of the AgentCapability union', () => {
      const allCapabilities: AgentCapability[] = [
        'agent',
        'docker',
        'kubernetes',
        'proxmox',
        'pbs',
        'pmg',
        'truenas',
        'availability',
      ];
      const labels = new Set<string>();
      for (const capability of allCapabilities) {
        const label = getAgentCapabilityLabel(capability);
        expect(typeof label).toBe('string');
        expect(label.length).toBeGreaterThan(0);
        labels.add(label);
      }
      // Every capability must map to a unique, non-empty label.
      expect(labels.size).toBe(allCapabilities.length);
    });

    // The switch has no `default` clause, so a value outside the union (only
    // reachable at runtime via a cast) falls through every case and the
    // function implicitly returns undefined. This documents that runtime
    // behavior; the declared `string` return type is optimistic.
    it('returns undefined when an unrecognized value matches no switch case', () => {
      const unknown = 'not-a-real-capability' as unknown as AgentCapability;
      const result = getAgentCapabilityLabel(unknown);
      expect(result).toBeUndefined();
    });

    it('returns undefined for an empty string that matches no switch case', () => {
      const result = getAgentCapabilityLabel('' as unknown as AgentCapability);
      expect(result).toBeUndefined();
    });

    it('returns undefined for null coerced through the type layer', () => {
      const result = getAgentCapabilityLabel(null as unknown as AgentCapability);
      expect(result).toBeUndefined();
    });

    it('returns undefined for undefined coerced through the type layer', () => {
      const result = getAgentCapabilityLabel(undefined as unknown as AgentCapability);
      expect(result).toBeUndefined();
    });
  });

  describe('getAgentCapabilityBadgeClass', () => {
    // The sibling test covers proxmox, kubernetes, truenas, availability and
    // the default arm (via 'agent'). These cases close the remaining explicit
    // switch arms plus the other capability that routes to default.
    it('returns the orange-toned badge for the pbs capability', () => {
      expect(getAgentCapabilityBadgeClass('pbs')).toBe(PBS_BADGE);
    });

    it('returns the rose-toned badge for the pmg capability', () => {
      expect(getAgentCapabilityBadgeClass('pmg')).toBe(PMG_BADGE);
    });

    it('routes the docker capability to the default blue badge', () => {
      // 'docker' has no explicit case in the switch, so it must fall through to
      // the default arm rather than receiving a dedicated tone.
      expect(getAgentCapabilityBadgeClass('docker')).toBe(DEFAULT_BADGE);
    });

    it('returns the exact canonical badge class for every explicit switch arm', () => {
      expect(getAgentCapabilityBadgeClass('proxmox')).toBe(PROXMOX_BADGE);
      expect(getAgentCapabilityBadgeClass('pbs')).toBe(PBS_BADGE);
      expect(getAgentCapabilityBadgeClass('pmg')).toBe(PMG_BADGE);
      expect(getAgentCapabilityBadgeClass('truenas')).toBe(TRUENAS_BADGE);
      expect(getAgentCapabilityBadgeClass('availability')).toBe(AVAILABILITY_BADGE);
      expect(getAgentCapabilityBadgeClass('kubernetes')).toBe(KUBERNETES_BADGE);
    });

    it('routes both agent and docker capabilities to the same default badge tone', () => {
      // Neither 'agent' nor 'docker' has an explicit case; both must resolve to
      // the default arm and produce identical class strings.
      expect(getAgentCapabilityBadgeClass('agent')).toBe(DEFAULT_BADGE);
      expect(getAgentCapabilityBadgeClass('docker')).toBe(DEFAULT_BADGE);
      expect(getAgentCapabilityBadgeClass('agent')).toBe(getAgentCapabilityBadgeClass('docker'));
    });

    // The badge function DOES have a default clause, so unlike the label
    // function it always returns the default blue tone for any unrecognized
    // value rather than undefined.
    it('falls back to the default blue badge for an unrecognized value', () => {
      const unknown = 'not-a-real-capability' as unknown as AgentCapability;
      expect(getAgentCapabilityBadgeClass(unknown)).toBe(DEFAULT_BADGE);
    });

    it('falls back to the default blue badge for an empty string', () => {
      expect(getAgentCapabilityBadgeClass('' as unknown as AgentCapability)).toBe(DEFAULT_BADGE);
    });

    it('falls back to the default blue badge for null', () => {
      expect(getAgentCapabilityBadgeClass(null as unknown as AgentCapability)).toBe(DEFAULT_BADGE);
    });

    it('falls back to the default blue badge for undefined', () => {
      expect(getAgentCapabilityBadgeClass(undefined as unknown as AgentCapability)).toBe(
        DEFAULT_BADGE,
      );
    });
  });
});
