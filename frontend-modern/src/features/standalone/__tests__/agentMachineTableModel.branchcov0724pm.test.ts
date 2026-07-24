import { describe, expect, it } from 'vitest';
import type { HostRAIDArray } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getAgentMachineRaidArrayDetails,
  matchesAgentMachineSearch,
  sortAgentMachines,
} from '../agentMachineTableModel';

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'machine-1',
    name: overrides.name ?? overrides.id ?? 'machine-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'machine-1',
    type: 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const noopLabel = () => '';

describe('agentMachineTableModel branch coverage 0724pm', () => {
  describe('matchesAgentMachineSearch — canonicalIdentity optional-chain defined arm', () => {
    it('indexes canonical hostname, primaryId, and aliases when canonicalIdentity is present', () => {
      // Note: displayName and platformId are seeded in the fixture below but
      // are never searched for, so only hostname, primaryId, and aliases are
      // actually asserted — not "every" canonical identity field.
      const machine = resource({
        canonicalIdentity: {
          displayName: 'Canonical Display',
          hostname: 'canonical-host',
          platformId: 'canonical-platform',
          primaryId: 'canonical-primary',
          aliases: ['alias-one'],
        },
      });

      expect(matchesAgentMachineSearch(machine, 'canonical-host', noopLabel, noopLabel)).toBe(true);
      expect(matchesAgentMachineSearch(machine, 'canonical-primary', noopLabel, noopLabel)).toBe(
        true,
      );
      expect(matchesAgentMachineSearch(machine, 'alias-one', noopLabel, noopLabel)).toBe(true);
      expect(matchesAgentMachineSearch(machine, 'never-present', noopLabel, noopLabel)).toBe(false);
    });
  });

  describe('matchesAgentMachineSearch — sensorSearchValues non-standby arm', () => {
    it('emits no standby token for SMART disks that are actively reporting', () => {
      const machine = resource({
        agent: {
          sensors: {
            smart: [{ device: '/dev/nvme0', temperature: 41 }],
          },
        },
      });

      // The active disk identifier is searchable...
      expect(matchesAgentMachineSearch(machine, '/dev/nvme0', noopLabel, noopLabel)).toBe(true);
      // ...but the "standby" sentinel is not emitted for an active disk.
      expect(matchesAgentMachineSearch(machine, 'standby', noopLabel, noopLabel)).toBe(false);
    });
  });

  describe('getAgentMachineRaidArrayDetails — device fallback arms', () => {
    it('falls back to an empty device list when an array omits the devices field', () => {
      // Note: raw object deliberately lacks `devices` to exercise `devices ?? []`.
      const details = getAgentMachineRaidArrayDetails(
        resource({
          agent: {
            raid: [{ device: '/dev/md0', state: 'clean' } as unknown as HostRAIDArray],
          },
        }),
      );

      expect(details).toHaveLength(1);
      expect(details[0]?.device).toBe('/dev/md0');
      expect(details[0]?.devices).toEqual([]);
    });

    it('falls back the device slot to its index when slot is absent but the device is named', () => {
      const details = getAgentMachineRaidArrayDetails(
        resource({
          agent: {
            raid: [
              {
                device: '/dev/md0',
                state: 'clean',
                devices: [{ device: '/dev/sda', state: 'active' }],
              } as unknown as HostRAIDArray,
            ],
          },
        }),
      );

      // slot is undefined but the device is retained (named), so the index (0) is used.
      expect(details[0]?.devices).toEqual([{ device: '/dev/sda', state: 'active', slot: 0 }]);
    });
  });

  describe('sortAgentMachines — compareNullableNumber tie (both missing)', () => {
    it('treats two metric-less machines as equal and falls back to the name tiebreaker', () => {
      const sorted = sortAgentMachines(
        [resource({ id: 'beta', name: 'Beta' }), resource({ id: 'alpha', name: 'Alpha' })],
        'cpu',
        'asc',
        noopLabel,
        noopLabel,
      );

      // Neither machine has a CPU metric; compareNullableNumber returns 0 (both
      // undefined), so the ascending name tiebreaker decides the order.
      expect(sorted.map((m) => m.id)).toEqual(['alpha', 'beta']);
    });
  });

  describe('sortAgentMachines — compareText nullable operands', () => {
    it('pushes machines missing architecture below the defined one using the empty-string fallback', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'noarch-b', name: 'NoArchB' }),
          resource({ id: 'x86', agent: { architecture: 'x86_64' } }),
          resource({ id: 'noarch-a', name: 'NoArchA' }),
        ],
        'arch',
        'asc',
        noopLabel,
        noopLabel,
      );

      // The defined architecture sorts first; the two undefined ones fall back to
      // '' and are pushed to the bottom, tie-broken by name ascending.
      expect(sorted.map((m) => m.id)).toEqual(['x86', 'noarch-a', 'noarch-b']);
    });

    it('pushes a machine missing kernelVersion below one that defines it (direction-independent: the empty-value guard short-circuits before desc applies)', () => {
      // compareText returns 1 / -1 from the `!leftValue` / `!rightValue` guards
      // before the `direction` flag is ever consulted, so the 'desc' branch
      // (`-result`) is unreachable for this fixture. The assertion pins the
      // empty-value push-down only; it does NOT exercise descending order.
      const sorted = sortAgentMachines(
        [
          resource({ id: 'nokernel-a' }),
          resource({ id: 'kernel', agent: { kernelVersion: '6.6' } }),
        ],
        'kernel',
        'desc',
        noopLabel,
        noopLabel,
      );

      expect(sorted.map((m) => m.id)).toEqual(['kernel', 'nokernel-a']);
    });
  });

  describe('sortAgentMachines — uptime optional-chain arms on both operands', () => {
    it('resolves agent-reported uptime and pushes fully-missing machines below it', () => {
      // A mix forces the comparator to evaluate `uptime ?? agent?.uptimeSeconds` on
      // both the left and right operands, in both the agent-defined and
      // agent-absent arms of the optional chain.
      const sorted = sortAgentMachines(
        [
          resource({ id: 'none-1' }),
          resource({ id: 'reported-1', agent: { uptimeSeconds: 1_000 } }),
          resource({ id: 'direct', uptime: 100_000 }),
          resource({ id: 'none-2' }),
          resource({ id: 'reported-2', agent: { uptimeSeconds: 5_000 } }),
        ],
        'uptime',
        'asc',
        noopLabel,
        noopLabel,
      );

      // reported-1 (1000) < reported-2 (5000) < direct (100000); the two fully-missing
      // machines fall to the bottom, tie-broken by id ascending.
      expect(sorted.map((m) => m.id)).toEqual([
        'reported-1',
        'reported-2',
        'direct',
        'none-1',
        'none-2',
      ]);
    });
  });

  describe('sortAgentMachines — name fallback || chains', () => {
    it('resolves labels through the name fallback in the primary name sort', () => {
      // Empty displayName but set name → inner `displayName || name` falls to name.
      const sorted = sortAgentMachines(
        [
          resource({ id: 'id-1', name: 'gamma', displayName: '' }),
          resource({ id: 'id-2', name: 'alpha', displayName: '' }),
          resource({ id: 'id-3', name: 'echo', displayName: '' }),
        ],
        'name',
        'asc',
        noopLabel,
        noopLabel,
      );

      expect(sorted.map((m) => m.id)).toEqual(['id-2', 'id-3', 'id-1']);
    });

    it('resolves labels through the id fallback when both displayName and name are empty', () => {
      // Empty displayName AND empty name → outer `(... || name) || id` falls to id,
      // exercised on both comparator operands across the multi-element sort.
      const sorted = sortAgentMachines(
        [
          resource({ id: 'charlie', name: '', displayName: '' }),
          resource({ id: 'alpha', name: '', displayName: '' }),
          resource({ id: 'bravo', name: '', displayName: '' }),
        ],
        'name',
        'asc',
        noopLabel,
        noopLabel,
      );

      expect(sorted.map((m) => m.id)).toEqual(['alpha', 'bravo', 'charlie']);
    });

    it('uses the name fallback in the ascending tiebreaker when primary sort values tie', () => {
      // Equal CPU forces the tiebreaker; empty displayName routes through `name`.
      const sorted = sortAgentMachines(
        [
          resource({ id: 'tie-1', name: 'zulu', displayName: '', cpu: { current: 50 } }),
          resource({ id: 'tie-2', name: 'mike', displayName: '', cpu: { current: 50 } }),
          resource({ id: 'tie-3', name: 'alfa', displayName: '', cpu: { current: 50 } }),
        ],
        'cpu',
        'asc',
        noopLabel,
        noopLabel,
      );

      expect(sorted.map((m) => m.id)).toEqual(['tie-3', 'tie-2', 'tie-1']);
    });

    it('uses the id fallback in the tiebreaker when both displayName and name are empty', () => {
      // Equal CPU forces the tiebreaker; empty displayName AND name route through `id`.
      const sorted = sortAgentMachines(
        [
          resource({ id: 'tie-charlie', name: '', displayName: '', cpu: { current: 50 } }),
          resource({ id: 'tie-alpha', name: '', displayName: '', cpu: { current: 50 } }),
          resource({ id: 'tie-bravo', name: '', displayName: '', cpu: { current: 50 } }),
        ],
        'cpu',
        'asc',
        noopLabel,
        noopLabel,
      );

      expect(sorted.map((m) => m.id)).toEqual(['tie-alpha', 'tie-bravo', 'tie-charlie']);
    });
  });
});
