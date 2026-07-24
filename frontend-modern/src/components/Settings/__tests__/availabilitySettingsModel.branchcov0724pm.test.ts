/**
 * Branch-coverage tests for availabilitySettingsModel, targeting the arms left
 * cold by availabilitySettingsModel.test.ts and availabilitySettingsModel.branchcov2.test.ts.
 *
 * Scoped coverage against those two existing specs leaves exactly five branches
 * uncovered, all reachable from the public surface:
 *   - the 'udp' switch case in getAvailabilityTargetMethodLabel (both the
 *     with-port and without-port ternary arms inside it),
 *   - the `status.outcome === 'indeterminate'` consequent in
 *     getAvailabilityTargetStatusLabel ('Open or filtered'),
 *   - the `status.outcome === 'indeterminate'` consequent in
 *     getAvailabilityTargetStatusClass (amber badge),
 *   - the empty-targets consequent in getAvailabilityTargetsSummary
 *     ('No availability checks configured'),
 *   - the fall-through (no down, no indeterminate) to the final default return
 *     in getAvailabilityTargetsSummary ('N enabled · M total').
 *
 * Fixture builders and import style mirror the sibling branchcov2 file.
 */
import { describe, expect, it } from 'vitest';
import {
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetStatusClass,
  getAvailabilityTargetStatusLabel,
  getAvailabilityTargetsSummary,
} from '../availabilitySettingsModel';
import type { AvailabilityProbeStatus, AvailabilityTarget } from '@/api/availabilityTargets';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling test factories so the three files stay consistent.

const target = (overrides: Partial<AvailabilityTarget> = {}): AvailabilityTarget => ({
  id: 'mqtt-broker',
  name: 'MQTT broker',
  address: 'mqtt.local',
  protocol: 'tcp',
  port: 1883,
  enabled: true,
  ...overrides,
});

const status = (overrides: Partial<AvailabilityProbeStatus> = {}): AvailabilityProbeStatus => ({
  targetId: 'mqtt-broker',
  name: 'MQTT broker',
  address: 'mqtt.local',
  protocol: 'tcp',
  enabled: true,
  available: true,
  ...overrides,
});

// ---- getAvailabilityTargetMethodLabel --------------------------------------
// The 'udp' switch case is the only protocol arm never entered by the sibling
// specs. Drive both ternary operands inside it (port present vs absent/0).

describe('getAvailabilityTargetMethodLabel (udp arm)', () => {
  it("includes the port for a udp target with one ('UDP <port>' ternary arm)", () => {
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'udp', port: 27015 }))).toBe(
      'UDP 27015',
    );
  });

  it("returns 'UDP port' for a udp target without a port (port falsy ternary arm)", () => {
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'udp', port: undefined }))).toBe(
      'UDP port',
    );
  });
});

// ---- getAvailabilityTargetStatusLabel --------------------------------------
// The indeterminate-outcome consequent short-circuits before the available /
// lastError arms and was never hit. outcome takes priority over `available`, so
// confirm it wins even when `available` is false.

describe('getAvailabilityTargetStatusLabel (indeterminate arm)', () => {
  it("returns 'Open or filtered' when outcome is indeterminate (before the available branch)", () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({
          status: status({ available: false, outcome: 'indeterminate' }),
        }),
      ),
    ).toBe('Open or filtered');
  });
});

// ---- getAvailabilityTargetStatusClass --------------------------------------
// The indeterminate-outcome consequent returns the amber badge and was never
// hit. Like the label helper it is checked before the available branch.

describe('getAvailabilityTargetStatusClass (indeterminate arm)', () => {
  it('returns the amber badge classes for an indeterminate outcome', () => {
    expect(
      getAvailabilityTargetStatusClass(
        target({
          status: status({ available: false, outcome: 'indeterminate' }),
        }),
      ),
    ).toBe('bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300');
  });
});

// ---- getAvailabilityTargetsSummary -----------------------------------------
// Two cold arms live here: the empty-input early return, and the final default
// return reached only when no target is down and none are indeterminate.

describe('getAvailabilityTargetsSummary (empty + all-healthy arms)', () => {
  it("returns 'No availability checks configured' for an empty list (length === 0 arm)", () => {
    expect(getAvailabilityTargetsSummary([])).toBe('No availability checks configured');
  });

  it("returns the default '<enabled> enabled · <total> total' when nothing is down or indeterminate (final fall-through arm)", () => {
    // One online target and one paused target: enabled=1, down=0, indeterminate=0,
    // total=2 -> falls through every guard to the default return.
    expect(
      getAvailabilityTargetsSummary([
        target({ id: 'online', status: status({ targetId: 'online', available: true }) }),
        target({ id: 'paused', enabled: false }),
      ]),
    ).toBe('1 enabled · 2 total');
  });
});
